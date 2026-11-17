package sim

import (
	"math"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
)

// landReach is how far (in tiles) a packed building may slide to find a
// clear landing site.
const landReach = 2

// Shifted reports whether the unit is in (or switching into) its SHIFT mode.
func (u *Unit) Shifted() bool { return u.Mode == 1 }

// Shifting reports whether the unit is mid-transition between modes.
func (u *Unit) Shifting() bool { return u.shifting > 0 }

// Lifted reports whether a packed building is airborne (off its footprint).
func (u *Unit) Lifted() bool { return u.lifted }

// ShiftFrac is how far into its SHIFT mode the unit is, in [0, 1]: 0 in base
// mode, 1 fully shifted, in between while switching. Renderers animate
// mode changes with it.
func (u *Unit) ShiftFrac() float64 {
	f := 0.0
	if u.shifting > 0 && u.shiftTotal > 0 {
		f = float64(u.shifting) / float64(u.shiftTotal)
	}
	if u.Mode == 1 {
		return 1 - f
	}
	return f
}

// ModeLeft is the seconds remaining on a timed SHIFT mode (0 if none).
func (u *Unit) ModeLeft() float64 { return float64(u.modeLeft) / TicksPerSecond }

// shift applies a CmdShift: Index 0 or 1 sets that mode; -1 toggles per
// unit type (if any unit of a type is in base mode, all of that type
// shift; otherwise all of that type return to base). Timed modes like
// Overdrive always (re)activate.
func (w *World) shift(c *Command, all []*Unit) {
	var want [faction.UnitTypeCount]int
	if c.Index < 0 {
		for _, u := range all {
			if u.Def.Shift != nil && u.Mode == 0 {
				want[u.Type] = 1
			}
		}
	} else {
		for i := range want {
			want[i] = min(c.Index, 1)
		}
	}
	var refused Notice
	for _, u := range all {
		m := faction.Unit(u.Type).Shift
		if m == nil || !u.Complete || u.shifting > 0 {
			continue
		}
		to := want[u.Type]
		if m.Timed() && c.Index < 0 {
			to = 1
		}
		if to == u.Mode && !(to == 1 && m.Timed()) {
			continue
		}
		if n := w.setMode(u, to); n != NoticeNone {
			refused = n
		}
	}
	if refused != NoticeNone {
		w.notice(c.Player, refused, 0)
	}
}

// setMode switches u into mode `to`, starting its transition. It returns
// a notice if the switch isn't possible right now.
func (w *World) setMode(u *Unit, to int) Notice {
	base := faction.Unit(u.Type)
	m := base.Shift
	if to == 1 {
		switch {
		case m.HPCost > 0 && u.HP <= m.HPCost:
			return NoticeLowHP
		case m.Pack && len(u.Queue) > 0:
			return NoticeBusy
		}
		u.HP -= m.HPCost
		if m.Timed() {
			u.modeLeft = ticksFor(m.Duration)
		}
		if u.Mode == 1 {
			// Re-triggering a timed mode just pays again and refreshes it.
			w.shiftEvent(u)
			return NoticeNone
		}
	}
	if m.Pack && to == 0 {
		x, y, ok := w.landSite(u)
		if !ok {
			return NoticeCantLand
		}
		w.land(u, x, y)
	}
	u.Mode = to
	u.Def = base
	if to == 1 {
		u.Def = base.Shifted()
	} else {
		u.modeLeft = 0
	}
	u.HP = min(u.HP, u.Def.HP)
	u.shiftTotal = 0
	if m.Time > 0 {
		u.shiftTotal = ticksFor(m.Time)
	}
	u.shifting = u.shiftTotal
	if m.Immobile || m.Pack {
		u.clearOrders()
		u.Hold, u.mining = false, 0
	}
	w.shiftEvent(u)
	return NoticeNone
}

func (w *World) shiftEvent(u *Unit) {
	w.Events = append(w.Events, Event{Kind: EvShift, Owner: u.Owner, Type: u.Type, From: u.Pos})
}

// tickShift advances mode timers. It reports whether u is busy switching
// modes this tick (and so can't act).
func (w *World) tickShift(u *Unit) bool {
	if u.shifting > 0 {
		u.shifting--
		u.target = 0
		if u.shifting == 0 && u.Def.Building && u.Mode == 1 && !u.lifted {
			// Packing finished: lift off the footprint.
			u.lifted = true
			w.Map.Occupy(u.TileX, u.TileY, u.Def.Size, -1)
		}
		return true
	}
	if u.modeLeft > 0 {
		u.modeLeft--
		if u.modeLeft == 0 {
			w.setMode(u, 0)
		}
	}
	return false
}

// fly moves a lifted building straight toward its next order's point,
// ignoring terrain and units.
func (w *World) fly(u *Unit) geom.Vec {
	for len(u.Orders) > 0 && u.Orders[0].Point == (geom.Vec{}) {
		u.popOrder()
	}
	if len(u.Orders) == 0 {
		return geom.Vec{}
	}
	h := float64(u.Def.Size) / 2
	goal := w.Map.Clamp(u.Orders[0].Point, h)
	d := goal.Sub(u.Pos)
	step := u.Def.Speed / TicksPerSecond
	if d.Len() <= step {
		u.popOrder()
		return d
	}
	return d.Scale(step / d.Len())
}

// syncTiles keeps a lifted building's footprint coordinates (used for
// ghosts and the minimap) under its position.
func (u *Unit) syncTiles() {
	h := float64(u.Def.Size) / 2
	u.TileX = int(math.Floor(u.Pos.X - h + 0.5))
	u.TileY = int(math.Floor(u.Pos.Y - h + 0.5))
}

// landSite finds the clear footprint nearest a lifted building, within
// landReach tiles of where it hovers.
func (w *World) landSite(u *Unit) (int, int, bool) {
	if !u.lifted {
		// Still packing: it hasn't left its own footprint.
		return u.TileX, u.TileY, true
	}
	h := float64(u.Def.Size) / 2
	bestX, bestY, bestD := 0, 0, math.Inf(1)
	for dy := -landReach; dy <= landReach; dy++ {
		for dx := -landReach; dx <= landReach; dx++ {
			x, y := u.TileX+dx, u.TileY+dy
			if !w.clearSite(x, y, u.Def.Size) {
				continue
			}
			c := geom.V(float64(x)+h, float64(y)+h)
			if d := c.Dist2(u.Pos); d < bestD {
				bestX, bestY, bestD = x, y, d
			}
		}
	}
	return bestX, bestY, bestD < math.Inf(1)
}

func (w *World) clearSite(x, y, size int) bool {
	for ty := y; ty < y+size; ty++ {
		for tx := x; tx < x+size; tx++ {
			if w.Map.Blocked(tx, ty) {
				return false
			}
		}
	}
	return true
}

// land sets a lifted building down on tile (x, y), pushing units aside.
func (w *World) land(u *Unit, x, y int) {
	if !u.lifted {
		return
	}
	u.lifted = false
	u.TileX, u.TileY = x, y
	h := float64(u.Def.Size) / 2
	u.Pos = geom.V(float64(x)+h, float64(y)+h)
	w.Map.Occupy(x, y, u.Def.Size, 1)
	w.clearFootprint(u.Rect())
}

// clearFootprint pushes mobile units out of a newly occupied footprint.
func (w *World) clearFootprint(rect geom.Rect) {
	for _, o := range w.Units {
		if !o.Def.Building && rectDist(rect, o.Pos) < o.Def.Radius {
			o.Pos = w.Map.ResolveCircle(w.Map.SnapPassable(w.approach(o.Pos, o.Def.Radius, rect)), o.Def.Radius)
			o.resetNav()
		}
	}
}

// inReach reports whether t is inside u's weapon range band.
func inReach(u, t *Unit) bool {
	d := gap(u, t)
	return d <= u.Def.Weapon.Range && !tooClose(u, d)
}

// tooClose reports whether gap d is inside u's weapon minimum range.
func tooClose(u *Unit, d float64) bool {
	return u.Def.Weapon.MinRange > 0 && d < u.Def.Weapon.MinRange
}

// splash deals splash damage around a shot that hit t.
func (w *World) splash(u, t *Unit) {
	wp := u.Def.Weapon
	for _, o := range w.Units {
		if o == t || o.Owner == u.Owner || o.HP <= 0 {
			continue
		}
		d := o.Pos.Dist(t.Pos) - o.Def.Radius
		if o.Def.Building {
			d = rectDist(o.Rect(), t.Pos)
		}
		if d <= wp.Splash {
			w.hits = append(w.hits, hit{o.ID, wp.DamageTo(o.Def) * faction.SplashShare})
		}
	}
}

// aura heals every damaged Bio ally in range at once.
func (w *World) aura(u *Unit) {
	u.healing = 0
	for _, o := range w.Units {
		if o.Owner != u.Owner || !o.Def.Bio || o.HP >= o.Def.HP || gap(u, o) > u.Def.HealRange {
			continue
		}
		w.hits = append(w.hits, hit{o.ID, -u.Def.HealRate / TicksPerSecond})
		u.healing = o.ID
	}
	if (w.Tick+uint64(u.ID))%TicksPerSecond == 0 {
		w.Events = append(w.Events, Event{Kind: EvAura, Owner: u.Owner, Type: u.Type, From: u.Pos, Splash: u.Def.HealRange})
	}
}
