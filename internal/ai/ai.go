// Package ai is the computer opponent. It reads the sim world like a player
// would (its own units plus what its fog reveals) and answers with the same
// sim.Commands a human issues, so it can never cheat through the API. It is
// deterministic: the same world state always yields the same commands.
package ai

import (
	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
	"github.com/eprice555/shift-command/internal/sim"
)

// thinkEvery is how many sim ticks pass between decisions (0.5 s).
const thinkEvery = 10

type mode uint8

const (
	muster mode = iota // build up at the staging point
	attack             // push into the enemy base
	defend             // clear enemies out of our base
)

// AI plays one side.
type AI struct {
	P sim.PlayerID

	home, enemyHome geom.Vec
	staging         geom.Vec // where the army gathers, just outside the base
	exit            []geom.Vec

	mode        mode
	waveSize    float64
	attackStart float64
	target      geom.Vec
	scouts      []geom.Vec
	scout       int
	defendAt    geom.Vec
	expander    sim.UnitID            // worker walking to an unexplored expansion
	lastShift   map[sim.UnitID]uint64 // tick of each unit's last SHIFT toggle
	siegeOK     uint64                // no sieging before this tick

	s   snapshot
	out []sim.Command
}

// New returns an AI playing side p of w.
func New(w *sim.World, p sim.PlayerID) *AI {
	a := &AI{
		P:         p,
		home:      w.Map.Starts[p],
		enemyHome: w.Map.Starts[1-p],
		waveSize:  firstWave,
	}
	path := w.Map.FindPath(a.home, a.enemyHome, 0.75)
	a.exit = append([]geom.Vec{a.home}, path...)
	a.staging = along(a.exit, stagingDist)
	a.scouts = append(a.scouts, a.enemyHome)
	exps := append([]geom.Vec(nil), w.Map.Expansions...)
	for len(exps) > 0 {
		// Nearest to the enemy first.
		bi := 0
		for i, e := range exps {
			if e.Dist2(a.enemyHome) < exps[bi].Dist2(a.enemyHome) {
				bi = i
			}
		}
		a.scouts = append(a.scouts, exps[bi])
		exps = append(exps[:bi], exps[bi+1:]...)
	}
	mw, mh := float64(w.Map.W), float64(w.Map.H)
	a.scouts = append(a.scouts, geom.V(mw/2, mh/2), geom.V(mw*0.2, mh*0.2), geom.V(mw*0.8, mh*0.8),
		geom.V(mw*0.8, mh*0.2), geom.V(mw*0.2, mh*0.8))
	return a
}

// Think returns this tick's commands for the AI's side. Call it once per
// tick, before w.Step.
func (a *AI) Think(w *sim.World) []sim.Command {
	if w.Over || (w.Tick+uint64(a.P)*thinkEvery/2)%thinkEvery != 0 {
		return nil
	}
	a.out = a.out[:0]
	a.look(w)
	a.economy(w)
	a.spend(w)
	a.army(w)
	return a.out
}

func (a *AI) cmd(c sim.Command) {
	c.Player = a.P
	a.out = append(a.out, c)
}

// along returns the point dist tiles along the polyline pts.
func along(pts []geom.Vec, dist float64) geom.Vec {
	for i := 1; i < len(pts); i++ {
		seg := pts[i].Dist(pts[i-1])
		if seg >= dist {
			return pts[i-1].Lerp(pts[i], dist/seg)
		}
		dist -= seg
	}
	return pts[len(pts)-1]
}

// snapshot is the AI's per-decision view of its own side.
type snapshot struct {
	have    [faction.UnitTypeCount]int // live, queued, or ordered
	done    [faction.UnitTypeCount]int // complete
	hubs    []*sim.Unit                // complete drop-offs
	workers []*sim.Unit
	army    []*sim.Unit
	prod    []*sim.Unit // complete production buildings (not hubs)
	unfin   []*sim.Unit // buildings under construction
	blds    []*sim.Unit // every building
	build   map[sim.UnitID]bool
	// Spendable bank after reserving the cost of ordered buildings.
	crystals, flux int
	armySupply     float64
}

func (a *AI) look(w *sim.World) {
	s := &a.s
	*s = snapshot{build: s.build, hubs: s.hubs[:0], workers: s.workers[:0], army: s.army[:0],
		prod: s.prod[:0], unfin: s.unfin[:0], blds: s.blds[:0]}
	if s.build == nil {
		s.build = map[sim.UnitID]bool{}
	}
	clear(s.build)
	pl := w.Players[a.P]
	s.crystals, s.flux = pl.Crystals, pl.Flux
	for _, u := range w.Units {
		if u.Owner != a.P {
			continue
		}
		s.have[u.Type]++
		if u.Complete {
			s.done[u.Type]++
		}
		for _, t := range u.Queue {
			s.have[t]++
		}
		if u.Def.Building {
			s.blds = append(s.blds, u)
		}
		switch {
		case u.Def.Building && !u.Complete:
			s.unfin = append(s.unfin, u)
		case u.Def.Building:
			if u.Def.DropOff {
				s.hubs = append(s.hubs, u)
			} else if len(u.Def.Trains) > 0 {
				s.prod = append(s.prod, u)
			}
		case u.Def.Worker:
			s.workers = append(s.workers, u)
			for _, o := range u.Orders {
				if o.Kind == sim.OrderBuild {
					def := faction.Unit(o.Build)
					s.have[o.Build]++
					s.crystals -= def.Crystals
					s.flux -= def.Flux
					s.build[u.ID] = true
				}
			}
		default:
			s.army = append(s.army, u)
			s.armySupply += u.Def.Supply
		}
	}
	if a.expander != 0 {
		if u := w.Unit(a.expander); u == nil || len(u.Orders) > 0 && u.Orders[0].Kind != sim.OrderMove {
			a.expander = 0
		}
	}
}

// afford spends from the snapshot budget if possible. Either way the cost
// is reserved, so lower priorities can't starve this one.
func (a *AI) afford(t faction.UnitType) bool {
	def := faction.Unit(t)
	ok := a.s.crystals >= def.Crystals && a.s.flux >= def.Flux
	a.s.crystals -= def.Crystals
	a.s.flux -= def.Flux
	return ok
}

// canAfford reports whether t fits the budget without reserving anything.
func (a *AI) canAfford(t faction.UnitType) bool {
	def := faction.Unit(t)
	return a.s.crystals >= def.Crystals && a.s.flux >= def.Flux
}
