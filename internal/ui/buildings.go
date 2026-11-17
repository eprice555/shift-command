package ui

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
	"github.com/eprice555/shift-command/internal/sim"
	"github.com/eprice555/shift-command/internal/world"
)

var (
	colCrystal = color.RGBA{0x7F, 0xD8, 0xFF, 0xFF}
	colFlux    = color.RGBA{0xB4, 0x7C, 0xFF, 0xFF}
	colBuild   = color.RGBA{0x3C, 0xF0, 0xFF, 0xFF}
	colBad     = color.RGBA{0xFF, 0x46, 0x3C, 0xFF}
)

func resourceColor(k world.ResourceKind) color.RGBA {
	if k == world.Flux {
		return colFlux
	}
	return colCrystal
}

// poly appends a closed polygon in screen space to path.
func poly(path *vector.Path, pts ...geom.Vec) {
	for i, q := range pts {
		if i == 0 {
			path.MoveTo(float32(q.X), float32(q.Y))
		} else {
			path.LineTo(float32(q.X), float32(q.Y))
		}
	}
	path.Close()
}

func line(path *vector.Path, a, b geom.Vec) {
	path.MoveTo(float32(a.X), float32(a.Y))
	path.LineTo(float32(b.X), float32(b.Y))
}

// ngon returns n points on a circle of radius rad around c, starting at angle a0.
func ngon(c geom.Vec, rad float64, n int, a0 float64) []geom.Vec {
	pts := make([]geom.Vec, n)
	for i := range n {
		s, k := math.Sincos(a0 + float64(i)*2*math.Pi/float64(n))
		pts[i] = geom.V(c.X+k*rad, c.Y+s*rad)
	}
	return pts
}

// buildingOutline adds b's footprint (and, when finished, its type detail)
// to the fill and stroke paths in screen space. frac is construction
// progress and wob a wobble phase for organic (Brood) buildings.
func buildingOutline(fill, stroke *vector.Path, t faction.UnitType, rect geom.Rect, complete bool, frac, wob float64, cam *Camera) {
	if isBrood(t) {
		broodBuilding(fill, stroke, t, rect, complete, frac, wob, cam)
		return
	}
	const inset = 0.12
	a := cam.ToScreenV(rect.Min.Add(geom.V(inset, inset)))
	b := cam.ToScreenV(rect.Max.Sub(geom.V(inset, inset)))
	corners := []geom.Vec{a, geom.V(b.X, a.Y), b, geom.V(a.X, b.Y)}
	if fill != nil {
		poly(fill, corners...)
	}
	poly(stroke, corners...)
	if !complete {
		// Scaffold: a cross brace until construction finishes.
		line(stroke, a, b)
		line(stroke, geom.V(b.X, a.Y), geom.V(a.X, b.Y))
		return
	}
	c := a.Lerp(b, 0.5)
	s := b.X - a.X
	switch t {
	case faction.CommandHub:
		poly(stroke, ngon(c, s*0.3, 6, 0)...)
		q := s * 0.1
		poly(stroke, geom.V(c.X-q, c.Y-q), geom.V(c.X+q, c.Y-q), geom.V(c.X+q, c.Y+q), geom.V(c.X-q, c.Y+q))
	case faction.Generator:
		poly(stroke, ngon(c, s*0.3, 8, math.Pi/8)...)
		line(stroke, geom.V(c.X+s*0.06, c.Y-s*0.22), geom.V(c.X-s*0.08, c.Y+s*0.02))
		line(stroke, geom.V(c.X-s*0.08, c.Y+s*0.02), geom.V(c.X+s*0.08, c.Y-s*0.02))
		line(stroke, geom.V(c.X+s*0.08, c.Y-s*0.02), geom.V(c.X-s*0.06, c.Y+s*0.22))
	case faction.Extractor:
		poly(stroke, ngon(c, s*0.32, 4, 0)...)
		poly(stroke, ngon(c, s*0.12, 4, 0)...)
	case faction.DrillYard:
		for _, dx := range []float64{-0.16, 0.08} {
			x := c.X + s*dx
			line(stroke, geom.V(x, c.Y-s*0.22), geom.V(x+s*0.14, c.Y))
			line(stroke, geom.V(x+s*0.14, c.Y), geom.V(x, c.Y+s*0.22))
		}
	case faction.Foundry:
		hex := ngon(c, s*0.26, 6, math.Pi/6)
		poly(stroke, hex...)
		for i := 0; i < 6; i += 2 {
			line(stroke, c, hex[i])
		}
	}
}

func (r *Renderer) drawResources(screen, em *ebiten.Image, v *MatchView, vis geom.Rect) {
	cam := v.Cam
	var fill, crystal, flux vector.Path
	pulse := 0.5 + 0.5*math.Sin(float64(r.frame)*0.06)
	for _, res := range v.World.Resources {
		if res.Pos.X < vis.Min.X-2 || res.Pos.X > vis.Max.X+2 || res.Pos.Y < vis.Min.Y-2 || res.Pos.Y > vis.Max.Y+2 {
			continue
		}
		c := cam.ToScreenV(res.Pos)
		s := cam.Scale()
		if res.Kind == world.Crystal {
			k := 0.3 + 0.18*float64(res.Amount)/sim.CrystalAmount
			pts := []geom.Vec{
				geom.V(c.X, c.Y-s*k*1.3), geom.V(c.X+s*k, c.Y),
				geom.V(c.X, c.Y+s*k*1.3), geom.V(c.X-s*k, c.Y),
			}
			poly(&fill, pts...)
			poly(&crystal, pts...)
			continue
		}
		poly(&flux, ngon(c, s*(0.62+0.08*pulse), 12, 0)...)
		poly(&flux, ngon(c, s*0.3, 6, float64(r.frame)*0.01)...)
	}
	dop := &vector.DrawPathOptions{AntiAlias: true}
	dop.ColorScale.ScaleWithColor(scaleRGB(colCrystal, 0.18))
	vector.FillPath(screen, &fill, nil, dop)
	sop := &vector.StrokeOptions{Width: float32(max(1.2, 1.8*cam.Zoom)), LineJoin: vector.LineJoinRound}
	dop = &vector.DrawPathOptions{AntiAlias: true}
	dop.ColorScale.ScaleWithColor(scaleRGB(colCrystal, 0.85))
	vector.StrokePath(em, &crystal, sop, dop)
	dop = &vector.DrawPathOptions{AntiAlias: true}
	dop.ColorScale.ScaleWithColor(scaleRGB(colFlux, 0.6+0.3*pulse))
	vector.StrokePath(em, &flux, sop, dop)
}

// drawGhosts shows remembered enemy buildings that are currently out of sight.
func (r *Renderer) drawGhosts(screen *ebiten.Image, v *MatchView) {
	if v.RevealAll {
		return
	}
	var path vector.Path
	for id, g := range v.World.Seen[v.Player] {
		if u := v.World.Unit(id); u != nil && v.World.VisibleTo(v.Player, u) {
			continue
		}
		rect := world.TileRect(g.TileX, g.TileY, faction.Unit(g.Type).Size)
		buildingOutline(nil, &path, g.Type, rect, g.Complete, 0.5, 0, v.Cam)
	}
	sop := &vector.StrokeOptions{Width: 1.5, LineJoin: vector.LineJoinRound}
	dop := &vector.DrawPathOptions{AntiAlias: true}
	dop.ColorScale.ScaleWithColor(colEnemy)
	dop.ColorScale.ScaleAlpha(0.45)
	vector.StrokePath(screen, &path, sop, dop)
}

// drawPlacement shows the building footprint under the cursor.
func (r *Renderer) drawPlacement(screen *ebiten.Image, v *MatchView) {
	if !v.Placing {
		return
	}
	d := faction.Unit(v.PlaceType)
	cam := v.Cam
	if d.OnVent {
		for _, res := range v.World.Resources {
			if res.Kind == world.Flux && res.Extractor == 0 {
				c := cam.ToScreenV(res.Pos)
				vector.StrokeCircle(screen, float32(c.X), float32(c.Y), float32(cam.Scale()*1.1), 1.5, withAlpha(colBuild, 0.5), true)
			}
		}
	}
	rect := world.TileRect(v.PlaceX, v.PlaceY, d.Size)
	col := colSelect
	if v.PlaceNotice != sim.NoticeNone {
		col = colBad
	}
	a := cam.ToScreenV(rect.Min)
	b := cam.ToScreenV(rect.Max)
	vector.FillRect(screen, float32(a.X), float32(a.Y), float32(b.X-a.X), float32(b.Y-a.Y), withAlpha(col, 0.16), false)
	var path vector.Path
	buildingOutline(nil, &path, v.PlaceType, rect, true, 1, float64(r.frame)*0.06, cam)
	sop := &vector.StrokeOptions{Width: 2, LineJoin: vector.LineJoinRound}
	dop := &vector.DrawPathOptions{AntiAlias: true}
	dop.ColorScale.ScaleWithColor(col)
	vector.StrokePath(screen, &path, sop, dop)
	label := d.Name
	if v.PlaceNotice != sim.NoticeNone {
		label = noticeText(v.PlaceNotice, v.World, v.Player, v.PlaceType)
	}
	drawCentered(screen, label, r.smallFace, (a.X+b.X)/2, a.Y-12, col)
}

// noticeText describes notice n for player p trying to make t.
func noticeText(n sim.Notice, w *sim.World, p sim.PlayerID, t faction.UnitType) string {
	if n == sim.NoticeRequires {
		if req, ok := w.Missing(p, t); ok {
			return "Requires " + faction.Unit(req).Name
		}
	}
	if n == sim.NoticeSupply {
		return "Supply blocked: build a " + faction.Unit(faction.Get(w.Players[p].Faction).Supply).Name
	}
	return n.String()
}

// drawRally shows where the selected buildings send new units.
func (r *Renderer) drawRally(screen *ebiten.Image, v *MatchView) {
	for _, id := range v.Selected {
		u := v.World.Unit(id)
		if u == nil || u.Owner != v.Player || !u.Def.Building || !u.HasRally || len(u.Def.Trains) == 0 {
			continue
		}
		a := v.Cam.ToScreenV(u.Pos)
		b := v.Cam.ToScreenV(u.Rally)
		d := b.Sub(a)
		n := int(d.Len() / 10)
		for i := 0; i < n; i += 2 {
			p := a.Add(d.Scale(float64(i) / float64(n)))
			q := a.Add(d.Scale(float64(i+1) / float64(n)))
			vector.StrokeLine(screen, float32(p.X), float32(p.Y), float32(q.X), float32(q.Y), 1.5, withAlpha(colSelect, 0.6), true)
		}
		vector.StrokeCircle(screen, float32(b.X), float32(b.Y), 5, 1.5, colSelect, true)
		vector.StrokeLine(screen, float32(b.X), float32(b.Y), float32(b.X), float32(b.Y-16), 1.5, colSelect, true)
		vector.FillRect(screen, float32(b.X), float32(b.Y-16), 9, 6, withAlpha(colSelect, 0.8), false)
	}
}

// drawCargo marks workers carrying resources.
func drawCargo(em *ebiten.Image, v *MatchView, u *sim.Unit, pos geom.Vec, facing float64) {
	if u.Cargo == 0 {
		return
	}
	s, c := math.Sincos(facing)
	p := v.Cam.ToScreenV(pos.Add(geom.V(c, s).Scale(u.Def.Radius * 0.9)))
	vector.FillCircle(em, float32(p.X), float32(p.Y), float32(max(2, 0.14*v.Cam.Scale())), resourceColor(u.CargoKind), true)
}

// drawBuildBars shows construction and production progress on own buildings.
func drawBuildBars(screen *ebiten.Image, v *MatchView, vis geom.Rect) {
	for _, u := range v.World.Units {
		if u.Owner != v.Player || !vis.Contains(u.Pos) {
			continue
		}
		if u.Morphing() {
			drawUnitBar(screen, v, u, u.MorphFrac(), colBuild)
			continue
		}
		if !u.Def.Building {
			continue
		}
		var frac float64
		switch {
		case !u.Complete:
			frac = u.BuildFrac()
		case len(u.Queue) > 0:
			frac = u.TrainFrac()
		default:
			continue
		}
		rect := u.Rect()
		a := v.Cam.ToScreenV(geom.V(rect.Min.X+0.25, rect.Max.Y-0.45))
		b := v.Cam.ToScreenV(geom.V(rect.Max.X-0.25, rect.Max.Y-0.45))
		w := float32(b.X - a.X)
		vector.FillRect(screen, float32(a.X)-1, float32(a.Y)-1, w+2, 5, color.RGBA{0, 0, 0, 0xC0}, false)
		vector.FillRect(screen, float32(a.X), float32(a.Y), w*float32(frac), 3, colBuild, false)
	}
}
