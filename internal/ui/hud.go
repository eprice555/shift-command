package ui

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
	"github.com/eprice555/shift-command/internal/sim"
	"github.com/eprice555/shift-command/internal/world"
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
	return MinimapRect(w, h).Contains(p) || PanelRect(w, h).Contains(p) ||
		CardRect(w, h).Contains(p) || ResourceBarRect(w).Contains(p)
}

// ResourceBarRect returns the top-right resource readout's screen rect.
func ResourceBarRect(w float64) geom.Rect {
	return geom.Rect{Min: geom.V(w-hudMargin-400, 8), Max: geom.V(w-hudMargin, 34)}
}

func drawBox(dst *ebiten.Image, r geom.Rect, edge color.Color) {
	vector.FillRect(dst, float32(r.Min.X), float32(r.Min.Y), float32(r.W()), float32(r.H()), colPanel, false)
	vector.StrokeRect(dst, float32(r.Min.X)+0.5, float32(r.Min.Y)+0.5, float32(r.W())-1, float32(r.H())-1, 1, edge, false)
}

func (r *Renderer) drawHUD(screen *ebiten.Image, v *MatchView) {
	w, h := float64(r.w), float64(r.h)
	world := v.World

	// Top-left status.
	workers, army, buildings, spawn := 0, 0, 0, 0
	for _, u := range world.Units {
		switch {
		case u.Owner != v.Player:
		case u.Def.Building:
			buildings++
		case u.Def.Worker:
			workers++
		case u.Def.Morphs:
			spawn++
		default:
			army++
		}
	}
	secs := int(world.Tick / sim.TicksPerSecond)
	mode := "SKIRMISH"
	if v.Sandbox {
		mode = "SANDBOX"
	}
	drawLeft(screen, fmt.Sprintf("%s  ·  %s", strings.ToUpper(world.Map.Name), mode), r.hudFace, 16, 20, colText)
	counts := fmt.Sprintf("WORKERS %d   ARMY %d   BUILDINGS %d", workers, army, buildings)
	if world.Players[v.Player].Faction == faction.Brood {
		counts = fmt.Sprintf("WORKERS %d   SPAWN %d   ARMY %d   BUILDINGS %d", workers, spawn, army, buildings)
	}
	drawLeft(screen, fmt.Sprintf("%s   %02d:%02d", counts, secs/60, secs%60), r.smallFace, 16, 42, colDim)
	if v.RevealAll {
		drawLeft(screen, "FOG OFF (F4)", r.smallFace, 16, 60, colHPMid)
	}
	if !v.Sandbox {
		r.drawResourceBar(screen, v, ResourceBarRect(w))
	}
	drawRight(screen, "F1  CONTROLS", r.smallFace, w-16, 48, colDim)

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
	r.drawCard(screen, v, CardRect(w, h))
	if v.ShowHelp {
		r.drawHelp(screen)
	}
	if world.Over {
		r.drawGameOver(screen, v)
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
		help := [2]string{
			"Riggers mine crystals on their own. Select one and press",
			"Q-T to build. Select the Hub and press Q to train Riggers.",
		}
		switch v.World.Players[v.Player].Faction {
		case faction.Brood:
			help = [2]string{
				"Drones mine on their own and morph into buildings (Q-T,",
				"on creep). Select the Hive and press Q-T/A to morph Spawn.",
			}
		case faction.Phasebound:
			help = [2]string{
				"Drifters summon buildings (Q-T). Phasebound SHIFT phases.",
				"Power Gates with Anchors; research Warp at a Resonator.",
			}
		}
		drawLeft(screen, help[0], r.smallFace, x, y+26, colDim)
		drawLeft(screen, help[1], r.smallFace, x, y+44, colDim)
		drawLeft(screen, "Destroy every enemy building to win.", r.smallFace, x, y+62, colDim)
	case 1:
		u := units[0]
		if u.Def.Building {
			r.drawBuildingPanel(screen, v, u, box)
			return
		}
		col := PlayerColor(v.World, u.Owner, v.Player)
		d := u.Def
		drawLeft(screen, strings.ToUpper(d.Name), r.hudFace, x, y, col)
		drawRight(screen, unitState(u), r.smallFace, box.Max.X-14, y, colDim)
		drawLeft(screen, d.Role, r.smallFace, x, y+22, colDim)
		hp := fmt.Sprintf("HP %.0f/%.0f   ARMOR %s %g   SPEED %g", math.Max(u.HP, 0), d.HP, d.ArmorType, d.Armor, d.Speed)
		if d.Shield > 0 {
			hp = fmt.Sprintf("HP %.0f/%.0f   SHIELD %.0f/%.0f   %s %g   SPEED %g", math.Max(u.HP, 0), d.HP, u.Shield, d.Shield, d.ArmorType, d.Armor, d.Speed)
		}
		drawLeft(screen, hp, r.smallFace, x, y+44, colText)
		var line string
		switch wp := d.Weapon; {
		case d.Armed():
			line = fmt.Sprintf("WEAPON %g %s   RANGE %g   COOLDOWN %gs", wp.Damage, wp.Type, wp.Range, wp.Cooldown)
		case d.ShieldHeal:
			line = fmt.Sprintf("RESTORES %g SHIELD/s   RANGE %g", d.HealRate, d.HealRange)
		case d.HealRate > 0:
			line = fmt.Sprintf("HEALS %g HP/s   RANGE %g   (infantry)", d.HealRate, d.HealRange)
		}
		if wp := d.Weapon; wp.VsWorker > 0 {
			line += fmt.Sprintf("   +%g VS WORKERS", wp.VsWorker)
		}
		if d.Worker {
			line = "NO CARGO"
			if u.Cargo > 0 {
				kind := "CRYSTALS"
				if u.CargoKind == world.Flux {
					kind = "FLUX"
				}
				line = fmt.Sprintf("CARGO %d %s", u.Cargo, kind)
			}
		}
		drawLeft(screen, line, r.smallFace, x, y+62, colText)
		bw := float32(box.W() - 28)
		vector.FillRect(screen, float32(x), float32(y+78), bw, 4, color.RGBA{0x20, 0x26, 0x38, 0xFF}, false)
		frac := math.Max(u.HP, 0) / d.HP
		vector.FillRect(screen, float32(x), float32(y+78), bw*float32(frac), 4, hpColor(frac), false)
		if d.Shield > 0 {
			vector.FillRect(screen, float32(x), float32(y+84), bw*float32(u.Shield/d.Shield), 2, colShield, false)
		}
	default:
		r.drawWireframes(screen, v, units, box)
	}
}

func (r *Renderer) drawBuildingPanel(screen *ebiten.Image, v *MatchView, u *sim.Unit, box geom.Rect) {
	x, y := box.Min.X+14, box.Min.Y+18
	d := u.Def
	drawLeft(screen, strings.ToUpper(d.Name), r.hudFace, x, y, PlayerColor(v.World, u.Owner, v.Player))
	drawRight(screen, unitState(u), r.smallFace, box.Max.X-14, y, colDim)
	stats := fmt.Sprintf("HP %.0f/%.0f   ARMOR %g", math.Max(u.HP, 0), d.HP, d.Armor)
	if d.Shield > 0 {
		stats = fmt.Sprintf("HP %.0f/%.0f   SHIELD %.0f/%.0f", math.Max(u.HP, 0), d.HP, u.Shield, d.Shield)
	}
	if d.SupplyProvided > 0 {
		stats += fmt.Sprintf("   SUPPLY +%g", d.SupplyProvided)
	}
	drawLeft(screen, stats, r.smallFace, x, y+22, hpColor(u.HP/d.HP))
	if u.Owner != v.Player {
		drawLeft(screen, d.Role, r.smallFace, x, y+44, colDim)
		return
	}
	bar := func(label string, frac float64, bx, by float64) {
		bw := box.Max.X - 14 - bx
		drawLeft(screen, label, r.smallFace, bx, by, colText)
		vector.FillRect(screen, float32(bx), float32(by+12), float32(bw), 4, color.RGBA{0x20, 0x26, 0x38, 0xFF}, false)
		vector.FillRect(screen, float32(bx), float32(by+12), float32(bw*frac), 4, colBuild, false)
	}
	if !u.Complete {
		verb := "CONSTRUCTING"
		switch {
		case d.Warped:
			verb = "WARPING IN"
		case d.SelfBuild:
			verb = "GROWING"
		}
		bar(fmt.Sprintf("%s  %d%%", verb, int(u.BuildFrac()*100)), u.BuildFrac(), x, y+46)
		return
	}
	if d.SpawnMax > 0 {
		n := v.World.SpawnCount(u)
		label := fmt.Sprintf("SPAWN %d/%d · FULL", n, d.SpawnMax)
		frac := 1.0
		if n < d.SpawnMax {
			label = fmt.Sprintf("SPAWN %d/%d · NEXT HATCHING %d%%", n, d.SpawnMax, int(u.HatchFrac()*100))
			frac = u.HatchFrac()
		}
		bar(label, frac, x, y+46)
		drawLeft(screen, "Q-T/A morph a Spawn · right-click sets rally", r.smallFace, x, y+80, colDim)
		return
	}
	if len(d.Trains) == 0 {
		drawLeft(screen, d.Role, r.smallFace, x, y+44, colDim)
		return
	}
	if v.World.Unpowered(u) {
		drawLeft(screen, "NO POWER: summon an Anchor nearby to resume training", r.smallFace, x, y+44, colBad)
	}
	var pts []geom.Vec
	var path vector.Path
	for i := range sim.MaxQueue {
		q := QueueSlotRect(box, i)
		edge := colPanelEdge
		if i < len(u.Queue) && q.Contains(v.Cursor) {
			edge = colBad
		}
		vector.StrokeRect(screen, float32(q.Min.X)+0.5, float32(q.Min.Y)+0.5, float32(q.W())-1, float32(q.H())-1, 1, edge, false)
		if i >= len(u.Queue) {
			continue
		}
		c := q.Min.Lerp(q.Max, 0.5)
		path.Reset()
		pts = unitShape(pts[:0], u.Queue[i])
		for j, p := range pts {
			px, py := float32(c.X+p.Y*12), float32(c.Y-p.X*12)
			if j == 0 {
				path.MoveTo(px, py)
			} else {
				path.LineTo(px, py)
			}
		}
		path.Close()
		sop := &vector.StrokeOptions{Width: 1.5, LineJoin: vector.LineJoinRound}
		dop := &vector.DrawPathOptions{AntiAlias: true}
		dop.ColorScale.ScaleWithColor(PlayerColor(v.World, u.Owner, v.Player))
		vector.StrokePath(screen, &path, sop, dop)
	}
	q0 := QueueSlotRect(box, sim.MaxQueue)
	if len(u.Queue) > 0 {
		bar(fmt.Sprintf("TRAINING %s  %d%%", strings.ToUpper(faction.Unit(u.Queue[0]).Name), int(u.TrainFrac()*100)), u.TrainFrac(), q0.Min.X+10, q0.Min.Y+8)
	} else if u.WarpResearching() {
		bar(fmt.Sprintf("RESEARCHING WARP  %d%%", int(u.WarpResearchFrac()*100)), u.WarpResearchFrac(), x, y+66)
	} else {
		drawLeft(screen, "Press "+cardKeys[0][0]+" to train · right-click sets rally", r.smallFace, q0.Min.X+10, q0.Min.Y+20, colDim)
	}
}

// unitState describes what u is doing, led by its SHIFT mode.
func unitState(u *sim.Unit) string {
	if u.Morphing() {
		return fmt.Sprintf("MORPHING INTO %s · %d%%", strings.ToUpper(faction.Unit(u.Queue[0]).Name), int(u.MorphFrac()*100))
	}
	if u.WarpingIn() {
		return fmt.Sprintf("WARPING IN · %d%%", int(u.WarpFrac()*100))
	}
	m := faction.Unit(u.Type).Shift
	switch {
	case u.LifeLeft() > 0:
		return fmt.Sprintf("FADES IN %.0fs · %s", math.Ceil(u.LifeLeft()), baseState(u))
	case u.WarpResearching():
		return fmt.Sprintf("RESEARCHING WARP · %d%%", int(u.WarpResearchFrac()*100))
	case m == nil:
	case u.Dashing():
		return "PHASE DASH · " + baseState(u)
	case u.Phased():
		return fmt.Sprintf("PHASED %.1fs · %s", u.PhaseLeft(), baseState(u))
	case u.Leaping():
		return "POUNCING!"
	case m.Cast() && u.CastLeft() > 0:
		return fmt.Sprintf("%s IN %.0fs · %s", strings.ToUpper(m.Name), math.Ceil(u.CastLeft()), baseState(u))
	case m.Cast():
		return strings.ToUpper(m.Name) + " READY · " + baseState(u)
	case u.Shifting() && u.Def.Building && u.Mode == 1:
		return "PACKING UP..."
	case u.Shifting() && u.Def.Building:
		return "LANDING..."
	case u.Shifting() && u.Mode == 1:
		return "SHIFTING TO " + strings.ToUpper(m.Name) + "..."
	case u.Shifting():
		return "SHIFTING TO " + strings.ToUpper(m.Base) + "..."
	case u.Def.Building && u.Mode == 1:
		if len(u.Orders) > 0 {
			return "PACKED · FLYING"
		}
		return "PACKED · SHIFT TO LAND"
	case u.Mode == 1 && m.Timed():
		return fmt.Sprintf("%s %.0fs · %s", strings.ToUpper(m.Name), math.Ceil(u.ModeLeft()), baseState(u))
	case u.Mode == 1:
		return strings.ToUpper(m.Name) + " · " + baseState(u)
	}
	return baseState(u)
}

func baseState(u *sim.Unit) string {
	if u.Def.Building {
		switch {
		case !u.Complete:
			if u.Def.Warped {
				return "WARPING IN"
			}
			if u.Def.SelfBuild {
				return "GROWING"
			}
			return "UNDER CONSTRUCTION"
		case u.WarpResearching():
			return fmt.Sprintf("RESEARCHING WARP · %d%%", int(u.WarpResearchFrac()*100))
		case len(u.Queue) > 0:
			return fmt.Sprintf("TRAINING · %d QUEUED", len(u.Queue))
		}
		return "READY"
	}
	var kind sim.OrderKind = 255
	if len(u.Orders) > 0 {
		kind = u.Orders[0].Kind
	}
	switch {
	case u.Hold && u.Target() != 0:
		return "HOLDING · FIRING"
	case u.Hold:
		return "HOLDING"
	case u.HealTarget() != 0 && u.Def.ShieldHeal:
		return "RESTORING SHIELDS"
	case u.HealTarget() != 0:
		return "HEALING"
	case kind == sim.OrderGather && u.Cargo > 0, kind == sim.OrderReturn:
		return "RETURNING CARGO"
	case kind == sim.OrderGather && u.Mining():
		return "MINING"
	case kind == sim.OrderGather:
		return "GATHERING"
	case kind == sim.OrderBuild:
		return "GOING TO BUILD " + strings.ToUpper(faction.Unit(u.Orders[0].Build).Name)
	case kind == sim.OrderConstruct:
		return "CONSTRUCTING"
	case kind == sim.OrderRepair:
		return "REPAIRING"
	case kind == sim.OrderAttack:
		return "ATTACKING"
	case u.Target() != 0:
		return "ENGAGING"
	case kind == sim.OrderAttackMove:
		return "ATTACK-MOVING"
	case len(u.Orders) > 0:
		return "MOVING"
	}
	return "IDLE"
}

func costText(d faction.UnitDef) string {
	if d.Flux > 0 {
		return fmt.Sprintf("%d/%d", d.Crystals, d.Flux)
	}
	return fmt.Sprint(d.Crystals)
}

func (r *Renderer) drawCard(screen *ebiten.Image, v *MatchView, card geom.Rect) {
	drawBox(screen, card, colPanelEdge)
	for row := range CardRows {
		for col := range CardCols {
			b := ButtonRect(card, row, col)
			vector.StrokeRect(screen, float32(b.Min.X)+0.5, float32(b.Min.Y)+0.5, float32(b.W())-1, float32(b.H())-1, 1, color.RGBA{0x16, 0x20, 0x36, 0xFF}, false)
		}
	}
	var hover *Button
	for i := range v.Card {
		bt := &v.Card[i]
		b := ButtonRect(card, bt.Row, bt.Col)
		col := colText
		edge := colPanelEdge
		if !bt.Enabled {
			col, edge = colDim, color.RGBA{0x22, 0x2A, 0x40, 0xFF}
		}
		if b.Contains(v.Cursor) {
			hover = bt
			edge = colSelect
			vector.FillRect(screen, float32(b.Min.X), float32(b.Min.Y), float32(b.W()), float32(b.H()), withAlpha(colSelect, 0.08), false)
		}
		vector.StrokeRect(screen, float32(b.Min.X)+0.5, float32(b.Min.Y)+0.5, float32(b.W())-1, float32(b.H())-1, 1, edge, false)
		drawLeft(screen, bt.Key, r.smallFace, b.Min.X+4, b.Min.Y+8, colSelect)
		drawCentered(screen, bt.Label, r.smallFace, b.Min.X+b.W()/2, b.Min.Y+24, col)
		if bt.Kind == ActBuild || bt.Kind == ActTrain {
			c := colCrystal
			if !bt.Enabled {
				c = colDim
			}
			drawCentered(screen, costText(faction.Unit(bt.Type)), r.smallFace, b.Min.X+b.W()/2, b.Min.Y+38, c)
		}
		if bt.Kind == ActResearch {
			c := colCrystal
			if !bt.Enabled {
				c = colDim
			}
			if bt.Progress > 0 {
				drawCentered(screen, fmt.Sprintf("%.0f%%", bt.Progress*100), r.smallFace, b.Min.X+b.W()/2, b.Min.Y+38, c)
			} else {
				drawCentered(screen, costText(faction.UnitDef{Crystals: bt.Crystals, Flux: bt.Flux}), r.smallFace, b.Min.X+b.W()/2, b.Min.Y+38, c)
			}
		}
		if bt.Kind == ActMorph && (bt.Enabled || bt.Crystals+bt.Flux > 0) {
			c := colCrystal
			if !bt.Enabled {
				c = colDim
			}
			drawCentered(screen, costText(faction.UnitDef{Crystals: bt.Crystals, Flux: bt.Flux}), r.smallFace, b.Min.X+b.W()/2, b.Min.Y+38, c)
		}
		if bt.Cooldown > 0 {
			drawCentered(screen, fmt.Sprintf("%.0fs", math.Ceil(bt.Cooldown)), r.smallFace, b.Min.X+b.W()/2, b.Min.Y+38, colDim)
		}
	}
	if hover != nil {
		r.drawTooltip(screen, *hover, card)
	}
}

func (r *Renderer) drawTooltip(screen *ebiten.Image, bt Button, card geom.Rect) {
	var lines []string
	var cols []color.Color
	add := func(s string, c color.Color) { lines, cols = append(lines, s), append(cols, c) }
	switch bt.Kind {
	case ActBuild, ActTrain:
		d := faction.Unit(bt.Type)
		add(fmt.Sprintf("%s  [%s]", strings.ToUpper(d.Name), bt.Key), colText)
		cost := fmt.Sprintf("%d crystals", d.Crystals)
		if d.Flux > 0 {
			cost += fmt.Sprintf(" · %d flux", d.Flux)
		}
		if d.Supply > 0 {
			cost += fmt.Sprintf(" · %g supply", d.Supply)
		}
		add(fmt.Sprintf("%s · %gs", cost, d.BuildTime), colCrystal)
		add(d.Role, colDim)
		if bt.Reason != "" {
			add(bt.Reason, colBad)
		}
	case ActAttack:
		add("ATTACK-MOVE  [A]", colText)
		add("Click a point: move there, fighting on the way", colDim)
	case ActStop:
		add("STOP  [S]", colText)
	case ActHold:
		add("HOLD POSITION  [H]", colText)
		add("Fire at enemies in range, never move", colDim)
	case ActShift:
		if bt.Mixed {
			add("GEAR SHIFT  [SHIFT]", colText)
			add("Switch every selected unit type's mode", colDim)
			break
		}
		d := faction.Unit(bt.Type)
		if d.Shift.Cast() {
			add(fmt.Sprintf("%s: %s  [SHIFT]", strings.ToUpper(d.Name), strings.ToUpper(d.Shift.Name)), colText)
		} else {
			add(fmt.Sprintf("%s: %s / %s  [SHIFT]", strings.ToUpper(d.Name), strings.ToUpper(d.Shift.Base), strings.ToUpper(d.Shift.Name)), colText)
		}
		add(d.Shift.Desc, colDim)
		if d.Shift.Time > 0 {
			add(fmt.Sprintf("Takes %gs to switch", d.Shift.Time), colDim)
		}
		if bt.Cooldown > 0 {
			add(fmt.Sprintf("Ready in %.0fs", math.Ceil(bt.Cooldown)), colBad)
		}
	case ActMorph:
		d := faction.Unit(bt.Type)
		add(fmt.Sprintf("RE-MORPH INTO %s  [%s]", strings.ToUpper(d.Name), bt.Key), colText)
		cost := fmt.Sprintf("%d crystals", bt.Crystals)
		if bt.Flux > 0 {
			cost += fmt.Sprintf(" · %d flux", bt.Flux)
		}
		add(fmt.Sprintf("%s · %gs", cost, d.BuildTime/2), colCrystal)
		add("One selected unit cocoons and hatches as a "+d.Name, colDim)
		add("Pays the difference in value; cheaper forms are free", colDim)
		if bt.Reason != "" {
			add(bt.Reason, colBad)
		}
	case ActFuse:
		add(fmt.Sprintf("FUSE SEERS  [%s]", bt.Key), colText)
		add("Combine two nearby Seers into a Radiant", colDim)
		add("Requires a completed Resonator", colDim)
		add("Radiant keeps the pair's current HP and shields", colDim)
		if bt.Reason != "" {
			add(bt.Reason, colBad)
		}
	case ActResearch:
		add(fmt.Sprintf("RESEARCH WARP  [%s]", bt.Key), colText)
		add(fmt.Sprintf("%d crystals · %d flux · %ds", bt.Crystals, bt.Flux, faction.WarpResearchTime), colCrystal)
		add("Phasebound units warp in beside an Anchor", colDim)
		if bt.Progress > 0 {
			add(fmt.Sprintf("Progress: %d%%", int(bt.Progress*100)), colDim)
		}
		if bt.Reason != "" {
			add(bt.Reason, colBad)
		}
	case ActCancel:
		add("CANCEL  [ESC]", colText)
		add("Cancel construction or the last queued unit", colDim)
		add(fmt.Sprintf("Refunds %d%%", int(sim.CancelRefund*100)), colDim)
	}
	const lh = 18.0
	h := 14 + lh*float64(len(lines))
	box := geom.Rect{Min: geom.V(card.Min.X, card.Min.Y-h-6), Max: geom.V(card.Max.X, card.Min.Y-6)}
	drawBox(screen, box, colPanelEdge)
	for i, l := range lines {
		drawLeft(screen, l, r.smallFace, box.Min.X+10, box.Min.Y+16+lh*float64(i), cols[i])
	}
}

func (r *Renderer) drawResourceBar(screen *ebiten.Image, v *MatchView, box geom.Rect) {
	drawBox(screen, box, colPanelEdge)
	pl := v.World.Players[v.Player]
	y := box.Min.Y + box.H()/2
	x := box.Min.X + 14
	vector.FillRect(screen, float32(x), float32(y-4), 8, 8, colCrystal, false)
	drawLeft(screen, fmt.Sprintf("CRYSTALS %d", pl.Crystals), r.smallFace, x+14, y, colText)
	x += 140
	vector.FillCircle(screen, float32(x+4), float32(y), 4.5, colFlux, true)
	drawLeft(screen, fmt.Sprintf("FLUX %d", pl.Flux), r.smallFace, x+14, y, colText)
	x += 108
	col := color.Color(colText)
	if pl.Supply >= pl.SupplyCap && pl.SupplyCap < faction.MaxSupply {
		col = colBad
	}
	drawLeft(screen, fmt.Sprintf("SUPPLY %g/%g", pl.Supply, pl.SupplyCap), r.smallFace, x, y, col)
}

func (r *Renderer) drawGameOver(screen *ebiten.Image, v *MatchView) {
	w, h := float64(r.w), float64(r.h)
	vector.FillRect(screen, 0, 0, float32(w), float32(h), color.NRGBA{0x04, 0x05, 0x0A, 0xB0}, false)
	world := v.World
	title, col := "DEFEAT", color.RGBA(colEnemy)
	switch {
	case world.Draw:
		title, col = "DRAW", colText
	case world.Winner == v.Player:
		title, col = "VICTORY", PlayerColor(v.World, v.Player, v.Player)
	}
	drawCentered(screen, title, r.titleFace, w/2, h*0.4, col)
	secs := int(world.Tick / sim.TicksPerSecond)
	drawCentered(screen, fmt.Sprintf("MATCH TIME %02d:%02d", secs/60, secs%60), r.hudFace, w/2, h*0.4+80, colText)
	drawCentered(screen, "ENTER  ·  RETURN TO TITLE", r.hudFace, w/2, h*0.4+112, colDim)
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

// alertShow is how long (updates) an attack alert pulses on the minimap.
const alertShow = 5 * 60

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
	for _, res := range v.World.Resources {
		px, py := mm.Min.X+res.Pos.X*sx, mm.Min.Y+res.Pos.Y*sy
		vector.FillRect(screen, float32(px-1), float32(py-1), 2, 2, scaleRGB(resourceColor(res.Kind), 0.8), false)
	}
	if !v.RevealAll {
		for id, g := range v.World.Seen[v.Player] {
			if u := v.World.Unit(id); u != nil && v.World.VisibleTo(v.Player, u) {
				continue
			}
			n := float64(faction.Unit(g.Type).Size)
			px, py := mm.Min.X+float64(g.TileX)*sx, mm.Min.Y+float64(g.TileY)*sy
			vector.FillRect(screen, float32(px), float32(py), float32(n*sx), float32(n*sy), scaleRGB(colEnemy, 0.5), false)
		}
	}
	for _, u := range v.World.Units {
		if !unitVisible(v, u) {
			continue
		}
		col := PlayerColor(v.World, u.Owner, v.Player)
		if isSelected(v, u.ID) {
			col = colSelect
		}
		if u.Def.Building {
			n := float64(u.Def.Size)
			px, py := mm.Min.X+float64(u.TileX)*sx, mm.Min.Y+float64(u.TileY)*sy
			vector.FillRect(screen, float32(px), float32(py), float32(n*sx), float32(n*sy), col, false)
			continue
		}
		px, py := mm.Min.X+u.Pos.X*sx, mm.Min.Y+u.Pos.Y*sy
		vector.FillRect(screen, float32(px-1.5), float32(py-1.5), 3, 3, col, false)
	}
	if v.HasAlert && v.AlertAge < alertShow {
		// Shrinking red rings mark the attack on the minimap.
		px, py := mm.Min.X+v.Alert.X*sx, mm.Min.Y+v.Alert.Y*sy
		ph := float64(v.AlertAge%40) / 40
		for i := range 2 {
			rad := 4 + 14*(1-math.Mod(ph+float64(i)*0.5, 1))
			vector.StrokeCircle(screen, float32(px), float32(py), float32(rad), 1.5, colEnemy, true)
		}
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
	{"RIGHT CLICK", "move · attack · gather · build/repair"},
	{"RIGHT CLICK (BUILDING)", "set rally point"},
	{"Q W E R T", "command card: build · train"},
	{"A, THEN CLICK", "attack-move"},
	{"S / H", "stop / hold position"},
	{"ESC", "cancel placement · production · deselect"},
	{"CTRL or ALT + 0-9", "set control group"},
	{"0-9 (TWICE)", "select group (center camera)"},
	{"ARROWS / MIDDLE-DRAG", "scroll camera"},
	{"WHEEL", "zoom"},
	{"MINIMAP", "left: jump · right: move"},
	{"SPACE", "jump to the last attack alert"},
	{"SHIFT", "faction ability: switch modes · cast"},
	{"F2 / F3  ·  F4", "debug spawn  ·  toggle fog"},
}

func (r *Renderer) drawHelp(screen *ebiten.Image) {
	const lh = 19.0
	bw, bh := 540.0, 44+lh*float64(len(helpLines))
	box := geom.Rect{Min: geom.V(float64(r.w)-bw-hudMargin, 60), Max: geom.V(float64(r.w)-hudMargin, 60+bh)}
	drawBox(screen, box, colPanelEdge)
	drawLeft(screen, "CONTROLS", r.hudFace, box.Min.X+14, box.Min.Y+18, colText)
	for i, l := range helpLines {
		y := box.Min.Y + 44 + lh*float64(i)
		drawLeft(screen, l[0], r.smallFace, box.Min.X+14, y, colSelect)
		drawLeft(screen, l[1], r.smallFace, box.Min.X+210, y, colDim)
	}
}
