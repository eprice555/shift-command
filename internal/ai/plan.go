package ai

import "github.com/eprice555/shift-command/internal/faction"

// Step is one entry of a scripted build order: once the AI's supply
// (used plus queued) reaches Supply, it makes sure it owns at least Count
// buildings of type Build (finished, under construction, or ordered).
// Steps run in order; a step waits for the ones before it.
type Step struct {
	Supply float64
	Build  faction.UnitType
	Count  int
}

// ForgeOpening is the Forge AI's build order. A second Command Hub is an
// expansion and goes on the nearest free expansion site.
var ForgeOpening = []Step{
	{Supply: 9, Build: faction.Generator, Count: 1},
	{Supply: 10, Build: faction.DrillYard, Count: 1},
	{Supply: 11, Build: faction.Extractor, Count: 1},
	{Supply: 14, Build: faction.Generator, Count: 2},
	{Supply: 16, Build: faction.Foundry, Count: 1},
	{Supply: 19, Build: faction.DrillYard, Count: 2},
	{Supply: 23, Build: faction.CommandHub, Count: 2},
	{Supply: 25, Build: faction.Generator, Count: 3},
	{Supply: 28, Build: faction.Extractor, Count: 2},
}

// Army tuning.
const (
	firstWave    = 14 // army supply that triggers the first attack
	waveGrowth   = 6  // each later wave waits for this much more
	maxWave      = 60
	maxProdBuild = 6 // Drill Yards + Foundries
	maxWorkers   = 40
)
