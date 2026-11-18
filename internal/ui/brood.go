package ui

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
	"github.com/eprice555/shift-command/internal/sim"
	"github.com/eprice555/shift-command/internal/world"
)

// Brood visuals: soft blobby outlines that wobble, eggs for cocoons, and a
// pulsing magenta creep field.

func isBrood(t faction.UnitType) bool { return faction.Unit(t).Faction == faction.Brood }

// blob returns an n-point closed curve of radius 1 (scaled by sx, sy)
// whose edge bulges by lobes sine bumps of size bump.
func blob(dst []geom.Vec, n, lobes int, bump, sx, sy float64) []geom.Vec {
	for i := range n {
		a := float64(i) * 2 * math.Pi / float64(n)
		r := 1 + bump*math.Cos(float64(lobes)*a)
		s, c := math.Sincos(a)
		dst = append(dst, geom.V(c*r*sx, s*r*sy))
	}
	return dst
}

// broodShape appends a Brood unit's outline (radius 1, facing +X) to dst.
func broodShape(dst []geom.Vec, t faction.UnitType) []geom.Vec {
	switch t {
	case faction.Spawn:
		return blob(dst, 10, 0, 0, 0.95, 0.7)
	case faction.Drone:
		// Round body with two mandibles.
		dst = append(dst, geom.V(1.05, 0.35), geom.V(0.55, 0.2))
		for i := 1; i < 10; i++ {
			s, c := math.Sincos(0.45 + float64(i)*(2*math.Pi-0.9)/10)
			dst = append(dst, geom.V(c*0.82, s*0.82))
		}
		return append(dst, geom.V(0.55, -0.2), geom.V(1.05, -0.35))
	case faction.Biter:
		// Teardrop pointing forward.
		dst = append(dst, geom.V(1.15, 0))
		for i := 1; i < 10; i++ {
			s, c := math.Sincos(math.Pi/3 + float64(i)*(4*math.Pi/3)/10)
			dst = append(dst, geom.V(c*0.75-0.1, s*0.75))
		}
		return dst
	case faction.Leaper:
		// Crescent with its horns forward.
		for i := range 9 {
			s, c := math.Sincos(math.Pi*0.3 + float64(i)*math.Pi*1.4/8)
			dst = append(dst, geom.V(c, s))
		}
		for i := range 9 {
			s, c := math.Sincos(math.Pi*1.7 - float64(i)*math.Pi*1.4/8)
			dst = append(dst, geom.V(c*0.45-0.35, s*0.55))
		}
		return dst
	case faction.Spitter:
		return blob(dst, 14, 3, 0.12, 0.85, 0.85)
	case faction.Broodmother:
		return blob(dst, 20, 5, 0.18, 0.9, 0.9)
	case faction.Behemoth:
		return blob(dst, 24, 7, 0.1, 0.95, 0.85)
	case faction.Mite:
		// Spiky dart.
		return append(dst, geom.V(1.2, 0), geom.V(0.1, 0.45), geom.V(-0.2, 0.9), geom.V(-0.5, 0.3), geom.V(-0.9, 0), geom.V(-0.5, -0.3), geom.V(-0.2, -0.9), geom.V(0.1, -0.45))
	}
	return blob(dst, 12, 0, 0, 0.8, 0.8)
}

// cocoon appends a Spawn cocoon's egg outline, swelling as frac grows.
func cocoon(dst []geom.Vec, frac float64) []geom.Vec {
	k := 1.15 + 0.35*frac
	return blob(dst, 14, 0, 0, k*1.15, k*0.85)
}

// wobble jiggles a Brood outline point i so the creature looks alive.
func wobble(q geom.Vec, i int, id sim.UnitID, frame int, amt float64) geom.Vec {
	return q.Scale(1 + amt*math.Sin(float64(frame)*0.11+float64(i)*1.9+float64(id)*0.7))
}

// broodBuilding adds a Brood building to the paths: a wobbling organic
// mound over its footprint. Unfinished ones are a swelling egg.
func broodBuilding(fill, stroke *vector.Path, t faction.UnitType, rect geom.Rect, complete bool, frac, wob float64, cam *Camera) {
	c := cam.ToScreenV(rect.Min.Lerp(rect.Max, 0.5))
	s := (rect.Max.X - rect.Min.X) * cam.Scale()
	mound := func(r float64, lobes int, bump, phase float64) []geom.Vec {
		pts := blob(nil, 24, lobes, bump, r, r)
		for i := range pts {
			pts[i] = c.Add(pts[i].Scale(1 + 0.03*math.Sin(wob+float64(i)*0.8+phase)))
		}
		return pts
	}
	if !complete {
		egg := mound(s*(0.22+0.22*frac), 0, 0, 0)
		if fill != nil {
			poly(fill, egg...)
		}
		poly(stroke, egg...)
		poly(stroke, mound(s*0.12, 3, 0.3, 1)...)
		return
	}
	outer := mound(s*0.46, 5, 0.06, 0)
	if fill != nil {
		poly(fill, outer...)
	}
	poly(stroke, outer...)
	switch t {
	case faction.Hive:
		poly(stroke, mound(s*0.24, 3, 0.15, 2)...)
		for i := range 3 {
			sn, cs := math.Sincos(float64(i)*2*math.Pi/3 + wob*0.05)
			poly(stroke, ngon(c.Add(geom.V(cs, sn).Scale(s*0.34)), s*0.06, 8, 0)...)
		}
	case faction.NestPod:
		poly(stroke, mound(s*0.24, 0, 0, 1)...)
		poly(stroke, mound(s*0.1, 0, 0, 2)...)
	case faction.BroodExtractor:
		poly(stroke, mound(s*0.2, 4, 0.2, 1)...)
		for i := range 4 {
			sn, cs := math.Sincos(float64(i)*math.Pi/2 + math.Pi/4)
			line(stroke, c.Add(geom.V(cs, sn).Scale(s*0.2)), c.Add(geom.V(cs, sn).Scale(s*0.42)))
		}
	case faction.SpawningPit:
		poly(stroke, mound(s*0.26, 0, 0, 1)...)
		for i := range 5 {
			sn, cs := math.Sincos(float64(i) * 2 * math.Pi / 5)
			line(stroke, c.Add(geom.V(cs, sn).Scale(s*0.26)), c.Add(geom.V(cs, sn).Scale(s*0.12)))
		}
	case faction.GestationDen:
		poly(stroke, mound(s*0.28, 2, 0.25, 1)...)
		poly(stroke, ngon(c, s*0.08, 8, 0)...)
	}
}

// creepColor is creep's tint (straight alpha is applied per tile).
var creepColor = faction.Get(faction.Brood).Color

// drawCreep draws every player's creep the viewer can see as a pulsing
// magenta noise field on the floor.
func (r *Renderer) drawCreep(screen *ebiten.Image, v *MatchView) {
	c := &r.match
	w := v.World
	has := false
	for p := range sim.PlayerID(sim.NumPlayers) {
		f := w.Creep[p]
		for y := range f.H {
			for x := range f.W {
				if !f.At(x, y) {
					continue
				}
				if p != v.Player && !v.RevealAll && w.Fog[v.Player].At(x, y) != world.Visible {
					continue
				}
				if !has {
					clear(c.creepPix)
					has = true
				}
				h := uint32(x*73856093^y*19349663) % 97
				a := 0x26 + h/3
				i := 4 * (y*f.W + x)
				c.creepPix[i] = byte(uint32(creepColor.R) * a / 0xFF)
				c.creepPix[i+1] = byte(uint32(creepColor.G) * a / 0xFF)
				c.creepPix[i+2] = byte(uint32(creepColor.B) * a / 0xFF)
				c.creepPix[i+3] = byte(a)
			}
		}
	}
	if !has {
		return
	}
	c.creep.WritePixels(c.creepPix)
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Scale(v.Cam.Scale(), v.Cam.Scale())
	ox, oy := v.Cam.ToScreen(geom.Vec{})
	op.GeoM.Translate(ox, oy)
	k := float32(0.8 + 0.2*math.Sin(float64(r.frame)*0.04))
	op.ColorScale.Scale(k, k, k, k)
	screen.DrawImage(c.creep, op)
}

// drawUnitBar draws a progress bar under mobile unit u.
func drawUnitBar(screen *ebiten.Image, v *MatchView, u *sim.Unit, frac float64, col color.RGBA) {
	pos, _ := DrawPos(u, v.Alpha)
	sx, sy := v.Cam.ToScreen(pos)
	scale := v.Cam.Scale()
	w := max(18, 2.6*u.Def.Radius*scale)
	x, y := float32(sx-w/2), float32(sy+(u.Def.Radius*1.5+0.2)*scale)
	vector.FillRect(screen, x-1, y-1, float32(w)+2, 5, color.RGBA{0, 0, 0, 0xC0}, false)
	vector.FillRect(screen, x, y, float32(w*frac), 3, col, false)
}
