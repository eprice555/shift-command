package sim

import (
	"testing"

	"github.com/eprice555/shift-command/internal/faction"
)

func TestNewWorldIsSettled(t *testing.T) {
	w := NewWorld()
	if w.Morphing() {
		t.Fatal("new world should not be morphing")
	}
	if got := w.MorphProgress(0.5); got != 1 {
		t.Errorf("MorphProgress = %v, want 1", got)
	}
}

func TestShiftMorphsToNextFaction(t *testing.T) {
	w := NewWorld()
	w.Step([]Command{CmdShift})
	if w.PrevForm != faction.Forge || w.Form != faction.Brood {
		t.Fatalf("after shift: prev=%d form=%d", w.PrevForm, w.Form)
	}
	if !w.Morphing() {
		t.Fatal("should be morphing right after shift")
	}
	for i := 1; i < MorphTicks; i++ {
		w.Step(nil)
	}
	if w.Morphing() {
		t.Errorf("still morphing after %d ticks", MorphTicks)
	}
	if w.Tick != MorphTicks || w.Shifts != 1 {
		t.Errorf("tick=%d shifts=%d", w.Tick, w.Shifts)
	}
}

func TestShiftCyclesAllFactions(t *testing.T) {
	w := NewWorld()
	for range faction.Count {
		w.Step([]Command{CmdShift})
	}
	if w.Form != faction.Forge {
		t.Errorf("after %d shifts form=%d, want Forge", faction.Count, w.Form)
	}
}

func TestMorphProgressMonotonicAndClamped(t *testing.T) {
	w := NewWorld()
	w.Step([]Command{CmdShift})
	prev := -1.0
	for range MorphTicks + 2 {
		for _, a := range []float64{0, 0.33, 0.66, 0.99} {
			p := w.MorphProgress(a)
			if p < 0 || p > 1 {
				t.Fatalf("progress %v out of range", p)
			}
			if p < prev {
				t.Fatalf("progress went backwards: %v -> %v", prev, p)
			}
			prev = p
		}
		w.Step(nil)
	}
}

func TestDeterministic(t *testing.T) {
	run := func() World {
		w := NewWorld()
		for i := range 100 {
			var cmds []Command
			if i%7 == 0 {
				cmds = []Command{CmdShift}
			}
			w.Step(cmds)
		}
		return *w
	}
	if a, b := run(), run(); a != b {
		t.Errorf("non-deterministic: %+v vs %+v", a, b)
	}
}
