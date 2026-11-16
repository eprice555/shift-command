package ui

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/eprice555/shift-command/internal/geom"
	"github.com/eprice555/shift-command/internal/sim"
)

// HUD layout (screen px).
const (
	hudMargin   = 12
	MinimapSize = 176
	panelW      = 520
	panelH      = 112
	// HUDBottom is how much of the screen bottom the HUD panels cover.
	HUDBottom = MinimapSize + 2*hudMargin
)

// MinimapRect returns the minimap's screen rect for a w×h screen.
func MinimapRect(w, h float64) geom.Rect {
	return geom.Rect{
		Min: geom.V(hudMargin, h-hudMargin-MinimapSize),
		Max: geom.V(hudMargin+MinimapSize, h-hudMargin),
	}
}

// MinimapToWorld converts a screen point on the minimap to a world point.
func MinimapToWorld(mm geom.Rect, sx, sy, mapW, mapH float64) geom.Vec {
	x := (sx - mm.Min.X) / mm.W() * mapW
	y := (sy - mm.Min.Y) / mm.H() * mapH
	return geom.V(min(max(x, 0), mapW), min(max(y, 0), mapH))
}

// PanelRect returns the selection panel's screen rect.
func PanelRect(w, h float64) geom.Rect {
	x := math.Round((w - panelW) / 2)
	return geom.Rect{Min: geom.V(x, h-hudMargin-panelH), Max: geom.V(x+panelW, h-hudMargin)}
}

// OverHUD reports whether a screen point is on a HUD panel.
func OverHUD(w, h, sx, sy float64) bool {
	p := geom.V(sx, sy)
	return MinimapRect(w, h).Contains(p) || PanelRect(w, h).Contains(p)
}

func drawBox(dst *ebiten.Image, r geom.Rect, edge color.Color) {
	vector.FillRect(dst, float32(r.Min.X), float32(r.Min.Y), float32(r.W()), float32(r.H()), colPanel, false)
	vector.StrokeRect(dst, float32(r.Min.X)+0.5, float32(r.Min.Y)+0.5, float32(r.W())-1, float32(r.H())-1, 1, edge, false)
}

func (r *Renderer) drawHUD(screen *ebiten.Image, v *MatchView) {
	w, h := float64(r.w), float64(r.h)
	world := v.World

	// Top-left status.
	own, seen := 0, 0
	for _, u := range world.Units {
		switch {
		case u.Owner == v.Player:
			own++
		case unitVisible(v, u):
			seen++
		}
	}
	secs := int(world.Tick / sim.TicksPerSecond)
	drawLeft(screen, fmt.Sprintf("%s  ·  SANDBOX", strings.ToUpper(world.Map.Name)), r.hudFace, 16, 20, colText)
	drawLeft(screen, fmt.Sprintf("UNITS %d   ENEMIES IN SIGHT %d   %02d:%02d", own, seen, secs/60, secs%60), r.smallFace, 16, 42, colDim)
	if v.RevealAll {
		drawLeft(screen, "FOG OFF (F4)", r.smallFace, 16, 60, colHPMid)
	}
	drawRight(screen, "F1  CONTROLS", r.smallFace, w-16, 20, colDim)

	if v.Message != "" && v.MessageAge < 150 {
		a := min(1, float64(150-v.MessageAge)/30)
		drawCentered(screen, v.Message, r.hudFace, w/2, 92, withAlpha(colText, a))
	}
	if !v.Focused {
		blink := 0.75 + 0.25*math.Sin(float64(r.frame)*0.15)
		drawCentered(screen, "CLICK TO FOCUS", r.labelFace, w/2, h*0.4, withAlpha(colText, blink))
	}

	r.drawPanel(screen, v, PanelRect(w, h))
	r.drawGroups(screen, v, PanelRect(w, h))
	r.drawMinimap(screen, v, MinimapRect(w, h))
	if v.ShowHelp {
		r.drawHelp(screen)
	}
	if v.Targeting {
		x, y := float32(v.Cursor.X), float32(v.Cursor.Y)
		vector.StrokeCircle(screen, x, y, 11, 1.5, colEnemy, true)
		vector.StrokeLine(screen, x-16, y, x-6, y, 1.5, colEnemy, true)
		vector.StrokeLine(screen, x+6, y, x+16, y, 1.5, colEnemy, true)
		vector.StrokeLine(screen, x, y-16, x, y-6, 1.5, colEnemy, true)
		vector.StrokeLine(screen, x, y+6, x, y+16, 1.5, colEnemy, true)
		drawLeft(screen, "ATTACK-MOVE", r.smallFace, v.Cursor.X+20, v.Cursor.Y+14, colEnemy)
	}
}

func (r *Renderer) drawPanel(screen *ebiten.Image, v *MatchView, box geom.Rect) {
	drawBox(screen, box, colPanelEdge)
	x, y := box.Min.X+14, box.Min.Y+18
	var units []*sim.Unit
	for _, id := range v.Selected {
		if u := v.World.Unit(id); u != nil {
			units = append(units, u)
		}
	}
	switch len(units) {
	case 0:
		drawLeft(screen, "NO SELECTION", r.hudFace, x, y, colDim)
		drawLeft(screen, "Drag a box or click to select your units.", r.smallFace, x, y+26, colDim)
		drawLeft(screen, "Right-click ground to move, an enemy to attack.", r.smallFace, x, y+44, colDim)
	case 1:
		u := units[0]
		col := PlayerColor(u.Owner, v.Player)
		d := u.Def
		drawLeft(screen, strings.ToUpper(d.Name), r.hudFace, x, y, col)
		drawRight(screen, unitState(u), r.smallFace, box.Max.X-14, y, colDim)
		drawLeft(screen, d.Role, r.smallFace, x, y+22, colDim)
		hp := fmt.Sprintf("HP %.0f/%.0f   ARMOR %s %g   SPEED %g", math.Max(u.HP, 0), d.HP, d.ArmorType, d.Armor, d.Speed)
		drawLeft(screen, hp, r.smallFace, x, y+44, colText)
		wp := d.Weapon
		drawLeft(screen, fmt.Sprintf("WEAPON %g %s   RANGE %g   COOLDOWN %gs", wp.Damage, wp.Type, wp.Range, wp.Cooldown), r.smallFace, x, y+62, colText)
		bw := float32(box.W() - 28)
		vector.FillRect(screen, float32(x), float32(y+78), bw, 4, color.RGBA{0x20, 0x26, 0x38, 0xFF}, false)
		frac := math.Max(u.HP, 0) / d.HP
		vector.FillRect(screen, float32(x), float32(y+78), bw*float32(frac), 4, hpColor(frac), false)
	default:
		r.drawWireframes(screen, v, units, box)
	}
}

func unitState(u *sim.Unit) string {
	switch {
	case u.Hold && u.Target() != 0:
		return "HOLDING · FIRING"
	case u.Hold:
		return "HOLDING"
	case len(u.Orders) > 0 && u.Orders[0].Kind == sim.OrderAttack:
		return "ATTACKING"
	case u.Target() != 0:
		return "ENGAGING"
	case len(u.Orders) > 0 && u.Orders[0].Kind == sim.OrderAttackMove:
		return "ATTACK-MOVING"
	case len(u.Orders) > 0:
		return "MOVING"
	}
	return "IDLE"
}

// drawWireframes shows a grid of unit icons with health, like a classic RTS.
func (r *Renderer) drawWireframes(screen *ebiten.Image, v *MatchView, units []*sim.Unit, box geom.Rect) {
	const cell, cols, rows = 40.0, 12, 2
	x0, y0 := box.Min.X+14, box.Min.Y+14
	var path vector.Path
	var pts []geom.Vec
	for i, u := range units {
		if i == cols*rows-1 && len(units) > cols*rows {
			drawCentered(screen, fmt.Sprintf("+%d", len(units)-i), r.hudFace, x0+float64(i%cols)*cell+cell/2-2, y0+float64(i/cols)*cell+cell/2-4, colText)
			break
		}
		cx := x0 + float64(i%cols)*cell + cell/2 - 2
		cy := y0 + float64(i/cols)*cell + cell/2 - 4
		frac := math.Max(u.HP, 0) / u.Def.HP
		path.Reset()
		pts = unitShape(pts[:0], u.Type)
		for j, q := range pts {
			// Icons point up.
			px, py := float32(cx+q.Y*11), float32(cy-q.X*11)
			if j == 0 {
				path.MoveTo(px, py)
			} else {
				path.LineTo(px, py)
			}
		}
		path.Close()
		sop := &vector.StrokeOptions{Width: 1.5, LineJoin: vector.LineJoinRound}
		dop := &vector.DrawPathOptions{AntiAlias: true}
		dop.ColorScale.ScaleWithColor(hpColor(frac))
		vector.StrokePath(screen, &path, sop, dop)
		vector.FillRect(screen, float32(cx-14), float32(cy+15), 28, 3, color.RGBA{0x20, 0x26, 0x38, 0xFF}, false)
		vector.FillRect(screen, float32(cx-14), float32(cy+15), 28*float32(frac), 3, hpColor(frac), false)
	}
	counts := map[string]int{}
	var order []string
	for _, u := range units {
		if counts[u.Def.Name] == 0 {
			order = append(order, u.Def.Name)
		}
		counts[u.Def.Name]++
	}
	parts := make([]string, len(order))
	for i, n := range order {
		parts[i] = fmt.Sprintf("%d %s", counts[n], strings.ToUpper(n))
	}
	drawRight(screen, strings.Join(parts, "  ·  "), r.smallFace, box.Max.X-14, box.Max.Y-12, colDim)
}

func (r *Renderer) drawGroups(screen *ebiten.Image, v *MatchView, panel geom.Rect) {
	x := panel.Min.X
	y := panel.Min.Y - 24
	for i := 1; i <= 10; i++ {
		g := i % 10
		n := v.Groups[g]
		if n == 0 {
			continue
		}
		box := geom.Rect{Min: geom.V(x, y), Max: geom.V(x+52, y+20)}
		drawBox(screen, box, colPanelEdge)
		drawLeft(screen, fmt.Sprint(g), r.smallFace, x+6, y+10, colSelect)
		drawRight(screen, fmt.Sprintf("×%d", n), r.smallFace, box.Max.X-6, y+10, colText)
		x += 56
	}
}

func (r *Renderer) drawMinimap(screen *ebiten.Image, v *MatchView, mm geom.Rect) {
	c := &r.match
	m := v.World.Map
	drawBox(screen, mm, colPanelEdge)
	sx, sy := mm.W()/float64(m.W), mm.H()/float64(m.H)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(sx, sy)
	op.GeoM.Translate(mm.Min.X, mm.Min.Y)
	screen.DrawImage(c.miniWall, op)
	if !v.RevealAll {
		// Unexplored terrain stays faintly visible on the minimap for orientation.
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		op.ColorScale.ScaleAlpha(0.82)
		op.GeoM.Scale(sx, sy)
		op.GeoM.Translate(mm.Min.X, mm.Min.Y)
		screen.DrawImage(c.fog, op)
	}
	for _, u := range v.World.Units {
		if !unitVisible(v, u) {
			continue
		}
		col := PlayerColor(u.Owner, v.Player)
		if isSelected(v, u.ID) {
			col = colSelect
		}
		px, py := mm.Min.X+u.Pos.X*sx, mm.Min.Y+u.Pos.Y*sy
		vector.FillRect(screen, float32(px-1.5), float32(py-1.5), 3, 3, col, false)
	}
	vis := v.Cam.Visible()
	x0 := max(vis.Min.X, 0)*sx + mm.Min.X
	y0 := max(vis.Min.Y, 0)*sy + mm.Min.Y
	x1 := min(vis.Max.X, float64(m.W))*sx + mm.Min.X
	y1 := min(vis.Max.Y, float64(m.H))*sy + mm.Min.Y
	vector.StrokeRect(screen, float32(x0), float32(y0), float32(x1-x0), float32(y1-y0), 1, colText, false)
}

var helpLines = [][2]string{
	{"LEFT CLICK / DRAG", "select units"},
	{"DOUBLE-CLICK", "select same type on screen"},
	{"TAB / F + CLICK", "add to selection · queue orders"},
	{"RIGHT CLICK", "move · attack enemy"},
	{"A, THEN CLICK", "attack-move"},
	{"S / H", "stop / hold position"},
	{"CTRL or ALT + 0-9", "set control group"},
	{"0-9 (TWICE)", "select group (center camera)"},
	{"ARROWS / MIDDLE-DRAG", "scroll camera"},
	{"WHEEL", "zoom"},
	{"MINIMAP", "left: jump · right: move"},
	{"SHIFT", "faction SHIFT (coming in M2)"},
	{"F2 / F3", "spawn your / enemy squad"},
	{"F4", "toggle fog of war"},
	{"ESC", "cancel targeting · deselect"},
}

func (r *Renderer) drawHelp(screen *ebiten.Image) {
	const lh = 19.0
	bw, bh := 440.0, 44+lh*float64(len(helpLines))
	box := geom.Rect{Min: geom.V(float64(r.w)-bw-hudMargin, 36), Max: geom.V(float64(r.w)-hudMargin, 36+bh)}
	drawBox(screen, box, colPanelEdge)
	drawLeft(screen, "CONTROLS", r.hudFace, box.Min.X+14, box.Min.Y+18, colText)
	for i, l := range helpLines {
		y := box.Min.Y + 44 + lh*float64(i)
		drawLeft(screen, l[0], r.smallFace, box.Min.X+14, y, colSelect)
		drawLeft(screen, l[1], r.smallFace, box.Min.X+196, y, colDim)
	}
}
