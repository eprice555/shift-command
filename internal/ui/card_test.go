package ui

import (
	"testing"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
	"github.com/eprice555/shift-command/internal/sim"
)

func shiftLabel(w *sim.World, ids ...sim.UnitID) string {
	for _, b := range CommandCard(w, 0, ids) {
		if b.Kind == ActShift {
			return b.Label
		}
	}
	return ""
}

func TestCardShiftButton(t *testing.T) {
	w := sim.NewSandbox()
	tank := w.Spawn(0, faction.Bastion, geom.V(20, 20))
	trooper := w.Spawn(0, faction.Trooper, geom.V(22, 20))
	worker := w.Spawn(0, faction.Rigger, geom.V(24, 20))

	if got := shiftLabel(w, tank.ID); got != "ARTY" {
		t.Errorf("Bastion SHIFT label = %q, want ARTY", got)
	}
	if got := shiftLabel(w, worker.ID); got != "" {
		t.Errorf("Rigger has a SHIFT button %q", got)
	}
	if got := shiftLabel(w, tank.ID, trooper.ID, worker.ID); got != "SHIFT" {
		t.Errorf("mixed SHIFT label = %q, want SHIFT", got)
	}
	w.Step([]sim.Command{{Kind: sim.CmdShift, Player: 0, Units: []sim.UnitID{tank.ID}, Index: -1}})
	if got := shiftLabel(w, tank.ID); got != "TANK" {
		t.Errorf("sieged Bastion SHIFT label = %q, want TANK", got)
	}
}
