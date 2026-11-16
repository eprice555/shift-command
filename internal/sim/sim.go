// Package sim is the deterministic game simulation (units, combat, orders).
// It advances in fixed ticks and must not depend on rendering or wall-clock
// time, so it stays replayable and testable.
package sim

import "github.com/eprice555/shift-command/internal/faction"

// TicksPerSecond is the fixed simulation rate.
const TicksPerSecond = 20

// MorphTicks is how long the M0 demo emblem takes to shift forms.
const MorphTicks = 8

// Command is a player order fed into the sim on a tick boundary.
type Command uint8

const (
	CmdNone Command = iota
	// CmdShift triggers the SHIFT ability (the Shift key).
	CmdShift
)

// World is the full simulation state.
type World struct {
	Tick uint64
	// Form and PrevForm drive the M0 shift demo: the emblem morphs from
	// PrevForm to Form over MorphTicks.
	Form      faction.ID
	PrevForm  faction.ID
	MorphTick int
	Shifts    int
}

// NewWorld returns a world at tick 0 with no morph in progress.
func NewWorld() *World {
	return &World{Form: faction.Forge, PrevForm: faction.Forge, MorphTick: MorphTicks}
}

// Step applies cmds and advances the world by one tick.
func (w *World) Step(cmds []Command) {
	for _, c := range cmds {
		switch c {
		case CmdShift:
			w.PrevForm = w.Form
			w.Form = w.Form.Next()
			w.MorphTick = 0
			w.Shifts++
		}
	}
	if w.MorphTick < MorphTicks {
		w.MorphTick++
	}
	w.Tick++
}

// Morphing reports whether a shift animation is in progress.
func (w *World) Morphing() bool { return w.MorphTick < MorphTicks }

// MorphProgress returns shift progress in [0,1]. alpha in [0,1) is the
// fraction of the way to the next tick, used for smooth rendering.
func (w *World) MorphProgress(alpha float64) float64 {
	if !w.Morphing() {
		return 1
	}
	p := (float64(w.MorphTick) + alpha) / MorphTicks
	return min(max(p, 0), 1)
}
