package sim

import (
	"math"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
)

// Brood SHIFT abilities: Burrow hides a unit, Pounce leaps onto an enemy,
// Brood Burst summons short-lived Mites, and re-morph turns a unit back
// into a cocoon that hatches as another form.

const (
	// leapSpeed is how fast (tiles per second) a pouncing unit flies.
	leapSpeed = 12
	// remorphTimeShare is re-morph time as a share of the form's BuildTime.
	remorphTimeShare = 0.5
)

// remorph records what a re-morphing cocoon was and what it paid, so a
// cancel can restore and refund it.
type remorph struct {
	from           faction.UnitType
	crystals, flux int
}

// Hidden reports whether u is burrowed (fully underground), so enemies
// can't see or target it.
func (u *Unit) Hidden() bool { return u.Def.Burrowed && u.shifting == 0 }

// Leaping reports whether u is mid-Pounce.
func (u *Unit) Leaping() bool { return u.leapTo != 0 }

// CastLeft is the seconds until u's SHIFT ability can be cast again.
func (u *Unit) CastLeft() float64 { return float64(u.castCD) / TicksPerSecond }

// LifeLeft is the seconds a summoned unit has left (0 if it doesn't expire).
func (u *Unit) LifeLeft() float64 { return float64(u.life) / TicksPerSecond }

// Remorphing reports whether u is a cocoon re-morphing from another form
// (rather than a Spawn morphing for the first time).
func (u *Unit) Remorphing() bool { return u.remorph != nil }

// RemorphFrom is the form a re-morphing cocoon started as.
func (u *Unit) RemorphFrom() faction.UnitType {
	if u.remorph == nil {
		return u.Type
	}
	return u.remorph.from
}

// morphTicks is how long u's current cocoon takes to hatch.
func (u *Unit) morphTicks() int {
	bt := faction.Unit(u.Queue[0]).BuildTime
	if u.remorph != nil {
		bt *= remorphTimeShare
	}
	return ticksFor(bt)
}

// queueSupply is the supply u reserves for queue entry t. A re-morph
// hatches one unit, even of a pair form like Biters.
func (u *Unit) queueSupply(t faction.UnitType) float64 {
	if u.remorph != nil {
		return faction.Unit(t).Supply
	}
	return faction.QueueSupply(t)
}

// unitValue is what one unit of type t is worth: its cost split across
// the units one morph yields (a Biter is half a pair).
func unitValue(t faction.UnitType) (crystals, flux int) {
	d := faction.Unit(t)
	return d.Crystals / d.Yield(), d.Flux / d.Yield()
}

// RemorphCost is what re-morphing a `from` unit into a `to` unit costs:
// the difference in value, never negative (a cheaper form is no refund).
func RemorphCost(from, to faction.UnitType) (crystals, flux int) {
	fc, ff := unitValue(from)
	tc, tf := unitValue(to)
	return max(0, tc-fc), max(0, tf-ff)
}

// Remorphable reports whether u is a Brood unit that can re-morph: not a
// building, Spawn, cocoon, or summon.
func Remorphable(u *Unit) bool {
	d := faction.Unit(u.Type)
	return d.Faction == faction.Brood && !d.Building && !d.Morphs && d.Life == 0
}

// CanRemorph reports whether u could ever re-morph into t: a different
// Spawn form.
func CanRemorph(u *Unit, t faction.UnitType) bool {
	return Remorphable(u) && u.Type != t && trains(faction.Unit(faction.Spawn), t)
}

// remorphOne turns the first eligible unit in us into a cocoon that will
// hatch as t, paying the value difference. One unit per command.
func (w *World) remorphOne(p PlayerID, us []*Unit, t faction.UnitType) {
	var u *Unit
	same := false
	for _, c := range us {
		if c.Type == t {
			same = true
		}
		if CanRemorph(c, t) && c.Complete && c.shifting == 0 && !c.Leaping() && !c.Morphing() {
			u = c
			break
		}
	}
	if u == nil {
		if same {
			w.notice(p, NoticeSameForm, t)
		}
		return
	}
	if r, missing := w.Missing(p, t); missing {
		w.notice(p, NoticeRequires, r)
		return
	}
	pl := &w.Players[p]
	from := faction.Unit(u.Type)
	to := faction.Unit(t)
	if d := to.Supply - from.Supply; d > 0 && pl.Supply+d > pl.SupplyCap+1e-9 {
		w.notice(p, NoticeSupply, faction.Get(pl.Faction).Supply)
		return
	}
	c, f := RemorphCost(u.Type, t)
	if n := w.pay(p, faction.UnitDef{Crystals: c, Flux: f}); n != NoticeNone {
		w.notice(p, n, t)
		return
	}
	pl.Supply += to.Supply - from.Supply

	cocoon := faction.Unit(faction.Spawn)
	cocoon.HP, cocoon.Radius = from.HP, from.Radius
	u.remorph = &remorph{from: u.Type, crystals: c, flux: f}
	u.Type, u.Def = faction.Spawn, cocoon
	u.HP = min(u.HP, cocoon.HP)
	u.Mode, u.modeLeft, u.shifting, u.shiftTotal = 0, 0, 0, 0
	u.Queue, u.Progress = []faction.UnitType{t}, 0
	u.clearOrders()
	u.Hold, u.Cooldown, u.castCD = false, 0, 0
	w.shiftEvent(u)
}

// unRemorph cancels a re-morph: the cocoon reverts to its old form and
// the cost is refunded.
func (w *World) unRemorph(u *Unit) {
	r := u.remorph
	pl := &w.Players[u.Owner]
	pl.Crystals += r.crystals
	pl.Flux += r.flux
	def := faction.Unit(r.from)
	pl.Supply -= u.Def.Supply + faction.Unit(u.Queue[0]).Supply - def.Supply
	u.Type, u.Def = r.from, def
	u.HP = min(u.HP, def.HP)
	u.Queue, u.Progress, u.remorph = nil, 0, nil
}

// cast fires u's instant SHIFT ability. It returns a notice if it can't.
func (w *World) cast(u *Unit, m *faction.Mode) Notice {
	if u.castCD > 0 {
		return NoticeCooldown
	}
	switch {
	case m.Leap > 0:
		t := w.pounceTarget(u, m.Leap)
		if t == nil {
			return NoticeNoTarget
		}
		u.leapTo, u.leapLeft = t.ID, ticksFor((m.Leap+1)/leapSpeed)
		u.Hold, u.mining = false, 0
		u.clearOrders()
		u.Orders = append(u.Orders, Order{Kind: OrderAttack, Target: t.ID})
	case m.SummonCount > 0:
		w.summon(u, m.Summon, m.SummonCount)
	}
	u.castCD = ticksFor(m.Cooldown)
	w.shiftEvent(u)
	return NoticeNone
}

// pounceTarget is what u leaps at: its current target if close enough,
// else the nearest visible enemy (units before buildings) within reach.
func (w *World) pounceTarget(u *Unit, reach float64) *Unit {
	if t := w.byID[u.target]; t != nil && w.VisibleTo(u.Owner, t) && gap(u, t) <= reach {
		return t
	}
	var best *Unit
	bestScore := math.Inf(1)
	for _, e := range w.Units {
		if e.Owner == u.Owner || e.HP <= 0 || !w.VisibleTo(u.Owner, e) {
			continue
		}
		d := gap(u, e)
		if d > reach {
			continue
		}
		if e.Def.Building {
			d += 100
		}
		if d < bestScore {
			best, bestScore = e, d
		}
	}
	return best
}

// leap moves a pouncing unit toward its target, ignoring other units, and
// lands with a hit once it's in melee reach.
func (w *World) leap(u *Unit) geom.Vec {
	t := w.byID[u.leapTo]
	u.leapLeft--
	if t == nil || t.HP <= 0 || u.leapLeft <= 0 {
		u.leapTo = 0
		return geom.Vec{}
	}
	d := t.Pos.Sub(u.Pos)
	u.Facing = d.Angle()
	step := leapSpeed / float64(TicksPerSecond)
	if g := gap(u, t); g <= u.Def.Weapon.Range+step {
		u.leapTo = 0
		u.target = t.ID
		dmg := faction.Unit(u.Type).Shift.LeapDamage
		w.hits = append(w.hits, hit{t.ID, faction.Weapon{Damage: dmg}.DamageTo(t.Def)})
		w.Events = append(w.Events, Event{Kind: EvShot, Owner: u.Owner, Type: u.Type, From: u.Pos, To: t.Pos, Splash: 0.6})
		return d.Scale(max(0, g-u.Def.Weapon.Range) / d.Len())
	}
	return d.Scale(step / d.Len())
}

// summon spawns n units of type t in a ring around u. They join u's fight
// if it has one.
func (w *World) summon(u *Unit, t faction.UnitType, n int) {
	def := faction.Unit(t)
	target := w.byID[u.target]
	for i := range n {
		a := u.Facing + float64(i)*2*math.Pi/float64(n)
		pos := u.Pos.Add(geom.V(math.Cos(a), math.Sin(a)).Scale(u.Def.Radius + def.Radius + 0.2))
		s := w.Spawn(u.Owner, t, pos)
		s.Facing, s.PrevFacing = a, a
		s.life = ticksFor(def.Life)
		if target != nil {
			s.Orders = append(s.Orders, Order{Kind: OrderAttack, Target: target.ID})
		}
		w.Events = append(w.Events, Event{Kind: EvUnitReady, Owner: u.Owner, Type: t, From: s.Pos})
	}
}

// tickBrood advances cast cooldowns and summon lifetimes. It reports
// whether u expired this tick.
func (w *World) tickBrood(u *Unit) bool {
	if u.castCD > 0 {
		u.castCD--
	}
	if u.life > 0 {
		u.life--
		if u.life == 0 {
			u.HP = 0
			return true
		}
	}
	return false
}
