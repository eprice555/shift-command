package ui

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
	"github.com/eprice555/shift-command/internal/sim"
)

type fxKind uint8

const (
	fxShot fxKind = iota
	fxImpact
	fxDeath
	fxPingMove
	fxPingAttack
)

type fx struct {
	kind   fxKind
	owner  sim.PlayerID
	attack faction.AttackType
	a, b   geom.Vec
	born   int // renderer frame it was created on
	life   int // frames
	seed   uint32
}

// effects holds short-lived, presentation-only visuals.
type effects struct {
	list  []fx
	frame int // last frame seen, so new effects start at age 0
	seed  uint32
}

// AddEvents turns sim events into visual effects.
func (r *Renderer) AddEvents(evs []sim.Event) {
	e := &r.fx
	for _, ev := range evs {
		e.seed = e.seed*1664525 + 1013904223
		switch ev.Kind {
		case sim.EvShot:
			e.add(fx{kind: fxShot, owner: ev.Owner, attack: ev.Attack, a: ev.From, b: ev.To, life: 7})
			if ev.Attack == faction.Explosive {
				e.add(fx{kind: fxImpact, owner: ev.Owner, a: ev.To, life: 16})
			}
		case sim.EvDeath:
			e.add(fx{kind: fxDeath, owner: ev.Owner, a: ev.From, life: 36, seed: e.seed})
		}
	}
}

// AddPing shows a command confirmation marker at p.
func (r *Renderer) AddPing(p geom.Vec, attack bool) {
	k := fxPingMove
	if attack {
		k = fxPingAttack
	}
	r.fx.add(fx{kind: k, a: p, life: 22})
}

func (e *effects) add(f fx) {
	f.born = e.frame
	if len(e.list) < 512 {
		e.list = append(e.list, f)
	}
}

func (e *effects) draw(em *ebiten.Image, v *MatchView, frame int) {
	e.frame = frame
	cam := v.Cam
	scale := float32(cam.Scale())
	fogged := func(p geom.Vec) bool {
		return !v.RevealAll && !v.World.Fog[v.Player].VisibleAt(p)
	}
	live := e.list[:0]
	for _, f := range e.list {
		age := frame - f.born
		if age >= f.life {
			continue
		}
		live = append(live, f)
		t := float64(age) / float64(f.life)
		fade := 1 - t
		ax, ay := cam.ToScreen(f.a)
		col := PlayerColor(f.owner, v.Player)
		switch f.kind {
		case fxShot:
			if fogged(f.a) && fogged(f.b) {
				continue
			}
			bx, by := cam.ToScreen(f.b)
			w := float32(1.5)
			switch f.attack {
			case faction.Concussive:
				w = 2.2
			case faction.Explosive:
				w = 3
			}
			vector.StrokeLine(em, float32(ax), float32(ay), float32(bx), float32(by), w*max(scale/TilePx, 0.7), scaleRGB(Whiten(col, 0.3), fade), true)
		case fxImpact:
			if fogged(f.a) {
				continue
			}
			rad := scale * float32(0.25+0.75*t)
			vector.StrokeCircle(em, float32(ax), float32(ay), rad, 2, scaleRGB(col, fade), true)
		case fxDeath:
			if fogged(f.a) {
				continue
			}
			ease := 1 - (1-t)*(1-t)
			vector.StrokeCircle(em, float32(ax), float32(ay), scale*float32(0.3+1.4*ease), 2.5, scaleRGB(col, fade*0.8), true)
			s := f.seed
			for range 10 {
				s = s*1664525 + 1013904223
				ang := float64(s>>8) / float64(1<<24) * 2 * math.Pi
				spd := 0.6 + float64(s&0xFF)/255*1.2
				sn, cs := math.Sincos(ang)
				d0, d1 := spd*ease, spd*ease+0.25*fade
				vector.StrokeLine(em,
					float32(ax+cs*d0*float64(scale)), float32(ay+sn*d0*float64(scale)),
					float32(ax+cs*d1*float64(scale)), float32(ay+sn*d1*float64(scale)),
					2, scaleRGB(Whiten(col, 0.4), fade), true)
			}
		case fxPingMove, fxPingAttack:
			c := colSelect
			if f.kind == fxPingAttack {
				c = colEnemy
			}
			rad := scale * float32(0.9-0.6*t)
			vector.StrokeCircle(em, float32(ax), float32(ay), rad, 2, scaleRGB(c, fade), true)
			vector.StrokeCircle(em, float32(ax), float32(ay), rad*0.35, 1.5, scaleRGB(c, fade), true)
		}
	}
	e.list = live
}
