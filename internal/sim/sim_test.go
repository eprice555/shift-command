package sim

import (
	"fmt"
	"testing"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
	"github.com/eprice555/shift-command/internal/world"
)

// openWorld returns a world on an empty w×h map.
func openWorld(w, h int) *World {
	return NewWorld(&world.Map{Grid: world.NewGrid(w, h), Name: "test"})
}

func ids(us []*Unit) []UnitID {
	out := make([]UnitID, len(us))
	for i, u := range us {
		out[i] = u.ID
	}
	return out
}

func run(w *World, ticks int, cmds ...Command) {
	w.Step(cmds)
	for range ticks - 1 {
		w.Step(nil)
	}
}

func assertPassable(t *testing.T, w *World) {
	t.Helper()
	for _, u := range w.Units {
		if !u.Def.Building && w.Map.Blocked(world.TileOf(u.Pos)) {
			t.Fatalf("tick %d: unit %d inside wall at %v", w.Tick, u.ID, u.Pos)
		}
	}
}

func TestSingleUnitMovesAndArrives(t *testing.T) {
	w := openWorld(30, 30)
	u := w.Spawn(0, faction.Trooper, geom.V(3, 3))
	goal := geom.V(20.5, 12.5)
	run(w, 1, Command{Kind: CmdMove, Units: []UnitID{u.ID}, Point: goal})
	if !u.Moving {
		t.Fatal("unit should be moving after a move command")
	}
	// ~19.6 tiles at 2.8 tiles/s ≈ 7s = 140 ticks.
	run(w, 160)
	if d := u.Pos.Dist(goal); d > 0.1 {
		t.Fatalf("unit at %v, %.2f from goal", u.Pos, d)
	}
	if !u.Idle() {
		t.Fatalf("unit should be idle after arriving, orders=%v", u.Orders)
	}
}

func TestMoveAroundWall(t *testing.T) {
	w := openWorld(30, 30)
	for y := range 25 {
		w.Map.SetBlocked(15, y, true)
	}
	u := w.Spawn(0, faction.Strider, geom.V(5, 5))
	goal := geom.V(25, 5)
	run(w, 1, Command{Kind: CmdMove, Units: []UnitID{u.ID}, Point: goal})
	for range 40 * TicksPerSecond {
		w.Step(nil)
		assertPassable(t, w)
		if u.Idle() {
			break
		}
	}
	if d := u.Pos.Dist(goal); d > 0.2 {
		t.Fatalf("strider at %v, %.2f from goal", u.Pos, d)
	}
}

func TestGroupMoveAllArrive(t *testing.T) {
	w := NewSandbox()
	var mine []*Unit
	for _, u := range w.Units {
		if u.Owner == 0 {
			mine = append(mine, u)
		}
	}
	// Out of the main through the ramp to the open middle.
	goal := geom.V(30, 46)
	run(w, 1, Command{Kind: CmdMove, Units: ids(mine), Point: goal})
	for range 60 * TicksPerSecond {
		w.Step(nil)
		assertPassable(t, w)
		idle := true
		for _, u := range mine {
			idle = idle && u.Idle()
		}
		if idle {
			break
		}
	}
	for _, u := range mine {
		if !u.Idle() {
			t.Errorf("unit %d (%s) still has orders at %v", u.ID, u.Def.Name, u.Pos)
		}
		if d := u.Pos.Dist(goal); d > 5 {
			t.Errorf("unit %d (%s) stopped %.1f from goal at %v", u.ID, u.Def.Name, d, u.Pos)
		}
	}
	t.Logf("group of %d arrived by tick %d", len(mine), w.Tick)
}

func TestAttackKillsTarget(t *testing.T) {
	w := openWorld(30, 30)
	a := w.Spawn(0, faction.Strider, geom.V(5, 5))
	b := w.Spawn(1, faction.Trooper, geom.V(13, 5)) // in sight, out of range
	// Make the trooper passive so the duel is one-sided.
	run(w, 1, Command{Kind: CmdHold, Player: 1, Units: []UnitID{b.ID}})
	b.Def.Weapon.Damage = 0

	shots := 0
	cmd := Command{Kind: CmdAttack, Units: []UnitID{a.ID}, Target: b.ID}
	for i := 0; i < 30*TicksPerSecond && w.Unit(b.ID) != nil; i++ {
		if i == 0 {
			w.Step([]Command{cmd})
		} else {
			w.Step(nil)
		}
		for _, e := range w.Events {
			if e.Kind == EvShot && e.Owner == 0 {
				shots++
			}
		}
	}
	if w.Unit(b.ID) != nil {
		t.Fatalf("target survived with %.1f HP", b.HP)
	}
	w.Step(nil) // the attacker notices the kill on its next think
	dmg := faction.Unit(faction.Strider).Weapon.DamageTo(faction.Unit(faction.Trooper))
	want := 0
	for hp := faction.Unit(faction.Trooper).HP; hp > 0; hp -= dmg {
		want++
	}
	if shots != want {
		t.Errorf("took %d shots, want %d (%.1f dmg each)", shots, want, dmg)
	}
	if !a.Idle() {
		t.Errorf("attacker should go idle after the kill, orders=%v", a.Orders)
	}
}

func TestIdleUnitsAutoAcquire(t *testing.T) {
	w := openWorld(40, 20)
	a := w.Spawn(0, faction.Trooper, geom.V(5, 10))
	b := w.Spawn(1, faction.Strider, geom.V(12, 10)) // inside sight, outside range
	run(w, 10)
	if a.Target() != b.ID || b.Target() != a.ID {
		t.Fatalf("targets: a→%d b→%d", a.Target(), b.Target())
	}
	run(w, 30*TicksPerSecond)
	if w.Count(0)+w.Count(1) != 1 {
		t.Fatalf("expected the fight to finish, got %d+%d survivors", w.Count(0), w.Count(1))
	}
}

func TestHoldDoesNotChase(t *testing.T) {
	w := openWorld(40, 20)
	a := w.Spawn(0, faction.Trooper, geom.V(5, 10))
	b := w.Spawn(1, faction.Trooper, geom.V(12, 10))
	start := a.Pos
	run(w, 2*TicksPerSecond,
		Command{Kind: CmdHold, Units: []UnitID{a.ID}},
		Command{Kind: CmdHold, Player: 1, Units: []UnitID{b.ID}})
	if !a.Hold || a.Pos.Dist(start) > 0.01 {
		t.Fatalf("holding unit moved from %v to %v", start, a.Pos)
	}
	if a.Target() != 0 {
		t.Fatalf("holding unit targeted something out of range")
	}
}

func TestMoveIgnoresEnemiesAndStopClears(t *testing.T) {
	w := openWorld(40, 20)
	a := w.Spawn(0, faction.Outrider, geom.V(3, 10))
	w.Spawn(1, faction.Trooper, geom.V(20, 13))
	run(w, 20, Command{Kind: CmdMove, Units: []UnitID{a.ID}, Point: geom.V(37, 10)})
	if a.Target() != 0 {
		t.Fatal("moving unit should not acquire targets")
	}
	run(w, 1, Command{Kind: CmdStop, Units: []UnitID{a.ID}})
	if !a.Idle() || a.Hold {
		t.Fatalf("stop should clear orders: %v hold=%v", a.Orders, a.Hold)
	}
}

func TestAttackMoveEngages(t *testing.T) {
	w := openWorld(40, 20)
	a := w.SpawnSquad(0, faction.Strider, 3, geom.V(4, 10))
	e := w.Spawn(1, faction.Trooper, geom.V(22, 10))
	run(w, 1, Command{Kind: CmdAttackMove, Units: ids(a), Point: geom.V(36, 10)})
	for range 30 * TicksPerSecond {
		w.Step(nil)
		if w.Unit(e.ID) == nil {
			break
		}
	}
	if w.Unit(e.ID) != nil {
		t.Fatal("attack-moving group never killed the enemy in its path")
	}
	run(w, 30*TicksPerSecond)
	for _, u := range a {
		if d := u.Pos.Dist(geom.V(36, 10)); d > 4 {
			t.Errorf("unit %d didn't resume to goal: %.1f away", u.ID, d)
		}
	}
}

func TestCommandsRespectOwnershipAndVisibility(t *testing.T) {
	w := openWorld(60, 20)
	a := w.Spawn(0, faction.Trooper, geom.V(5, 10))
	b := w.Spawn(1, faction.Trooper, geom.V(50, 10))
	// Player 0 can't order an enemy unit or attack an unseen one.
	run(w, 1,
		Command{Kind: CmdMove, Player: 0, Units: []UnitID{b.ID}, Point: geom.V(5, 5)},
		Command{Kind: CmdAttack, Player: 0, Units: []UnitID{a.ID}, Target: b.ID},
	)
	if !b.Idle() || !a.Idle() {
		t.Fatalf("invalid commands were accepted: a=%v b=%v", a.Orders, b.Orders)
	}
}

func TestFogFollowsUnits(t *testing.T) {
	w := openWorld(40, 20)
	u := w.Spawn(0, faction.Outrider, geom.V(3, 10))
	far := geom.V(35, 10)
	if w.Fog[0].VisibleAt(far) {
		t.Fatal("far point visible at start")
	}
	run(w, 1, Command{Kind: CmdMove, Units: []UnitID{u.ID}, Point: far})
	run(w, 15*TicksPerSecond)
	if !w.Fog[0].VisibleAt(far) {
		t.Fatal("arrival point not visible")
	}
	if got := w.Fog[0].At(world.TileOf(geom.V(3, 10))); got != world.Explored {
		t.Fatalf("start tile visibility = %v, want Explored", got)
	}
}

func TestSandboxBattleDeterministic(t *testing.T) {
	sim := func() string {
		w := NewSandbox()
		var mine []UnitID
		for _, u := range w.Units {
			if u.Owner == 0 {
				mine = append(mine, u.ID)
			}
		}
		run(w, 1, Command{Kind: CmdAttackMove, Units: mine, Point: geom.V(46, 34)})
		run(w, 90*TicksPerSecond)
		assertPassable(t, w)
		s := fmt.Sprint(w.Tick)
		for _, u := range w.Units {
			s += fmt.Sprintf("|%d %.9f %.9f %.3f", u.ID, u.Pos.X, u.Pos.Y, u.HP)
		}
		return s
	}
	a, b := sim(), sim()
	if a != b {
		t.Fatal("two identical runs diverged")
	}
}

func TestSpawnCommand(t *testing.T) {
	w := openWorld(20, 20)
	run(w, 1, Command{Kind: CmdSpawn, Player: 1, Point: geom.V(10, 10), Type: faction.Strider, Count: 4})
	if w.Count(1) != 4 {
		t.Fatalf("spawned %d, want 4", w.Count(1))
	}
	for _, u := range w.Units {
		for _, o := range w.Units {
			if u != o && u.Pos.Dist(o.Pos) < u.Def.Radius+o.Def.Radius-0.01 {
				t.Fatalf("spawned units overlap: %v %v", u.Pos, o.Pos)
			}
		}
	}
}
