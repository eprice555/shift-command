// Package faction holds data-driven faction definitions. Unit and building
// stat tables will live here so balance changes never touch sim logic.
package faction

import "image/color"

// ID identifies a faction.
type ID int

const (
	Forge ID = iota
	Brood
	Phasebound

	// Count is the number of playable factions.
	Count
)

// Def describes a faction's identity and visual language.
type Def struct {
	ID    ID
	Name  string
	Shift string // which meaning of SHIFT the faction embodies
	Color color.RGBA
	// Sides is the emblem polygon side count; 0 means a circle.
	Sides int

	// Key unit types: headquarters, worker, supply building, and flux
	// extractor.
	HQ, Worker, Supply, Extractor UnitType
	// Playable is false until the faction's units exist.
	Playable bool
}

var defs = [Count]Def{
	Forge: {ID: Forge, Name: "The Forge", Shift: "Gear Shift", Color: color.RGBA{0xFF, 0xA6, 0x30, 0xFF}, Sides: 6,
		HQ: CommandHub, Worker: Rigger, Supply: Generator, Extractor: Extractor, Playable: true},
	Brood: {ID: Brood, Name: "The Brood", Shift: "Shapeshift", Color: color.RGBA{0xFF, 0x3C, 0xAC, 0xFF}, Sides: 0,
		HQ: Hive, Worker: Drone, Supply: NestPod, Extractor: BroodExtractor, Playable: true},
	Phasebound: {ID: Phasebound, Name: "The Phasebound", Shift: "Phase Shift", Color: color.RGBA{0x3C, 0xF0, 0xFF, 0xFF}, Sides: 3},
}

// Get returns the definition for id. It panics on an out-of-range id.
func Get(id ID) Def { return defs[id] }

// All returns every faction definition in ID order.
func All() []Def { return defs[:] }

// Acid is the Brood's secondary color (acid green).
var Acid = color.RGBA{0x7C, 0xFF, 0x4F, 0xFF}

// NextPlayable returns the next playable faction after id, wrapping around.
func (id ID) NextPlayable() ID {
	for n := id.Next(); ; n = n.Next() {
		if defs[n].Playable {
			return n
		}
	}
}

// Next returns the faction after id, wrapping around.
func (id ID) Next() ID { return (id + 1) % Count }
