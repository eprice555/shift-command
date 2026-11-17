package ai

import (
	"math"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/sim"
)

// SHIFT tuning, in tiles and ticks.
const (
	shiftHold    = 3 * sim.TicksPerSecond  // min time between a unit's mode toggles
	siegeLockout = 10 * sim.TicksPerSecond // no re-sieging this long after a retreat
	siegeIn      = 12                      // Bastion: siege with an enemy this close...
	siegeMin     = 3                       // ... but no closer than this
	siegeOut     = 14                      // unsiege with nothing this close
	plantIn      = 8                       // Strider
	plantOut     = 10
	overdriveIn  = 6  // Trooper: overdrive with enemies this close
	overdriveHP  = 25 // ... and at least this much HP
	boostClear   = 9  // Outrider: boost only with no enemy this close
	postHurt     = 3  // Mender: field post with this many hurt allies nearby
)

// shifts uses the army's SHIFT modes: siege up near enemies, pack up when
// they're gone, overdrive into fights, and boost while travelling.
func (a *AI) shifts(w *sim.World) {
	if a.lastShift == nil {
		a.lastShift = map[sim.UnitID]uint64{}
	}
	var on, off []sim.UnitID
	for _, u := range a.s.army {
		m := faction.Unit(u.Type).Shift
		if m == nil || u.Shifting() || !u.Complete {
			continue
		}
		if last, ok := a.lastShift[u.ID]; ok && w.Tick-last < shiftHold && !m.Timed() {
			continue
		}
		d := a.nearestEnemy(w, u)
		want := u.Mode
		switch u.Type {
		case faction.Bastion:
			switch {
			case u.Mode == 0 && d <= siegeIn && d >= siegeMin && w.Tick >= a.siegeOK:
				want = 1
			case u.Mode == 1 && d > siegeOut:
				want = 0
			}
		case faction.Strider:
			switch {
			case u.Mode == 0 && d <= plantIn && w.Tick >= a.siegeOK:
				want = 1
			case u.Mode == 1 && d > plantOut:
				want = 0
			}
		case faction.Trooper:
			if u.Mode == 0 && d <= overdriveIn && u.HP >= overdriveHP {
				want = 1
			}
		case faction.Outrider:
			switch {
			case u.Mode == 0 && d > boostClear && len(u.Orders) > 0 && u.Target() == 0:
				want = 1
			case u.Mode == 1 && (d <= boostClear || len(u.Orders) == 0):
				want = 0
			}
		case faction.Mender:
			hurt := a.hurtNear(u, m.AuraRange)
			switch {
			case u.Mode == 0 && hurt >= postHurt && w.Tick >= a.siegeOK:
				want = 1
			case u.Mode == 1 && hurt == 0:
				want = 0
			}
		}
		if want == u.Mode {
			continue
		}
		a.lastShift[u.ID] = w.Tick
		if want == 1 {
			on = append(on, u.ID)
		} else {
			off = append(off, u.ID)
		}
	}
	if len(on) > 0 {
		a.cmd(sim.Command{Kind: sim.CmdShift, Units: on, Index: 1})
	}
	if len(off) > 0 {
		a.cmd(sim.Command{Kind: sim.CmdShift, Units: off, Index: 0})
	}
}

// unsiege returns immobile units to their mobile modes (before a retreat)
// and holds off re-sieging for a while.
func (a *AI) unsiege(w *sim.World) {
	a.siegeOK = w.Tick + siegeLockout
	var ids []sim.UnitID
	for _, u := range a.s.army {
		if m := faction.Unit(u.Type).Shift; m != nil && m.Immobile && u.Mode == 1 {
			ids = append(ids, u.ID)
		}
	}
	if len(ids) > 0 {
		a.cmd(sim.Command{Kind: sim.CmdShift, Units: ids, Index: 0})
	}
}

// nearestEnemy is the distance from u to the closest enemy it can see.
func (a *AI) nearestEnemy(w *sim.World, u *sim.Unit) float64 {
	best := math.Inf(1)
	for _, e := range w.Units {
		if e.Owner == a.P || !w.VisibleTo(a.P, e) {
			continue
		}
		best = min(best, u.Pos.Dist(e.Pos)-e.Def.Radius)
	}
	return best
}

// hurtNear counts damaged own non-building units within r of u.
func (a *AI) hurtNear(u *sim.Unit, r float64) int {
	n := 0
	for _, o := range a.s.army {
		if o.HP < o.Def.HP && o.Pos.Dist(u.Pos) <= r {
			n++
		}
	}
	return n
}
