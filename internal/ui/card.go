package ui

import (
	"math"

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
	ActCancel // cancel construction or the last queued unit
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
	{"A", "S", "H", "", "ESC"},
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
	case !d.Building && d.Supply > 0 && pl.Supply+d.Supply > pl.SupplyCap+1e-9:
		return false, sim.NoticeSupply.String()
	}
	return true, ""
}

// CommandCard builds the card for player p's part of the selection.
func CommandCard(w *sim.World, p sim.PlayerID, sel []sim.UnitID) []Button {
	var units []*sim.Unit
	worker, mobile, armed := false, false, false
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
	switch {
	case mobile:
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
			for i, t := range building.Def.Trains {
				if i < CardCols {
					slot(0, i, ActTrain, t, shortName(t))
				}
			}
		}
		if !building.Complete || len(building.Queue) > 0 {
			slot(1, 4, ActCancel, 0, "CANCEL")
		}
	}
	return out
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
