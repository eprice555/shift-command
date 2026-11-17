package ai

import (
	"math"
	"slices"

	"github.com/eprice555/shift-command/internal/geom"
	"github.com/eprice555/shift-command/internal/sim"
)

// Army tuning.
const (
	stagingDist  = 13   // staging point, in path tiles from home
	threatReach  = 13   // enemies this close to our buildings are a threat
	localReach   = 10   // radius for comparing army strength in a fight
	retreatShare = 0.35 // retreat when the army falls below this share
	retreatOdds  = 1.6  // ... or the local enemy is this much stronger
	reinforceMin = 4    // reinforcements leave staging in groups this big
)

// army rallies new units, defends the base, and launches waves.
func (a *AI) army(w *sim.World) {
	s := &a.s
	for _, b := range s.prod {
		if !b.HasRally || b.Rally.Dist(a.staging) > 1 {
			a.cmd(sim.Command{Kind: sim.CmdRally, Units: []sim.UnitID{b.ID}, Point: a.staging})
		}
	}
	threat, threatened := a.threat(w)

	switch a.mode {
	case attack:
		a.push(w)
		return
	case defend:
		if !threatened {
			a.mode = muster
			a.order(sim.CmdAttackMove, a.ids(s.army), a.staging)
			return
		}
	}
	if threatened {
		if a.mode != defend || threat.Dist(a.defendAt) > 5 {
			a.mode, a.defendAt = defend, threat
			a.order(sim.CmdAttackMove, a.ids(s.army), threat)
			return
		}
		a.order(sim.CmdAttackMove, a.idle(s.army), a.defendAt)
		return
	}

	// Muster: gather stragglers, then attack once the wave is big enough.
	var far []sim.UnitID
	for _, u := range s.army {
		if u.Idle() && u.Target() == 0 && u.Pos.Dist(a.staging) > 6 {
			far = append(far, u.ID)
		}
	}
	a.order(sim.CmdAttackMove, far, a.staging)
	if s.armySupply >= a.waveSize || w.Players[a.P].Supply >= 90 && s.armySupply >= firstWave {
		a.mode, a.attackStart = attack, s.armySupply
		a.target = a.pickTarget(w, a.centroid(s.army))
		a.order(sim.CmdAttackMove, a.ids(s.army), a.target)
	}
}

// push runs an attack: chase known enemy buildings (or scout for them),
// feed in reinforcements, and fall back if the fight is lost.
func (a *AI) push(w *sim.World) {
	s := &a.s
	var front, staged []*sim.Unit
	for _, u := range s.army {
		if u.Pos.Dist(a.staging) <= 8 && u.Idle() && u.Target() == 0 {
			staged = append(staged, u)
		} else {
			front = append(front, u)
		}
	}
	if len(front) == 0 && len(staged) < reinforceMin {
		a.fallBack()
		return
	}
	at := a.centroid(front)
	if len(front) == 0 {
		at = a.centroid(staged)
	}
	if s.armySupply < retreatShare*a.attackStart || a.power(w, at, true) > retreatOdds*a.power(w, at, false) {
		a.fallBack()
		a.order(sim.CmdMove, a.ids(s.army), a.staging)
		return
	}

	t := a.pickTarget(w, at)
	if t.Dist(a.target) > 3 {
		a.target = t
		a.order(sim.CmdAttackMove, a.ids(front), t)
	} else {
		a.order(sim.CmdAttackMove, a.idle(front), t)
	}
	if len(staged) >= reinforceMin {
		a.order(sim.CmdAttackMove, a.ids(staged), a.target)
	}
}

func (a *AI) fallBack() {
	a.mode = muster
	a.waveSize = min(maxWave, a.waveSize+waveGrowth)
}

// pickTarget returns the known enemy building nearest `from`, or the next
// place to scout if none is known.
func (a *AI) pickTarget(w *sim.World, from geom.Vec) geom.Vec {
	ids := make([]sim.UnitID, 0, len(w.Seen[a.P]))
	for id := range w.Seen[a.P] {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	best, bestD := geom.Vec{}, math.Inf(1)
	for _, id := range ids {
		p := ghostPos(w.Seen[a.P][id])
		if d := p.Dist(from); d < bestD {
			best, bestD = p, d
		}
	}
	if bestD < math.Inf(1) {
		return best
	}
	// Nothing known: sweep the scouting points in turn.
	for range a.scouts {
		p := a.scouts[a.scout%len(a.scouts)]
		if from.Dist(p) > 5 && !w.Fog[a.P].VisibleAt(p) {
			return p
		}
		a.scout++
	}
	return a.scouts[a.scout%len(a.scouts)]
}

// threat returns the enemy fighter nearest home that is close to one of
// our buildings.
func (a *AI) threat(w *sim.World) (geom.Vec, bool) {
	best, bestD := geom.Vec{}, math.Inf(1)
	for _, e := range w.Units {
		if e.Owner == a.P || e.Def.Building || !w.VisibleTo(a.P, e) || e.Def.Worker && e.Target() == 0 {
			continue
		}
		near := false
		for _, b := range a.s.blds {
			if b.Pos.Dist(e.Pos) <= threatReach {
				near = true
				break
			}
		}
		if !near {
			continue
		}
		if d := e.Pos.Dist(a.home); d < bestD {
			best, bestD = e.Pos, d
		}
	}
	return best, bestD < math.Inf(1)
}

// power sums the fighting strength (HP × DPS) of units near p: the enemy's
// visible fighters, or our own army.
func (a *AI) power(w *sim.World, p geom.Vec, enemy bool) float64 {
	sum := 0.0
	for _, u := range w.Units {
		if u.Def.Building || u.Def.Worker || !u.Def.Armed() || (u.Owner != a.P) != enemy ||
			u.Pos.Dist(p) > localReach || enemy && !w.VisibleTo(a.P, u) {
			continue
		}
		sum += u.HP * u.Def.Weapon.Damage / u.Def.Weapon.Cooldown
	}
	return sum
}

func (a *AI) centroid(us []*sim.Unit) geom.Vec {
	if len(us) == 0 {
		return a.staging
	}
	var c geom.Vec
	for _, u := range us {
		c = c.Add(u.Pos)
	}
	return c.Scale(1 / float64(len(us)))
}

func (a *AI) ids(us []*sim.Unit) []sim.UnitID {
	out := make([]sim.UnitID, 0, len(us))
	for _, u := range us {
		out = append(out, u.ID)
	}
	return out
}

// idle returns the units with nothing to do.
func (a *AI) idle(us []*sim.Unit) []sim.UnitID {
	var out []sim.UnitID
	for _, u := range us {
		if u.Idle() && u.Target() == 0 {
			out = append(out, u.ID)
		}
	}
	return out
}

func (a *AI) order(k sim.CommandKind, ids []sim.UnitID, p geom.Vec) {
	if len(ids) > 0 {
		a.cmd(sim.Command{Kind: k, Units: ids, Point: p})
	}
}
