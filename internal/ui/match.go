package ui

import (
	"image/color"
	"math"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
	"github.com/eprice555/shift-command/internal/sim"
	"github.com/eprice555/shift-command/internal/world"
)

var (
	colWallFill  = color.RGBA{0x0C, 0x10, 0x22, 0xFF}
	colWallEdge  = color.RGBA{0x5A, 0x4E, 0xF0, 0xFF}
	colSelect    = color.RGBA{0x6C, 0xFF, 0xB4, 0xFF}
	colEnemy     = color.RGBA{0xFF, 0x46, 0x3C, 0xFF}
	colHPGood    = color.RGBA{0x6C, 0xFF, 0x7A, 0xFF}
	colHPMid     = color.RGBA{0xFF, 0xD8, 0x40, 0xFF}
	colHPLow     = color.RGBA{0xFF, 0x46, 0x3C, 0xFF}
	colPanel     = color.NRGBA{0x08, 0x0B, 0x16, 0xE0}
	colPanelEdge = color.RGBA{0x2A, 0x3C, 0x62, 0xFF}
)

// MatchView is everything the renderer needs to draw one match frame. The
// engine owns this state; the renderer only reads it.
type MatchView struct {
	// Sandbox is the M1 combat sandbox: no economy UI.
	Sandbox bool
	World   *sim.World
	Player  sim.PlayerID
	// Alpha is the fraction [0,1) between the last sim tick and the next.
	Alpha float64
	Cam   *Camera
	// Selected is the selection in ascending ID order.
	Selected []sim.UnitID
	Hover    sim.UnitID
	// Dragging shows a selection box from DragFrom to Cursor (screen px).
	Dragging bool
	DragFrom geom.Vec
	Cursor   geom.Vec
	// Targeting shows the attack-move cursor.
	Targeting bool
	RevealAll bool
	Focused   bool
	ShowHelp  bool
	// Message is a transient notice; MessageAge counts updates since it was set.
	Message    string
	MessageAge int
	// Alert is where the enemy last attacked off-screen; AlertAge counts
	// updates since.
	HasAlert bool
	Alert    geom.Vec
	AlertAge int
	// Groups holds the live unit count of each control group 0–9.
	Groups [10]int

	// Card is the command card for the selection.
	Card []Button
	// Placing shows a building footprint at tile (PlaceX, PlaceY);
	// PlaceNotice is why it can't go there (NoticeNone if it can).
	Placing        bool
	PlaceType      faction.UnitType
	PlaceX, PlaceY int
	PlaceNotice    sim.Notice
}

// PlayerColor returns the draw color for units owned by p, seen by viewer.
func PlayerColor(p, viewer sim.PlayerID) color.RGBA {
	if p != viewer {
		return colEnemy
	}
	return faction.Get(faction.Forge).Color
}

type seg struct{ a, b geom.Vec }

// matchCache holds per-map resources built once.
type matchCache struct {
	m        *world.Map
	edges    []seg
	miniWall *ebiten.Image
	fog      *ebiten.Image
	fogPix   []byte

	fill, stroke [sim.NumPlayers]vector.Path
	scratch      vector.Path
}

func (c *matchCache) ensure(m *world.Map) {
	if c.m == m {
		return
	}
	c.m = m
	c.edges = wallEdges(m.Grid)
	c.fog = ebiten.NewImage(m.W, m.H)
	c.fogPix = make([]byte, 4*m.W*m.H)
	pix := make([]byte, 4*m.W*m.H)
	for y := range m.H {
		for x := range m.W {
			i := 4 * (y*m.W + x)
			if m.Terrain(x, y) {
				pix[i], pix[i+1], pix[i+2], pix[i+3] = 0x4A, 0x44, 0xB8, 0xFF
			} else {
				pix[i], pix[i+1], pix[i+2], pix[i+3] = 0x10, 0x16, 0x2A, 0xFF
			}
		}
	}
	c.miniWall = ebiten.NewImage(m.W, m.H)
	c.miniWall.WritePixels(pix)
}

// wallEdges returns the boundary between blocked and open tiles as merged
// straight runs, so the glow outline is a handful of long lines.
func wallEdges(g *world.Grid) []seg {
	var out []seg
	edge := func(x, y, dx, dy int) bool {
		return g.Terrain(x, y) && g.InBounds(x+dx, y+dy) && !g.Terrain(x+dx, y+dy)
	}
	for _, dy := range []int{-1, 1} {
		off := 0.0
		if dy == 1 {
			off = 1
		}
		for y := range g.H {
			for x := 0; x < g.W; {
				if !edge(x, y, 0, dy) {
					x++
					continue
				}
				x0 := x
				for x < g.W && edge(x, y, 0, dy) {
					x++
				}
				out = append(out, seg{geom.V(float64(x0), float64(y)+off), geom.V(float64(x), float64(y)+off)})
			}
		}
	}
	for _, dx := range []int{-1, 1} {
		off := 0.0
		if dx == 1 {
			off = 1
		}
		for x := range g.W {
			for y := 0; y < g.H; {
				if !edge(x, y, dx, 0) {
					y++
					continue
				}
				y0 := y
				for y < g.H && edge(x, y, dx, 0) {
					y++
				}
				out = append(out, seg{geom.V(float64(x)+off, float64(y0)), geom.V(float64(x)+off, float64(y))})
			}
		}
	}
	return out
}

// unitVisible reports whether the viewer may see u this frame.
func unitVisible(v *MatchView, u *sim.Unit) bool {
	return v.RevealAll || v.World.VisibleTo(v.Player, u)
}

func lerpAngle(a, b, t float64) float64 {
	d := math.Mod(b-a+3*math.Pi, 2*math.Pi) - math.Pi
	return a + d*t
}

// DrawPos returns u's interpolated position and facing for this frame.
func DrawPos(u *sim.Unit, alpha float64) (geom.Vec, float64) {
	return u.PrevPos.Lerp(u.Pos, alpha), lerpAngle(u.PrevFacing, u.Facing, alpha)
}

// DrawMatch renders a match frame.
func (r *Renderer) DrawMatch(screen *ebiten.Image, v *MatchView) {
	r.frame++
	b := screen.Bounds()
	r.ensureTargets(b.Dx(), b.Dy())
	w := v.World
	c := &r.match
	c.ensure(w.Map)
	cam := v.Cam
	vis := cam.Visible()
	scale := cam.Scale()

	screen.Fill(colBackground)
	r.drawFloor(screen, cam, vis)

	// Walls: dark fill on screen, glowing outline in the emissive layer.
	em := r.bloom.src
	em.Clear()
	x0, y0 := max(int(vis.Min.X)-1, 0), max(int(vis.Min.Y)-1, 0)
	x1, y1 := min(int(vis.Max.X)+1, w.Map.W-1), min(int(vis.Max.Y)+1, w.Map.H-1)
	for ty := y0; ty <= y1; ty++ {
		for tx := x0; tx <= x1; tx++ {
			if w.Map.Terrain(tx, ty) {
				sx, sy := cam.ToScreen(geom.V(float64(tx), float64(ty)))
				vector.FillRect(screen, float32(sx), float32(sy), float32(scale)+0.5, float32(scale)+0.5, colWallFill, false)
			}
		}
	}
	edgeCol := scaleRGB(colWallEdge, 0.7)
	for _, e := range c.edges {
		if max(e.a.X, e.b.X) < vis.Min.X-1 || min(e.a.X, e.b.X) > vis.Max.X+1 ||
			max(e.a.Y, e.b.Y) < vis.Min.Y-1 || min(e.a.Y, e.b.Y) > vis.Max.Y+1 {
			continue
		}
		ax, ay := cam.ToScreen(e.a)
		bx, by := cam.ToScreen(e.b)
		vector.StrokeLine(em, float32(ax), float32(ay), float32(bx), float32(by), float32(max(1.5, 2*cam.Zoom)), edgeCol, true)
	}
	mx0, my0 := cam.ToScreen(geom.V(0, 0))
	mx1, my1 := cam.ToScreen(geom.V(float64(w.Map.W), float64(w.Map.H)))
	vector.StrokeRect(em, float32(mx0), float32(my0), float32(mx1-mx0), float32(my1-my0), 2, scaleRGB(colWallEdge, 0.5), false)

	// Selection and hover rings under the units.
	ringW := float32(max(1.2, 1.6*cam.Zoom))
	for _, u := range w.Units {
		sel := isSelected(v, u.ID)
		if !sel && u.ID != v.Hover || !unitVisible(v, u) {
			continue
		}
		p, _ := DrawPos(u, v.Alpha)
		sx, sy := cam.ToScreen(p)
		col := colSelect
		if u.Owner != v.Player {
			col = colEnemy
		}
		a := 1.0
		if !sel {
			a = 0.45
		}
		if u.Def.Building {
			rect := u.Rect()
			p0 := cam.ToScreenV(rect.Min.Sub(geom.V(0.05, 0.05)))
			p1 := cam.ToScreenV(rect.Max.Add(geom.V(0.05, 0.05)))
			vector.StrokeRect(screen, float32(p0.X), float32(p0.Y), float32(p1.X-p0.X), float32(p1.Y-p0.Y), ringW, withAlpha(col, a), true)
			continue
		}
		vector.StrokeCircle(screen, float32(sx), float32(sy), float32((u.Def.Radius+0.18)*scale), ringW, withAlpha(col, a), true)
	}

	r.drawResources(screen, em, v, vis)
	r.drawUnits(screen, em, v, vis)
	r.fx.draw(em, v, r.frame)
	r.bloom.composite(screen)

	// Tube cores on top of the glow.
	for p := range sim.PlayerID(sim.NumPlayers) {
		col := Whiten(PlayerColor(p, v.Player), 0.6)
		sop := &vector.StrokeOptions{Width: float32(max(0.8, cam.Zoom)), LineJoin: vector.LineJoinRound}
		dop := &vector.DrawPathOptions{AntiAlias: true}
		dop.ColorScale.ScaleWithColor(col)
		dop.ColorScale.ScaleAlpha(0.7)
		vector.StrokePath(screen, &c.stroke[p], sop, dop)
	}

	r.drawFog(screen, v)
	r.drawGhosts(screen, v)
	r.drawRally(screen, v)
	r.drawHPBars(screen, v, vis)
	drawBuildBars(screen, v, vis)
	r.drawPlacement(screen, v)
	if v.Dragging {
		box := geom.RectFrom(v.DragFrom, v.Cursor)
		vector.FillRect(screen, float32(box.Min.X), float32(box.Min.Y), float32(box.W()), float32(box.H()), withAlpha(colSelect, 0.08), false)
		vector.StrokeRect(screen, float32(box.Min.X), float32(box.Min.Y), float32(box.W()), float32(box.H()), 1, colSelect, false)
	}
	r.drawHUD(screen, v)
}

func isSelected(v *MatchView, id sim.UnitID) bool {
	_, ok := slices.BinarySearch(v.Selected, id)
	return ok
}

func (r *Renderer) drawFloor(screen *ebiten.Image, cam *Camera, vis geom.Rect) {
	mw, mh := float64(r.match.m.W), float64(r.match.m.H)
	top, bot := max(vis.Min.Y, 0), min(vis.Max.Y, mh)
	left, right := max(vis.Min.X, 0), min(vis.Max.X, mw)
	if top >= bot || left >= right {
		return
	}
	for x := math.Ceil(left); x <= right; x++ {
		c := colGridMinor
		if int(x)%gridMajor == 0 {
			c = colGridMajor
		}
		ax, ay := cam.ToScreen(geom.V(x, top))
		_, by := cam.ToScreen(geom.V(x, bot))
		vector.StrokeLine(screen, float32(math.Round(ax))+0.5, float32(ay), float32(math.Round(ax))+0.5, float32(by), 1, c, false)
	}
	for y := math.Ceil(top); y <= bot; y++ {
		c := colGridMinor
		if int(y)%gridMajor == 0 {
			c = colGridMajor
		}
		ax, ay := cam.ToScreen(geom.V(left, y))
		bx, _ := cam.ToScreen(geom.V(right, y))
		vector.StrokeLine(screen, float32(ax), float32(math.Round(ay))+0.5, float32(bx), float32(math.Round(ay))+0.5, 1, c, false)
	}
}

// unitShape appends u's outline (unit radius 1, facing +X) to dst.
func unitShape(dst []geom.Vec, t faction.UnitType) []geom.Vec {
	switch t {
	case faction.Outrider:
		return append(dst, geom.V(1.1, 0), geom.V(-0.85, 0.8), geom.V(-0.35, 0), geom.V(-0.85, -0.8))
	case faction.Strider:
		for i := range 6 {
			s, c := math.Sincos(float64(i) * math.Pi / 3)
			dst = append(dst, geom.V(c, s))
		}
		return dst
	case faction.Rigger:
		return append(dst, geom.V(0.95, 0), geom.V(0.3, 0.8), geom.V(-0.8, 0.8), geom.V(-0.8, -0.8), geom.V(0.3, -0.8))
	case faction.Mender:
		const a, b = 0.95, 0.36
		return append(dst, geom.V(a, b), geom.V(b, b), geom.V(b, a), geom.V(-b, a), geom.V(-b, b), geom.V(-a, b),
			geom.V(-a, -b), geom.V(-b, -b), geom.V(-b, -a), geom.V(b, -a), geom.V(b, -b), geom.V(a, -b))
	case faction.Bastion:
		return append(dst, geom.V(0.9, 0.62), geom.V(-0.9, 0.62), geom.V(-0.9, -0.62), geom.V(0.9, -0.62))
	default:
		const k = 0.78
		return append(dst, geom.V(k, k), geom.V(-k, k), geom.V(-k, -k), geom.V(k, -k))
	}
}

// barrel returns the weapon line length in unit radii (0 = none).
func barrel(t faction.UnitType) float64 {
	switch t {
	case faction.Strider:
		return 1.6
	case faction.Trooper:
		return 1.35
	case faction.Bastion:
		return 1.7
	}
	return 0
}

func (r *Renderer) drawUnits(screen, em *ebiten.Image, v *MatchView, vis geom.Rect) {
	c := &r.match
	cam := v.Cam
	scale := cam.Scale()
	for p := range c.fill {
		c.fill[p].Reset()
		c.stroke[p].Reset()
	}
	var pts []geom.Vec
	var cargo []*sim.Unit
	var rings []ring
	for _, u := range v.World.Units {
		if !unitVisible(v, u) {
			continue
		}
		pos, facing := DrawPos(u, v.Alpha)
		if pos.X < vis.Min.X-2 || pos.X > vis.Max.X+2 || pos.Y < vis.Min.Y-2 || pos.Y > vis.Max.Y+2 {
			continue
		}
		fill, stroke := &c.fill[u.Owner], &c.stroke[u.Owner]
		frac := 0.0
		if faction.Unit(u.Type).Shift != nil {
			frac = u.ShiftFrac()
		}
		if u.Def.Building {
			rect := u.Rect()
			if frac > 0 {
				// Packed buildings hover: draw them raised over a shadow.
				h := float64(u.Def.Size) / 2
				if u.Lifted() {
					rect = geom.Rect{Min: pos.Sub(geom.V(h, h)), Max: pos.Add(geom.V(h, h))}
				}
				a, b := cam.ToScreenV(rect.Min), cam.ToScreenV(rect.Max)
				vector.FillRect(screen, float32(a.X), float32(a.Y), float32(b.X-a.X), float32(b.Y-a.Y), color.RGBA{0, 0, 0, uint8(0x70 * frac)}, false)
				lift := geom.V(0, -liftHeight*frac)
				rect = geom.Rect{Min: rect.Min.Add(lift), Max: rect.Max.Add(lift)}
				thrusters(stroke, rect, frac, cam)
			}
			buildingOutline(fill, stroke, u.Type, rect, u.Complete, cam)
			continue
		}
		sx, sy := cam.ToScreen(pos)
		rad := u.Def.Radius * scale
		sin, cos := math.Sincos(facing)
		at := func(q geom.Vec) (float32, float32) {
			return float32(sx + (q.X*cos-q.Y*sin)*rad), float32(sy + (q.X*sin+q.Y*cos)*rad)
		}
		pts = unitShape(pts[:0], u.Type)
		for i, q := range pts {
			x, y := at(q)
			if i == 0 {
				fill.MoveTo(x, y)
				stroke.MoveTo(x, y)
			} else {
				fill.LineTo(x, y)
				stroke.LineTo(x, y)
			}
		}
		fill.Close()
		stroke.Close()
		if l := barrel(u.Type); l > 0 {
			switch u.Type {
			case faction.Bastion:
				l += 0.9 * frac
			case faction.Strider:
				l += 0.5 * frac
			}
			x, y := at(geom.V(0.3, 0))
			stroke.MoveTo(x, y)
			x, y = at(geom.V(l, 0))
			stroke.LineTo(x, y)
		}
		if frac > 0 {
			rings = modeVisual(stroke, rings, u, frac, at, sx, sy, rad, scale, r.frame)
		}
		if u.Cargo > 0 {
			cargo = append(cargo, u)
		}
	}
	for p := range sim.PlayerID(sim.NumPlayers) {
		col := PlayerColor(p, v.Player)
		dop := &vector.DrawPathOptions{AntiAlias: true}
		dop.ColorScale.ScaleWithColor(scaleRGB(col, 0.16))
		vector.FillPath(screen, &c.fill[p], nil, dop)

		sop := &vector.StrokeOptions{Width: float32(max(1.5, 2.4*cam.Zoom)), LineJoin: vector.LineJoinRound, LineCap: vector.LineCapRound}
		dop = &vector.DrawPathOptions{AntiAlias: true}
		dop.ColorScale.ScaleWithColor(col)
		vector.StrokePath(em, &c.stroke[p], sop, dop)
	}
	for _, u := range cargo {
		pos, facing := DrawPos(u, v.Alpha)
		drawCargo(em, v, u, pos, facing)
	}
	for _, g := range rings {
		vector.StrokeCircle(em, g.x, g.y, g.r, g.w, scaleRGB(PlayerColor(g.owner, v.Player), g.a), true)
	}
}

// liftHeight is how far (in tiles) packed buildings float.
const liftHeight = 0.4

// ring is an extra circle drawn around a unit in a SHIFT mode.
type ring struct {
	owner      sim.PlayerID
	x, y, r, w float32
	a          float64 // brightness
}

// modeVisual adds u's SHIFT-mode decorations, faded in by frac.
func modeVisual(stroke *vector.Path, rings []ring, u *sim.Unit, frac float64, at func(geom.Vec) (float32, float32), sx, sy, rad, scale float64, frame int) []ring {
	seg := func(a, b geom.Vec) {
		x, y := at(a)
		stroke.MoveTo(x, y)
		x, y = at(b)
		stroke.LineTo(x, y)
	}
	pulse := 0.5 + 0.5*math.Sin(float64(frame)*0.15+float64(u.ID))
	switch u.Type {
	case faction.Bastion:
		// Outriggers swing out and brace.
		for _, c := range [4]geom.Vec{{X: 0.5, Y: 0.6}, {X: 0.5, Y: -0.6}, {X: -0.5, Y: 0.6}, {X: -0.5, Y: -0.6}} {
			d := geom.V(math.Copysign(0.35, c.X), math.Copysign(0.55, c.Y)).Scale(frac)
			seg(c, c.Add(d))
			e := c.Add(d)
			seg(e.Add(geom.V(-0.15, 0)), e.Add(geom.V(0.15, 0)))
		}
	case faction.Strider:
		// Plant spikes drive into the ground.
		for i := range 3 {
			sn, cs := math.Sincos(math.Pi/2 + float64(i)*2*math.Pi/3)
			seg(geom.V(cs*0.6, sn*0.6), geom.V(cs*(0.6+0.6*frac), sn*(0.6+0.6*frac)))
		}
	case faction.Outrider:
		// Boost exhaust.
		l := (0.6 + 0.5*pulse) * frac
		seg(geom.V(-0.8, 0.3), geom.V(-0.8-l, 0.3))
		seg(geom.V(-0.8, -0.3), geom.V(-0.8-l, -0.3))
	case faction.Trooper:
		rings = append(rings, ring{u.Owner, float32(sx), float32(sy), float32(rad * (1.45 + 0.15*pulse)), 1.5, frac * (0.5 + 0.5*pulse)})
	case faction.Mender:
		seg(geom.V(0, -0.9), geom.V(0, -0.9-0.6*frac))
		seg(geom.V(-0.25, -1.2-0.3*frac), geom.V(0.25, -1.2-0.3*frac))
		if m := faction.Unit(u.Type).Shift; m != nil {
			rings = append(rings, ring{u.Owner, float32(sx), float32(sy), float32(m.AuraRange * scale * frac), 1.2, 0.25 + 0.15*pulse})
		}
	}
	return rings
}

// thrusters draws jets under a packed building's corners.
func thrusters(stroke *vector.Path, rect geom.Rect, frac float64, cam *Camera) {
	l := 0.3 * frac
	for _, x := range [2]float64{rect.Min.X + 0.3, rect.Max.X - 0.3} {
		a := cam.ToScreenV(geom.V(x, rect.Max.Y))
		b := cam.ToScreenV(geom.V(x, rect.Max.Y+l))
		line(stroke, a, b)
	}
}

func hpColor(frac float64) color.RGBA {
	switch {
	case frac > 0.6:
		return colHPGood
	case frac > 0.3:
		return colHPMid
	}
	return colHPLow
}

func (r *Renderer) drawHPBars(screen *ebiten.Image, v *MatchView, vis geom.Rect) {
	cam := v.Cam
	scale := cam.Scale()
	for _, u := range v.World.Units {
		if !unitVisible(v, u) {
			continue
		}
		frac := u.HP / u.Def.HP
		if frac >= 1 && u.ID != v.Hover && !isSelected(v, u.ID) {
			continue
		}
		pos, _ := DrawPos(u, v.Alpha)
		if !vis.Contains(pos) {
			continue
		}
		sx, sy := cam.ToScreen(pos)
		w := max(18, 2*u.Def.Radius*scale)
		x, y := float32(sx-w/2), float32(sy-(u.Def.Radius+0.3)*scale-5)
		vector.FillRect(screen, x-1, y-1, float32(w)+2, 5, color.RGBA{0, 0, 0, 0xC0}, false)
		vector.FillRect(screen, x, y, float32(w*max(frac, 0)), 3, hpColor(frac), false)
	}
}

func (r *Renderer) drawFog(screen *ebiten.Image, v *MatchView) {
	if v.RevealAll {
		return
	}
	c := &r.match
	f := v.World.Fog[v.Player]
	bg := colBackground
	for y := range f.H {
		for x := range f.W {
			var a uint32
			switch f.At(x, y) {
			case world.Unexplored:
				a = 0xF2
			case world.Explored:
				a = 0x90
			}
			i := 4 * (y*f.W + x)
			// Premultiplied alpha.
			c.fogPix[i] = byte(uint32(bg.R) * a / 0xFF)
			c.fogPix[i+1] = byte(uint32(bg.G) * a / 0xFF)
			c.fogPix[i+2] = byte(uint32(bg.B) * a / 0xFF)
			c.fogPix[i+3] = byte(a)
		}
	}
	c.fog.WritePixels(c.fogPix)
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Scale(v.Cam.Scale(), v.Cam.Scale())
	ox, oy := v.Cam.ToScreen(geom.Vec{})
	op.GeoM.Translate(ox, oy)
	screen.DrawImage(c.fog, op)
}
