package engine

import (
	"fmt"
	"math"
	"runtime"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/eprice555/shift-command/internal/ai"
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
	alertCooldown      = 15 * UpdateTPS
)

type ping struct {
	p      geom.Vec
	attack bool
}

// Match is the in-game scene: it turns input into sim commands and steps
// the sim at its fixed rate.
type Match struct {
	sandbox bool
	ai      *ai.AI // the opponent; nil in the sandbox
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

	card      []ui.Button
	placing   bool
	placeType faction.UnitType
	placeX    int
	placeY    int
	done      bool // the player left a finished match

	follow     bool // demo: keep the camera on the selection
	revealAll  bool
	showHelp   bool
	msg        string
	msgAt      int
	spawnCycle int
	cursorMode ebiten.CursorShapeType

	// Under-attack alert: where the enemy last fired on us off-screen.
	alert    geom.Vec
	alertAt  int
	hasAlert bool

	// Presentation output since the last draw.
	events []sim.Event
	pings  []ping
}

// NewMatch starts a skirmish (or the M1 combat sandbox) with the camera on
// the player's start.
func NewMatch(sandbox bool, f faction.ID) *Match {
	// The AI plays the Forge until it learns the other factions (M5).
	w := sim.NewSkirmishFor([sim.NumPlayers]faction.ID{f, faction.Forge})
	if sandbox {
		w = sim.NewSandbox()
	}
	m := &Match{
		sandbox:   sandbox,
		world:     w,
		clock:     NewFixedStep(sim.TicksPerSecond, UpdateTPS),
		lastGroup: -1,
		showHelp:  true,
		cam: ui.Camera{
			Zoom: 1, ViewW: ScreenWidth, ViewH: ScreenHeight, ViewBottom: ui.HUDBottom,
			MapW: float64(w.Map.W), MapH: float64(w.Map.H),
		},
	}
	if !sandbox {
		m.ai = ai.New(w, 1-m.player)
	}
	m.cam.CenterOn(w.Map.Starts[m.player].Add(geom.V(2, -3)))
	return m
}

// fastForward runs n sim ticks with the AI playing and the human idle.
func (m *Match) fastForward(n int) {
	for range n {
		if m.world.Over {
			return
		}
		var cmds []sim.Command
		if m.ai != nil {
			cmds = m.ai.Think(m.world)
		}
		m.world.Step(cmds)
	}
}

// Done reports whether the player has left a finished match.
func (m *Match) Done() bool { return m.done }

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
	m.card = ui.CommandCard(m.world, m.player, m.sel)

	m.updateCamera()
	if m.world.Over {
		m.placing, m.targeting = false, false
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter) {
			m.done = true
		}
	} else {
		m.updateMouse()
		m.updateKeys()
	}
	if m.placing {
		p := m.cam.ToWorld(m.cursor.X, m.cursor.Y)
		m.placeX, m.placeY = ui.PlacementTile(m.world, m.placeType, p)
	}

	m.hover = 0
	if !ui.OverHUD(ScreenWidth, ScreenHeight, m.cursor.X, m.cursor.Y) {
		if u := m.pick(m.cursor); u != nil {
			m.hover = u.ID
		}
	}
	shape := ebiten.CursorShapeDefault
	if m.targeting || m.placing {
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
		if m.ai != nil {
			m.pending = append(m.pending, m.ai.Think(m.world)...)
		}
		m.world.Step(m.pending)
		m.pending = m.pending[:0]
		if len(m.events) < 1024 {
			m.events = append(m.events, m.world.Events...)
		}
		for _, ev := range m.world.Events {
			if ev.Kind == sim.EvNotice && ev.Owner == m.player {
				msg := ev.Notice.String()
				switch ev.Notice {
				case sim.NoticeRequires:
					msg += " " + faction.Unit(ev.Type).Name
				case sim.NoticeSupply:
					msg += ": build a " + faction.Unit(ev.Type).Name
				}
				m.say(msg)
			}
			if ev.Kind == sim.EvShot && ev.Owner != m.player && m.ai != nil {
				m.underAttack(ev.To)
			}
		}
	}
}

// underAttack raises an alert when the enemy hits something of ours that
// is off-screen, at most once per alertCooldown.
func (m *Match) underAttack(p geom.Vec) {
	if m.cam.Visible().Contains(p) || m.hasAlert && m.updates-m.alertAt < alertCooldown {
		return
	}
	m.alert, m.alertAt, m.hasAlert = p, m.updates, true
	m.say("Under attack!  (SPACE to jump there)")
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
		if u.Def.Building {
			// Units in front of a building win the click.
			if u.Rect().Contains(p) && u.Def.Radius < bestD {
				best, bestD = u, u.Def.Radius
			}
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

	card := ui.CardRect(ScreenWidth, ScreenHeight)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		b, onButton := ui.ButtonAt(m.card, card, m.cursor)
		switch mp, onMap := m.minimapPoint(); {
		case onButton:
			m.press(b)
		case m.placing:
			if !overHUD {
				m.place()
			}
		case m.clickQueue():
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
		if m.targeting || m.placing {
			m.targeting, m.placing = false, false
			return
		}
		if mp, onMap := m.minimapPoint(); onMap {
			m.smartCommand(mp, nil)
		} else if !overHUD {
			m.smartCommand(m.cam.ToWorld(m.cursor.X, m.cursor.Y), m.pick(m.cursor))
		}
	}
}

// place orders the selected workers to build at the placement ghost.
func (m *Match) place() {
	if n := m.world.CanPlace(m.player, m.placeType, m.placeX, m.placeY); n != sim.NoticeNone {
		msg := n.String()
		if req, ok := m.world.Missing(m.player, m.placeType); ok && n == sim.NoticeRequires {
			msg += " " + faction.Unit(req).Name
		}
		m.say(msg)
		return
	}
	m.issue(sim.Command{Kind: sim.CmdBuild, Units: m.own(), Type: m.placeType, TX: m.placeX, TY: m.placeY, Queue: modifierHeld()})
	d := faction.Unit(m.placeType)
	n := float64(d.Size) / 2
	m.pings = append(m.pings, ping{geom.V(float64(m.placeX)+n, float64(m.placeY)+n), false})
	if !modifierHeld() {
		m.placing = false
	}
}

// selectedBuilding returns the single selected own building, if any.
func (m *Match) selectedBuilding() *sim.Unit {
	if len(m.sel) != 1 {
		return nil
	}
	u := m.world.Unit(m.sel[0])
	if u == nil || u.Owner != m.player || !u.Def.Building {
		return nil
	}
	return u
}

// clickQueue cancels the clicked production queue entry, if any.
func (m *Match) clickQueue() bool {
	b := m.selectedBuilding()
	if b == nil {
		return false
	}
	panel := ui.PanelRect(ScreenWidth, ScreenHeight)
	for i := range b.Queue {
		if ui.QueueSlotRect(panel, i).Contains(m.cursor) {
			m.issue(sim.Command{Kind: sim.CmdCancel, Units: []sim.UnitID{b.ID}, Index: i})
			return true
		}
	}
	return false
}

// press runs a command-card button.
func (m *Match) press(b ui.Button) {
	own := m.own()
	m.targeting = false
	switch b.Kind {
	case ui.ActBuild:
		if !b.Enabled {
			m.say(b.Reason)
			return
		}
		m.placing, m.placeType = true, b.Type
	case ui.ActTrain:
		m.issue(sim.Command{Kind: sim.CmdTrain, Units: own, Type: b.Type})
	case ui.ActAttack:
		m.placing, m.targeting = false, true
	case ui.ActStop:
		m.issue(sim.Command{Kind: sim.CmdStop, Units: own})
	case ui.ActHold:
		m.issue(sim.Command{Kind: sim.CmdHold, Units: own})
	case ui.ActCancel:
		m.issue(sim.Command{Kind: sim.CmdCancel, Units: own, Index: -1})
	case ui.ActShift:
		m.gearShift()
	case ui.ActMorph:
		if !b.Enabled {
			m.say(b.Reason)
			return
		}
		m.issue(sim.Command{Kind: sim.CmdMorph, Units: own, Type: b.Type})
	}
}

// gearShift toggles the SHIFT modes of the selection.
func (m *Match) gearShift() {
	var ids []sim.UnitID
	for _, id := range m.own() {
		if faction.Unit(m.world.Unit(id).Type).Shift != nil {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		if len(m.sel) > 0 {
			m.say("Nothing selected can SHIFT")
		}
		return
	}
	m.issue(sim.Command{Kind: sim.CmdShift, Units: ids, Index: -1})
}

// resourceNear returns the resource node under world point p, with a
// little slop so single-tile crystals are easy to click.
func (m *Match) resourceNear(p geom.Vec) *sim.Resource {
	var best *sim.Resource
	bestD := 0.35
	for _, r := range m.world.Resources {
		rect := r.Rect()
		dx := max(rect.Min.X-p.X, 0, p.X-rect.Max.X)
		dy := max(rect.Min.Y-p.Y, 0, p.Y-rect.Max.Y)
		if d := math.Hypot(dx, dy); d <= bestD {
			best, bestD = r, d
		}
	}
	return best
}

// smartCommand is a right-click at world point p on unit u (may be nil):
// attack enemies, gather resources, assist own buildings, otherwise move.
// Selected buildings set their rally point instead.
func (m *Match) smartCommand(p geom.Vec, u *sim.Unit) {
	own := m.own()
	if len(own) == 0 {
		return
	}
	if b := m.world.Unit(own[0]); b.Def.Building && b.Mode == 1 {
		// Packed buildings fly where they're sent.
		m.issue(sim.Command{Kind: sim.CmdMove, Units: own, Point: p, Queue: modifierHeld()})
		m.pings = append(m.pings, ping{p, false})
		return
	}
	res := m.resourceNear(p)
	// Your finished extractor covers its vent: clicking it means the vent.
	if u != nil && u.Owner == m.player && u.Complete {
		if v := m.world.VentOf(u); v != nil {
			u, res = nil, v
		}
	}
	rally := sim.Command{Kind: sim.CmdRally, Units: own, Point: p}
	if u == nil && res != nil {
		rally.Res = res.ID
	}
	if u != nil {
		rally.Point = u.Pos
	}
	m.issue(rally)

	switch {
	case u != nil && u.Owner != m.player:
		m.commandAt(p, u, false)
	case u != nil:
		m.issue(sim.Command{Kind: sim.CmdAssist, Units: own, Target: u.ID, Queue: modifierHeld()})
		m.pings = append(m.pings, ping{u.Pos, false})
	case res != nil:
		m.issue(sim.Command{Kind: sim.CmdGather, Units: own, Res: res.ID, Queue: modifierHeld()})
		m.pings = append(m.pings, ping{res.Pos, false})
	default:
		m.commandAt(p, nil, false)
	}
}

// commandAt issues a move, attack, or attack-move for the selection.
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
	// Like classic RTSes, a box keeps only its most army-like units: units
	// over Spawn (and cocoons), and those over buildings.
	tier := func(id sim.UnitID) int {
		switch d := m.world.Unit(id).Def; {
		case d.Building:
			return 2
		case d.Morphs:
			return 1
		}
		return 0
	}
	if len(ids) > 0 {
		best := tier(slices.MinFunc(ids, func(a, b sim.UnitID) int { return tier(a) - tier(b) }))
		ids = slices.DeleteFunc(ids, func(id sim.UnitID) bool { return tier(id) != best })
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

var cardKey = map[string]ebiten.Key{
	"Q": ebiten.KeyQ, "W": ebiten.KeyW, "E": ebiten.KeyE, "R": ebiten.KeyR, "T": ebiten.KeyT,
	"A": ebiten.KeyA, "S": ebiten.KeyS, "H": ebiten.KeyH,
}

var digitKeys = [10]ebiten.Key{
	ebiten.KeyDigit0, ebiten.KeyDigit1, ebiten.KeyDigit2, ebiten.KeyDigit3, ebiten.KeyDigit4,
	ebiten.KeyDigit5, ebiten.KeyDigit6, ebiten.KeyDigit7, ebiten.KeyDigit8, ebiten.KeyDigit9,
}

func (m *Match) updateKeys() {
	pressed := inpututil.IsKeyJustPressed
	if pressed(ebiten.KeyShiftLeft) || pressed(ebiten.KeyShiftRight) {
		m.gearShift()
	}
	if pressed(ebiten.KeySpace) && m.hasAlert {
		m.cam.CenterOn(m.alert)
		m.follow = false
	}
	if pressed(ebiten.KeyEscape) {
		switch {
		case m.placing:
			m.placing = false
		case m.targeting:
			m.targeting = false
		case slices.ContainsFunc(m.card, func(b ui.Button) bool { return b.Kind == ui.ActCancel }):
			m.press(ui.Button{Kind: ui.ActCancel})
		default:
			m.sel = m.sel[:0]
		}
	}
	if !groupAssignHeld() {
		for _, b := range m.card {
			if k, ok := cardKey[b.Key]; ok && pressed(k) {
				m.press(b)
			}
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
		owner, who := m.player, "Spawned"
		if pressed(ebiten.KeyF3) {
			owner, who = 1-m.player, "Spawned enemy"
		}
		cycle := debugSquads[m.world.Players[owner].Faction]
		if len(cycle) == 0 {
			cycle = debugSquads[faction.Forge]
		}
		s := cycle[m.spawnCycle%len(cycle)]
		m.spawnCycle++
		p := m.cam.ToWorld(m.cursor.X, m.cursor.Y)
		m.pending = append(m.pending, sim.Command{
			Kind: sim.CmdSpawn, Player: owner, Point: p, Type: s.t, Count: s.n,
		})
		m.say(fmt.Sprintf("%s %d %ss", who, s.n, faction.Unit(s.t).Name))
	}
	if pressed(ebiten.KeyF4) {
		m.revealAll = !m.revealAll
	}
}

// debugSquads are the squads F2/F3 cycle through for each faction.
var debugSquads = map[faction.ID][]struct {
	t faction.UnitType
	n int
}{
	faction.Forge: {{faction.Trooper, 6}, {faction.Outrider, 4}, {faction.Strider, 3}},
	faction.Brood: {{faction.Biter, 8}, {faction.Leaper, 4}, {faction.Spitter, 4}, {faction.Broodmother, 2}, {faction.Behemoth, 2}},
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
		Sandbox: m.sandbox, World: m.world, Player: m.player, Alpha: m.clock.Alpha(), Cam: &m.cam,
		Selected: m.sel, Hover: m.hover,
		Dragging: m.dragging, DragFrom: m.dragFrom, Cursor: m.cursor,
		Targeting: m.targeting, RevealAll: m.revealAll, Focused: ebiten.IsFocused(),
		ShowHelp: m.showHelp, Message: m.msg, MessageAge: m.updates - m.msgAt,
		HasAlert: m.hasAlert, Alert: m.alert, AlertAge: m.updates - m.alertAt,
		Card: m.card, Placing: m.placing, PlaceType: m.placeType, PlaceX: m.placeX, PlaceY: m.placeY,
	}
	if m.placing {
		v.PlaceNotice = m.world.CanPlace(m.player, m.placeType, m.placeX, m.placeY)
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
