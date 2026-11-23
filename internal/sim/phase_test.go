package sim

import (
	"testing"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
)

func phaseMatch() *World {
	w := NewSkirmishFor([NumPlayers]faction.ID{faction.Phasebound, faction.Forge})
	w.Players[0].Crystals = 2000
	w.Players[0].Flux = 2000
	return w
}

func spire(w *World) *Unit { return find(w, 0, faction.Spire)[0] }

// phaseSite finds a free site for t near the Spire that passes the power
// check only if powered is true.
func phaseSite(t *testing.T, w *World, typ faction.UnitType, near geom.Vec, minR int) (int, int) {
	t.Helper()
	w.Fog[0].Reveal(near, 30)
	def := faction.Unit(typ)
	cx, cy := int(near.X), int(near.Y)
	for r := minR; r < 30; r++ {
		for y := cy - r; y <= cy+r; y++ {
			for x := cx - r; x <= cx+r; x++ {
				if w.siteNotice(0, def, x, y) == NoticeNone {
					return x, y
				}
			}
		}
	}
	t.Fatal("no site")
	return 0, 0
}

func summon(t *testing.T, w *World, d *Unit, typ faction.UnitType, x, y int) *Unit {
	t.Helper()
	run(w, 1, Command{Kind: CmdBuild, Player: 0, Units: []UnitID{d.ID}, Type: typ, TX: x, TY: y})
	for range secs(20) {
		for _, e := range w.Events {
			if e.Kind == EvDeath && e.Type == faction.Drifter {
				t.Fatal("Drifter died summoning")
			}
		}
		if bs := find(w, 0, typ); len(bs) > 0 {
			return bs[0]
		}
		w.Step(nil)
	}
	t.Fatalf("%v never summoned", typ)
	return nil
}

func TestPhaseboundStart(t *testing.T) {
	w := NewSkirmishFor([NumPlayers]faction.ID{faction.Phasebound, faction.Forge})
	s := spire(w)
	if !s.Complete || s.Shield != s.Def.Shield || s.Shield <= 0 {
		t.Fatalf("spire complete %v shield %v", s.Complete, s.Shield)
	}
	ds := find(w, 0, faction.Drifter)
	if len(ds) != StartWorkers || ds[0].Shield != ds[0].Def.Shield {
		t.Fatalf("%d drifters", len(ds))
	}
	if pl := w.Players[0]; pl.SupplyCap != 10 || pl.Supply != StartWorkers {
		t.Errorf("supply %v/%v", pl.Supply, pl.SupplyCap)
	}
	run(w, secs(30))
	if w.Players[0].Crystals <= StartCrystals {
		t.Error("drifters don't mine")
	}
}

func TestShieldsAbsorbThenRecharge(t *testing.T) {
	w := openWorld(30, 30)
	wd := w.Spawn(0, faction.Warden, geom.V(10, 10))
	tr := w.Spawn(1, faction.Trooper, geom.V(13, 10))
	run(w, secs(2))
	if wd.HP != wd.Def.HP || wd.Shield >= wd.Def.Shield {
		t.Fatalf("hits should drain shield first: hp %v shield %v", wd.HP, wd.Shield)
	}
	tr.HP = 0
	run(w, 1)
	s := wd.Shield
	run(w, secs(faction.ShieldDelay-1.5))
	if wd.Shield != s || w.ShieldsRecharging(wd) {
		t.Fatalf("shield recharged too soon: %v -> %v", s, wd.Shield)
	}
	run(w, secs(3))
	if wd.Shield <= s || !w.ShieldsRecharging(wd) {
		t.Fatalf("shield not recharging: %v -> %v", s, wd.Shield)
	}
	run(w, secs(60))
	if wd.Shield != wd.Def.Shield {
		t.Errorf("shield %v of %v", wd.Shield, wd.Def.Shield)
	}

	wd.Shield = 5
	if left := w.absorb(wd, 8); left != 3 || wd.Shield != 0 {
		t.Errorf("overflow: left %v shield %v", left, wd.Shield)
	}
	if left := w.absorb(wd, -4); left != -4 {
		t.Errorf("healing passes through shields: %v", left)
	}
}

func TestDrifterSummonsAndLeaves(t *testing.T) {
	w := phaseMatch()
	run(w, secs(5))
	d := find(w, 0, faction.Drifter)[0]
	x, y := phaseSite(t, w, faction.Anchor, spire(w).Pos, 5)
	a := summon(t, w, d, faction.Anchor, x, y)
	if d.HP <= 0 || d.consumed {
		t.Fatal("Drifter was consumed")
	}
	if a.Complete || a.Shield != 0 {
		t.Fatalf("anchor complete %v shield %v", a.Complete, a.Shield)
	}
	run(w, secs(2))
	if len(d.Orders) == 0 || d.Orders[0].Kind != OrderGather {
		t.Errorf("Drifter should go back to mining, orders %v", d.Orders)
	}
	run(w, secs(faction.Unit(faction.Anchor).BuildTime))
	if !a.Complete || a.Shield != a.Def.Shield {
		t.Fatalf("anchor complete %v shield %v", a.Complete, a.Shield)
	}
	if w.Players[0].SupplyCap != 18 {
		t.Errorf("supply cap %v", w.Players[0].SupplyCap)
	}
}

func TestWarpedCancelKeepsDrifters(t *testing.T) {
	w := phaseMatch()
	d := find(w, 0, faction.Drifter)[0]
	x, y := phaseSite(t, w, faction.Anchor, spire(w).Pos, 5)
	a := summon(t, w, d, faction.Anchor, x, y)
	n := len(find(w, 0, faction.Drifter))
	run(w, 1, Command{Kind: CmdCancel, Player: 0, Units: []UnitID{a.ID}})
	if len(find(w, 0, faction.Anchor)) != 0 {
		t.Fatal("anchor not cancelled")
	}
	if got := len(find(w, 0, faction.Drifter)); got != n {
		t.Errorf("cancel changed Drifters %d -> %d", n, got)
	}
}

func TestPowerFields(t *testing.T) {
	w := phaseMatch()
	s := spire(w)
	w.Fog[0].Reveal(s.Pos, 40)
	gate := faction.Unit(faction.Gate)
	unpowered := false
	for y := int(s.Pos.Y) - 12; y < int(s.Pos.Y)+12 && !unpowered; y++ {
		for x := int(s.Pos.X) - 12; x < int(s.Pos.X)+12; x++ {
			if w.siteNotice(0, gate, x, y) == NoticeNeedPower {
				unpowered = true
				break
			}
			if w.siteNotice(0, gate, x, y) == NoticeNone {
				t.Fatalf("gate placeable at %d,%d with no Anchor", x, y)
			}
		}
	}
	if !unpowered {
		t.Fatal("no unpowered site found")
	}
	ax, ay := phaseSite(t, w, faction.Anchor, s.Pos, 5)
	anchor := w.PlaceBuilding(0, faction.Anchor, ax, ay, true)
	if !w.PoweredAt(0, anchor.Pos) || w.PoweredAt(1, anchor.Pos) {
		t.Fatal("anchor field ownership")
	}
	far := anchor.Pos.Add(geom.V(anchor.Def.PowerRadius+3, 0))
	if w.PoweredAt(0, far) {
		t.Fatal("power reaches too far")
	}
	gx, gy := phaseSite(t, w, faction.Gate, anchor.Pos, 2)
	if n := w.CanPlace(0, faction.Gate, gx, gy); n != NoticeNone {
		t.Fatalf("powered site: %v", n)
	}
	g := w.PlaceBuilding(0, faction.Gate, gx, gy, true)
	run(w, 1, Command{Kind: CmdTrain, Player: 0, Units: []UnitID{g.ID}, Type: faction.Warden})
	run(w, secs(3))
	if g.Progress == 0 || w.Unpowered(g) {
		t.Fatalf("powered gate not training: progress %d", g.Progress)
	}
	anchor.HP = 0
	run(w, 1)
	p := g.Progress
	run(w, secs(5))
	if !w.Unpowered(g) || g.Progress != p {
		t.Errorf("unpowered gate kept training: %d -> %d", p, g.Progress)
	}
}

func TestConduitRestoresShields(t *testing.T) {
	w := openWorld(30, 30)
	w.Spawn(0, faction.Conduit, geom.V(10, 10))
	wd := w.Spawn(0, faction.Warden, geom.V(12, 10))
	hurt := w.Spawn(0, faction.Glint, geom.V(10, 12))
	wd.Shield = 10
	wd.hurtTick = w.Tick + 1
	hurt.HP = 20
	run(w, secs(2))
	if wd.Shield < 25 || w.ShieldsRecharging(wd) {
		t.Errorf("conduit restored shield to %v", wd.Shield)
	}
	if hurt.HP != 20 {
		t.Errorf("conduit healed HP: %v", hurt.HP)
	}
}
