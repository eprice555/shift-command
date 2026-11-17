package sim

import (
	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
	"github.com/eprice555/shift-command/internal/world"
)

// NewSandbox returns the M1 test match: both sides get a mixed army at their
// main, and the enemy has a forward squad mid-map to fight.
func NewSandbox() *World {
	m := world.NewSkirmishMap()
	w := NewWorld(m)
	for p := range PlayerID(NumPlayers) {
		s := m.Starts[p]
		dir := 1.0
		if p == 1 {
			dir = -1
		}
		w.SpawnSquad(p, faction.Trooper, 8, s.Add(geom.V(3*dir, -3*dir)))
		w.SpawnSquad(p, faction.Outrider, 4, s.Add(geom.V(6*dir, 0)))
		w.SpawnSquad(p, faction.Strider, 3, s.Add(geom.V(0, -6*dir)))
	}
	w.SpawnSquad(1, faction.Trooper, 6, geom.V(46, 34))
	w.SpawnSquad(1, faction.Strider, 2, geom.V(49, 32))
	return w
}

// StartWorkers is how many workers each side opens with.
const StartWorkers = 6

// NewSkirmish returns a standard match start: each side has a finished
// Command Hub at its start location, workers already mining, and
// StartCrystals in the bank.
func NewSkirmish() *World {
	m := world.NewSkirmishMap()
	w := NewWorld(m)
	for p := range PlayerID(NumPlayers) {
		w.Players[p].Crystals = StartCrystals
		hub := w.Spawn(p, faction.CommandHub, m.Starts[p])
		for range StartWorkers {
			r := w.findResource(&Unit{Owner: p}, world.Crystal, hub.Pos)
			if r == nil {
				break
			}
			u := w.Spawn(p, faction.Rigger, w.approach(r.Pos, faction.Unit(faction.Rigger).Radius, hub.Rect()))
			u.Orders = append(u.Orders, Order{Kind: OrderGather, Res: r.ID})
			r.assigned++
		}
	}
	w.recount()
	w.updateFog()
	return w
}
