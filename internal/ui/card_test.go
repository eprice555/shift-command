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

func cardButton(w *sim.World, kind ActionKind, typ faction.UnitType, ids ...sim.UnitID) (Button, bool) {
	for _, b := range CommandCard(w, 0, ids) {
		if b.Kind == kind && (typ == 0 || b.Type == typ) {
			return b, true
		}
	}
	return Button{}, false
}

func TestCardRemorph(t *testing.T) {
	w := sim.NewSkirmishFor([sim.NumPlayers]faction.ID{faction.Brood, faction.Forge})
	w.Players[0].Crystals, w.Players[0].Flux = 1000, 1000
	biter := w.Spawn(0, faction.Biter, geom.V(20, 20))

	b, ok := cardButton(w, ActMorph, faction.Spitter, biter.ID)
	if !ok {
		t.Fatal("Biter card has no Spitter re-morph")
	}
	if b.Enabled || b.Reason != "Requires Spawning Pit" {
		t.Errorf("Spitter re-morph without a Spawning Pit: enabled %v, reason %q", b.Enabled, b.Reason)
	}
	w.Spawn(0, faction.SpawningPit, geom.V(30, 30))
	w.Step(nil)
	b, _ = cardButton(w, ActMorph, faction.Spitter, biter.ID)
	if !b.Enabled || b.Crystals != 50 || b.Flux != 25 {
		t.Errorf("Spitter re-morph: enabled %v (%q), cost %d/%d, want 50/25", b.Enabled, b.Reason, b.Crystals, b.Flux)
	}
	if b, _ := cardButton(w, ActMorph, faction.Biter, biter.ID); b.Enabled {
		t.Error("Biter can re-morph into a Biter")
	}

	w.Step([]sim.Command{{Kind: sim.CmdMorph, Player: 0, Units: []sim.UnitID{biter.ID}, Type: faction.Spitter}})
	if _, ok := cardButton(w, ActCancel, 0, biter.ID); !ok {
		t.Error("re-morph cocoon has no CANCEL")
	}
	if _, ok := cardButton(w, ActMorph, 0, biter.ID); ok {
		t.Error("cocoon-only selection offers re-morphs")
	}
}

func TestCardCastCooldown(t *testing.T) {
	w := sim.NewSandbox()
	mother := w.Spawn(0, faction.Broodmother, geom.V(20, 20))
	if b, _ := cardButton(w, ActShift, 0, mother.ID); !b.Enabled || b.Label != "BURST" {
		t.Fatalf("Broodmother SHIFT: enabled %v, label %q", b.Enabled, b.Label)
	}
	w.Step([]sim.Command{{Kind: sim.CmdShift, Player: 0, Units: []sim.UnitID{mother.ID}, Index: -1}})
	b, _ := cardButton(w, ActShift, 0, mother.ID)
	if b.Enabled || b.Cooldown <= 0 {
		t.Errorf("Brood Burst on cooldown: enabled %v, cooldown %v", b.Enabled, b.Cooldown)
	}
}

func TestCardPhasebound(t *testing.T) {
	w := sim.NewSkirmishFor([sim.NumPlayers]faction.ID{faction.Phasebound, faction.Forge})
	var drifter, spire *sim.Unit
	for _, u := range w.Units {
		switch {
		case u.Owner == 0 && u.Type == faction.Drifter:
			drifter = u
		case u.Owner == 0 && u.Type == faction.Spire:
			spire = u
		}
	}
	for _, typ := range []faction.UnitType{faction.Anchor, faction.PhaseExtractor, faction.Gate, faction.Resonator} {
		b, ok := cardButton(w, ActBuild, typ, drifter.ID)
		if !ok || b.Label == "?" {
			t.Errorf("Drifter build %v: %+v", typ, b)
		}
	}
	if b, ok := cardButton(w, ActTrain, faction.Drifter, spire.ID); !ok || b.Label != "DRIFT" {
		t.Errorf("Spire train Drifter: %+v", b)
	}
	if b, _ := cardButton(w, ActBuild, faction.Gate, drifter.ID); b.Enabled {
		t.Error("Gate should need an Anchor first")
	}
}

func TestCardPhaseboundShiftResearchAndFuse(t *testing.T) {
	w := sim.NewSkirmishFor([sim.NumPlayers]faction.ID{faction.Phasebound, faction.Forge})
	w.Players[0].Crystals, w.Players[0].Flux = 1000, 1000
	seerA := w.Spawn(0, faction.Seer, geom.V(12, 12))
	seerB := w.Spawn(0, faction.Seer, geom.V(13, 12))
	if b, ok := cardButton(w, ActShift, 0, seerA.ID); !ok || !b.Enabled || b.Label != "BLINK" {
		t.Errorf("Seer phase button: %+v, exists %v", b, ok)
	}
	if b, ok := cardButton(w, ActFuse, faction.Radiant, seerA.ID, seerB.ID); !ok || b.Enabled || b.Reason != "Requires Resonator" {
		t.Errorf("Seer fusion without Resonator: %+v, exists %v", b, ok)
	}

	anchor := w.Spawn(0, faction.Anchor, geom.V(22, 20))
	resonator := w.Spawn(0, faction.Resonator, geom.V(26, 20))
	if b, ok := cardButton(w, ActFuse, faction.Radiant, seerA.ID, seerB.ID); !ok || !b.Enabled || b.Label != "FUSE" {
		t.Errorf("Seer fusion button: %+v, exists %v", b, ok)
	}
	b, ok := cardButton(w, ActResearch, faction.Resonator, resonator.ID)
	if !ok || !b.Enabled || b.Crystals != faction.WarpResearchCrystals || b.Flux != faction.WarpResearchFlux {
		t.Errorf("Warp research button: %+v, exists %v", b, ok)
	}
	w.Step([]sim.Command{{Kind: sim.CmdResearchWarp, Player: 0, Units: []sim.UnitID{resonator.ID}}})
	b, ok = cardButton(w, ActResearch, faction.Resonator, resonator.ID)
	if !ok || b.Enabled || b.Progress <= 0 {
		t.Errorf("researching Warp card: %+v, exists %v", b, ok)
	}
	if _, ok := cardButton(w, ActCancel, 0, resonator.ID); !ok {
		t.Error("Warp research can't be cancelled from the card")
	}
	progress := resonator.WarpResearchFrac()
	anchor.Complete = false
	w.Step(nil)
	b, _ = cardButton(w, ActResearch, faction.Resonator, resonator.ID)
	if b.Reason != "No power; research paused" || resonator.WarpResearchFrac() != progress {
		t.Errorf("unpowered research state: button %+v, progress %.3f -> %.3f", b, progress, resonator.WarpResearchFrac())
	}
}
