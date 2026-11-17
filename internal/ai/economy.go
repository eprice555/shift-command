package ai

import (
	"math"

	"github.com/eprice555/shift-command/internal/geom"
	"github.com/eprice555/shift-command/internal/sim"
	"github.com/eprice555/shift-command/internal/world"
)

// hubReach is how far from a drop-off its resource nodes may be.
const hubReach = 10

// economy keeps workers busy: idle ones mine, extractors get crews, and
// buildings whose builder died get finished.
func (a *AI) economy(w *sim.World) {
	s := &a.s
	if len(s.hubs) == 0 {
		return
	}
	// Local gather counts so several assignments in one think spread out.
	load := map[sim.ResourceID]int{}
	for _, r := range w.Resources {
		load[r.ID] = r.Workers()
	}

	for _, b := range s.unfin {
		if a.constructing(w, b) {
			continue
		}
		if u := a.pickWorker(w, b.Pos); u != nil {
			a.cmd(sim.Command{Kind: sim.CmdAssist, Units: []sim.UnitID{u.ID}, Target: b.ID})
			s.build[u.ID] = true
		}
	}

	for _, r := range w.Resources {
		if r.Kind != world.Flux {
			continue
		}
		x := w.Unit(r.Extractor)
		if x == nil || x.Owner != a.P || !x.Complete {
			continue
		}
		for n := load[r.ID]; n < 3; n++ {
			u := a.pickWorker(w, r.Pos)
			if u == nil {
				break
			}
			a.cmd(sim.Command{Kind: sim.CmdGather, Units: []sim.UnitID{u.ID}, Res: r.ID})
			s.build[u.ID] = true
			load[r.ID]++
		}
	}

	for _, u := range s.workers {
		if !u.Idle() || s.build[u.ID] || u.ID == a.expander {
			continue
		}
		if r := a.bestCrystal(w, u.Pos, load); r != nil {
			a.cmd(sim.Command{Kind: sim.CmdGather, Units: []sim.UnitID{u.ID}, Res: r.ID})
			load[r.ID]++
		}
	}
}

// constructing reports whether any of our workers is working on b.
func (a *AI) constructing(w *sim.World, b *sim.Unit) bool {
	for _, u := range a.s.workers {
		for _, o := range u.Orders {
			if o.Kind == sim.OrderConstruct && o.Target == b.ID {
				return true
			}
		}
	}
	return false
}

// bestCrystal picks a crystal node by a drop-off, favoring lightly worked
// and close ones.
func (a *AI) bestCrystal(w *sim.World, from geom.Vec, load map[sim.ResourceID]int) *sim.Resource {
	var best *sim.Resource
	bestScore := math.Inf(1)
	for _, r := range w.Resources {
		if r.Kind != world.Crystal || r.Amount <= 0 || !a.nearHub(r.Pos) {
			continue
		}
		if sc := 6*float64(load[r.ID]) + r.Pos.Dist(from)*0.1; sc < bestScore {
			best, bestScore = r, sc
		}
	}
	return best
}

func (a *AI) nearHub(p geom.Vec) bool {
	for _, h := range a.s.hubs {
		if h.Pos.Dist(p) <= hubReach {
			return true
		}
	}
	return false
}

// pickWorker returns the crystal gatherer (or idle worker) nearest p that
// isn't already busy with a job this think, or nil.
func (a *AI) pickWorker(w *sim.World, p geom.Vec) *sim.Unit {
	var best *sim.Unit
	bestD := math.Inf(1)
	for _, u := range a.s.workers {
		if a.s.build[u.ID] || u.ID == a.expander || u.Hold {
			continue
		}
		if len(u.Orders) > 0 {
			o := u.Orders[0]
			switch o.Kind {
			case sim.OrderGather:
				if r := w.Resource(o.Res); r != nil && r.Kind == world.Flux {
					continue
				}
			case sim.OrderReturn:
				if u.CargoKind == world.Flux {
					continue
				}
			default:
				continue
			}
		}
		d := u.Pos.Dist(p)
		if u.Cargo > 0 {
			d += 4
		}
		if d < bestD {
			best, bestD = u, d
		}
	}
	return best
}
