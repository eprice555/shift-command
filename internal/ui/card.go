package ui

import (
	"math"
	"slices"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
	"github.com/eprice555/shift-command/internal/sim"
	"github.com/eprice555/shift-command/internal/world"
)

// ActionKind is what a command-card button does.
type ActionKind uint8

const (
	ActBuild  ActionKind = iota + 1 // enter placement for Type
	ActTrain                        // queue Type at the selected buildings
	ActAttack                       // attack-move targeting
	ActStop
	ActHold
	ActCancel   // cancel construction or the last queued unit
	ActShift    // toggle the selection's SHIFT modes
	ActMorph    // Brood: re-morph one selected unit into Type
	ActFuse     // Phasebound: fuse two Seers into a Radiant
	ActResearch // Phasebound: research warp-in at Anchors
)

// Command card layout: two rows of five buttons.
const (
	CardCols = 5
	CardRows = 2
	cardBtnW = 56
	cardBtnH = 46
	cardGap  = 4
	cardPad  = 8
	cardW    = CardCols*cardBtnW + (CardCols-1)*cardGap + 2*cardPad
	cardH    = panelH
)

// cardKeys are the hotkeys by grid position.
var cardKeys = [CardRows][CardCols]string{
	{"Q", "W", "E", "R", "T"},
	{"A", "S", "H", "SHIFT", "ESC"},
}

// Button is one command-card slot.
type Button struct {
	Kind     ActionKind
	Type     faction.UnitType
	Row, Col int
	Key      string
	Label    string
	// Enabled is false when the action is unaffordable or locked; the
	// engine still lets the sim explain why when it is pressed.
	Enabled bool
	// Reason explains why the button is disabled.
	Reason string
	// Mixed is set on a SHIFT button covering several unit types.
	Mixed bool
	// Crystals and Flux are a re-morph button's cost.
	Crystals, Flux int
	// Cooldown is the seconds until a SHIFT ability is ready again.
	Cooldown float64
	Progress float64
}

// CardRect returns the command card's screen rect.
func CardRect(w, h float64) geom.Rect {
	return geom.Rect{Min: geom.V(w-hudMargin-cardW, h-hudMargin-cardH), Max: geom.V(w-hudMargin, h-hudMargin)}
}

// ButtonRect returns the screen rect of grid slot (row, col).
func ButtonRect(card geom.Rect, row, col int) geom.Rect {
	x := card.Min.X + cardPad + float64(col)*(cardBtnW+cardGap)
	y := card.Min.Y + cardPad + float64(row)*(cardBtnH+cardGap)
	return geom.Rect{Min: geom.V(x, y), Max: geom.V(x+cardBtnW, y+cardBtnH)}
}

// ButtonAt returns the button under screen point p.
func ButtonAt(buttons []Button, card geom.Rect, p geom.Vec) (Button, bool) {
	for _, b := range buttons {
		if ButtonRect(card, b.Row, b.Col).Contains(p) {
			return b, true
		}
	}
	return Button{}, false
}

func shortName(t faction.UnitType) string {
	switch t {
	case faction.CommandHub:
		return "HUB"
	case faction.Generator:
		return "GEN"
	case faction.Extractor:
		return "EXTR"
	case faction.DrillYard:
		return "YARD"
	case faction.Foundry:
		return "FNDRY"
	case faction.Outrider:
		return "RIDER"
	case faction.Strider:
		return "STRDR"
	case faction.Bastion:
		return "BSTN"
	case faction.Trooper:
		return "TROOP"
	case faction.Mender:
		return "MENDR"
	case faction.Rigger:
		return "RIGGR"
	case faction.Hive:
		return "HIVE"
	case faction.NestPod:
		return "NEST"
	case faction.BroodExtractor:
		return "EXTR"
	case faction.SpawningPit:
		return "PIT"
	case faction.GestationDen:
		return "DEN"
	case faction.Drone:
		return "DRONE"
	case faction.Biter:
		return "BITER"
	case faction.Leaper:
		return "LEAPR"
	case faction.Spitter:
		return "SPITR"
	case faction.Broodmother:
		return "MOTHR"
	case faction.Behemoth:
		return "BHMTH"
	case faction.Spawn:
		return "SPAWN"
	case faction.Mite:
		return "MITE"
	case faction.Spire:
		return "SPIRE"
	case faction.Anchor:
		return "ANCHR"
	case faction.PhaseExtractor:
		return "EXTR"
	case faction.Gate:
		return "GATE"
	case faction.Resonator:
		return "RESON"
	case faction.Drifter:
		return "DRIFT"
	case faction.Warden:
		return "WARDN"
	case faction.Glint:
		return "GLINT"
	case faction.Seer:
		return "SEER"
	case faction.Conduit:
		return "CONDT"
	case faction.Monolith:
		return "MONO"
	case faction.Radiant:
		return "RADNT"
	}
	return "?"
}

// availability reports whether p can start t right now and, if not, why.
func availability(w *sim.World, p sim.PlayerID, t faction.UnitType) (bool, string) {
	d := faction.Unit(t)
	pl := w.Players[p]
	if req, missing := w.Missing(p, t); missing {
		return false, "Requires " + faction.Unit(req).Name
	}
	switch {
	case pl.Crystals < d.Crystals:
		return false, sim.NoticeCrystals.String()
	case pl.Flux < d.Flux:
		return false, sim.NoticeFlux.String()
	case !d.Building && d.Supply > 0 && pl.Supply+faction.QueueSupply(t) > pl.SupplyCap+1e-9:
		return false, sim.NoticeSupply.String()
	}
	return true, ""
}

// CommandCard builds the card for player p's part of the selection.
func CommandCard(w *sim.World, p sim.PlayerID, sel []sim.UnitID) []Button {
	var units []*sim.Unit
	worker, mobile, armed := false, false, false
	var seers []*sim.Unit
	// spawn: every mobile unit is an idle Spawn; cocoon: some are morphing;
	// larva: every mobile unit is a Spawn or cocoon.
	spawn, cocoon, larva := true, false, true
	// army: some mobile unit could re-morph into another Brood form.
	army := false
	var building *sim.Unit
	for _, id := range sel {
		u := w.Unit(id)
		if u == nil || u.Owner != p {
			continue
		}
		units = append(units, u)
		switch {
		case u.Def.Building:
			if building == nil {
				building = u
			}
		default:
			mobile = true
			worker = worker || u.Def.Worker
			armed = armed || u.Def.Armed()
			if u.Type == faction.Seer && u.Complete && !u.Morphing() && !u.Shifting() && !u.Leaping() && !u.Phased() && !u.WarpingIn() {
				seers = append(seers, u)
			}
			spawn = spawn && u.Def.Morphs && !u.Morphing()
			cocoon = cocoon || u.Morphing()
			larva = larva && u.Def.Morphs
			army = army || sim.Remorphable(u)
		}
	}
	if len(units) == 0 {
		return nil
	}
	var out []Button
	slot := func(row, col int, k ActionKind, t faction.UnitType, label string) {
		ok, why := true, ""
		if k == ActBuild || k == ActTrain {
			ok, why = availability(w, p, t)
		}
		out = append(out, Button{Kind: k, Type: t, Row: row, Col: col, Key: cardKeys[row][col], Label: label, Enabled: ok, Reason: why})
	}
	// forms lays out a trains list row by row (Q–T, then A).
	forms := func(ts []faction.UnitType) {
		for i, t := range ts {
			// Row 1 keeps its last two slots for SHIFT and CANCEL.
			if row, col := i/CardCols, i%CardCols; row == 0 || row == 1 && col < 3 {
				slot(row, col, ActTrain, t, shortName(t))
			}
		}
	}
	switch {
	case mobile && larva:
		if spawn {
			forms(units[0].Def.Trains)
		}
		if cocoon {
			slot(1, 4, ActCancel, 0, "CANCEL")
		}
	case mobile:
		if len(seers) >= 2 {
			nearby := false
			for i, a := range seers {
				for _, b := range seers[i+1:] {
					nearby = nearby || a.Pos.Dist(b.Pos) <= 3
				}
			}
			b := Button{Kind: ActFuse, Type: faction.Radiant, Row: 0, Col: 0, Key: cardKeys[0][0], Label: "FUSE", Enabled: nearby}
			if req, missing := w.Missing(p, faction.Radiant); missing {
				b.Enabled, b.Reason = false, "Requires "+faction.Unit(req).Name
			} else if !nearby {
				b.Reason = "Select two Seers within 3 tiles"
			}
			out = append(out, b)
		}
		if army && !worker {
			for i, t := range remorphForms {
				out = append(out, morphButton(w, p, units, t, i))
			}
		}
		if cocoon {
			slot(1, 4, ActCancel, 0, "CANCEL")
		}
		if worker {
			for i, t := range faction.Buildable(units[0].Def.Faction) {
				if i < CardCols {
					slot(0, i, ActBuild, t, shortName(t))
				}
			}
		}
		if armed || worker {
			slot(1, 0, ActAttack, 0, "ATTACK")
		}
		slot(1, 1, ActStop, 0, "STOP")
		slot(1, 2, ActHold, 0, "HOLD")
	case building != nil:
		if building.Complete {
			n := len(out)
			forms(building.Def.Trains)
			if building.Def.SpawnMax > 0 && w.IdleSpawn(building) == nil {
				for i := n; i < len(out); i++ {
					if out[i].Enabled {
						out[i].Enabled, out[i].Reason = false, "No Spawn ready"
					}
				}
			}
		}
		if building.Mode == 1 && !building.Shifting() {
			slot(1, 1, ActStop, 0, "STOP")
		}
		if !building.Complete || len(building.Queue) > 0 {
			slot(1, 4, ActCancel, 0, "CANCEL")
		}
		if building.WarpResearching() {
			slot(1, 4, ActCancel, 0, "CANCEL")
		}
		if building.Type == faction.Resonator && !w.Players[p].WarpResearched {
			b := Button{Kind: ActResearch, Type: faction.Resonator, Row: 0, Col: 2, Key: cardKeys[0][2], Label: "WARP",
				Crystals: faction.WarpResearchCrystals, Flux: faction.WarpResearchFlux}
			switch {
			case building.WarpResearching():
				b.Label, b.Progress, b.Reason = "RESEARCH", building.WarpResearchFrac(), "Warp research in progress"
				if w.Unpowered(building) {
					b.Reason = "No power; research paused"
				}
			case !building.Complete:
				b.Enabled, b.Reason = false, "Resonator must be complete"
			case w.Unpowered(building):
				b.Enabled, b.Reason = false, "Requires power"
			case len(building.Queue) > 0:
				b.Enabled, b.Reason = false, "Finish production first"
			case w.Players[p].Crystals < b.Crystals:
				b.Enabled, b.Reason = false, sim.NoticeCrystals.String()
			case w.Players[p].Flux < b.Flux:
				b.Enabled, b.Reason = false, sim.NoticeFlux.String()
			default:
				b.Enabled = true
			}
			out = append(out, b)
		}
	}
	if b, ok := shiftButton(units, !mobile); ok {
		out = append(out, b)
	}
	return out
}

// remorphForms are the Brood combat forms offered on an army's card.
var remorphForms = []faction.UnitType{faction.Biter, faction.Leaper, faction.Spitter, faction.Broodmother, faction.Behemoth}

// morphButton is the card slot for re-morphing one of units into t, priced
// for the first unit that would take it.
func morphButton(w *sim.World, p sim.PlayerID, units []*sim.Unit, t faction.UnitType, col int) Button {
	b := Button{Kind: ActMorph, Type: t, Row: 0, Col: col, Key: cardKeys[0][col], Label: shortName(t)}
	var u *sim.Unit
	for _, c := range units {
		if sim.CanRemorph(c, t) && c.Complete && !c.Morphing() && !c.Shifting() && !c.Leaping() {
			u = c
			break
		}
	}
	if u == nil {
		b.Reason = "Already " + faction.Unit(t).Name + "s"
		return b
	}
	b.Crystals, b.Flux = sim.RemorphCost(u.Type, t)
	pl := w.Players[p]
	switch req, missing := w.Missing(p, t); {
	case missing:
		b.Reason = "Requires " + faction.Unit(req).Name
	case pl.Crystals < b.Crystals:
		b.Reason = sim.NoticeCrystals.String()
	case pl.Flux < b.Flux:
		b.Reason = sim.NoticeFlux.String()
	case pl.Supply+faction.Unit(t).Supply-faction.Unit(u.Type).Supply > pl.SupplyCap+1e-9 && faction.Unit(t).Supply > faction.Unit(u.Type).Supply:
		b.Reason = sim.NoticeSupply.String()
	default:
		b.Enabled = true
	}
	return b
}

// modeLabels abbreviates mode names for the command card.
var modeLabels = map[string]string{
	"Burrowed": "BURROW", "Surface": "RISE",
	"Pounce": "POUNCE", "Brood Burst": "BURST",
	"Rooted": "ROOT",
	"Shell":  "SHELL", "Open": "OPEN",
	"Overdrive": "OVRDV", "Normal": "OVRDV",
	"Boost": "BOOST", "Cruise": "CRUISE",
	"Planted": "PLANT", "Walk": "WALK",
	"Field Post": "POST", "Mobile": "MOBILE",
	"Artillery": "ARTY", "Tank": "TANK",
	"Packed": "PACK", "Landed": "LAND",
	"Phase": "PHASE", "Phase Dash": "DASH", "Long Phase": "LONG",
	"Phase Blink": "BLINK", "Phase Field": "FIELD",
}

// shiftButton returns the SHIFT slot for the selection, labeled with the
// mode a press would switch to. A building selection only counts its
// first building, like the rest of the building card.
func shiftButton(units []*sim.Unit, buildings bool) (Button, bool) {
	var types []faction.UnitType
	toShift := map[faction.UnitType]bool{}
	cooldown := map[faction.UnitType]float64{} // soonest-ready caster per type
	for _, u := range units {
		if u.Def.Building != buildings {
			continue
		}
		if buildings && len(types) > 0 {
			break
		}
		m := faction.Unit(u.Type).Shift
		if m == nil || !u.Complete {
			if buildings {
				break
			}
			continue
		}
		if !slices.Contains(types, u.Type) {
			types = append(types, u.Type)
		}
		if u.Mode == 0 || m.Timed() {
			toShift[u.Type] = true
		}
		if m.Cast() {
			if cd, ok := cooldown[u.Type]; !ok || u.CastLeft() < cd {
				cooldown[u.Type] = u.CastLeft()
			}
		}
	}
	if len(types) == 0 {
		return Button{}, false
	}
	b := Button{Kind: ActShift, Type: types[0], Row: 1, Col: 3, Key: cardKeys[1][3], Label: "SHIFT", Enabled: true, Mixed: len(types) > 1}
	if !b.Mixed {
		m := faction.Unit(types[0]).Shift
		name := m.Base
		if toShift[types[0]] {
			name = m.Name
		}
		b.Label = modeLabels[name]
		if cd := cooldown[types[0]]; cd > 0 {
			b.Cooldown, b.Enabled, b.Reason = cd, false, sim.NoticeCooldown.String()
		}
	}
	return b, true
}

// Queue slots in the selection panel for a single building.
const (
	queueSlot = 40
	queueGap  = 6
)

// QueueSlotRect returns the screen rect of production queue entry i.
func QueueSlotRect(panel geom.Rect, i int) geom.Rect {
	x := panel.Min.X + 14 + float64(i)*(queueSlot+queueGap)
	y := panel.Max.Y - 14 - queueSlot
	return geom.Rect{Min: geom.V(x, y), Max: geom.V(x+queueSlot, y+queueSlot)}
}

// PlacementTile returns the footprint top-left tile for placing a building
// of type t centered under world point p. Extractors snap onto the nearest
// vent within reach.
func PlacementTile(w *sim.World, t faction.UnitType, p geom.Vec) (int, int) {
	d := faction.Unit(t)
	if d.OnVent {
		best, bestD := (*sim.Resource)(nil), 3.0
		for _, r := range w.Resources {
			if r.Kind == world.Flux && r.Pos.Dist(p) < bestD {
				best, bestD = r, r.Pos.Dist(p)
			}
		}
		if best != nil {
			return best.X, best.Y
		}
	}
	h := float64(d.Size) / 2
	return int(math.Floor(p.X - h + 0.5)), int(math.Floor(p.Y - h + 0.5))
}
