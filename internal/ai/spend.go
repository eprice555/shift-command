package ai

import (
	"math"
	"slices"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
	"github.com/eprice555/shift-command/internal/sim"
	"github.com/eprice555/shift-command/internal/world"
)

// spend decides what to build and train, in priority order: an HQ if all
// are lost, supply, workers, the scripted opening, then extra production
// and army with whatever is left.
func (a *AI) spend(w *sim.World) {
	s := &a.s
	pl := w.Players[a.P]
	supply := pl.Supply

	if s.have[faction.CommandHub] == 0 {
		if len(s.workers) > 0 && a.afford(faction.CommandHub) {
			a.place(w, faction.CommandHub)
		}
	}

	// Supply: keep ahead of what production can use up.
	building := s.have[faction.Generator] - s.done[faction.Generator]
	capSoon := pl.SupplyCap + float64(building)*faction.Unit(faction.Generator).SupplyProvided +
		float64(s.have[faction.CommandHub]-s.done[faction.CommandHub])*faction.Unit(faction.CommandHub).SupplyProvided
	need := 2 + 2*float64(len(s.prod)) + float64(len(s.hubs))
	if capSoon < faction.MaxSupply && capSoon-supply < need && building < 1+len(s.prod)/3 {
		if a.afford(faction.Generator) {
			a.place(w, faction.Generator)
		}
	}

	// Workers: two per crystal node and three per extractor.
	want := 3 * s.done[faction.Extractor]
	for _, h := range s.hubs {
		for _, r := range w.Resources {
			if r.Kind == world.Crystal && r.Pos.Dist(h.Pos) <= hubReach {
				want += 2
			}
		}
	}
	want = min(want, maxWorkers)
	for _, h := range s.hubs {
		if s.have[faction.Rigger] >= want || len(h.Queue) > 0 || supply+1 > pl.SupplyCap {
			continue
		}
		if a.afford(faction.Rigger) {
			a.cmd(sim.Command{Kind: sim.CmdTrain, Units: []sim.UnitID{h.ID}, Type: faction.Rigger})
			s.have[faction.Rigger]++
			supply++
		}
	}

	// The scripted opening, one step at a time.
	opened := true
	for _, st := range ForgeOpening {
		if s.have[st.Build] >= st.Count {
			continue
		}
		opened = false
		if supply < st.Supply {
			break
		}
		if _, missing := w.Missing(a.P, st.Build); missing {
			a.afford(st.Build) // save up while the requirement finishes
			break
		}
		if a.afford(st.Build) {
			a.place(w, st.Build)
		}
		break
	}

	if opened {
		a.macro(w)
	}

	// Army: Foundries first so the cheap Drill Yard units don't starve them.
	prod := slices.Clone(s.prod)
	slices.SortStableFunc(prod, func(x, y *sim.Unit) int {
		return boolInt(x.Type != faction.Foundry) - boolInt(y.Type != faction.Foundry)
	})
	for _, b := range prod {
		if len(b.Queue) >= 2 {
			continue
		}
		for i, t := range a.unitChoices(b.Type) {
			def := faction.Unit(t)
			if supply+def.Supply > pl.SupplyCap {
				break
			}
			if !a.canAfford(t) {
				// Short on flux: fall back to the next choice. Short on
				// crystals: wait for the preferred unit.
				if i == 0 && s.crystals < def.Crystals {
					break
				}
				continue
			}
			a.afford(t)
			a.cmd(sim.Command{Kind: sim.CmdTrain, Units: []sim.UnitID{b.ID}, Type: t})
			s.have[t]++
			supply += def.Supply
			break
		}
	}
}

// macro adds extractors and production once the opening is done.
func (a *AI) macro(w *sim.World) {
	s := &a.s
	if _, _, ok := a.ventSite(w); ok && a.canAfford(faction.Extractor) {
		a.afford(faction.Extractor)
		a.place(w, faction.Extractor)
	}
	nProd := s.have[faction.DrillYard] + s.have[faction.Foundry]
	if s.crystals < 300 || nProd >= maxProdBuild {
		return
	}
	t := faction.DrillYard
	if 2*s.have[faction.Foundry] < s.have[faction.DrillYard] && s.done[faction.DrillYard] > 0 {
		t = faction.Foundry
	}
	if a.canAfford(t) {
		a.afford(t)
		a.place(w, t)
	}
}

// unitChoices lists what a production building should train next, best
// first; the first affordable one is queued.
func (a *AI) unitChoices(b faction.UnitType) []faction.UnitType {
	h := &a.s.have
	switch b {
	case faction.DrillYard:
		tr, out, med := h[faction.Trooper], h[faction.Outrider], h[faction.Mender]
		switch {
		case med < min(3, tr/6):
			return []faction.UnitType{faction.Mender, faction.Trooper}
		case out*3 < tr:
			return []faction.UnitType{faction.Outrider, faction.Trooper}
		}
		return []faction.UnitType{faction.Trooper}
	case faction.Foundry:
		if 2*h[faction.Bastion] < h[faction.Strider] {
			return []faction.UnitType{faction.Bastion, faction.Strider}
		}
		return []faction.UnitType{faction.Strider}
	}
	return nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// place sends a worker to build t on a site the AI picks. It reports
// whether an order went out.
func (a *AI) place(w *sim.World, t faction.UnitType) bool {
	def := faction.Unit(t)
	var x, y int
	var ok bool
	builder := sim.UnitID(0)
	switch {
	case def.OnVent:
		x, y, ok = a.ventSite(w)
	case t == faction.CommandHub:
		var c geom.Vec
		if c, ok = a.hubSite(w); !ok {
			return false
		}
		x, y = int(c.X)-def.Size/2, int(c.Y)-def.Size/2
		if !a.explored(w, x, y, def.Size) {
			if a.expander == 0 {
				if u := a.pickWorker(w, c); u != nil {
					a.expander = u.ID
					a.cmd(sim.Command{Kind: sim.CmdMove, Units: []sim.UnitID{u.ID}, Point: c})
				}
			}
			return false
		}
		builder = a.expander
	default:
		x, y, ok = a.site(w, t)
	}
	if !ok {
		return false
	}
	n := float64(def.Size) / 2
	c := geom.V(float64(x)+n, float64(y)+n)
	if builder == 0 || w.Unit(builder) == nil {
		u := a.pickWorker(w, c)
		if u == nil {
			return false
		}
		builder = u.ID
	}
	a.cmd(sim.Command{Kind: sim.CmdBuild, Units: []sim.UnitID{builder}, Type: t, TX: x, TY: y})
	a.s.build[builder] = true
	a.s.have[t]++
	if builder == a.expander {
		a.expander = 0
	}
	return true
}

func (a *AI) explored(w *sim.World, x, y, n int) bool {
	for ty := y; ty < y+n; ty++ {
		for tx := x; tx < x+n; tx++ {
			if w.Fog[a.P].At(tx, ty) == world.Unexplored {
				return false
			}
		}
	}
	return true
}

// hubSite picks where a new HQ goes: the home start if it is free (after
// losing every HQ), else the free expansion nearest home that the enemy
// isn't known to hold.
func (a *AI) hubSite(w *sim.World) (geom.Vec, bool) {
	size := faction.Unit(faction.CommandHub).Size
	cands := append([]geom.Vec{a.home}, w.Map.Expansions...)
	best, bestD := geom.Vec{}, math.Inf(1)
	for _, c := range cands {
		x, y := int(c.X)-size/2, int(c.Y)-size/2
		free := true
		for ty := y; ty < y+size && free; ty++ {
			for tx := x; tx < x+size; tx++ {
				if w.Map.Blocked(tx, ty) {
					free = false
					break
				}
			}
		}
		if !free || a.enemyNear(w, c, 12) {
			continue
		}
		if d := c.Dist(a.home); d < bestD {
			best, bestD = c, d
		}
	}
	return best, bestD < math.Inf(1)
}

// enemyNear reports whether we know of an enemy building within r of p.
func (a *AI) enemyNear(w *sim.World, p geom.Vec, r float64) bool {
	for _, g := range w.Seen[a.P] {
		if ghostPos(g).Dist(p) <= r {
			return true
		}
	}
	return false
}

func ghostPos(g sim.Ghost) geom.Vec {
	n := float64(faction.Unit(g.Type).Size) / 2
	return geom.V(float64(g.TileX)+n, float64(g.TileY)+n)
}

// ventSite returns a free, explored vent next to one of our HQs.
func (a *AI) ventSite(w *sim.World) (int, int, bool) {
	var best *sim.Resource
	for _, r := range w.Resources {
		if r.Kind != world.Flux || w.Unit(r.Extractor) != nil || !a.nearHub(r.Pos) ||
			w.Fog[a.P].At(r.X, r.Y) == world.Unexplored {
			continue
		}
		if best == nil || r.Pos.Dist2(a.home) < best.Pos.Dist2(a.home) {
			best = r
		}
	}
	if best == nil {
		return 0, 0, false
	}
	return best.X, best.Y, true
}

// site finds the closest good spot for a building of type t around one of
// our HQs (home first): on open explored ground, with walking room around
// it, and clear of mining lanes and the path out of the base.
func (a *AI) site(w *sim.World, t faction.UnitType) (int, int, bool) {
	hubs := slices.Clone(a.s.hubs)
	slices.SortStableFunc(hubs, func(x, y *sim.Unit) int {
		return cmpFloat(x.Pos.Dist2(a.home), y.Pos.Dist2(a.home))
	})
	for _, h := range hubs {
		if x, y, ok := a.siteNear(w, t, h.Pos); ok {
			return x, y, true
		}
	}
	return 0, 0, false
}

func cmpFloat(x, y float64) int {
	switch {
	case x < y:
		return -1
	case x > y:
		return 1
	}
	return 0
}

const siteReach = 10

func (a *AI) siteNear(w *sim.World, t faction.UnitType, c geom.Vec) (int, int, bool) {
	n := faction.Unit(t).Size
	half := float64(n) / 2
	ring := 1
	if n >= 3 {
		ring = 2
	}
	minD := 2 + half + 1.5
	bx, by, bestD := 0, 0, math.Inf(1)
	cx, cy := int(c.X), int(c.Y)
	for y := cy - siteReach - n; y <= cy+siteReach; y++ {
		for x := cx - siteReach - n; x <= cx+siteReach; x++ {
			ctr := geom.V(float64(x)+half, float64(y)+half)
			d := ctr.Dist(c)
			if d < minD || d > siteReach || d >= bestD {
				continue
			}
			if !a.open(w, x, y, n, ring) || !a.offLanes(w, ctr, half, c) {
				continue
			}
			bx, by, bestD = x, y, d
		}
	}
	return bx, by, bestD < math.Inf(1)
}

// open reports whether the n×n footprint at (x, y) is explored, open
// ground and the ring of tiles around it is walkable.
func (a *AI) open(w *sim.World, x, y, n, ring int) bool {
	for ty := y - ring; ty < y+n+ring; ty++ {
		for tx := x - ring; tx < x+n+ring; tx++ {
			if w.Map.Blocked(tx, ty) {
				return false
			}
			inside := tx >= x && tx < x+n && ty >= y && ty < y+n
			if inside && w.Fog[a.P].At(tx, ty) == world.Unexplored {
				return false
			}
		}
	}
	return true
}

// offLanes keeps a footprint centered at ctr away from the HQ's mining
// lanes and the road out of the base.
func (a *AI) offLanes(w *sim.World, ctr geom.Vec, half float64, hub geom.Vec) bool {
	for _, r := range w.Resources {
		if r.Pos.Dist(hub) > hubReach+2 {
			continue
		}
		if ctr.Dist(r.Pos) < half+2.5 || segDist(ctr, hub, r.Pos) < half+1.2 {
			return false
		}
	}
	for i := 1; i < len(a.exit); i++ {
		p, q := a.exit[i-1], a.exit[i]
		if p.Dist(a.home) > 30 {
			break
		}
		if segDist(ctr, p, q) < half+1.8 {
			return false
		}
	}
	return true
}

// segDist is the distance from p to the segment ab.
func segDist(p, a, b geom.Vec) float64 {
	ab := b.Sub(a)
	l2 := ab.Len2()
	if l2 == 0 {
		return p.Dist(a)
	}
	t := min(max(p.Sub(a).Dot(ab)/l2, 0), 1)
	return p.Dist(a.Add(ab.Scale(t)))
}
