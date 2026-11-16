package ui

import "github.com/eprice555/shift-command/internal/faction"

// TitleMorphUpdates is how many updates (at 60/s) a title emblem morph takes.
const TitleMorphUpdates = 24

// Title is the title-screen state: the M0 emblem that morphs to the next
// faction whenever Shift is pressed. It is presentation-only, not sim state.
type Title struct {
	Form, PrevForm faction.ID
	MorphTick      int // updates since the last shift
	Shifts         int
}

// NewTitle returns a settled title on the first faction.
func NewTitle() *Title {
	return &Title{MorphTick: TitleMorphUpdates}
}

// Update advances one update; shift starts a morph to the next faction.
func (t *Title) Update(shift bool) {
	if t.MorphTick < TitleMorphUpdates {
		t.MorphTick++
	}
	if shift {
		t.PrevForm = t.Form
		t.Form = t.Form.Next()
		t.MorphTick = 0
		t.Shifts++
	}
}

// Morphing reports whether a morph is in progress.
func (t *Title) Morphing() bool { return t.MorphTick < TitleMorphUpdates }

// Progress is the morph progress in [0, 1].
func (t *Title) Progress() float64 {
	return min(float64(t.MorphTick)/TitleMorphUpdates, 1)
}
