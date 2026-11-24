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
	OrderGather                      // worker: mine Res and bring cargo home, forever
	OrderReturn                      // worker: bank cargo, then resume gathering
	OrderBuild                       // worker: lay a Build foundation at tile (TX, TY)
	OrderConstruct                   // worker: finish building Target
	OrderRepair                      // worker: repair Target
)

// Order is one entry in a unit's order queue.
type Order struct {
	Kind   OrderKind
	Point  geom.Vec
	Target UnitID
	Res    ResourceID
	Build  faction.UnitType
	TX, TY int
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
	Shield             float64 // Phasebound shields; absorb damage first
	Cooldown           int     // ticks until the weapon is ready
	Orders             []Order
	Hold               bool // hold position: fire in range, never move
	Moving             bool // moved under its own power last tick
	Mode               int  // SHIFT mode: 0 base, 1 shifted (see faction.Mode)

	// Buildings.
	TileX, TileY int                // footprint top-left
	Complete     bool               // finished (always true for mobile units)
	Progress     int                // construction ticks, or ticks on Queue[0]
	Queue        []faction.UnitType // production queue
	HasRally     bool
	Rally        geom.Vec
	RallyRes     ResourceID // rally onto a resource: new workers gather it

	// Workers.
	Cargo     int
	CargoKind world.ResourceKind

	target  UnitID // what it's currently fighting (ordered or auto-acquired)
	firing  bool   // standing still shooting this tick
	working bool   // constructing or repairing this tick
	healing UnitID // patient this tick

	hurtTick  uint64     // Tick+1 when it last took damage (shields wait)
	mining    int        // ticks spent on the current trip's mining
	lastRes   ResourceID // node to go back to after building or banking
	builtTick uint64     // Tick+1 when construction last advanced

	path     []geom.Vec
	pathGoal geom.Vec
	repathAt uint64
	bestDist float64 // closest straight-line approach to the goal so far
	stall    int     // ticks without getting closer to the goal
	stuck    int     // ticks spent trying to move but barely moving
	arrived  *moveGroup

	shifting   int  // ticks left switching modes
	shiftTotal int  // length of the current switch
	modeLeft   int  // ticks left on a timed mode
	lifted     bool // packed building in the air

	home     UnitID  // Spawn: the Hive that hatched it
	hatch    int     // Hive: ticks toward the next Spawn
	creepR   float64 // creep radius spread so far
	consumed bool    // Drone that became a building: removed without a death

	castCD       int      // ticks until the SHIFT ability can be cast again
	leapTo       UnitID   // Pounce target while leaping
	leapLeft     int      // ticks before an unfinished leap gives up
	dashTo       UnitID   // Phase Dash target
	dashLeft     int      // ticks before an unfinished dash gives up
	phaseLeft    int      // ticks left while intangible
	warpIn       int      // ticks left before a warped unit can act
	warpTotal    int      // total warp-in duration for presentation
	warpResearch int      // Resonator ticks left on Warp research
	life         int      // summoned unit: ticks until it expires (0 = never)
	remorph      *remorph // cocoon re-morphing from another form
}

// Target returns the unit this one is currently fighting, or 0.
func (u *Unit) Target() UnitID { return u.target }

// Working reports whether a worker is constructing or repairing.
func (u *Unit) Working() bool { return u.working }

// Mining reports whether a worker is mining this tick.
func (u *Unit) Mining() bool { return u.mining > 0 }

// HealTarget returns the unit this healer is tending, or 0.
func (u *Unit) HealTarget() UnitID { return u.healing }

// Idle reports whether the unit has no orders and isn't holding.
func (u *Unit) Idle() bool { return len(u.Orders) == 0 && !u.Hold }

// gathering reports whether u is on a mining/return trip; gatherers pass
// through each other so mineral lines don't jam.
func (u *Unit) gathering() bool {
	return len(u.Orders) > 0 && (u.Orders[0].Kind == OrderGather || u.Orders[0].Kind == OrderReturn)
}

// yield is how readily the unit gets shoved by others in separation.
func (u *Unit) yield() float64 {
	switch {
	case u.Hold:
		return 0.1
	case u.firing, u.working, u.mining > 0:
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

// think runs one tick of decision-making for u and returns its desired
// movement this tick. Shots are queued in w.hits.
func (w *World) think(u *Unit) geom.Vec {
	if u.Cooldown > 0 {
		u.Cooldown--
	}
	u.firing, u.working = false, false
	if w.tickAbilities(u) {
		return geom.Vec{}
	}
	if w.tickShift(u) {
		return geom.Vec{}
	}
	if u.warpIn > 0 {
		u.warpIn--
		return geom.Vec{}
	}
	if u.Def.Building {
		if u.lifted {
			return w.fly(u)
		}
		if !u.Complete && u.Def.SelfBuild {
			w.grow(u)
		}
		if u.warpResearch > 0 {
			if !w.Unpowered(u) {
				w.researchWarpTick(u)
			}
			return geom.Vec{}
		}
		if u.Mode == 0 {
			w.produce(u)
		}
		if u.Complete && u.Def.SpawnMax > 0 {
			w.hatchSpawn(u)
		}
		return geom.Vec{}
	}
	if u.consumed {
		return geom.Vec{}
	}
	if u.Morphing() {
		w.morph(u)
		return geom.Vec{}
	}
	if u.dashTo != 0 {
		return w.dash(u)
	}
	if u.leapTo != 0 {
		return w.leap(u)
	}

	// An explicit attack order overrides everything until the target dies
	// or is lost in the fog (or, for a unit that can't move, leaves its
	// range).
	for len(u.Orders) > 0 && u.Orders[0].Kind == OrderAttack {
		t := w.byID[u.Orders[0].Target]
		if t == nil || !w.VisibleTo(u.Owner, t) || u.Def.Speed == 0 && !inReach(u, t) {
			u.popOrder()
			continue
		}
		u.target = t.ID
		if u.Phased() {
			return w.steer(u, t.Pos, nil)
		}
		return w.engage(u, t, true)
	}

	var o *Order
	if len(u.Orders) > 0 {
		o = &u.Orders[0]
	}
	if o != nil && o.Kind != OrderGather {
		u.mining = 0
	}
	if o != nil {
		switch o.Kind {
		case OrderGather:
			return w.gather(u, o)
		case OrderReturn:
			return w.returnOrder(u)
		case OrderBuild:
			return w.build(u, o)
		case OrderConstruct:
			return w.construct(u, o)
		case OrderRepair:
			return w.repair(u, o)
		}
	}
	if u.Def.HealRate > 0 && (o == nil || o.Kind == OrderAttackMove) {
		if v, busy := w.heal(u); busy {
			return v
		}
	} else {
		u.healing = 0
	}
	// Workers only fight when told to (attack order or attack-move).
	if u.Phased() {
		u.target = 0
	} else if (o == nil && !u.Def.Worker || o != nil && o.Kind == OrderAttackMove) && u.Def.Armed() {
		if t := w.byID[u.target]; t == nil || !w.VisibleTo(u.Owner, t) || (u.pinned() && !inReach(u, t)) {
			u.target = 0
		}
		// Scanning for targets is staggered across ticks to spread the cost.
		if u.target == 0 && (w.Tick+uint64(u.ID))%3 == 0 {
			u.target = w.acquire(u)
		}
		if t := w.byID[u.target]; t != nil {
			return w.engage(u, t, !u.pinned())
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

// pinned reports whether the unit stays put when fighting: holding
// position, or in an immobile SHIFT mode.
func (u *Unit) pinned() bool { return u.Hold || u.Def.Speed == 0 }

// acquire picks the nearest visible enemy within the unit's acquisition
// range (weapon range when pinned, sight otherwise), preferring units over
// buildings. Enemies inside a weapon's minimum range are ignored.
func (w *World) acquire(u *Unit) UnitID {
	reach := u.Def.Sight
	if u.pinned() {
		reach = u.Def.Weapon.Range
	}
	best, bestScore := UnitID(0), math.Inf(1)
	for _, e := range w.Units {
		if e.Owner == u.Owner || e.HP <= 0 || !w.VisibleTo(u.Owner, e) {
			continue
		}
		d := gap(u, e)
		if d > reach || tooClose(u, d) {
			continue
		}
		if e.Def.Building {
			d += 100
		}
		if d < bestScore {
			best, bestScore = e.ID, d
		}
	}
	return best
}

// engage fires at t if in range, otherwise chases it (if allowed).
func (w *World) engage(u, t *Unit, canMove bool) geom.Vec {
	u.resetProgress()
	wp := u.Def.Weapon
	if d := gap(u, t); tooClose(u, d) {
		// Too close to hit: let it pick something else.
		u.target = 0
		return geom.Vec{}
	} else if d <= wp.Range {
		u.Facing = t.Pos.Sub(u.Pos).Angle()
		u.firing = true
		if u.Cooldown == 0 {
			u.Cooldown = cooldownTicks(wp.Cooldown)
			w.hits = append(w.hits, hit{t.ID, wp.DamageTo(t.Def)})
			if wp.Splash > 0 {
				w.splash(u, t)
			}
			dir := t.Pos.Sub(u.Pos).Norm()
			w.Events = append(w.Events, Event{
				Kind: EvShot, Owner: u.Owner, Type: u.Type, Attack: wp.Type,
				From:   u.Pos.Add(dir.Scale(u.Def.Radius)),
				To:     t.Pos.Sub(dir.Scale(min(t.Def.Radius, 1) * 0.5)),
				Splash: wp.Splash,
			})
		}
		return geom.Vec{}
	}
	if !canMove || u.Def.Speed == 0 {
		return geom.Vec{}
	}
	if t.Def.Building {
		return w.walk(u, w.approach(u.Pos, u.Def.Radius, t.Rect()))
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
	if u.Type == faction.Glint && u.Phased() {
		d := goal.Sub(u.Pos)
		if d.Len() <= speed {
			return d
		}
		return d.Scale(speed / d.Len())
	}
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
