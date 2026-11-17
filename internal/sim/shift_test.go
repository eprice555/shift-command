package sim

import (
	"slices"
	"testing"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
)

func shiftCmd(p PlayerID, index int, us ...*Unit) Command {
	return Command{Kind: CmdShift, Player: p, Units: ids(us), Index: index}
}

// shifted spawns a unit and fully switches it into its SHIFT mode.
func shifted(w *World, p PlayerID, t faction.UnitType, pos geom.Vec) *Unit {
	u := w.Spawn(p, t, pos)
	run(w, secs(faction.Unit(t).Shift.Time)+2, shiftCmd(p, 1, u))
	return u
}

func TestSiegeTransitionAndImmobile(t *testing.T) {
	w := openWorld(30, 30)
	b := w.Spawn(0, faction.Bastion, geom.V(5, 5))
	run(w, 1, shiftCmd(0, -1, b))
	if b.Mode != 1 || !b.Shifting() || b.Def.Weapon.Range != 13 {
		t.Fatalf("mode %d shifting %v range %.0f", b.Mode, b.Shifting(), b.Def.Weapon.Range)
	}
	if f := b.ShiftFrac(); f <= 0 || f >= 1 {
		t.Errorf("mid-transition ShiftFrac %.2f", f)
	}
	run(w, secs(2))
	if b.Shifting() || b.ShiftFrac() != 1 {
		t.Fatalf("still shifting after 2s (frac %.2f)", b.ShiftFrac())
	}
	start := b.Pos
	run(w, secs(2), Command{Kind: CmdMove, Units: []UnitID{b.ID}, Point: geom.V(20, 20)})
	if b.Pos != start || len(b.Orders) != 0 {
		t.Fatalf("sieged Bastion moved to %v (orders %v)", b.Pos, b.Orders)
	}
	run(w, secs(2)+1, shiftCmd(0, -1, b))
	if b.Mode != 0 || b.Def.Speed == 0 || b.Def.Weapon.Range != 7 {
		t.Fatalf("unsiege: mode %d speed %.1f range %.0f", b.Mode, b.Def.Speed, b.Def.Weapon.Range)
	}
	run(w, secs(2), Command{Kind: CmdMove, Units: []UnitID{b.ID}, Point: geom.V(20, 20)})
	if b.Pos == start {
		t.Fatal("unsieged Bastion didn't move")
	}
}

func TestArtilleryMinRangeAndSplash(t *testing.T) {
	w := openWorld(40, 20)
	b := shifted(w, 0, faction.Bastion, geom.V(5, 10))
	// A friendly spotter stands among the targets: the Bastion's sight is
	// shorter than its range, and splash must not hurt it.
	spot := w.Spawn(0, faction.Rigger, geom.V(16.5, 11.2))
	near := w.Spawn(1, faction.Rigger, geom.V(6.9, 10))
	cluster := w.SpawnSquad(1, faction.Rigger, 3, geom.V(16.5, 10))
	run(w, secs(4))
	if near.HP != near.Def.HP {
		t.Errorf("unit inside minimum range took damage: %.0f", near.HP)
	}
	if spot.HP != spot.Def.HP {
		t.Errorf("splash hurt a friendly unit: %.0f", spot.HP)
	}
	hurt := 0
	for _, c := range cluster {
		if c.HP < c.Def.HP {
			hurt++
		}
	}
	if hurt < 2 {
		t.Errorf("splash hit %d of the cluster, want ≥ 2", hurt)
	}
	if b.Mode != 1 {
		t.Error("Bastion left siege mode")
	}
}

func TestOverdriveCostsHPAndWearsOff(t *testing.T) {
	w := openWorld(20, 20)
	tr := w.Spawn(0, faction.Trooper, geom.V(5, 5))
	base := tr.Def
	run(w, 1, shiftCmd(0, -1, tr))
	if tr.Mode != 1 || tr.HP != base.HP-10 || tr.Def.Speed <= base.Speed || tr.Def.Weapon.Cooldown >= base.Weapon.Cooldown {
		t.Fatalf("overdrive: mode %d hp %.0f speed %.1f cd %.2f", tr.Mode, tr.HP, tr.Def.Speed, tr.Def.Weapon.Cooldown)
	}
	// Pressing again pays again and refreshes the timer.
	run(w, secs(4), shiftCmd(0, -1, tr))
	if tr.HP != base.HP-20 || tr.ModeLeft() < 3.9 {
		t.Fatalf("re-trigger: hp %.0f left %.1fs", tr.HP, tr.ModeLeft())
	}
	run(w, secs(4)+2)
	if tr.Mode != 0 || tr.Def.Speed != base.Speed {
		t.Fatalf("overdrive didn't wear off: mode %d speed %.1f", tr.Mode, tr.Def.Speed)
	}
	tr.HP = 10
	if n := notices(w, 1, shiftCmd(0, -1, tr)); !slices.Contains(n, NoticeLowHP) || tr.Mode != 0 || tr.HP != 10 {
		t.Fatalf("low-HP overdrive: notices %v mode %d hp %.0f", n, tr.Mode, tr.HP)
	}
}

func TestBoostedOutriderIsUnarmed(t *testing.T) {
	w := openWorld(20, 20)
	o := w.Spawn(0, faction.Outrider, geom.V(5, 5))
	e := w.Spawn(1, faction.Generator, geom.V(8, 5))
	run(w, secs(2), shiftCmd(0, -1, o))
	if o.Def.Armed() || o.Def.Speed != 2*faction.Unit(faction.Outrider).Speed {
		t.Fatalf("boost: armed %v speed %.1f", o.Def.Armed(), o.Def.Speed)
	}
	if o.Target() != 0 || e.HP != e.Def.HP {
		t.Fatal("boosted Outrider attacked")
	}
}

func TestFieldPostHealsEveryone(t *testing.T) {
	w := openWorld(20, 20)
	m := shifted(w, 0, faction.Mender, geom.V(10, 10))
	ts := w.SpawnSquad(0, faction.Trooper, 3, geom.V(12, 10))
	far := w.Spawn(0, faction.Trooper, geom.V(18, 18))
	for _, u := range append(ts, far) {
		u.HP = 10
	}
	run(w, secs(2))
	for _, u := range ts {
		if u.HP < 20 {
			t.Errorf("trooper %d only healed to %.1f", u.ID, u.HP)
		}
	}
	if far.HP != 10 {
		t.Errorf("aura healed a unit out of range: %.1f", far.HP)
	}
	if m.Pos != geom.V(10, 10) {
		t.Errorf("Field Post moved to %v", m.Pos)
	}
}

func TestShiftTogglesPerType(t *testing.T) {
	w := openWorld(30, 30)
	b1 := shifted(w, 0, faction.Bastion, geom.V(5, 5))
	b2 := w.Spawn(0, faction.Bastion, geom.V(9, 5))
	s := shifted(w, 0, faction.Strider, geom.V(5, 9))
	g := w.Spawn(0, faction.Generator, geom.V(15, 15))
	run(w, 1, shiftCmd(0, -1, b1, b2, s, g))
	if b1.Mode != 1 || b2.Mode != 1 {
		t.Errorf("mixed Bastions should all siege: %d %d", b1.Mode, b2.Mode)
	}
	if s.Mode != 0 {
		t.Error("all-planted Striders should unplant")
	}
	if g.Mode != 0 {
		t.Error("Generator has no SHIFT mode")
	}
}

func TestBuildingPacksFliesAndLands(t *testing.T) {
	w := openWorld(40, 30)
	d := w.Spawn(0, faction.DrillYard, geom.V(6.5, 6.5))
	x0, y0 := d.TileX, d.TileY

	d.Queue = []faction.UnitType{faction.Trooper}
	if n := notices(w, 1, shiftCmd(0, -1, d)); !slices.Contains(n, NoticeBusy) || d.Mode != 0 {
		t.Fatalf("packing with a queue: notices %v mode %d", n, d.Mode)
	}
	d.Queue = nil

	run(w, secs(1.5)+1, shiftCmd(0, -1, d))
	if !d.Lifted() || w.Map.Blocked(x0, y0) || len(d.Def.Trains) != 0 {
		t.Fatalf("pack: lifted %v blocked %v trains %v", d.Lifted(), w.Map.Blocked(x0, y0), d.Def.Trains)
	}
	goal := geom.V(20.5, 6.5)
	run(w, secs(13), Command{Kind: CmdMove, Units: []UnitID{d.ID}, Point: goal})
	if d.Pos.Dist(goal) > 0.01 {
		t.Fatalf("flying building at %v, want %v", d.Pos, goal)
	}
	run(w, secs(2)+1, shiftCmd(0, -1, d))
	if d.Mode != 0 || d.Lifted() || d.Shifting() || !w.Map.Blocked(d.TileX, d.TileY) || len(d.Def.Trains) == 0 {
		t.Fatalf("land: mode %d lifted %v tiles (%d,%d)", d.Mode, d.Lifted(), d.TileX, d.TileY)
	}
	if d.Rect().Contains(geom.V(6.5, 6.5)) || w.Map.Blocked(x0+1, y0+1) {
		t.Error("old footprint still occupied")
	}

	// Can't land on a wall.
	run(w, secs(1.5)+1, shiftCmd(0, -1, d))
	for y := 0; y < 15; y++ {
		for x := 25; x < 40; x++ {
			w.Map.SetBlocked(x, y, true)
		}
	}
	run(w, secs(10), Command{Kind: CmdMove, Units: []UnitID{d.ID}, Point: geom.V(32, 6)})
	if n := notices(w, 1, shiftCmd(0, -1, d)); !slices.Contains(n, NoticeCantLand) || d.Mode != 1 || !d.Lifted() {
		t.Fatalf("landing on a wall: notices %v mode %d", n, d.Mode)
	}
}

func TestPackedHubIsNoDropOff(t *testing.T) {
	w := NewSkirmish()
	h := hub(w, 0)
	r := find(w, 0, faction.Rigger)[0]
	run(w, secs(2), shiftCmd(0, -1, h))
	if !h.Lifted() || w.nearestDropOff(r) != nil {
		t.Fatal("packed hub still accepts cargo")
	}
	if w.Players[0].SupplyCap == 0 {
		t.Error("packed hub stopped providing supply")
	}
}
