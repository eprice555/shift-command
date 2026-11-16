package sim

import (
	"math"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
	"github.com/eprice555/shift-command/internal/world"
)

// OrderKind is the type of a queued unit order.
type OrderKind uint8

const (
	OrderMove       OrderKind = iota // go to Point, ignoring enemies
	OrderAttackMove                  // go to Point, fighting anything on the way
	OrderAttack                      // kill Target
)

// Order is one entry in a unit's order queue.
type Order struct {
	Kind   OrderKind
	Point  geom.Vec
	Target UnitID
	group  *moveGroup
}

// moveGroup is shared by every unit given the same move command: one flow
// field for the group plus arrival bookkeeping so a blob doesn't jostle
// forever over a single goal point.
type moveGroup struct {
	size   int
	radius float64 // units this close to the goal may stop once settled
	flow   *world.FlowField
}

// Unit is a live unit in the sim.
type Unit struct {
	ID    UnitID
	Owner PlayerID
	Type  faction.UnitType
	Def   faction.UnitDef

	Pos, PrevPos       geom.Vec
	Facing, PrevFacing float64
	HP                 float64
	Cooldown           int // ticks until the weapon is ready
	Orders             []Order
	Hold               bool // hold position: fire in range, never move
	Moving             bool // moved under its own power last tick

	target UnitID // what it's currently fighting (ordered or auto-acquired)
	firing bool   // standing still shooting this tick

	path     []geom.Vec
	pathGoal geom.Vec
	repathAt uint64
	bestDist float64 // closest straight-line approach to the goal so far
	stall    int     // ticks without getting closer to the goal
	stuck    int     // ticks spent trying to move but barely moving
	arrived  *moveGroup
}

// Target returns the unit this one is currently fighting, or 0.
func (u *Unit) Target() UnitID { return u.target }

// Idle reports whether the unit has no orders and isn't holding.
func (u *Unit) Idle() bool { return len(u.Orders) == 0 && !u.Hold }

// yield is how readily the unit gets shoved by others in separation.
func (u *Unit) yield() float64 {
	switch {
	case u.Hold:
		return 0.1
	case u.firing:
		return 0.3
	case u.Moving:
		return 0.6
	}
	return 1
}

func (u *Unit) resetNav() {
	u.path = nil
	u.resetProgress()
}

func (u *Unit) resetProgress() {
	u.bestDist = math.Inf(1)
	u.stall = 0
	u.stuck = 0
}

func (u *Unit) clearOrders() {
	clear(u.Orders)
	u.Orders = u.Orders[:0]
	u.target = 0
	u.resetNav()
}

func (u *Unit) popOrder() {
	copy(u.Orders, u.Orders[1:])
	u.Orders[len(u.Orders)-1] = Order{}
	u.Orders = u.Orders[:len(u.Orders)-1]
	u.target = 0
	u.resetNav()
}

// cooldownTicks converts a weapon cooldown in seconds to ticks.
func cooldownTicks(sec float64) int {
	return max(1, int(math.Round(sec*TicksPerSecond)))
}

// edgeDist is the gap between two units' bodies.
func edgeDist(a, b *Unit) float64 {
	return a.Pos.Dist(b.Pos) - a.Def.Radius - b.Def.Radius
}

// think runs one tick of decision-making for u and returns its desired
// movement this tick. Shots are queued in w.hits.
func (w *World) think(u *Unit) geom.Vec {
	if u.Cooldown > 0 {
		u.Cooldown--
	}
	u.firing = false

	// An explicit attack order overrides everything until the target dies
	// or is lost in the fog.
	for len(u.Orders) > 0 && u.Orders[0].Kind == OrderAttack {
		t := w.byID[u.Orders[0].Target]
		if t == nil || !w.VisibleTo(u.Owner, t) {
			u.popOrder()
			continue
		}
		u.target = t.ID
		return w.engage(u, t, true)
	}

	var o *Order
	if len(u.Orders) > 0 {
		o = &u.Orders[0]
	}
	if o == nil || o.Kind == OrderAttackMove {
		if t := w.byID[u.target]; t == nil || !w.VisibleTo(u.Owner, t) || (u.Hold && edgeDist(u, t) > u.Def.Weapon.Range) {
			u.target = 0
		}
		// Scanning for targets is staggered across ticks to spread the cost.
		if u.target == 0 && (w.Tick+uint64(u.ID))%3 == 0 {
			u.target = w.acquire(u)
		}
		if t := w.byID[u.target]; t != nil {
			return w.engage(u, t, !u.Hold)
		}
	} else {
		u.target = 0
	}
	if o == nil {
		return geom.Vec{}
	}
	v, arrived := w.navigate(u, o.Point, o.group)
	if arrived {
		u.arrived = o.group
		u.popOrder()
	}
	return v
}

// acquire picks the nearest visible enemy within the unit's acquisition
// range: weapon range when holding, sight otherwise.
func (w *World) acquire(u *Unit) UnitID {
	reach := u.Def.Sight
	if u.Hold {
		reach = u.Def.Weapon.Range
	}
	best, bestD := UnitID(0), math.Inf(1)
	for _, e := range w.Units {
		if e.Owner == u.Owner || e.HP <= 0 || !w.VisibleTo(u.Owner, e) {
			continue
		}
		if d := edgeDist(u, e); d <= reach && d < bestD {
			best, bestD = e.ID, d
		}
	}
	return best
}

// engage fires at t if in range, otherwise chases it (if allowed).
func (w *World) engage(u, t *Unit, canMove bool) geom.Vec {
	u.resetProgress()
	wp := u.Def.Weapon
	if edgeDist(u, t) <= wp.Range {
		u.Facing = t.Pos.Sub(u.Pos).Angle()
		u.firing = true
		if u.Cooldown == 0 {
			u.Cooldown = cooldownTicks(wp.Cooldown)
			w.hits = append(w.hits, hit{t.ID, wp.DamageTo(t.Def)})
			dir := t.Pos.Sub(u.Pos).Norm()
			w.Events = append(w.Events, Event{
				Kind: EvShot, Owner: u.Owner, Type: u.Type, Attack: wp.Type,
				From: u.Pos.Add(dir.Scale(u.Def.Radius)),
				To:   t.Pos.Sub(dir.Scale(t.Def.Radius * 0.5)),
			})
		}
		return geom.Vec{}
	}
	if !canMove {
		return geom.Vec{}
	}
	return w.steer(u, t.Pos, nil)
}

// navigate steers u toward goal and reports whether it has arrived.
func (w *World) navigate(u *Unit, goal geom.Vec, g *moveGroup) (geom.Vec, bool) {
	d := u.Pos.Dist(goal)
	speed := u.Def.Speed / TicksPerSecond
	if d < u.bestDist-0.05 {
		u.bestDist, u.stall = d, 0
	} else {
		u.stall++
	}

	arrived := d <= 0.05
	switch {
	case arrived:
	case g != nil && g.size > 1:
		// In a group, stop once near the goal and blocked by groupmates
		// that already arrived, or once progress stalls.
		arrived = d <= g.radius && (u.stall > 6 || w.touchingArrived(u, g)) ||
			d <= g.radius*2 && u.stall > 40
	default:
		arrived = d < 1 && u.stall > 30
	}
	// Give up if physically blocked for a while (e.g. a walled-in goal).
	if arrived || u.stuck > 3*TicksPerSecond {
		return geom.Vec{}, true
	}
	if d <= speed {
		return goal.Sub(u.Pos), false
	}
	var flow *world.FlowField
	if g != nil {
		flow = g.flow
	}
	return w.steer(u, goal, flow), false
}

func (w *World) touchingArrived(u *Unit, g *moveGroup) bool {
	for _, o := range w.Units {
		if o != u && o.arrived == g && o.Owner == u.Owner &&
			o.Pos.Dist(u.Pos) < o.Def.Radius+u.Def.Radius+0.2 {
			return true
		}
	}
	return false
}

// steer returns a velocity toward goal, following the flow field when one is
// given and usable, else a cached A* path.
func (w *World) steer(u *Unit, goal geom.Vec, flow *world.FlowField) geom.Vec {
	speed := u.Def.Speed / TicksPerSecond
	r := u.Def.Radius
	next, ok := geom.Vec{}, false
	if flow != nil && flow.Reachable(u.Pos) {
		next, ok = flow.Next(w.Map.Grid, u.Pos, r)
	}
	if !ok {
		next = w.followPath(u, goal)
	}
	dir := next.Sub(u.Pos)
	l := dir.Len()
	if l == 0 {
		return geom.Vec{}
	}
	if l <= speed {
		return dir
	}
	return dir.Scale(speed / l)
}

const repathTicks = 30

func (w *World) followPath(u *Unit, goal geom.Vec) geom.Vec {
	r := u.Def.Radius
	if u.path == nil || u.pathGoal.Dist(goal) > 1 || w.Tick >= u.repathAt {
		if w.Map.ClearPath(u.Pos, goal, r) {
			u.path = append(u.path[:0], goal)
		} else {
			u.path = w.Map.FindPath(u.Pos, goal, r)
		}
		u.pathGoal = goal
		u.repathAt = w.Tick + repathTicks + uint64(u.ID%8)
	}
	for len(u.path) > 1 && (u.Pos.Dist(u.path[0]) < 0.4 || w.Map.ClearPath(u.Pos, u.path[1], r)) {
		u.path = u.path[1:]
	}
	if len(u.path) == 0 {
		return goal
	}
	return u.path[0]
}
