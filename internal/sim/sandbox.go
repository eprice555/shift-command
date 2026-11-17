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

// NewSkirmish returns a Forge mirror match start. See NewSkirmishFor.
func NewSkirmish() *World {
	return NewSkirmishFor([NumPlayers]faction.ID{faction.Forge, faction.Forge})
}

// StartSpawn is how many Spawn a Brood Hive opens with.
const StartSpawn = 3

// NewSkirmishFor returns a standard match start with player p playing fs[p]:
// each side has a finished HQ at its start location, workers already
// mining, and StartCrystals in the bank. Brood Hives also start with their
// Spawn hatched and creep fully spread.
func NewSkirmishFor(fs [NumPlayers]faction.ID) *World {
	m := world.NewSkirmishMap()
	w := NewWorld(m)
	for p := range PlayerID(NumPlayers) {
		fd := faction.Get(fs[p])
		w.Players[p].Faction = fs[p]
		w.Players[p].Crystals = StartCrystals
		hq := w.Spawn(p, fd.HQ, m.Starts[p])
		hq.creepR = hq.Def.Creep
		for range StartWorkers {
			r := w.findResource(&Unit{Owner: p}, world.Crystal, hq.Pos)
			if r == nil {
				break
			}
			u := w.Spawn(p, fd.Worker, w.approach(r.Pos, faction.Unit(fd.Worker).Radius, hq.Rect()))
			u.Orders = append(u.Orders, Order{Kind: OrderGather, Res: r.ID})
			r.assigned++
		}
		for i := range min(StartSpawn, hq.Def.SpawnMax) {
			w.newSpawn(hq, i)
		}
	}
	w.recount()
	w.updateCreep(true)
	w.updateFog()
	return w
}
