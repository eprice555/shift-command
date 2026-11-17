package sim

import (
	"math"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
	"github.com/eprice555/shift-command/internal/world"
)

// Resource tuning (GDD §2): crystals 5 per trip, flux 4 per trip.
const (
	CrystalAmount = 1500
	FluxAmount    = 2500
	CrystalTrip   = 5
	FluxTrip      = 4

	crystalMineTicks = 52 // 2.6s
	fluxMineTicks    = 32 // 1.6s

	// StartCrystals is each player's opening bank.
	StartCrystals = 50

	workReach    = 0.35 // max gap to mine, construct, or repair
	depositReach = 0.45 // max gap to a drop-off to hand in cargo
	buildReach   = 1.0  // max gap to a site before placing a building
)

// Player is one side's economy.
type Player struct {
	Faction        faction.ID
	Crystals, Flux int
	// Supply is used plus reserved (queued) supply; SupplyCap is what
	// completed buildings provide, capped at faction.MaxSupply.
	Supply, SupplyCap float64

	hadBuilding bool
}

// ResourceID identifies a resource node for the whole match.
type ResourceID uint32

// Resource is a crystal node or flux vent.
type Resource struct {
	ID        ResourceID
	Kind      world.ResourceKind
	X, Y      int // footprint top-left tile
	Size      int
	Pos       geom.Vec // footprint center
	Amount    int
	Extractor UnitID // building on a vent, if any (may be dead or unfinished)

	miner     UnitID // who mined it most recently
	claimTick uint64 // when miner last worked it
	assigned  int    // workers with a gather order on it this tick
}

// Rect is the node's footprint.
func (r *Resource) Rect() geom.Rect { return world.TileRect(r.X, r.Y, r.Size) }

// Workers is how many workers are currently gathering from r.
func (r *Resource) Workers() int { return r.assigned }

func (r *Resource) busy(tick uint64, by UnitID) bool {
	return r.miner != 0 && r.miner != by && r.claimTick+1 >= tick
}

func (w *World) addResources() {
	for _, s := range w.Map.Resources {
		n := s.Kind.Size()
		r := &Resource{
			ID: w.nextRes, Kind: s.Kind, X: s.X, Y: s.Y, Size: n,
			Pos: geom.V(float64(s.X)+float64(n)/2, float64(s.Y)+float64(n)/2),
		}
		r.Amount = CrystalAmount
		if s.Kind == world.Flux {
			r.Amount = FluxAmount
		}
		w.nextRes++
		w.Resources = append(w.Resources, r)
		w.resByID[r.ID] = r
		w.Map.Occupy(s.X, s.Y, n, 1)
	}
}

// Resource returns the live resource node with id, or nil.
func (w *World) Resource(id ResourceID) *Resource { return w.resByID[id] }

// ResourceAt returns the node whose footprint contains p, or nil.
func (w *World) ResourceAt(p geom.Vec) *Resource {
	for _, r := range w.Resources {
		if r.Rect().Contains(p) {
			return r
		}
	}
	return nil
}

func (w *World) depleteCrystal(r *Resource) {
	w.Map.Occupy(r.X, r.Y, r.Size, -1)
	delete(w.resByID, r.ID)
	for i, o := range w.Resources {
		if o == r {
			w.Resources = append(w.Resources[:i], w.Resources[i+1:]...)
			break
		}
	}
}

// mineable reports whether player p's workers can gather from r now.
func (w *World) mineable(p PlayerID, r *Resource) bool {
	if r == nil || r.Amount <= 0 {
		return false
	}
	if r.Kind == world.Crystal {
		return true
	}
	x := w.byID[r.Extractor]
	return x != nil && x.Owner == p && x.Complete
}

// findResource picks a node of kind k near pos for u, preferring close,
// lightly worked nodes. It returns nil if none is within reach.
func (w *World) findResource(u *Unit, k world.ResourceKind, near geom.Vec) *Resource {
	const reach = 14
	var best *Resource
	bestScore := math.Inf(1)
	for _, r := range w.Resources {
		if r.Kind != k || !w.mineable(u.Owner, r) {
			continue
		}
		d := r.Pos.Dist(near)
		if d > reach {
			continue
		}
		if s := d + 2.5*float64(r.assigned); s < bestScore {
			best, bestScore = r, s
		}
	}
	return best
}

// freeNeighbour returns a free node of r's kind close to r, nearest to u.
func (w *World) freeNeighbour(u *Unit, r *Resource) *Resource {
	var best *Resource
	bestD := math.Inf(1)
	for _, o := range w.Resources {
		if o == r || o.Kind != r.Kind || o.busy(w.Tick, u.ID) || !w.mineable(u.Owner, o) || o.Pos.Dist(r.Pos) > 3.5 {
			continue
		}
		if d := o.Pos.Dist(u.Pos) + float64(o.assigned); d < bestD {
			best, bestD = o, d
		}
	}
	return best
}

// nearestDropOff returns u's closest completed drop-off building.
func (w *World) nearestDropOff(u *Unit) *Unit {
	var best *Unit
	bestD := math.Inf(1)
	for _, b := range w.Units {
		if b.Owner != u.Owner || !b.Def.DropOff || !b.Complete {
			continue
		}
		if d := b.Pos.Dist2(u.Pos); d < bestD {
			best, bestD = b, d
		}
	}
	return best
}

// countAssignments refreshes Resource.assigned from gather orders.
func (w *World) countAssignments() {
	for _, r := range w.Resources {
		r.assigned = 0
	}
	for _, u := range w.Units {
		if len(u.Orders) > 0 && u.Orders[0].Kind == OrderGather {
			if r := w.resByID[u.Orders[0].Res]; r != nil {
				r.assigned++
			}
		}
	}
}

// gather runs a worker's mine → return → mine loop.
func (w *World) gather(u *Unit, o *Order) geom.Vec {
	r := w.resByID[o.Res]
	if u.Cargo > 0 {
		if v, done := w.returnCargo(u); !done {
			return v
		}
	}
	if !w.mineable(u.Owner, r) {
		kind, near := world.Crystal, u.Pos
		if r != nil {
			kind, near = r.Kind, r.Pos
		}
		if r = w.findResource(u, kind, near); r == nil {
			u.popOrder()
			return geom.Vec{}
		}
		o.Res = r.ID
	}
	u.lastRes = r.ID
	if gapRect(u, r.Rect()) > workReach {
		u.mining = 0
		return w.walk(u, w.approach(u.Pos, u.Def.Radius, r.Rect()))
	}
	if r.busy(w.Tick, u.ID) {
		if alt := w.freeNeighbour(u, r); alt != nil {
			o.Res = alt.ID
		}
		return geom.Vec{}
	}
	r.miner, r.claimTick = u.ID, w.Tick
	u.Facing = r.Pos.Sub(u.Pos).Angle()
	u.mining++
	need, trip := crystalMineTicks, CrystalTrip
	if r.Kind == world.Flux {
		need, trip = fluxMineTicks, FluxTrip
	}
	if u.mining >= need {
		u.mining = 0
		n := min(trip, r.Amount)
		r.Amount -= n
		u.Cargo, u.CargoKind = n, r.Kind
		if r.Amount == 0 && r.Kind == world.Crystal {
			w.depleteCrystal(r)
		}
	}
	return geom.Vec{}
}

// returnCargo walks u to a drop-off and banks its cargo. done is true once
// the cargo is in.
func (w *World) returnCargo(u *Unit) (v geom.Vec, done bool) {
	d := w.nearestDropOff(u)
	if d == nil {
		return geom.Vec{}, false
	}
	if gap(u, d) > depositReach {
		return w.walk(u, w.approach(u.Pos, u.Def.Radius, d.Rect())), false
	}
	p := &w.Players[u.Owner]
	if u.CargoKind == world.Flux {
		p.Flux += u.Cargo
	} else {
		p.Crystals += u.Cargo
	}
	u.Cargo = 0
	return geom.Vec{}, true
}

// recount recomputes supply used/reserved and supply caps.
func (w *World) recount() {
	for i := range w.Players {
		w.Players[i].Supply, w.Players[i].SupplyCap = 0, 0
	}
	for _, u := range w.Units {
		p := &w.Players[u.Owner]
		p.Supply += u.Def.Supply
		for _, t := range u.Queue {
			p.Supply += faction.QueueSupply(t)
		}
		if u.Complete {
			p.SupplyCap += u.Def.SupplyProvided
		}
	}
	for i := range w.Players {
		w.Players[i].SupplyCap = min(w.Players[i].SupplyCap, faction.MaxSupply)
	}
}

// pay deducts def's cost from p, or returns the notice explaining why not.
func (w *World) pay(p PlayerID, def faction.UnitDef) Notice {
	pl := &w.Players[p]
	switch {
	case pl.Crystals < def.Crystals:
		return NoticeCrystals
	case pl.Flux < def.Flux:
		return NoticeFlux
	}
	pl.Crystals -= def.Crystals
	pl.Flux -= def.Flux
	return NoticeNone
}

func (w *World) refund(p PlayerID, def faction.UnitDef, frac float64) {
	w.Players[p].Crystals += int(float64(def.Crystals) * frac)
	w.Players[p].Flux += int(float64(def.Flux) * frac)
}

// checkOver ends the match once a player who had buildings has none.
func (w *World) checkOver() {
	if w.Over {
		return
	}
	var has [NumPlayers]bool
	for _, u := range w.Units {
		if u.Def.Building {
			has[u.Owner] = true
		}
	}
	var lost [NumPlayers]bool
	n := 0
	for i := range w.Players {
		if has[i] {
			w.Players[i].hadBuilding = true
		} else if w.Players[i].hadBuilding {
			lost[i] = true
			n++
		}
	}
	switch {
	case n == NumPlayers:
		w.Over, w.Draw = true, true
	case n > 0:
		w.Over = true
		for i := range lost {
			if !lost[i] {
				w.Winner = PlayerID(i)
			}
		}
	}
}
