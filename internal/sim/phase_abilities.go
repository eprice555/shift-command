package sim

import (
	"math"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
)

const (
	warpInTime  = 2
	fusionRange = 3
)

// Hidden reports whether enemies can't see or target u.
func (u *Unit) Hidden() bool {
	return u.Def.Burrowed && u.shifting == 0 || u.Phased() || u.WarpingIn()
}

// Phased reports whether u is intangible.
func (u *Unit) Phased() bool { return u.phaseLeft > 0 }

// PhaseLeft is the seconds u remains phased.
func (u *Unit) PhaseLeft() float64 { return float64(u.phaseLeft) / TicksPerSecond }

// WarpingIn reports whether u is still materializing at an Anchor.
func (u *Unit) WarpingIn() bool { return u.warpIn > 0 }

// WarpResearching reports whether this Resonator is researching Warp.
func (u *Unit) WarpResearching() bool { return u.warpResearch > 0 }

// WarpFrac is the fraction of u's warp-in animation that has elapsed.
func (u *Unit) WarpFrac() float64 {
	if !u.WarpingIn() || u.warpTotal == 0 {
		return 0
	}
	return 1 - float64(u.warpIn)/float64(u.warpTotal)
}

// Dashing reports whether u is performing a Phasebound dash.
func (u *Unit) Dashing() bool { return u.dashTo != 0 }

func (w *World) castPhase(u *Unit, m *faction.Mode) Notice {
	if m.DashRange > 0 {
		t := w.pounceTarget(u, m.DashRange)
		if t == nil {
			return NoticeNoTarget
		}
		u.clearOrders()
		u.dashTo, u.dashLeft = t.ID, ticksFor(1)
		u.Orders = append(u.Orders, Order{Kind: OrderAttack, Target: t.ID})
	}
	if m.BlinkRange > 0 {
		d := geom.V(math.Cos(u.Facing), math.Sin(u.Facing)).Scale(m.BlinkRange)
		u.Pos = w.Map.Grid.ResolveCircle(w.Map.Grid.SnapPassable(w.Map.Clamp(u.Pos.Add(d), u.Def.Radius)), u.Def.Radius)
		u.PrevPos = u.Pos
		u.resetNav()
	}
	if m.PhaseRadius > 0 {
		r2 := m.PhaseRadius * m.PhaseRadius
		for _, ally := range w.Units {
			if ally.Owner == u.Owner && !ally.Def.Building && ally.Pos.Dist2(u.Pos) <= r2 {
				ally.phaseLeft = max(ally.phaseLeft, ticksFor(m.PhaseDuration))
				ally.target = 0
			}
		}
	} else {
		u.phaseLeft = max(u.phaseLeft, ticksFor(m.PhaseDuration))
	}
	return NoticeNone
}

func (u *Unit) WarpResearchFrac() float64 {
	if u.warpResearch <= 0 {
		return 0
	}
	return float64(u.Progress) / float64(ticksFor(faction.WarpResearchTime))
}

func (w *World) researchWarp(p PlayerID, units []*Unit) {
	pl := &w.Players[p]
	if pl.WarpResearched {
		return
	}
	for _, u := range w.Units {
		if u.Owner == p && u.warpResearch > 0 {
			w.notice(p, NoticeBusy, faction.Resonator)
			return
		}
	}
	for _, b := range units {
		if b.Type != faction.Resonator || !b.Complete {
			continue
		}
		if w.Unpowered(b) {
			w.notice(p, NoticeNeedPower, faction.Resonator)
			return
		}
		if len(b.Queue) > 0 {
			w.notice(p, NoticeBusy, faction.Resonator)
			return
		}
		cost := faction.UnitDef{Crystals: faction.WarpResearchCrystals, Flux: faction.WarpResearchFlux}
		if n := w.pay(p, cost); n != NoticeNone {
			w.notice(p, n, faction.Resonator)
			return
		}
		b.Progress = 0
		b.warpResearch = ticksFor(faction.WarpResearchTime)
		return
	}
}

func (w *World) researchWarpTick(b *Unit) {
	b.warpResearch--
	b.Progress++
	if b.warpResearch == 0 {
		w.Players[b.Owner].WarpResearched = true
		b.Progress = 0
	}
}

func (w *World) warpTrainedUnit(b *Unit, t faction.UnitType) *Unit {
	def := faction.Unit(t)
	from := b.Pos
	if b.HasRally {
		from = b.Rally
	}
	var anchor *Unit
	best := math.Inf(1)
	for _, u := range w.Units {
		if u.Owner != b.Owner || u.Type != faction.Anchor || !u.Complete || u.HP <= 0 {
			continue
		}
		if d := u.Pos.Dist2(from); d < best {
			anchor, best = u, d
		}
	}
	if anchor == nil {
		return w.Spawn(b.Owner, t, w.approach(from, def.Radius, b.Rect()))
	}
	pos := w.approach(from, def.Radius, anchor.Rect())
	nu := w.Spawn(b.Owner, t, pos)
	nu.warpTotal = ticksFor(warpInTime)
	nu.warpIn = nu.warpTotal
	return nu
}

func (w *World) fuse(p PlayerID, units []*Unit) {
	if req, missing := w.Missing(p, faction.Radiant); missing {
		w.notice(p, NoticeRequires, req)
		return
	}
	var seers []*Unit
	for _, u := range units {
		if u.Owner != p || u.Type != faction.Seer || !u.Complete || u.Morphing() ||
			u.Shifting() || u.Leaping() || u.Phased() || u.WarpingIn() {
			continue
		}
		for _, other := range seers {
			if other.Pos.Dist(u.Pos) <= fusionRange {
				a, b := other, u
				def := faction.Unit(faction.Radiant)
				pos := a.Pos.Lerp(b.Pos, 0.5)
				a.Pos = w.Map.Grid.ResolveCircle(w.Map.Grid.SnapPassable(pos), def.Radius)
				a.PrevPos = a.Pos
				a.Type, a.Def = faction.Radiant, def
				a.HP = min(def.HP, a.HP+b.HP)
				a.Shield = min(def.Shield, a.Shield+b.Shield)
				a.Mode, a.modeLeft, a.castCD = 0, 0, 0
				a.clearOrders()
				b.clearOrders()
				b.HP, b.consumed = 0, true
				w.Events = append(w.Events, Event{Kind: EvUnitReady, Owner: p, Type: faction.Radiant, From: a.Pos})
				return
			}
		}
		seers = append(seers, u)
	}
	w.notice(p, NoticeNeedSeers, faction.Radiant)
}
