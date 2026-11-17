package sim

import (
	"testing"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
	"github.com/eprice555/shift-command/internal/world"
)

func broodMatch() *World { return NewSkirmishFor([NumPlayers]faction.ID{faction.Brood, faction.Forge}) }

func hive(w *World) *Unit { return find(w, 0, faction.Hive)[0] }

func idleSpawn(w *World) []*Unit {
	var out []*Unit
	for _, u := range find(w, 0, faction.Spawn) {
		if !u.Morphing() {
			out = append(out, u)
		}
	}
	return out
}

// creepSite finds a free tile for a size-s building on p's creep near the
// Hive, or (if want is false) a free tile off creep.
func creepSite(t *testing.T, w *World, s int, want bool) (int, int) {
	t.Helper()
	h := hive(w)
	w.Fog[0].Reveal(h.Pos, 30)
	cx, cy := int(h.Pos.X), int(h.Pos.Y)
	for r := 3; r < 30; r++ {
		for y := cy - r; y <= cy+r; y++ {
			for x := cx - r; x <= cx+r; x++ {
				if w.siteNotice(0, faction.Unit(faction.NestPod), x, y) == NoticeNone && want {
					return x, y
				}
				free := true
				for ty := y; ty < y+s && free; ty++ {
					for tx := x; tx < x+s; tx++ {
						if w.Map.Blocked(tx, ty) || w.Fog[0].At(tx, ty) == world.Unexplored || w.Creep[0].At(tx, ty) {
							free = false
							break
						}
					}
				}
				if free && !want {
					return x, y
				}
			}
		}
	}
	t.Fatal("no site")
	return 0, 0
}

func TestBroodStart(t *testing.T) {
	w := broodMatch()
	h := hive(w)
	if !h.Complete || w.Players[0].Faction != faction.Brood {
		t.Fatalf("hive %+v", h)
	}
	if n := len(find(w, 0, faction.Drone)); n != StartWorkers {
		t.Errorf("%d drones", n)
	}
	if n := w.SpawnCount(h); n != StartSpawn {
		t.Errorf("%d spawn", n)
	}
	pl := w.Players[0]
	if pl.Supply != StartWorkers || pl.SupplyCap != 10 {
		t.Errorf("supply %v/%v", pl.Supply, pl.SupplyCap)
	}
	if !w.Creep[0].At(int(h.Pos.X), int(h.Pos.Y)+5) || w.Creep[1].At(int(h.Pos.X), int(h.Pos.Y)) {
		t.Error("creep not spread around the Hive")
	}
	// The Forge side is unchanged.
	if len(find(w, 1, faction.CommandHub)) != 1 || len(find(w, 1, faction.Spawn)) != 0 {
		t.Error("forge start changed")
	}
}

func TestHiveMorphsAndHatches(t *testing.T) {
	w := broodMatch()
	h := hive(w)
	before := w.Players[0].Crystals
	run(w, 1, Command{Kind: CmdTrain, Player: 0, Units: []UnitID{h.ID}, Type: faction.Drone})
	if n := w.SpawnCount(h); n != StartSpawn-1 {
		t.Fatalf("%d idle spawn after morph order", n)
	}
	if w.Players[0].Crystals != before-50 || w.Players[0].Supply != StartWorkers+1 {
		t.Fatalf("not charged: %+v", w.Players[0])
	}
	ready := false
	w.Step(nil)
	for range secs(12) {
		w.Step(nil)
		for _, e := range w.Events {
			ready = ready || (e.Kind == EvUnitReady && e.Type == faction.Drone)
		}
	}
	ds := find(w, 0, faction.Drone)
	if len(ds) != StartWorkers+1 || !ready {
		t.Fatalf("%d drones, ready %v", len(ds), ready)
	}
	nd := ds[len(ds)-1]
	if len(nd.Orders) == 0 || nd.Orders[0].Kind != OrderGather {
		t.Errorf("new drone not gathering: %+v", nd.Orders)
	}
	// 10 s after the morph started, a replacement Spawn has hatched.
	if n := w.SpawnCount(h); n != StartSpawn {
		t.Errorf("%d spawn after hatch", n)
	}
	run(w, secs(30))
	if n := w.SpawnCount(h); n != StartSpawn {
		t.Errorf("hive over-hatched: %d", n)
	}
}

func TestNoSpawn(t *testing.T) {
	w := broodMatch()
	h := hive(w)
	w.Players[0].Crystals = 1000
	cmd := Command{Kind: CmdTrain, Player: 0, Units: []UnitID{h.ID}, Type: faction.Drone}
	ns := notices(w, 1, cmd, cmd, cmd, cmd)
	if len(ns) != 1 || ns[0] != NoticeNoSpawn {
		t.Fatalf("notices %v", ns)
	}
	if w.SpawnCount(h) != 0 || w.Players[0].Supply != StartWorkers+3 {
		t.Fatalf("spawn %d supply %v", w.SpawnCount(h), w.Players[0].Supply)
	}
}

func TestBiterPair(t *testing.T) {
	w := broodMatch()
	w.Spawn(0, faction.SpawningPit, geom.V(20, 50))
	w.Players[0].Crystals = 1000
	s := idleSpawn(w)[0]
	pos := s.Pos
	run(w, 1, Command{Kind: CmdTrain, Player: 0, Units: []UnitID{s.ID}, Type: faction.Biter})
	if !s.Morphing() || w.Players[0].Supply != StartWorkers+1 {
		t.Fatalf("morphing %v supply %v", s.Morphing(), w.Players[0].Supply)
	}
	// A cocoon can't be moved.
	run(w, secs(3), Command{Kind: CmdMove, Player: 0, Units: []UnitID{s.ID}, Point: pos.Add(geom.V(5, 5))})
	if s.Pos.Dist(pos) > 0.5 || !s.Morphing() {
		t.Fatalf("cocoon moved to %v", s.Pos)
	}
	run(w, secs(14))
	bs := find(w, 0, faction.Biter)
	if len(bs) != 2 || s.Type != faction.Biter {
		t.Fatalf("%d biters, spawn is now %d", len(bs), s.Type)
	}
	if w.Players[0].Supply != StartWorkers+1 {
		t.Errorf("supply %v", w.Players[0].Supply)
	}
}

func TestCancelCocoon(t *testing.T) {
	w := broodMatch()
	s := idleSpawn(w)[0]
	before := w.Players[0]
	run(w, secs(2), Command{Kind: CmdTrain, Player: 0, Units: []UnitID{s.ID}, Type: faction.Drone})
	run(w, 1, Command{Kind: CmdCancel, Player: 0, Units: []UnitID{s.ID}})
	pl := w.Players[0]
	if s.Morphing() || s.Type != faction.Spawn || s.HP <= 0 {
		t.Fatalf("spawn %+v", s)
	}
	if pl.Supply != before.Supply || pl.Crystals < before.Crystals {
		t.Errorf("not refunded: %+v vs %+v", pl, before)
	}
}

func TestDroneMorphsIntoBuilding(t *testing.T) {
	w := broodMatch()
	w.Players[0].Crystals = 1000
	d := find(w, 0, faction.Drone)[0]
	ox, oy := creepSite(t, w, 2, false)
	ns := notices(w, 1, Command{Kind: CmdBuild, Player: 0, Units: []UnitID{d.ID}, Type: faction.NestPod, TX: ox, TY: oy})
	if len(ns) != 1 || ns[0] != NoticeNeedCreep {
		t.Fatalf("off-creep notices %v", ns)
	}
	x, y := creepSite(t, w, 2, true)
	cr := w.Players[0].Crystals
	w.Step([]Command{{Kind: CmdBuild, Player: 0, Units: []UnitID{d.ID}, Type: faction.NestPod, TX: x, TY: y}})
	var pod *Unit
	for range secs(20) {
		w.Step(nil)
		for _, e := range w.Events {
			if e.Kind == EvDeath && e.Type == faction.Drone {
				t.Fatal("consumed drone reported a death")
			}
		}
		if ps := find(w, 0, faction.NestPod); len(ps) > 0 {
			pod = ps[0]
			break
		}
	}
	if pod == nil || w.byID[d.ID] != nil {
		t.Fatalf("pod %v, drone still alive %v", pod, w.byID[d.ID] != nil)
	}
	if w.Players[0].Crystals != cr-100 || len(find(w, 0, faction.Drone)) != StartWorkers-1 {
		t.Fatalf("crystals %d drones %d", w.Players[0].Crystals, len(find(w, 0, faction.Drone)))
	}
	run(w, secs(21))
	if !pod.Complete || w.Players[0].SupplyCap != 18 {
		t.Fatalf("pod complete %v cap %v", pod.Complete, w.Players[0].SupplyCap)
	}
}

func TestCancelBroodBuilding(t *testing.T) {
	w := broodMatch()
	w.Players[0].Crystals = 100
	d := find(w, 0, faction.Drone)[0]
	x, y := creepSite(t, w, 2, true)
	run(w, 1, Command{Kind: CmdBuild, Player: 0, Units: []UnitID{d.ID}, Type: faction.NestPod, TX: x, TY: y})
	for range secs(20) {
		if len(find(w, 0, faction.NestPod)) > 0 {
			break
		}
		w.Step(nil)
	}
	pod := find(w, 0, faction.NestPod)[0]
	cr := w.Players[0].Crystals
	run(w, 2, Command{Kind: CmdCancel, Player: 0, Units: []UnitID{pod.ID}})
	if len(find(w, 0, faction.NestPod)) != 0 || len(find(w, 0, faction.Drone)) != StartWorkers {
		t.Fatalf("drone not returned: pods %d drones %d", len(find(w, 0, faction.NestPod)), len(find(w, 0, faction.Drone)))
	}
	if got := w.Players[0].Crystals - cr; got < 75 {
		t.Errorf("refund %d", got)
	}
}

func TestRegenAndCreepGrowth(t *testing.T) {
	w := broodMatch()
	d := find(w, 0, faction.Drone)[0]
	d.HP -= 20
	hp := d.HP
	run(w, secs(5))
	if d.HP < hp+2.5 {
		t.Errorf("drone regen %v -> %v", hp, d.HP)
	}
	// A new building's creep grows outward once it finishes.
	p := w.Spawn(0, faction.NestPod, geom.V(30, 45))
	run(w, secs(1))
	if r := p.CreepRadius(); r <= 0 || r >= p.Def.Creep {
		t.Errorf("creep radius after 1 s %v", r)
	}
	run(w, secs(10))
	if p.CreepRadius() != p.Def.Creep || !w.Creep[0].At(30, 45+3) {
		t.Errorf("creep radius after 11 s %v", p.CreepRadius())
	}
	p.HP = 0
	run(w, creepEvery+1)
	if w.Creep[0].At(30, 45+3) {
		t.Error("creep outlived its source")
	}
}
