package sim

import (
	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
)

// absorb drains t's shield by a positive hit and returns the damage left
// for its HP. Negative damage (healing, repair) passes straight through.
func (w *World) absorb(t *Unit, dmg float64) float64 {
	if dmg <= 0 {
		return dmg
	}
	t.hurtTick = w.Tick + 1
	a := min(t.Shield, dmg)
	t.Shield -= a
	return dmg - a
}

// rechargeShield regenerates u's shield once it has gone ShieldDelay
// seconds without taking damage.
func (w *World) rechargeShield(u *Unit) {
	if u.Def.Shield <= 0 || !u.Complete || u.HP <= 0 || u.Shield >= u.Def.Shield {
		return
	}
	if w.Tick+1 < u.hurtTick+uint64(ticksFor(faction.ShieldDelay)) {
		return
	}
	u.Shield = min(u.Def.Shield, u.Shield+float64(faction.ShieldRegen)/TicksPerSecond)
}

// ShieldsRecharging reports whether u's shield is regenerating right now.
func (w *World) ShieldsRecharging(u *Unit) bool {
	return u.Def.Shield > 0 && u.Complete && u.Shield < u.Def.Shield &&
		w.Tick+1 >= u.hurtTick+uint64(ticksFor(faction.ShieldDelay))
}

// needsCare reports whether healer u would treat patient o: Bio units
// missing HP, or (for shield restorers) units missing shield.
func needsCare(u, o *Unit) bool {
	if u.Def.ShieldHeal {
		return !o.Def.Building && o.Def.Shield > 0 && o.Shield < o.Def.Shield
	}
	return o.Def.Bio && o.HP < o.Def.HP
}

// PoweredAt reports whether point at lies in a power field of one of p's
// finished Anchors.
func (w *World) PoweredAt(p PlayerID, at geom.Vec) bool {
	for _, u := range w.Units {
		if u.Owner == p && u.Complete && u.Def.PowerRadius > 0 && u.HP > 0 &&
			u.Pos.Dist2(at) <= u.Def.PowerRadius*u.Def.PowerRadius {
			return true
		}
	}
	return false
}

// Unpowered reports whether b needs power and has lost it. Unpowered
// buildings pause production.
func (w *World) Unpowered(b *Unit) bool {
	return b.Def.NeedsPower && b.Complete && !w.PoweredAt(b.Owner, b.Pos)
}

// PowerFields returns the centers and radii of p's finished power fields.
func (w *World) PowerFields(p PlayerID) (centers []geom.Vec, radii []float64) {
	for _, u := range w.Units {
		if u.Owner == p && u.Complete && u.Def.PowerRadius > 0 {
			centers = append(centers, u.Pos)
			radii = append(radii, u.Def.PowerRadius)
		}
	}
	return centers, radii
}

func footCenter(size, x, y int) geom.Vec {
	h := float64(size) / 2
	return geom.V(float64(x)+h, float64(y)+h)
}
