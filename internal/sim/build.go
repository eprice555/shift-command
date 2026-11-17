package sim

import (
	"math"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
	"github.com/eprice555/shift-command/internal/world"
)

// MaxQueue is the production queue length per building.
const MaxQueue = 5

// CancelRefund is the share of a building's cost returned when it is
// cancelled during construction.
const CancelRefund = 0.75

// Notice explains why a command was refused.
type Notice uint8

const (
	NoticeNone      Notice = iota
	NoticeCrystals         // not enough crystals
	NoticeFlux             // not enough flux
	NoticeSupply           // supply cap reached
	NoticeCantBuild        // invalid placement
	NoticeQueueFull        // production queue full
	NoticeRequires         // tech requirement missing (Event.Type names it)
	NoticeNeedVent         // extractor must go on a free vent
)

// String is the player-facing text for n.
func (n Notice) String() string {
	switch n {
	case NoticeCrystals:
		return "Not enough crystals"
	case NoticeFlux:
		return "Not enough flux"
	case NoticeSupply:
		return "Supply blocked: build a Generator"
	case NoticeCantBuild:
		return "Can't build there"
	case NoticeQueueFull:
		return "Queue is full"
	case NoticeRequires:
		return "Requires"
	case NoticeNeedVent:
		return "Must be placed on a free flux vent"
	}
	return ""
}

func (w *World) notice(p PlayerID, n Notice, t faction.UnitType) {
	w.Events = append(w.Events, Event{Kind: EvNotice, Owner: p, Notice: n, Type: t})
}

// ticksFor converts seconds to whole sim ticks (at least 1).
func ticksFor(sec float64) int { return max(1, int(math.Round(sec*TicksPerSecond))) }

// Rect is a building's footprint. For mobile units it is the bounding box
// of the body.
func (u *Unit) Rect() geom.Rect {
	if u.Def.Building {
		return world.TileRect(u.TileX, u.TileY, u.Def.Size)
	}
	r := geom.V(u.Def.Radius, u.Def.Radius)
	return geom.Rect{Min: u.Pos.Sub(r), Max: u.Pos.Add(r)}
}

// BuildFrac is construction progress in [0, 1] (1 once complete).
func (u *Unit) BuildFrac() float64 {
	if u.Complete || !u.Def.Building {
		return 1
	}
	return float64(u.Progress) / float64(ticksFor(u.Def.BuildTime))
}

// TrainFrac is progress on the front of the production queue in [0, 1].
func (u *Unit) TrainFrac() float64 {
	if !u.Complete || len(u.Queue) == 0 {
		return 0
	}
	return float64(u.Progress) / float64(ticksFor(faction.Unit(u.Queue[0]).BuildTime))
}

// HasCompleted reports whether p owns a finished building of type t.
func (w *World) HasCompleted(p PlayerID, t faction.UnitType) bool {
	for _, u := range w.Units {
		if u.Owner == p && u.Type == t && u.Complete {
			return true
		}
	}
	return false
}

// Missing returns the first unmet requirement for p to build or train t.
func (w *World) Missing(p PlayerID, t faction.UnitType) (faction.UnitType, bool) {
	for _, r := range faction.Unit(t).Requires {
		if !w.HasCompleted(p, r) {
			return r, true
		}
	}
	return 0, false
}

// ventAt returns the flux vent whose footprint starts at (x, y), or nil.
func (w *World) ventAt(x, y int) *Resource {
	for _, r := range w.Resources {
		if r.Kind == world.Flux && r.X == x && r.Y == y {
			return r
		}
	}
	return nil
}

// CanPlace checks whether p may start a building of type t with its
// footprint's top-left at tile (x, y): valid ground, tech, and funds.
func (w *World) CanPlace(p PlayerID, t faction.UnitType, x, y int) Notice {
	def := faction.Unit(t)
	if !def.Building {
		return NoticeCantBuild
	}
	if n := w.siteNotice(p, def, x, y); n != NoticeNone {
		return n
	}
	if _, missing := w.Missing(p, t); missing {
		return NoticeRequires
	}
	pl := w.Players[p]
	switch {
	case pl.Crystals < def.Crystals:
		return NoticeCrystals
	case pl.Flux < def.Flux:
		return NoticeFlux
	}
	return NoticeNone
}

func (w *World) siteNotice(p PlayerID, def faction.UnitDef, x, y int) Notice {
	if def.OnVent {
		v := w.ventAt(x, y)
		if v == nil || w.byID[v.Extractor] != nil || w.Fog[p].At(x, y) == world.Unexplored {
			return NoticeNeedVent
		}
		return NoticeNone
	}
	for ty := y; ty < y+def.Size; ty++ {
		for tx := x; tx < x+def.Size; tx++ {
			if w.Map.Blocked(tx, ty) || w.Fog[p].At(tx, ty) == world.Unexplored {
				return NoticeCantBuild
			}
		}
	}
	return NoticeNone
}

// PlaceBuilding creates a building with its footprint at tile (x, y). An
// incomplete building starts at 10% HP and needs a worker to finish it.
// Units standing on the site are pushed out.
func (w *World) PlaceBuilding(owner PlayerID, t faction.UnitType, x, y int, complete bool) *Unit {
	def := faction.Unit(t)
	n := float64(def.Size)
	u := &Unit{
		ID: w.nextID, Owner: owner, Type: t, Def: def,
		TileX: x, TileY: y, Complete: complete, HP: def.HP,
		Pos: geom.V(float64(x)+n/2, float64(y)+n/2),
	}
	u.PrevPos = u.Pos
	if !complete {
		u.HP = def.HP * 0.1
	}
	w.nextID++
	w.Units = append(w.Units, u)
	w.byID[u.ID] = u
	w.Map.Occupy(x, y, def.Size, 1)
	if def.OnVent {
		if v := w.ventAt(x, y); v != nil {
			v.Extractor = u.ID
		}
	}
	rect := u.Rect()
	for _, o := range w.Units {
		if !o.Def.Building && rectDist(rect, o.Pos) < o.Def.Radius {
			o.Pos = w.Map.ResolveCircle(w.Map.SnapPassable(w.approach(o.Pos, o.Def.Radius, rect)), o.Def.Radius)
			o.resetNav()
		}
	}
	w.Fog[owner].Reveal(u.Pos, def.Sight)
	return u
}

// rectDist is the distance from p to the nearest point of r (0 inside).
func rectDist(r geom.Rect, p geom.Vec) float64 {
	dx := max(r.Min.X-p.X, 0, p.X-r.Max.X)
	dy := max(r.Min.Y-p.Y, 0, p.Y-r.Max.Y)
	return math.Hypot(dx, dy)
}

// gap is the empty space between two bodies; buildings use their
// footprint.
func gap(a, b *Unit) float64 {
	switch {
	case b.Def.Building && !a.Def.Building:
		return rectDist(b.Rect(), a.Pos) - a.Def.Radius
	case a.Def.Building && !b.Def.Building:
		return rectDist(a.Rect(), b.Pos) - b.Def.Radius
	}
	return a.Pos.Dist(b.Pos) - a.Def.Radius - b.Def.Radius
}

func gapRect(u *Unit, r geom.Rect) float64 { return rectDist(r, u.Pos) - u.Def.Radius }

// approach returns an open point just outside r for a body of radius rad
// coming from `from`: straight out from r's nearest edge if that ground is
// open, else the nearest open tile hugging r.
func (w *World) approach(from geom.Vec, rad float64, r geom.Rect) geom.Vec {
	q := geom.V(min(max(from.X, r.Min.X), r.Max.X), min(max(from.Y, r.Min.Y), r.Max.Y))
	d := from.Sub(q)
	if d.Len2() < 1e-9 {
		// Inside: leave through the nearest face.
		faces := [4]float64{from.X - r.Min.X, r.Max.X - from.X, from.Y - r.Min.Y, r.Max.Y - from.Y}
		dirs := [4]geom.Vec{geom.V(-1, 0), geom.V(1, 0), geom.V(0, -1), geom.V(0, 1)}
		best := 0
		for i := range faces {
			if faces[i] < faces[best] {
				best = i
			}
		}
		d = dirs[best]
		switch best {
		case 0:
			q = geom.V(r.Min.X, from.Y)
		case 1:
			q = geom.V(r.Max.X, from.Y)
		case 2:
			q = geom.V(from.X, r.Min.Y)
		default:
			q = geom.V(from.X, r.Max.Y)
		}
	}
	p := q.Add(d.Norm().Scale(rad + 0.1))
	if w.Map.Passable(world.TileOf(p)) {
		return p
	}
	x0, y0 := int(math.Floor(r.Min.X))-1, int(math.Floor(r.Min.Y))-1
	x1, y1 := int(math.Ceil(r.Max.X)), int(math.Ceil(r.Max.Y))
	best, bestD := geom.Vec{}, math.Inf(1)
	for ty := y0; ty <= y1; ty++ {
		for tx := x0; tx <= x1; tx++ {
			if (tx != x0 && tx != x1 && ty != y0 && ty != y1) || w.Map.Blocked(tx, ty) {
				continue
			}
			c := world.TileCenter(tx, ty)
			if dd := c.Dist2(from); dd < bestD {
				best, bestD = c, dd
			}
		}
	}
	if bestD < math.Inf(1) {
		return best
	}
	return w.Map.SnapPassable(p)
}

// walk steers u toward goal without consuming an order or giving up; the
// caller decides when u is close enough.
func (w *World) walk(u *Unit, goal geom.Vec) geom.Vec {
	u.resetProgress()
	if d := goal.Sub(u.Pos); d.Len() <= u.Def.Speed/TicksPerSecond {
		return d
	}
	return w.steer(u, goal, nil)
}

// build walks a worker to its site and lays the foundation, turning the
// order into OrderConstruct.
func (w *World) build(u *Unit, o *Order) geom.Vec {
	def := faction.Unit(o.Build)
	rect := world.TileRect(o.TX, o.TY, def.Size)
	if gapRect(u, rect) > buildReach {
		return w.walk(u, w.approach(u.Pos, u.Def.Radius, rect))
	}
	if n := w.CanPlace(u.Owner, o.Build, o.TX, o.TY); n != NoticeNone {
		missing, _ := w.Missing(u.Owner, o.Build)
		w.notice(u.Owner, n, missing)
		u.popOrder()
		return geom.Vec{}
	}
	w.pay(u.Owner, def)
	b := w.PlaceBuilding(u.Owner, o.Build, o.TX, o.TY, false)
	*o = Order{Kind: OrderConstruct, Target: b.ID}
	u.resetNav()
	return geom.Vec{}
}

// construct advances an unfinished building while the worker stands at it.
// Only one worker's effort counts per tick.
func (w *World) construct(u *Unit, o *Order) geom.Vec {
	b := w.byID[o.Target]
	if b == nil || b.Owner != u.Owner || b.Complete {
		w.finishWork(u, b)
		return geom.Vec{}
	}
	if gap(u, b) > workReach+0.15 {
		return w.walk(u, w.approach(u.Pos, u.Def.Radius, b.Rect()))
	}
	u.Facing = b.Pos.Sub(u.Pos).Angle()
	u.working = true
	if b.builtTick == w.Tick+1 {
		return geom.Vec{}
	}
	b.builtTick = w.Tick + 1
	total := ticksFor(b.Def.BuildTime)
	b.Progress++
	b.HP = min(b.Def.HP, b.HP+b.Def.HP*0.9/float64(total))
	if b.Progress >= total {
		b.Complete, b.Progress = true, 0
		w.Events = append(w.Events, Event{Kind: EvBuildingDone, Owner: b.Owner, Type: b.Type, From: b.Pos})
		w.finishWork(u, b)
	}
	return geom.Vec{}
}

// finishWork pops a construct/repair order; an idle worker then goes back
// to gathering (onto the new Extractor's vent if it just built one).
func (w *World) finishWork(u *Unit, b *Unit) {
	u.popOrder()
	if len(u.Orders) > 0 {
		return
	}
	if b != nil && b.Complete && b.Def.OnVent {
		if v := w.ventAt(b.TileX, b.TileY); v != nil {
			u.Orders = append(u.Orders, Order{Kind: OrderGather, Res: v.ID})
			return
		}
	}
	if u.lastRes != 0 {
		u.Orders = append(u.Orders, Order{Kind: OrderGather, Res: u.lastRes})
	}
}

// repair restores a friendly mechanical unit or building. Repair is free.
func (w *World) repair(u *Unit, o *Order) geom.Vec {
	b := w.byID[o.Target]
	if b == nil || b.Owner != u.Owner || !b.Def.Mech || !b.Complete || b.HP >= b.Def.HP || b == u {
		w.finishWork(u, nil)
		return geom.Vec{}
	}
	if gap(u, b) > workReach+0.15 {
		return w.walk(u, w.approach(u.Pos, u.Def.Radius, b.Rect()))
	}
	u.Facing = b.Pos.Sub(u.Pos).Angle()
	u.working = true
	w.hits = append(w.hits, hit{b.ID, -b.Def.HP / (b.Def.BuildTime * TicksPerSecond)})
	return geom.Vec{}
}

// returnOrder banks cargo, then resumes gathering.
func (w *World) returnOrder(u *Unit) geom.Vec {
	if u.Cargo > 0 {
		if v, done := w.returnCargo(u); !done {
			return v
		}
	}
	w.finishWork(u, nil)
	return geom.Vec{}
}

// heal looks after a healer's patients. It reports whether it is busy
// healing (or walking to a patient).
func (w *World) heal(u *Unit) (geom.Vec, bool) {
	reach := u.Def.Sight
	if u.Hold {
		reach = u.Def.HealRange
	}
	var best *Unit
	bestD := math.Inf(1)
	for _, o := range w.Units {
		if o == u || o.Owner != u.Owner || !o.Def.Bio || o.HP >= o.Def.HP {
			continue
		}
		if d := gap(u, o); d <= reach && d < bestD {
			best, bestD = o, d
		}
	}
	u.healing = 0
	if best == nil {
		return geom.Vec{}, false
	}
	u.healing = best.ID
	if bestD > u.Def.HealRange {
		if u.Hold {
			return geom.Vec{}, false
		}
		return w.steer(u, best.Pos, nil), true
	}
	u.Facing = best.Pos.Sub(u.Pos).Angle()
	w.hits = append(w.hits, hit{best.ID, -u.Def.HealRate / TicksPerSecond})
	if (w.Tick+uint64(u.ID))%10 == 0 {
		w.Events = append(w.Events, Event{Kind: EvHeal, Owner: u.Owner, Type: u.Type, From: u.Pos, To: best.Pos})
	}
	return geom.Vec{}, true
}

// produce advances a building's production queue.
func (w *World) produce(b *Unit) {
	if !b.Complete || len(b.Queue) == 0 {
		return
	}
	t := b.Queue[0]
	def := faction.Unit(t)
	b.Progress++
	if b.Progress < ticksFor(def.BuildTime) {
		return
	}
	b.Progress = 0
	copy(b.Queue, b.Queue[1:])
	b.Queue = b.Queue[:len(b.Queue)-1]

	toward := geom.V(float64(w.Map.W)/2, float64(w.Map.H)/2)
	if b.HasRally {
		toward = b.Rally
	}
	nu := w.Spawn(b.Owner, t, w.approach(toward, def.Radius, b.Rect()))
	w.Events = append(w.Events, Event{Kind: EvUnitReady, Owner: b.Owner, Type: t, From: nu.Pos})
	r := w.resByID[b.RallyRes]
	switch {
	case def.Worker && b.HasRally && r != nil:
		nu.Orders = append(nu.Orders, Order{Kind: OrderGather, Res: r.ID})
	case b.HasRally:
		nu.Orders = append(nu.Orders, Order{Kind: OrderMove, Point: w.Map.SnapPassable(b.Rally)})
	case def.Worker:
		if r := w.findResource(nu, world.Crystal, b.Pos); r != nil {
			nu.Orders = append(nu.Orders, Order{Kind: OrderGather, Res: r.ID})
		}
	}
}

// train queues t at the least busy eligible building in bs.
func (w *World) train(p PlayerID, bs []*Unit, t faction.UnitType) {
	def := faction.Unit(t)
	if def.Building {
		return
	}
	var best *Unit
	for _, b := range bs {
		if !b.Complete || !trains(b.Def, t) {
			continue
		}
		if best == nil || len(b.Queue) < len(best.Queue) {
			best = b
		}
	}
	if best == nil {
		return
	}
	if len(best.Queue) >= MaxQueue {
		w.notice(p, NoticeQueueFull, t)
		return
	}
	if r, missing := w.Missing(p, t); missing {
		w.notice(p, NoticeRequires, r)
		return
	}
	pl := &w.Players[p]
	if pl.Supply+def.Supply > pl.SupplyCap {
		w.notice(p, NoticeSupply, t)
		return
	}
	if n := w.pay(p, def); n != NoticeNone {
		w.notice(p, n, t)
		return
	}
	best.Queue = append(best.Queue, t)
	pl.Supply += def.Supply
}

func trains(d faction.UnitDef, t faction.UnitType) bool {
	for _, x := range d.Trains {
		if x == t {
			return true
		}
	}
	return false
}

// cancel aborts an unfinished building (partial refund) or removes queue
// entry i (-1 for the last) with a full refund.
func (w *World) cancel(p PlayerID, b *Unit, i int) {
	if !b.Def.Building {
		return
	}
	if !b.Complete {
		w.refund(p, b.Def, CancelRefund)
		b.HP = 0
		return
	}
	if len(b.Queue) == 0 {
		return
	}
	if i < 0 || i >= len(b.Queue) {
		i = len(b.Queue) - 1
	}
	def := faction.Unit(b.Queue[i])
	w.refund(p, def, 1)
	w.Players[p].Supply -= def.Supply
	b.Queue = append(b.Queue[:i], b.Queue[i+1:]...)
	if i == 0 {
		b.Progress = 0
	}
}
