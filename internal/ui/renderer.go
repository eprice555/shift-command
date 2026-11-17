// Package ui draws the game: the neon renderer, HUD, minimap, and menus.
package ui

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/eprice555/shift-command/internal/assets"
	"github.com/eprice555/shift-command/internal/faction"
)

var (
	colBackground = color.RGBA{0x06, 0x07, 0x0D, 0xFF}
	colGridMinor  = color.RGBA{0x10, 0x1A, 0x2C, 0xFF}
	colGridMajor  = color.RGBA{0x1A, 0x2B, 0x48, 0xFF}
	colDim        = color.RGBA{0x6A, 0x78, 0x90, 0xFF}
	colText       = color.RGBA{0xC8, 0xD4, 0xE8, 0xFF}
)

const (
	gridSize    = 40
	gridMajor   = 4
	emblemVerts = 96 // multiple of 6 and 3 so hex/triangle corners are exact
	titleText   = "SHIFT COMMAND"
	titleTrack  = 10.0
)

// Renderer draws the title screen and matches. Render-only animation
// (flicker, effects) uses its own frame counter and never feeds the sim.
type Renderer struct {
	titleFace, labelFace, hudFace, smallFace text.Face

	w, h  int
	grid  *ebiten.Image
	bloom *bloom
	frame int

	ptsA, ptsB, pts []Vec
	path            vector.Path

	match matchCache
	fx    effects
}

// NewRenderer loads fonts and prepares a renderer.
func NewRenderer() (*Renderer, error) {
	bold, err := assets.BoldFont()
	if err != nil {
		return nil, fmt.Errorf("load bold font: %w", err)
	}
	mono, err := assets.MonoFont()
	if err != nil {
		return nil, fmt.Errorf("load mono font: %w", err)
	}
	return &Renderer{
		titleFace: &text.GoTextFace{Source: bold, Size: 96},
		labelFace: &text.GoTextFace{Source: bold, Size: 28},
		hudFace:   &text.GoTextFace{Source: mono, Size: 16},
		smallFace: &text.GoTextFace{Source: mono, Size: 13},
	}, nil
}

func (r *Renderer) ensureTargets(w, h int) {
	if r.w == w && r.h == h {
		return
	}
	r.w, r.h = w, h
	r.bloom = newBloom(w, h)
	r.grid = ebiten.NewImage(w, h)
	for i, x := 0, 0; x <= w; i, x = i+1, x+gridSize {
		c := colGridMinor
		if i%gridMajor == 0 {
			c = colGridMajor
		}
		vector.StrokeLine(r.grid, float32(x)+0.5, 0, float32(x)+0.5, float32(h), 1, c, false)
	}
	for i, y := 0, 0; y <= h; i, y = i+1, y+gridSize {
		c := colGridMinor
		if i%gridMajor == 0 {
			c = colGridMajor
		}
		vector.StrokeLine(r.grid, 0, float32(y)+0.5, float32(w), float32(y)+0.5, 1, c, false)
	}
}

// DrawTitle renders the title screen.
func (r *Renderer) DrawTitle(screen *ebiten.Image, ti *Title, focused bool) {
	r.frame++
	b := screen.Bounds()
	r.ensureTargets(b.Dx(), b.Dy())
	cx := float64(r.w) / 2

	progress := ti.Progress()
	t := EaseInOutCubic(progress)
	from, to := faction.Get(ti.PrevForm), faction.Get(ti.Form)
	col := LerpColor(from.Color, to.Color, t)

	screen.Fill(colBackground)
	screen.DrawImage(r.grid, nil)

	// Emissive pass: everything drawn into em glows.
	em := r.bloom.src
	em.Clear()
	flicker := r.flicker()
	titleY := float64(r.h) * 0.24
	drawTracked(em, titleText, r.titleFace, cx, titleY, titleTrack, scaleRGB(col, flicker))

	ecx, ecy := cx, float64(r.h)*0.56
	r.buildEmblem(from, to, t, 92)
	r.strokeEmblem(em, ecx, ecy, 5, col)
	r.drawOrbit(em, ecx, ecy, 132, col)
	if ti.Morphing() {
		rad := 92 + 150*progress
		a := float32(1 - progress)
		vector.StrokeCircle(em, float32(ecx), float32(ecy), float32(rad), 3, scaleRGB(col, float64(a)), true)
	}
	label := strings.ToUpper(to.Name + "  ·  " + to.Shift)
	drawCentered(em, label, r.labelFace, cx, float64(r.h)*0.80, scaleRGB(to.Color, 0.9))

	r.bloom.composite(screen)

	// White-hot tube cores on top of the glow.
	drawTracked(screen, titleText, r.titleFace, cx, titleY, titleTrack, withAlpha(Whiten(col, 0.5), 0.4*flicker))
	r.strokeEmblem(screen, ecx, ecy, 1.5, Whiten(col, 0.6))

	// HUD (no glow).
	blink := 0.55 + 0.45*math.Sin(float64(r.frame)*0.08)
	prompt := "[ SHIFT ]  TO SHIFT   ·   [ ENTER ] / CLICK  TO DEPLOY"
	if !focused {
		prompt = "CLICK TO FOCUS AND DEPLOY  ·  [ SHIFT ] TO SHIFT"
		blink = 0.75 + 0.25*math.Sin(float64(r.frame)*0.15)
	}
	drawCentered(screen, prompt, r.hudFace, cx, float64(r.h)*0.88, withAlpha(colText, blink))
	hud := fmt.Sprintf("M2c BUILD   shifts %d", ti.Shifts)
	drawLeft(screen, hud, r.hudFace, 16, float64(r.h)-28, colDim)
	drawRight(screen, "GITHUB GAME OFF 2026  ·  THEME: SHIFT", r.hudFace, float64(r.w)-16, float64(r.h)-28, colDim)
}

// flicker returns a neon-sign brightness in (0,1]: a slow hum plus rare dips.
func (r *Renderer) flicker() float64 {
	v := 0.93 + 0.07*math.Sin(float64(r.frame)*0.045)
	if m := r.frame % 211; m == 0 || m == 3 {
		v *= 0.6
	}
	return v
}

func emblemRot(d faction.Def) float64 {
	if d.Sides == 3 {
		return -math.Pi / 2 // point up
	}
	return 0
}

func (r *Renderer) buildEmblem(from, to faction.Def, t, radius float64) {
	spin := float64(r.frame) * 0.006
	r.ptsA = EmblemPoints(r.ptsA[:0], from.Sides, emblemVerts, radius, emblemRot(from)+spin)
	r.ptsB = EmblemPoints(r.ptsB[:0], to.Sides, emblemVerts, radius, emblemRot(to)+spin)
	r.pts = LerpPoints(r.pts[:0], r.ptsA, r.ptsB, t)
}

func (r *Renderer) strokeEmblem(dst *ebiten.Image, cx, cy float64, width float32, c color.RGBA) {
	r.path.Reset()
	for i, p := range r.pts {
		x, y := float32(cx+p.X), float32(cy+p.Y)
		if i == 0 {
			r.path.MoveTo(x, y)
		} else {
			r.path.LineTo(x, y)
		}
	}
	r.path.Close()
	sop := &vector.StrokeOptions{Width: width, LineJoin: vector.LineJoinRound}
	dop := &vector.DrawPathOptions{AntiAlias: true}
	dop.ColorScale.ScaleWithColor(c)
	vector.StrokePath(dst, &r.path, sop, dop)
}

// drawOrbit draws rotating tick marks around the emblem.
func (r *Renderer) drawOrbit(dst *ebiten.Image, cx, cy, radius float64, c color.RGBA) {
	const ticks = 24
	rot := -float64(r.frame) * 0.01
	dim := scaleRGB(c, 0.45)
	for i := range ticks {
		a := rot + 2*math.Pi*float64(i)/ticks
		l := 8.0
		if i%6 == 0 {
			l = 16
		}
		s, co := math.Sincos(a)
		vector.StrokeLine(dst,
			float32(cx+co*radius), float32(cy+s*radius),
			float32(cx+co*(radius+l)), float32(cy+s*(radius+l)), 2, dim, true)
	}
}

func scaleRGB(c color.RGBA, k float64) color.RGBA {
	s := func(v uint8) uint8 { return uint8(math.Round(float64(v) * min(max(k, 0), 1))) }
	return color.RGBA{s(c.R), s(c.G), s(c.B), c.A}
}

func withAlpha(c color.RGBA, a float64) color.Color {
	a = min(max(a, 0), 1)
	return color.NRGBA{c.R, c.G, c.B, uint8(math.Round(255 * a))}
}

func drawAligned(dst *ebiten.Image, s string, face text.Face, x, y float64, c color.Color, align text.Align) {
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.PrimaryAlign = align
	op.SecondaryAlign = text.AlignCenter
	op.ColorScale.ScaleWithColor(c)
	text.Draw(dst, s, face, op)
}

func drawCentered(dst *ebiten.Image, s string, face text.Face, x, y float64, c color.Color) {
	drawAligned(dst, s, face, x, y, c, text.AlignCenter)
}

func drawLeft(dst *ebiten.Image, s string, face text.Face, x, y float64, c color.Color) {
	drawAligned(dst, s, face, x, y, c, text.AlignStart)
}

func drawRight(dst *ebiten.Image, s string, face text.Face, x, y float64, c color.Color) {
	drawAligned(dst, s, face, x, y, c, text.AlignEnd)
}

// drawTracked draws s centered at x with extra letter spacing.
func drawTracked(dst *ebiten.Image, s string, face text.Face, x, y, track float64, c color.Color) {
	total := text.Advance(s, face) + track*float64(len(s)-1)
	cur := x - total/2
	for _, ch := range s {
		g := string(ch)
		drawLeft(dst, g, face, cur, y, c)
		cur += text.Advance(g, face) + track
	}
}
