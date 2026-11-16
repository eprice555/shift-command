package engine

import (
	"fmt"
	"math"
	"runtime"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
	"github.com/eprice555/shift-command/internal/sim"
	"github.com/eprice555/shift-command/internal/ui"
)

const (
	doubleClickUpdates = 20 // max gap for double-click / double-tap
	keyScrollSpeed     = 900.0 / UpdateTPS
	edgeScrollMargin   = 6
	clickSlop          = 5 // px a press may move and still count as a click
)

type ping struct {
	p      geom.Vec
	attack bool
}

// Match is the in-game scene: it turns input into sim commands and steps
// the sim at its fixed rate.
type Match struct {
	world   *sim.World
	player  sim.PlayerID
	clock   *FixedStep
	pending []sim.Command
	cam     ui.Camera
	updates int

	sel    []sim.UnitID // ascending
	groups [10][]sim.UnitID

	cursor      geom.Vec
	hover       sim.UnitID
	dragging    bool
	dragFrom    geom.Vec
	panFrom     geom.Vec
	minimapDrag bool
	targeting   bool

	lastClickAt   int
	lastClickType faction.UnitType
	lastGroup     int
	lastGroupAt   int

	follow     bool // demo: keep the camera on the selection
	revealAll  bool
	showHelp   bool
	msg        string
	msgAt      int
	spawnCycle int
	cursorMode ebiten.CursorShapeType

	// Presentation output since the last draw.
	events []sim.Event
	pings  []ping
}

// NewMatch starts the M1 sandbox with the camera on the player's start.
func NewMatch() *Match {
	w := sim.NewSandbox()
	m := &Match{
		world:     w,
		clock:     NewFixedStep(sim.TicksPerSecond, UpdateTPS),
		lastGroup: -1,
		showHelp:  true,
		cam: ui.Camera{
			Zoom: 1, ViewW: ScreenWidth, ViewH: ScreenHeight, ViewBottom: ui.HUDBottom,
			MapW: float64(w.Map.W), MapH: float64(w.Map.H),
		},
	}
	m.cam.CenterOn(w.Map.Starts[m.player].Add(geom.V(6, -6)))
	return m
}

func (m *Match) say(s string) { m.msg, m.msgAt = s, m.updates }

func (m *Match) issue(c sim.Command) {
	c.Player = m.player
	m.pending = append(m.pending, c)
}

// own returns the selected units the player can command.
func (m *Match) own() []sim.UnitID {
	var out []sim.UnitID
	for _, id := range m.sel {
		if u := m.world.Unit(id); u != nil && u.Owner == m.player {
			out = append(out, id)
		}
	}
	return out
}

func (m *Match) setSelection(ids []sim.UnitID) {
	m.sel = append(m.sel[:0], ids...)
	slices.Sort(m.sel)
	m.sel = slices.Compact(m.sel)
}

func (m *Match) addSelection(ids []sim.UnitID) {
	// Mixing in units only makes sense if the current selection is ours.
	if len(m.sel) > 0 && len(m.own()) == 0 {
		m.sel = m.sel[:0]
	}
	m.setSelection(append(slices.Clone(m.sel), ids...))
}

func (m *Match) prune() {
	alive := func(id sim.UnitID) bool { return m.world.Unit(id) == nil }
	m.sel = slices.DeleteFunc(m.sel, alive)
	for i := range m.groups {
		m.groups[i] = slices.DeleteFunc(m.groups[i], alive)
	}
}

func modifierHeld() bool {
	return ebiten.IsKeyPressed(ebiten.KeyTab) || ebiten.IsKeyPressed(ebiten.KeyF)
}

func groupAssignHeld() bool {
	return ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyAlt)
}

// Update handles one frame of input and steps the sim as needed.
func (m *Match) Update() {
	m.updates++
	cx, cy := ebiten.CursorPosition()
	m.cursor = geom.V(float64(cx), float64(cy))
	m.prune()

	m.updateCamera()
	m.updateMouse()
	m.updateKeys()

	m.hover = 0
	if !ui.OverHUD(ScreenWidth, ScreenHeight, m.cursor.X, m.cursor.Y) {
		if u := m.pick(m.cursor); u != nil {
			m.hover = u.ID
		}
	}
	shape := ebiten.CursorShapeDefault
	if m.targeting {
		shape = ebiten.CursorShapeCrosshair
	}
	if shape != m.cursorMode {
		ebiten.SetCursorShape(shape)
		m.cursorMode = shape
	}

	if m.follow && len(m.sel) > 0 {
		m.cam.CenterOn(m.centroid(m.sel))
	}

	for range m.clock.Advance() {
		m.world.Step(m.pending)
		m.pending = m.pending[:0]
		if len(m.events) < 1024 {
			m.events = append(m.events, m.world.Events...)
		}
	}
}

func (m *Match) updateCamera() {
	dx, dy := 0.0, 0.0
	if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
		dx -= keyScrollSpeed
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
		dx += keyScrollSpeed
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowUp) {
		dy -= keyScrollSpeed
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowDown) {
		dy += keyScrollSpeed
	}
	// Edge scrolling fights with the browser/itch.io page around the
	// canvas, so it is native-only.
	if runtime.GOOS != "js" && ebiten.IsFocused() && !m.dragging {
		x, y := m.cursor.X, m.cursor.Y
		if x >= 0 && y >= 0 && x < ScreenWidth && y < ScreenHeight {
			switch {
			case x < edgeScrollMargin:
				dx -= keyScrollSpeed
			case x >= ScreenWidth-edgeScrollMargin:
				dx += keyScrollSpeed
			}
			switch {
			case y < edgeScrollMargin:
				dy -= keyScrollSpeed
			case y >= ScreenHeight-edgeScrollMargin:
				dy += keyScrollSpeed
			}
		}
	}
	if dx != 0 || dy != 0 {
		m.cam.Pan(dx, dy)
	}

	if _, wy := ebiten.Wheel(); wy != 0 {
		m.cam.ZoomAt(m.cursor.X, m.cursor.Y, math.Pow(1.12, wy))
	}

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonMiddle) {
		m.panFrom = m.cursor
	} else if ebiten.IsMouseButtonPressed(ebiten.MouseButtonMiddle) {
		d := m.cursor.Sub(m.panFrom)
		m.cam.Pan(-d.X, -d.Y)
		m.panFrom = m.cursor
	}
}

func (m *Match) minimapRect() geom.Rect { return ui.MinimapRect(ScreenWidth, ScreenHeight) }

func (m *Match) minimapPoint() (geom.Vec, bool) {
	mm := m.minimapRect()
	if !mm.Contains(m.cursor) {
		return geom.Vec{}, false
	}
	return ui.MinimapToWorld(mm, m.cursor.X, m.cursor.Y, m.cam.MapW, m.cam.MapH), true
}

// pick returns the visible unit under screen point s, preferring the closest.
func (m *Match) pick(s geom.Vec) *sim.Unit {
	p := m.cam.ToWorld(s.X, s.Y)
	var best *sim.Unit
	bestD := math.Inf(1)
	for _, u := range m.world.Units {
		if !m.revealAll && !m.world.VisibleTo(m.player, u) {
			continue
		}
		pos, _ := ui.DrawPos(u, m.clock.Alpha())
		d := pos.Dist(p)
		if d <= u.Def.Radius+0.25 && d < bestD {
			best, bestD = u, d
		}
	}
	return best
}

func (m *Match) updateMouse() {
	overHUD := ui.OverHUD(ScreenWidth, ScreenHeight, m.cursor.X, m.cursor.Y)

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		switch mp, onMap := m.minimapPoint(); {
		case m.targeting:
			target := m.cam.ToWorld(m.cursor.X, m.cursor.Y)
			var enemy *sim.Unit
			if onMap {
				target = mp
			} else if u := m.pick(m.cursor); u != nil && u.Owner != m.player {
				enemy = u
			}
			if !onMap && overHUD {
				break
			}
			m.commandAt(target, enemy, true)
			if !modifierHeld() {
				m.targeting = false
			}
		case onMap:
			m.minimapDrag = true
		case overHUD:
		default:
			m.dragging = true
			m.dragFrom = m.cursor
		}
	}
	if m.minimapDrag {
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			mm := m.minimapRect()
			m.cam.CenterOn(ui.MinimapToWorld(mm, m.cursor.X, m.cursor.Y, m.cam.MapW, m.cam.MapH))
		} else {
			m.minimapDrag = false
		}
	}
	if m.dragging && !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		m.dragging = false
		m.finishSelect()
	}

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		if m.targeting {
			m.targeting = false
			return
		}
		if mp, onMap := m.minimapPoint(); onMap {
			m.commandAt(mp, nil, false)
		} else if !overHUD {
			var enemy *sim.Unit
			if u := m.pick(m.cursor); u != nil && u.Owner != m.player {
				enemy = u
			}
			m.commandAt(m.cam.ToWorld(m.cursor.X, m.cursor.Y), enemy, false)
		}
	}
}

// commandAt issues the smart command (or attack-move) for the selection.
func (m *Match) commandAt(p geom.Vec, enemy *sim.Unit, attackMove bool) {
	units := m.own()
	if len(units) == 0 {
		return
	}
	c := sim.Command{Units: units, Point: p, Queue: modifierHeld()}
	switch {
	case enemy != nil:
		c.Kind, c.Target = sim.CmdAttack, enemy.ID
		p = enemy.Pos
	case attackMove:
		c.Kind = sim.CmdAttackMove
	default:
		c.Kind = sim.CmdMove
	}
	m.issue(c)
	m.pings = append(m.pings, ping{p, c.Kind != sim.CmdMove})
}

func (m *Match) finishSelect() {
	add := modifierHeld()
	box := geom.RectFrom(m.dragFrom, m.cursor)
	if box.W() < clickSlop && box.H() < clickSlop {
		u := m.pick(m.cursor)
		switch {
		case u == nil:
			if !add {
				m.sel = m.sel[:0]
			}
		case u.Owner != m.player:
			if !add {
				m.setSelection([]sim.UnitID{u.ID})
			}
		case m.updates-m.lastClickAt <= doubleClickUpdates && u.Type == m.lastClickType:
			ids := m.onScreen(func(o *sim.Unit) bool { return o.Type == u.Type })
			if add {
				m.addSelection(ids)
			} else {
				m.setSelection(ids)
			}
		case add:
			if i, ok := slices.BinarySearch(m.sel, u.ID); ok {
				m.sel = slices.Delete(m.sel, i, i+1)
			} else {
				m.addSelection([]sim.UnitID{u.ID})
			}
		default:
			m.setSelection([]sim.UnitID{u.ID})
		}
		if u != nil && u.Owner == m.player {
			m.lastClickAt, m.lastClickType = m.updates, u.Type
		}
		return
	}

	var ids []sim.UnitID
	for _, u := range m.world.Units {
		if u.Owner != m.player {
			continue
		}
		pos, _ := ui.DrawPos(u, m.clock.Alpha())
		sx, sy := m.cam.ToScreen(pos)
		r := u.Def.Radius * m.cam.Scale()
		if sx+r >= box.Min.X && sx-r <= box.Max.X && sy+r >= box.Min.Y && sy-r <= box.Max.Y {
			ids = append(ids, u.ID)
		}
	}
	if add {
		m.addSelection(ids)
	} else {
		m.setSelection(ids)
	}
}

// onScreen returns the player's units in view that match keep.
func (m *Match) onScreen(keep func(*sim.Unit) bool) []sim.UnitID {
	vis := m.cam.Visible()
	var ids []sim.UnitID
	for _, u := range m.world.Units {
		if u.Owner == m.player && vis.Contains(u.Pos) && keep(u) {
			ids = append(ids, u.ID)
		}
	}
	return ids
}

var digitKeys = [10]ebiten.Key{
	ebiten.KeyDigit0, ebiten.KeyDigit1, ebiten.KeyDigit2, ebiten.KeyDigit3, ebiten.KeyDigit4,
	ebiten.KeyDigit5, ebiten.KeyDigit6, ebiten.KeyDigit7, ebiten.KeyDigit8, ebiten.KeyDigit9,
}

func (m *Match) updateKeys() {
	pressed := inpututil.IsKeyJustPressed
	if pressed(ebiten.KeyShiftLeft) || pressed(ebiten.KeyShiftRight) {
		m.say("SHIFT: The Forge's gear-shift modes arrive in M2")
	}
	if pressed(ebiten.KeyEscape) {
		if m.targeting {
			m.targeting = false
		} else {
			m.sel = m.sel[:0]
		}
	}
	if own := m.own(); len(own) > 0 {
		switch {
		case pressed(ebiten.KeyA):
			m.targeting = true
		case pressed(ebiten.KeyS):
			m.issue(sim.Command{Kind: sim.CmdStop, Units: own})
			m.targeting = false
		case pressed(ebiten.KeyH):
			m.issue(sim.Command{Kind: sim.CmdHold, Units: own})
			m.targeting = false
		}
	}
	for d, k := range digitKeys {
		if !pressed(k) {
			continue
		}
		if groupAssignHeld() {
			if own := m.own(); len(own) > 0 {
				m.groups[d] = own
				m.say(fmt.Sprintf("Group %d set: %d units", d, len(own)))
			}
			continue
		}
		g := m.groups[d]
		if len(g) == 0 {
			continue
		}
		if modifierHeld() {
			m.addSelection(g)
		} else {
			m.setSelection(g)
		}
		if m.lastGroup == d && m.updates-m.lastGroupAt <= doubleClickUpdates {
			m.cam.CenterOn(m.centroid(g))
		}
		m.lastGroup, m.lastGroupAt = d, m.updates
	}

	if pressed(ebiten.KeyF1) {
		m.showHelp = !m.showHelp
	}
	if pressed(ebiten.KeyF2) || pressed(ebiten.KeyF3) {
		types := [...]faction.UnitType{faction.Trooper, faction.Outrider, faction.Strider}
		counts := [...]int{6, 4, 3}
		i := m.spawnCycle % len(types)
		m.spawnCycle++
		owner, who := m.player, "Spawned"
		if pressed(ebiten.KeyF3) {
			owner, who = 1-m.player, "Spawned enemy"
		}
		p := m.cam.ToWorld(m.cursor.X, m.cursor.Y)
		m.pending = append(m.pending, sim.Command{
			Kind: sim.CmdSpawn, Player: owner, Point: p, SpawnType: types[i], SpawnCount: counts[i],
		})
		m.say(fmt.Sprintf("%s %d %ss", who, counts[i], faction.Unit(types[i]).Name))
	}
	if pressed(ebiten.KeyF4) {
		m.revealAll = !m.revealAll
	}
}

func (m *Match) centroid(ids []sim.UnitID) geom.Vec {
	var sum geom.Vec
	n := 0
	for _, id := range ids {
		if u := m.world.Unit(id); u != nil {
			sum = sum.Add(u.Pos)
			n++
		}
	}
	if n == 0 {
		return m.cam.Center()
	}
	return sum.Scale(1 / float64(n))
}

// View builds the renderer's view of this frame.
func (m *Match) View() *ui.MatchView {
	v := &ui.MatchView{
		World: m.world, Player: m.player, Alpha: m.clock.Alpha(), Cam: &m.cam,
		Selected: m.sel, Hover: m.hover,
		Dragging: m.dragging, DragFrom: m.dragFrom, Cursor: m.cursor,
		Targeting: m.targeting, RevealAll: m.revealAll, Focused: ebiten.IsFocused(),
		ShowHelp: m.showHelp, Message: m.msg, MessageAge: m.updates - m.msgAt,
	}
	for i, g := range m.groups {
		v.Groups[i] = len(g)
	}
	return v
}

// takeFX hands accumulated sim events and pings to the renderer.
func (m *Match) takeFX(r *ui.Renderer) {
	r.AddEvents(m.events)
	m.events = m.events[:0]
	for _, p := range m.pings {
		r.AddPing(p.p, p.attack)
	}
	m.pings = m.pings[:0]
}
