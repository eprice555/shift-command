package ui

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
	"github.com/eprice555/shift-command/internal/sim"
)

// Phasebound visuals: crisp triangles, diamonds, and rings that hover and
// flicker, with shield halos and Anchor power fields.

var colShield = color.RGBA{0x9A, 0xEE, 0xFF, 0xFF}

func isPhase(t faction.UnitType) bool { return faction.Unit(t).Faction == faction.Phasebound }

// phaseShape appends a Phasebound unit's outline (radius 1, facing +X).
func phaseShape(dst []geom.Vec, t faction.UnitType) []geom.Vec {
	switch t {
	case faction.Drifter:
		return append(dst, geom.V(1, 0), geom.V(-0.75, 0.8), geom.V(-0.75, -0.8))
	case faction.Warden:
		return append(dst, geom.V(1.05, 0), geom.V(-0.8, 0.95), geom.V(-0.45, 0), geom.V(-0.8, -0.95))
	case faction.Glint:
		return append(dst, geom.V(1.3, 0), geom.V(-0.5, 0.5), geom.V(-0.95, 0), geom.V(-0.5, -0.5))
	case faction.Seer:
		return append(dst, geom.V(1, 0), geom.V(0, 0.75), geom.V(-1, 0), geom.V(0, -0.75))
	case faction.Conduit:
		return ngonPts(dst, 0.9, 10, 0)
	case faction.Monolith:
		return append(dst, geom.V(1.1, 0), geom.V(0.35, 0.8), geom.V(-0.7, 0.62), geom.V(-1, 0), geom.V(-0.7, -0.62), geom.V(0.35, -0.8))
	}
	return ngonPts(dst, 0.8, 3, 0)
}

func ngonPts(dst []geom.Vec, r float64, n int, a0 float64) []geom.Vec {
	for i := range n {
		s, c := math.Sincos(a0 + float64(i)*2*math.Pi/float64(n))
		dst = append(dst, geom.V(c*r, s*r))
	}
	return dst
}

// phaseDetail adds a Phasebound unit's inner lines via seg (unit space).
func phaseDetail(seg func(a, b geom.Vec), t faction.UnitType) {
	switch t {
	case faction.Warden:
		seg(geom.V(0.45, 0), geom.V(-0.2, 0.4))
		seg(geom.V(0.45, 0), geom.V(-0.2, -0.4))
	case faction.Seer:
		d := []geom.Vec{geom.V(0.4, 0), geom.V(0, 0.3), geom.V(-0.4, 0), geom.V(0, -0.3)}
		for i := range d {
			seg(d[i], d[(i+1)%len(d)])
		}
	case faction.Conduit:
		tri := ngonPts(nil, 0.5, 3, 0)
		for i := range tri {
			seg(tri[i], tri[(i+1)%3])
		}
	case faction.Monolith:
		seg(geom.V(0.7, 0), geom.V(-0.6, 0))
		seg(geom.V(0.1, 0.45), geom.V(0.1, -0.45))
	}
}

// phaseBob is how far (screen px) a hovering Phasebound unit floats up.
func phaseBob(id sim.UnitID, frame int, scale float64) float64 {
	return 0.07 * scale * (1 + math.Sin(float64(frame)*0.06+float64(id)*1.7))
}

// phaseFlicker returns a shimmer scale and a rare sideways glitch for a
// Phasebound unit's outline.
func phaseFlicker(id sim.UnitID, frame int) (scale, glitch float64) {
	scale = 1 + 0.025*math.Sin(float64(frame)*0.31+float64(id)*2.3)
	if (frame+int(id)*37)%113 < 3 {
		glitch = 0.12
	}
	return scale, glitch
}

// phaseBuilding adds a Phasebound building: a hovering crystal lattice
// over its footprint. Unfinished ones are warping in: a faint outline
// with a spinning core that grows with progress.
func phaseBuilding(fill, stroke *vector.Path, t faction.UnitType, rect geom.Rect, complete bool, frac, wob float64, cam *Camera) {
	c := cam.ToScreenV(rect.Min.Lerp(rect.Max, 0.5))
	s := (rect.Max.X - rect.Min.X) * cam.Scale()
	c.Y -= s * 0.025 * (1 + math.Sin(wob*0.5))
	outer := ngon(c, s*0.48, 4, 0)
	if !complete {
		poly(stroke, ngon(c, s*(0.15+0.3*frac), 3, wob*0.6)...)
		poly(stroke, ngon(c, s*(0.1+0.2*frac), 3, -wob*0.9)...)
		for i := range outer {
			a, b := outer[i], outer[(i+1)%len(outer)]
			line(stroke, a, a.Lerp(b, 0.25))
			line(stroke, b.Lerp(a, 0.25), b)
		}
		return
	}
	if fill != nil {
		poly(fill, outer...)
	}
	poly(stroke, outer...)
	switch t {
	case faction.Spire:
		poly(stroke, ngon(c, s*0.3, 3, -math.Pi/2)...)
		poly(stroke, ngon(c, s*0.3, 3, math.Pi/2)...)
		poly(stroke, ngon(c, s*0.08, 10, wob*0.2)...)
	case faction.Anchor:
		poly(stroke, ngon(c, s*0.28, 3, -math.Pi/2+wob*0.15)...)
		poly(stroke, ngon(c, s*0.07, 8, 0)...)
	case faction.PhaseExtractor:
		poly(stroke, ngon(c, s*0.22, 4, math.Pi/4)...)
		for i := range 4 {
			sn, cs := math.Sincos(float64(i) * math.Pi / 2)
			line(stroke, c.Add(geom.V(cs, sn).Scale(s*0.22)), c.Add(geom.V(cs, sn).Scale(s*0.36)))
		}
	case faction.Gate:
		poly(stroke, ngon(c, s*0.3, 16, 0)...)
		poly(stroke, ngon(c, s*0.14, 4, wob*0.3)...)
	case faction.Resonator:
		poly(stroke, ngon(c, s*0.32, 16, 0)...)
		poly(stroke, ngon(c, s*0.2, 16, 0)...)
		poly(stroke, ngon(c, s*0.08, 6, wob*0.3)...)
	}
}

// showPower reports whether the viewer is planning Phasebound buildings:
// placing one, or with a Drifter or power-hungry building selected.
func showPower(v *MatchView) bool {
	if v.World.Players[v.Player].Faction != faction.Phasebound {
		return false
	}
	if v.Placing {
		return true
	}
	for _, id := range v.Selected {
		if u := v.World.Unit(id); u != nil && u.Owner == v.Player && (u.Def.Worker || u.Def.NeedsPower || u.Def.PowerRadius > 0) {
			return true
		}
	}
	return false
}

// drawPowerFields shades the player's Anchor power fields on the floor,
// and previews a new Anchor's field while placing one.
func (r *Renderer) drawPowerFields(screen *ebiten.Image, v *MatchView) {
	if !showPower(v) {
		return
	}
	cam := v.Cam
	col := faction.Get(faction.Phasebound).Color
	pulse := 0.85 + 0.15*math.Sin(float64(r.frame)*0.05)
	field := func(c geom.Vec, rad float64, a float64) {
		p := cam.ToScreenV(c)
		rr := float32(rad * cam.Scale())
		vector.FillCircle(screen, float32(p.X), float32(p.Y), rr, withAlpha(col, 0.07*a), true)
		vector.StrokeCircle(screen, float32(p.X), float32(p.Y), rr, 1.5, withAlpha(col, 0.45*a*pulse), true)
	}
	centers, radii := v.World.PowerFields(v.Player)
	for i, c := range centers {
		field(c, radii[i], 1)
	}
	if d := faction.Unit(v.PlaceType); v.Placing && d.PowerRadius > 0 {
		h := float64(d.Size) / 2
		field(geom.V(float64(v.PlaceX)+h, float64(v.PlaceY)+h), d.PowerRadius, 0.6)
	}
}

// drawUnpowered marks the viewer's buildings that have lost power.
func (r *Renderer) drawUnpowered(screen *ebiten.Image, v *MatchView, vis geom.Rect) {
	blink := 0.6 + 0.4*math.Sin(float64(r.frame)*0.2)
	for _, u := range v.World.Units {
		if u.Owner != v.Player || !vis.Contains(u.Pos) || !v.World.Unpowered(u) {
			continue
		}
		rect := u.Rect()
		p := v.Cam.ToScreenV(geom.V((rect.Min.X+rect.Max.X)/2, rect.Min.Y))
		drawCentered(screen, "NO POWER", r.smallFace, p.X, p.Y-8, withAlpha(colBad, blink))
	}
}
