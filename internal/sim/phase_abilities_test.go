package sim

import (
	"testing"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
)

func TestPhaseCastHidesAndProtectsUnit(t *testing.T) {
	w := openWorld(30, 20)
	drifter := w.Spawn(0, faction.Drifter, geom.V(5, 5))
	enemy := w.Spawn(1, faction.Trooper, geom.V(8, 5))
	enemy.Orders = []Order{{Kind: OrderAttack, Target: drifter.ID}}
	hp, shield := drifter.HP, drifter.Shield

	run(w, 1, Command{Kind: CmdShift, Player: 0, Units: []UnitID{drifter.ID}, Index: -1})
	if !drifter.Phased() || !drifter.Hidden() || drifter.CastLeft() <= 0 {
		t.Fatalf("phase state: phased %v hidden %v cooldown %.1f", drifter.Phased(), drifter.Hidden(), drifter.CastLeft())
	}
	run(w, secs(1))
	if drifter.HP != hp || drifter.Shield != shield {
		t.Fatalf("phased unit took damage: HP %.1f shield %.1f", drifter.HP, drifter.Shield)
	}
	if enemy.target != 0 {
		t.Fatalf("enemy acquired phased target %d", enemy.target)
	}
	run(w, secs(1))
	if drifter.Phased() || drifter.CastLeft() <= 0 {
		t.Fatalf("phase/cooldown after 2s: phased %v cooldown %.1f", drifter.Phased(), drifter.CastLeft())
	}
}

func TestPhasedUnitCannotAttack(t *testing.T) {
	w := openWorld(30, 20)
	warden := w.Spawn(0, faction.Warden, geom.V(5, 5))
	enemy := w.Spawn(1, faction.Trooper, geom.V(8, 5))
	hp := enemy.HP
	run(w, 1,
		Command{Kind: CmdAttack, Player: 0, Units: []UnitID{warden.ID}, Target: enemy.ID},
		Command{Kind: CmdShift, Player: 0, Units: []UnitID{warden.ID}, Index: -1},
	)
	run(w, secs(1.5))
	if enemy.HP != hp {
		t.Fatalf("phased Warden dealt damage: %v -> %v", hp, enemy.HP)
	}
	run(w, secs(1))
	if enemy.HP >= hp {
		t.Fatal("Warden did not resume attacking after Phase ended")
	}
}

func TestPhaseVariants(t *testing.T) {
	t.Run("warden dash", func(t *testing.T) {
		w := openWorld(30, 20)
		u := w.Spawn(0, faction.Warden, geom.V(5, 5))
		w.Spawn(1, faction.Trooper, geom.V(8, 5))
		run(w, 1, Command{Kind: CmdShift, Player: 0, Units: []UnitID{u.ID}, Index: -1})
		if !u.Phased() || !u.Dashing() {
			t.Fatalf("warden phase %v dash %v", u.Phased(), u.Dashing())
		}
		run(w, secs(1))
		if u.Dashing() || u.Pos.X <= 5.5 {
			t.Fatalf("warden did not dash: pos %v, dashing %v", u.Pos, u.Dashing())
		}
	})

	t.Run("seer blink", func(t *testing.T) {
		w := openWorld(30, 20)
		u := w.Spawn(0, faction.Seer, geom.V(5, 5))
		u.Facing = 1.5707963267948966
		run(w, 1, Command{Kind: CmdShift, Player: 0, Units: []UnitID{u.ID}, Index: -1})
		if !u.Phased() || u.Pos.Y < 8.5 {
			t.Fatalf("seer did not blink and phase: pos %v, phased %v", u.Pos, u.Phased())
		}
	})

	t.Run("conduit field", func(t *testing.T) {
		w := openWorld(30, 20)
		c := w.Spawn(0, faction.Conduit, geom.V(10, 10))
		ally := w.Spawn(0, faction.Warden, geom.V(12, 10))
		far := w.Spawn(0, faction.Glint, geom.V(20, 10))
		enemy := w.Spawn(1, faction.Warden, geom.V(10, 12))
		run(w, 1, Command{Kind: CmdShift, Player: 0, Units: []UnitID{c.ID}, Index: -1})
		if !c.Phased() || !ally.Phased() || far.Phased() || enemy.Phased() {
			t.Fatalf("phase field: conduit %v ally %v far %v enemy %v", c.Phased(), ally.Phased(), far.Phased(), enemy.Phased())
		}
	})
}

func TestGlintLongPhaseCrossesTerrain(t *testing.T) {
	w := openWorld(20, 10)
	for y := range w.Map.H {
		w.Map.SetBlocked(8, y, true)
	}
	u := w.Spawn(0, faction.Glint, geom.V(2.5, 5.5))
	goal := geom.V(14.5, 5.5)
	run(w, 1, Command{Kind: CmdMove, Player: 0, Units: []UnitID{u.ID}, Point: goal})
	run(w, 1, Command{Kind: CmdShift, Player: 0, Units: []UnitID{u.ID}, Index: -1})
	run(w, secs(3))
	if u.Pos.X <= 8 || !u.Phased() {
		t.Fatalf("Glint did not pass the cliff while phased: pos %v, phased %v", u.Pos, u.Phased())
	}
	run(w, secs(2))
	if u.Phased() || u.Pos.X <= 8 {
		t.Fatalf("Glint failed to complete Long Phase: pos %v, phased %v", u.Pos, u.Phased())
	}
}

func TestSeersFuseIntoRadiant(t *testing.T) {
	w := phaseMatch()
	a := w.Spawn(0, faction.Seer, geom.V(20, 20))
	b := w.Spawn(0, faction.Seer, geom.V(21, 20))
	w.Spawn(0, faction.Resonator, geom.V(30, 30))
	a.HP, b.HP, a.Shield, b.Shield = 30, 40, 20, 30
	w.recount()
	before := w.Players[0].Supply

	run(w, 1, Command{Kind: CmdFuse, Player: 0, Units: []UnitID{a.ID, b.ID}})
	if a.Type != faction.Radiant || a.HP != 70 || a.Shield != 50 || w.Unit(b.ID) != nil {
		t.Fatalf("fusion: survivor %+v, second seer %v", a, w.Unit(b.ID))
	}
	if w.Players[0].Supply != before {
		t.Errorf("fusion changed supply from %v to %v", before, w.Players[0].Supply)
	}
	if a.Def.Weapon.Splash <= 0 || a.Def.Shift == nil {
		t.Errorf("Radiant missing splash or Phase: %+v", a.Def)
	}
}

func TestWarpResearchWarpsUnitsAtAnchors(t *testing.T) {
	w := phaseMatch()
	for _, u := range find(w, 0, faction.Drifter) {
		u.clearOrders()
	}
	s := spire(w)
	ax, ay := phaseSite(t, w, faction.Anchor, s.Pos, 5)
	anchor := w.PlaceBuilding(0, faction.Anchor, ax, ay, true)
	gx, gy := phaseSite(t, w, faction.Gate, anchor.Pos, 2)
	gate := w.PlaceBuilding(0, faction.Gate, gx, gy, true)
	rx, ry := phaseSite(t, w, faction.Resonator, anchor.Pos, 4)
	resonator := w.PlaceBuilding(0, faction.Resonator, rx, ry, true)

	run(w, 1, Command{Kind: CmdResearchWarp, Player: 0, Units: []UnitID{resonator.ID}})
	if !resonator.WarpResearching() || w.Players[0].WarpResearched {
		t.Fatal("Warp research did not start")
	}
	run(w, secs(faction.WarpResearchTime/2))
	progress := resonator.WarpResearchFrac()
	anchor.Complete = false
	run(w, 1)
	if resonator.WarpResearchFrac() != progress {
		t.Fatal("Warp research continued without power")
	}
	anchor.Complete = true
	run(w, secs(faction.WarpResearchTime/2))
	if !w.Players[0].WarpResearched || resonator.WarpResearching() {
		t.Fatal("Warp research did not complete")
	}
	run(w, 1, Command{Kind: CmdTrain, Player: 0, Units: []UnitID{gate.ID}, Type: faction.Warden})
	run(w, secs(faction.Unit(faction.Warden).BuildTime))
	var warped *Unit
	for _, u := range find(w, 0, faction.Warden) {
		if u.WarpingIn() {
			warped = u
			break
		}
	}
	if warped == nil || !warped.Hidden() || warped.Pos.Dist(anchor.Pos) > 3 {
		t.Fatalf("trained unit did not warp beside Anchor: %+v", warped)
	}
	run(w, secs(warpInTime))
	if warped.WarpingIn() || warped.Hidden() {
		t.Fatal("unit remained hidden after warp-in")
	}
}
