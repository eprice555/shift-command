package ui

import (
	"testing"

	"github.com/eprice555/shift-command/internal/faction"
)

func TestTitleSettledAtStart(t *testing.T) {
	ti := NewTitle()
	if ti.Morphing() || ti.Progress() != 1 {
		t.Fatalf("new title morphing=%v progress=%v", ti.Morphing(), ti.Progress())
	}
}

func TestTitleShiftMorphsAndCycles(t *testing.T) {
	ti := NewTitle()
	ti.Update(true)
	if ti.PrevForm != faction.Forge || ti.Form != faction.Brood || !ti.Morphing() {
		t.Fatalf("after shift: %+v", ti)
	}
	prev := -1.0
	for range TitleMorphUpdates {
		p := ti.Progress()
		if p < prev || p < 0 || p > 1 {
			t.Fatalf("progress %v after %v", p, prev)
		}
		prev = p
		ti.Update(false)
	}
	if ti.Morphing() {
		t.Fatal("still morphing")
	}
	for range faction.Count - 1 {
		ti.Update(true)
	}
	if ti.Form != faction.Forge || ti.Shifts != int(faction.Count) {
		t.Fatalf("after full cycle: %+v", ti)
	}
}
