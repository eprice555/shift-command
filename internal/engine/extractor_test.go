package engine

import (
	"testing"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/sim"
	"github.com/eprice555/shift-command/internal/world"
)

// Right-clicking your finished extractor gathers from its vent, and a
// building's rally onto it targets the vent.
func TestRightClickExtractorGathers(t *testing.T) {
	for _, f := range []faction.ID{faction.Forge, faction.Brood} {
		m := NewMatch(false, f)
		def := faction.Get(f)
		start := m.world.Map.Starts[m.player]
		var vent *sim.Resource
		for _, r := range m.world.Resources {
			if r.Kind == world.Flux && (vent == nil || r.Pos.Dist(start) < vent.Pos.Dist(start)) {
				vent = r
			}
		}
		x := m.world.PlaceBuilding(m.player, def.Extractor, vent.X, vent.Y, true)
		var workers []sim.UnitID
		var hq *sim.Unit
		for _, u := range m.world.Units {
			switch {
			case u.Owner != m.player:
			case u.Type == def.Worker:
				workers = append(workers, u.ID)
			case u.Type == def.HQ:
				hq = u
			}
		}
		m.setSelection(workers[:2])
		m.smartCommand(x.Pos, x)
		m.setSelection([]sim.UnitID{hq.ID})
		m.smartCommand(x.Pos, x)
		m.world.Step(m.pending)
		for _, id := range workers[:2] {
			u := m.world.Unit(id)
			if len(u.Orders) == 0 || u.Orders[0].Kind != sim.OrderGather || u.Orders[0].Res != vent.ID {
				t.Errorf("%v: worker orders %+v, want gather vent %d", def.Name, u.Orders, vent.ID)
			}
		}
		if hq.RallyRes != vent.ID {
			t.Errorf("%v: HQ rally res %d, want vent %d", def.Name, hq.RallyRes, vent.ID)
		}
	}
}
