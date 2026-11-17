package sim

import (
	"math"
	"testing"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
	"github.com/eprice555/shift-command/internal/world"
)

func secs(s float64) int { return int(s * TicksPerSecond) }

func find(w *World, p PlayerID, t faction.UnitType) []*Unit {
	var out []*Unit
	for _, u := range w.Units {
		if u.Owner == p && u.Type == t {
			out = append(out, u)
		}
	}
	return out
}

func hub(w *World, p PlayerID) *Unit { return find(w, p, faction.CommandHub)[0] }

func notices(w *World, ticks int, cmds ...Command) []Notice {
	var out []Notice
	w.Step(cmds)
	for i := 0; ; i++ {
		for _, e := range w.Events {
			if e.Kind == EvNotice {
				out = append(out, e.Notice)
			}
		}
		if i >= ticks-1 {
			return out
		}
		w.Step(nil)
	}
}

func TestSkirmishStart(t *testing.T) {
	w := NewSkirmish()
	for p := range PlayerID(NumPlayers) {
		h := hub(w, p)
		if !h.Complete || h.HP != h.Def.HP {
			t.Errorf("p%d hub not complete: %+v", p, h)
		}
		rs := find(w, p, faction.Rigger)
		if len(rs) != StartWorkers {
			t.Fatalf("p%d has %d riggers", p, len(rs))
		}
		for _, r := range rs {
			if len(r.Orders) == 0 || r.Orders[0].Kind != OrderGather {
				t.Errorf("p%d rigger %d not gathering", p, r.ID)
			}
		}
		pl := w.Players[p]
		if pl.Crystals != StartCrystals || pl.Supply != StartWorkers || pl.SupplyCap != 10 {
			t.Errorf("p%d economy %+v", p, pl)
		}
		if w.Map.Passable(h.TileX, h.TileY) {
			t.Errorf("p%d hub footprint not blocked", p)
		}
	}
}

func TestGatheringIncome(t *testing.T) {
	w := NewSkirmish()
	run(w, secs(60))
	for p := range PlayerID(NumPlayers) {
		got := w.Players[p].Crystals - StartCrystals
		// 6 workers, ~5 crystals per ~6s trip each.
		if got < 220 || got > 400 {
			t.Errorf("p%d mined %d crystals in 60s", p, got)
		}
	}
	a, b := w.Players[0].Crystals, w.Players[1].Crystals
	if math.Abs(float64(a-b)) > 0.15*float64(a) {
		t.Errorf("mirrored income differs: %d vs %d", a, b)
	}
	assertPassable(t, w)
}

func TestTrainWorkerJoinsMining(t *testing.T) {
	w := NewSkirmish()
	h := hub(w, 0)
	run(w, 1, Command{Kind: CmdTrain, Player: 0, Units: []UnitID{h.ID}, Type: faction.Rigger})
	if len(h.Queue) != 1 || w.Players[0].Crystals != 0 || w.Players[0].Supply != StartWorkers+1 {
		t.Fatalf("queue %v, economy %+v", h.Queue, w.Players[0])
	}
	run(w, secs(12.5))
	rs := find(w, 0, faction.Rigger)
	if len(rs) != StartWorkers+1 || len(h.Queue) != 0 {
		t.Fatalf("%d riggers, queue %v", len(rs), h.Queue)
	}
	if o := rs[len(rs)-1].Orders; len(o) == 0 || o[0].Kind != OrderGather {
		t.Errorf("new rigger orders %v", o)
	}
}

func TestSupplyBlockAndQueueLimit(t *testing.T) {
	w := NewSkirmish()
	w.Players[0].Crystals = 1000
	h := hub(w, 0)
	cmd := Command{Kind: CmdTrain, Player: 0, Units: []UnitID{h.ID}, Type: faction.Rigger}
	got := notices(w, 1, cmd, cmd, cmd, cmd, cmd)
	if len(h.Queue) != 4 || len(got) != 1 || got[0] != NoticeSupply {
		t.Fatalf("queue %d, notices %v", len(h.Queue), got)
	}
	if w.Players[0].Crystals != 800 {
		t.Errorf("crystals %d, want 800", w.Players[0].Crystals)
	}
	// Cancelling the last entry refunds it.
	run(w, 1, Command{Kind: CmdCancel, Player: 0, Units: []UnitID{h.ID}, Index: -1})
	if len(h.Queue) != 3 || w.Players[0].Crystals < 850 || w.Players[0].Supply != StartWorkers+3 {
		t.Errorf("after cancel: queue %d, %+v", len(h.Queue), w.Players[0])
	}
}

func TestBuildGeneratorRaisesSupply(t *testing.T) {
	w := NewSkirmish()
	w.Players[0].Crystals = 200
	r := find(w, 0, faction.Rigger)[0]
	run(w, 1, Command{Kind: CmdBuild, Player: 0, Units: []UnitID{r.ID}, Type: faction.Generator, TX: 15, TY: 62})
	run(w, secs(8))
	gens := find(w, 0, faction.Generator)
	if len(gens) != 1 || gens[0].Complete {
		t.Fatalf("generator not started: %v", gens)
	}
	if c := w.Players[0].Crystals; c < 100 || c > 200 {
		t.Errorf("cost not deducted: %d", c)
	}
	run(w, secs(22))
	g := gens[0]
	if !g.Complete || g.HP < g.Def.HP-1 {
		t.Fatalf("generator incomplete: progress %d hp %.0f", g.Progress, g.HP)
	}
	if cap := w.Players[0].SupplyCap; cap != 18 {
		t.Errorf("supply cap %v, want 18", cap)
	}
	if len(r.Orders) == 0 || r.Orders[0].Kind != OrderGather {
		t.Errorf("builder didn't go back to mining: %v", r.Orders)
	}
}

func TestPlacementRules(t *testing.T) {
	w := NewSkirmish()
	w.Players[0].Crystals = 1000
	w.Players[0].Flux = 1000
	h := hub(w, 0)
	cases := []struct {
		t    faction.UnitType
		x, y int
		want Notice
	}{
		{faction.Generator, 15, 62, NoticeNone},
		{faction.Generator, h.TileX + 1, h.TileY + 1, NoticeCantBuild}, // on the hub
		{faction.Generator, 23, 62, NoticeCantBuild},                   // into the wall
		{faction.Generator, 60, 10, NoticeCantBuild},                   // unexplored
		{faction.Foundry, 15, 60, NoticeRequires},
		{faction.Extractor, 15, 62, NoticeNeedVent},
		{faction.Extractor, 17, 73, NoticeNone},
	}
	for _, c := range cases {
		if got := w.CanPlace(0, c.t, c.x, c.y); got != c.want {
			t.Errorf("%s at (%d,%d) = %v, want %v", faction.Unit(c.t).Name, c.x, c.y, got, c.want)
		}
	}
	w.Players[0].Crystals = 50
	if got := w.CanPlace(0, faction.Generator, 15, 62); got != NoticeCrystals {
		t.Errorf("broke = %v", got)
	}
}

func TestCancelConstructionRefundsAndUnblocks(t *testing.T) {
	w := NewSkirmish()
	w.Players[0].Crystals = 0
	b := w.PlaceBuilding(0, faction.DrillYard, 15, 60, false)
	run(w, 1, Command{Kind: CmdCancel, Player: 0, Units: []UnitID{b.ID}})
	if w.Unit(b.ID) != nil {
		t.Fatal("cancelled building still alive")
	}
	if c := w.Players[0].Crystals; c < 112 || c > 120 {
		t.Errorf("refund gave %d crystals, want 112 (+ mining)", c)
	}
	for y := 60; y < 63; y++ {
		for x := 15; x < 18; x++ {
			if w.Map.Blocked(x, y) {
				t.Fatalf("tile (%d,%d) still blocked", x, y)
			}
		}
	}
}

func TestExtractorFlux(t *testing.T) {
	w := NewSkirmish()
	w.Players[0].Crystals = 75
	rs := find(w, 0, faction.Rigger)
	run(w, 1, Command{Kind: CmdBuild, Player: 0, Units: []UnitID{rs[0].ID}, Type: faction.Extractor, TX: 17, TY: 73})
	run(w, secs(30))
	x := find(w, 0, faction.Extractor)
	if len(x) != 1 || !x[0].Complete {
		t.Fatalf("extractor not finished: %v", x)
	}
	vent := w.ventAt(17, 73)
	if vent.Extractor != x[0].ID {
		t.Fatal("vent not linked")
	}
	run(w, 1, Command{Kind: CmdGather, Player: 0, Units: []UnitID{rs[1].ID, rs[2].ID}, Res: vent.ID})
	run(w, secs(30))
	if f := w.Players[0].Flux; f < 40 {
		t.Errorf("flux %d after 30s with 3 workers", f)
	}
	// Enemy workers can't use our extractor.
	if w.mineable(1, vent) {
		t.Error("enemy could mine our vent")
	}
}

func TestTechTreeTraining(t *testing.T) {
	w := NewSkirmish()
	w.Players[0].Crystals, w.Players[0].Flux = 2000, 2000
	yard := w.PlaceBuilding(0, faction.DrillYard, 15, 60, true)
	gen := w.PlaceBuilding(0, faction.Generator, 19, 62, true)
	_ = gen
	run(w, 1,
		Command{Kind: CmdTrain, Player: 0, Units: []UnitID{yard.ID}, Type: faction.Trooper},
		Command{Kind: CmdTrain, Player: 0, Units: []UnitID{yard.ID}, Type: faction.Bastion}, // wrong building: ignored
		Command{Kind: CmdRally, Player: 0, Units: []UnitID{yard.ID}, Point: geom.V(18, 50)},
	)
	if len(yard.Queue) != 1 {
		t.Fatalf("queue %v", yard.Queue)
	}
	run(w, secs(15))
	tr := find(w, 0, faction.Trooper)
	if len(tr) != 1 {
		t.Fatalf("%d troopers", len(tr))
	}
	run(w, secs(8))
	if d := tr[0].Pos.Dist(geom.V(18, 50)); d > 1 {
		t.Errorf("trooper %.1f from rally", d)
	}
}

func TestArmyDestroysBaseAndWins(t *testing.T) {
	w := NewSkirmish()
	// Strip player 1 to just its hub and give player 0 a strike force next
	// to it.
	for _, u := range find(w, 1, faction.Rigger) {
		u.HP = 0
	}
	h := hub(w, 1)
	h.HP = 200
	army := w.SpawnSquad(0, faction.Strider, 4, h.Pos.Add(geom.V(-4, 4)))
	run(w, 1, Command{Kind: CmdAttackMove, Player: 0, Units: ids(army), Point: h.Pos})
	for i := 0; i < secs(30) && !w.Over; i++ {
		w.Step(nil)
	}
	if !w.Over || w.Draw || w.Winner != 0 {
		t.Fatalf("over=%v draw=%v winner=%d hub hp=%.0f", w.Over, w.Draw, w.Winner, h.HP)
	}
	for y := h.TileY; y < h.TileY+4; y++ {
		for x := h.TileX; x < h.TileX+4; x++ {
			if w.Map.Blocked(x, y) {
				t.Fatalf("dead hub tile (%d,%d) still blocked", x, y)
			}
		}
	}
	if _, ok := w.Seen[0][h.ID]; ok {
		t.Error("ghost of destroyed hub remains")
	}
}

func TestGhostsPersistInFog(t *testing.T) {
	w := NewSkirmish()
	h := hub(w, 1)
	scout := w.Spawn(0, faction.Outrider, h.Pos.Add(geom.V(-5, 5)))
	run(w, 2)
	if _, ok := w.Seen[0][h.ID]; !ok {
		t.Fatal("hub not remembered once seen")
	}
	scout.HP = 0
	run(w, 2)
	if w.VisibleTo(0, h) {
		t.Fatal("hub still visible without scout")
	}
	if g, ok := w.Seen[0][h.ID]; !ok || g.Type != faction.CommandHub {
		t.Error("ghost lost after scout left")
	}
}

func TestMenderHeals(t *testing.T) {
	w := openWorld(20, 20)
	m := w.Spawn(0, faction.Mender, geom.V(5, 5))
	tr := w.Spawn(0, faction.Trooper, geom.V(9, 5))
	st := w.Spawn(0, faction.Strider, geom.V(5, 9))
	tr.HP, st.HP = 10, 10
	run(w, secs(6))
	if tr.HP <= 40 {
		t.Errorf("trooper hp %.1f after 6s of healing", tr.HP)
	}
	if st.HP != 10 {
		t.Errorf("mender healed a mechanical unit: %.1f", st.HP)
	}
	if tr.HP > tr.Def.HP || m.HealTarget() != 0 && tr.HP == tr.Def.HP && m.HealTarget() == tr.ID {
		t.Errorf("overheal or stuck target: hp %.1f", tr.HP)
	}
}

func TestRiggerRepairs(t *testing.T) {
	w := NewSkirmish()
	h := hub(w, 0)
	h.HP = 1000
	r := find(w, 0, faction.Rigger)[0]
	run(w, 1, Command{Kind: CmdAssist, Player: 0, Units: []UnitID{r.ID}, Target: h.ID})
	run(w, secs(30))
	if h.HP != h.Def.HP {
		t.Errorf("hub hp %.0f after repair", h.HP)
	}
	if len(r.Orders) == 0 || r.Orders[0].Kind != OrderGather {
		t.Errorf("repairer orders %v", r.Orders)
	}
}

func TestSkirmishDeterministic(t *testing.T) {
	play := func() *World {
		w := NewSkirmish()
		w.Players[0].Crystals = 500
		r := find(w, 0, faction.Rigger)[0]
		run(w, 1,
			Command{Kind: CmdBuild, Player: 0, Units: []UnitID{r.ID}, Type: faction.Generator, TX: 15, TY: 62},
			Command{Kind: CmdTrain, Player: 0, Units: []UnitID{hub(w, 0).ID}, Type: faction.Rigger},
			Command{Kind: CmdTrain, Player: 1, Units: []UnitID{hub(w, 1).ID}, Type: faction.Rigger},
		)
		run(w, secs(90))
		return w
	}
	a, b := play(), play()
	if len(a.Units) != len(b.Units) || a.Players != b.Players {
		t.Fatalf("diverged: %d vs %d units, %+v vs %+v", len(a.Units), len(b.Units), a.Players, b.Players)
	}
	for i := range a.Units {
		if a.Units[i].Pos != b.Units[i].Pos || a.Units[i].HP != b.Units[i].HP {
			t.Fatalf("unit %d diverged", a.Units[i].ID)
		}
	}
}

func TestCrystalDepletes(t *testing.T) {
	w := NewSkirmish()
	var r *Resource
	for _, x := range w.Resources {
		if x.Kind == world.Crystal && x.assigned > 0 {
			r = x
			break
		}
	}
	r.Amount = CrystalTrip
	run(w, secs(20))
	if w.Resource(r.ID) != nil || w.Map.Blocked(r.X, r.Y) {
		t.Fatal("depleted crystal not removed")
	}
	for _, u := range find(w, 0, faction.Rigger) {
		if len(u.Orders) == 0 {
			t.Errorf("rigger %d went idle after depletion", u.ID)
		}
	}
}
