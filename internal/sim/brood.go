package sim

import (
	"math"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
	"github.com/eprice555/shift-command/internal/world"
)

// Brood mechanics: Hives hatch Spawn, Spawn morph into units in place,
// Drones are consumed to grow buildings, buildings spread creep, and
// everything regenerates.

const (
	// creepGrowth is how fast (tiles per second) a finished building's
	// creep spreads out to its full radius.
	creepGrowth = 0.6
	// creepEvery is how often (ticks) the creep grid is recomputed.
	creepEvery = 5
)

// Creep is one player's creep coverage, a tile grid.
type Creep struct {
	W, H int
	on   []bool
	// Version changes whenever coverage changes, so renderers can cache.
	Version int
}

func newCreep(w, h int) *Creep { return &Creep{W: w, H: h, on: make([]bool, w*h)} }

// At reports whether tile (x, y) has creep. Off-map tiles never do.
func (c *Creep) At(x, y int) bool {
	if x < 0 || y < 0 || x >= c.W || y >= c.H {
		return false
	}
	return c.on[y*c.W+x]
}

// CreepRadius is how far u's creep currently reaches (0 if it spreads none).
func (u *Unit) CreepRadius() float64 { return u.creepR }

// Morphing reports whether u is a Spawn cocooned into another form.
func (u *Unit) Morphing() bool { return u.Def.Morphs && len(u.Queue) > 0 }

// MorphFrac is a cocoon's progress in [0, 1].
func (u *Unit) MorphFrac() float64 {
	if !u.Morphing() {
		return 0
	}
	return float64(u.Progress) / float64(u.morphTicks())
}

// HatchFrac is a Hive's progress toward its next Spawn in [0, 1].
func (u *Unit) HatchFrac() float64 {
	return float64(u.hatch) / float64(ticksFor(faction.SpawnTime))
}

// SpawnCount returns how many idle Spawn belong to Hive h.
func (w *World) SpawnCount(h *Unit) int {
	n := 0
	for _, u := range w.Units {
		if u.home == h.ID && u.Def.Morphs && !u.Morphing() {
			n++
		}
	}
	return n
}

// IdleSpawn returns an idle Spawn of Hive h, or nil.
func (w *World) IdleSpawn(h *Unit) *Unit {
	for _, u := range w.Units {
		if u.home == h.ID && u.Def.Morphs && !u.Morphing() {
			return u
		}
	}
	return nil
}

// hatchSpawn ticks a Hive's Spawn timer and hatches one when due.
func (w *World) hatchSpawn(h *Unit) {
	n := w.SpawnCount(h)
	if n >= h.Def.SpawnMax {
		h.hatch = 0
		return
	}
	h.hatch++
	if h.hatch < ticksFor(faction.SpawnTime) {
		return
	}
	h.hatch = 0
	w.newSpawn(h, n)
}

// newSpawn hatches a Spawn in slot i along the Hive's face toward the map
// center.
func (w *World) newSpawn(h *Unit, i int) *Unit {
	mid := geom.V(float64(w.Map.W)/2, float64(w.Map.H)/2)
	dir := mid.Sub(h.Pos).Norm()
	if dir.Len2() == 0 {
		dir = geom.V(0, 1)
	}
	side := geom.V(-dir.Y, dir.X)
	off := float64(h.Def.Size)/2 + 0.45
	pos := h.Pos.Add(dir.Scale(off)).Add(side.Scale(float64(i%3-1) * 0.85))
	s := w.Spawn(h.Owner, faction.Spawn, pos)
	s.home = h.ID
	s.Facing = w.rng(s.ID) * 2 * math.Pi
	s.PrevFacing = s.Facing
	return s
}

// rng is a cheap deterministic per-id value in [0, 1).
func (w *World) rng(id UnitID) float64 {
	x := uint32(id)*2654435761 ^ uint32(w.Tick)*40503
	x ^= x >> 15
	return float64(x%1000) / 1000
}

// morph advances a cocoon; when done the Spawn becomes its new form in
// place (plus extra copies for pair morphs).
func (w *World) morph(u *Unit) {
	t := u.Queue[0]
	def := faction.Unit(t)
	u.Progress++
	if u.Progress < u.morphTicks() {
		return
	}
	home := w.byID[u.home]
	n := def.Yield()
	hp := def.HP
	if u.remorph != nil {
		// A re-morph hatches one unit and keeps its wounds.
		n, hp = 1, def.HP*u.HP/u.Def.HP
	}
	u.Type, u.Def, u.HP = t, def, hp
	u.Queue, u.Progress, u.home, u.Hold, u.remorph = nil, 0, 0, false, nil
	out := []*Unit{u}
	for i := 1; i < n; i++ {
		a := float64(i) * 2 * math.Pi / float64(def.Yield())
		out = append(out, w.Spawn(u.Owner, t, u.Pos.Add(geom.V(math.Cos(a), math.Sin(a)).Scale(def.Radius*2))))
	}
	for _, nu := range out {
		w.Events = append(w.Events, Event{Kind: EvUnitReady, Owner: nu.Owner, Type: t, From: nu.Pos})
		w.sendOff(nu, home)
	}
}

// sendOff gives a freshly made unit its first order from building b's
// rally point (b may be nil): workers gather, others move.
func (w *World) sendOff(nu *Unit, b *Unit) {
	def := nu.Def
	hasRally := b != nil && b.HasRally
	var r *Resource
	if hasRally {
		r = w.resByID[b.RallyRes]
	}
	switch {
	case def.Worker && hasRally && r != nil:
		nu.Orders = append(nu.Orders, Order{Kind: OrderGather, Res: r.ID})
	case hasRally:
		nu.Orders = append(nu.Orders, Order{Kind: OrderMove, Point: w.Map.SnapPassable(b.Rally)})
	case def.Worker:
		from := nu.Pos
		if b != nil {
			from = b.Pos
		}
		if r := w.findResource(nu, world.Crystal, from); r != nil {
			nu.Orders = append(nu.Orders, Order{Kind: OrderGather, Res: r.ID})
		}
	}
}

// grow advances an unfinished building by one tick of work (at most once
// per tick). Brood buildings call it themselves; others need a worker.
func (w *World) grow(b *Unit) {
	if b.HP <= 0 || b.builtTick == w.Tick+1 {
		return
	}
	b.builtTick = w.Tick + 1
	total := ticksFor(b.Def.BuildTime)
	b.Progress++
	b.HP = min(b.Def.HP, b.HP+b.Def.HP*0.9/float64(total))
	if b.Progress >= total {
		b.Complete, b.Progress = true, 0
		b.Shield = b.Def.Shield
		w.Events = append(w.Events, Event{Kind: EvBuildingDone, Owner: b.Owner, Type: b.Type, From: b.Pos})
	}
}

// consume removes a worker that has morphed into a building. It leaves no
// corpse or death event.
func (w *World) consume(u *Unit) {
	u.HP, u.consumed = 0, true
	u.clearOrders()
	u.Cargo = 0
}

// regenerate heals every unit with natural regeneration.
func (w *World) regenerate() {
	for _, u := range w.Units {
		if r := u.Def.Regen; r > 0 && u.Complete && u.HP > 0 && u.HP < u.Def.HP {
			u.HP = min(u.Def.HP, u.HP+r/TicksPerSecond)
		}
		w.rechargeShield(u)
	}
}

// updateCreep spreads creep from finished buildings and recomputes each
// player's coverage. Creep never covers walls.
func (w *World) updateCreep(force bool) {
	for _, u := range w.Units {
		if u.Def.Creep > 0 && u.Complete && u.creepR < u.Def.Creep {
			u.creepR = min(u.Def.Creep, u.creepR+creepGrowth/TicksPerSecond)
		}
	}
	if !force && w.Tick%creepEvery != 0 {
		return
	}
	for p, c := range w.Creep {
		next := make([]bool, len(c.on))
		for _, u := range w.Units {
			if u.Owner != PlayerID(p) || u.creepR <= 0 || u.lifted {
				continue
			}
			r := u.creepR + float64(u.Def.Size)/2
			x0, y0 := max(0, int(u.Pos.X-r)), max(0, int(u.Pos.Y-r))
			x1, y1 := min(c.W-1, int(u.Pos.X+r)), min(c.H-1, int(u.Pos.Y+r))
			for y := y0; y <= y1; y++ {
				for x := x0; x <= x1; x++ {
					if w.Map.Terrain(x, y) || world.TileCenter(x, y).Dist2(u.Pos) > r*r {
						continue
					}
					next[y*c.W+x] = true
				}
			}
		}
		changed := false
		for i := range next {
			if next[i] != c.on[i] {
				changed = true
				break
			}
		}
		if changed {
			c.on = next
			c.Version++
		}
	}
}

// onCreep reports whether the footprint at (x, y) is all on p's creep.
func (w *World) onCreep(p PlayerID, size, x, y int) bool {
	for ty := y; ty < y+size; ty++ {
		for tx := x; tx < x+size; tx++ {
			if !w.Creep[p].At(tx, ty) {
				return false
			}
		}
	}
	return true
}
