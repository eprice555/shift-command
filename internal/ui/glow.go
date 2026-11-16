package ui

import "github.com/hajimehoshi/ebiten/v2"

// bloom is a cheap glow pass: emissive content is drawn into src, which is
// repeatedly halved with linear filtering and then added back to the screen
// at increasing scales (additive blending). It runs fine on WebGL.
type bloom struct {
	src     *ebiten.Image
	mips    []*ebiten.Image
	weights []float32
}

func newBloom(w, h int) *bloom {
	b := &bloom{src: ebiten.NewImage(w, h), weights: []float32{0.9, 0.75, 0.6, 0.5}}
	for range b.weights {
		w, h = max(w/2, 1), max(h/2, 1)
		b.mips = append(b.mips, ebiten.NewImage(w, h))
	}
	return b
}

// composite adds src plus its blurred mips onto dst.
func (b *bloom) composite(dst *ebiten.Image) {
	prev := b.src
	for _, m := range b.mips {
		m.Clear()
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		op.GeoM.Scale(0.5, 0.5)
		m.DrawImage(prev, op)
		prev = m
	}

	op := &ebiten.DrawImageOptions{Blend: ebiten.BlendLighter}
	dst.DrawImage(b.src, op)
	scale := 1.0
	for i, m := range b.mips {
		scale *= 2
		op := &ebiten.DrawImageOptions{Blend: ebiten.BlendLighter, Filter: ebiten.FilterLinear}
		op.GeoM.Scale(scale, scale)
		w := b.weights[i]
		op.ColorScale.Scale(w, w, w, w)
		dst.DrawImage(m, op)
	}
}
