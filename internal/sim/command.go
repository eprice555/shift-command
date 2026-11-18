package sim

import (
	"math"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
	"github.com/eprice555/shift-command/internal/world"
)

// CommandKind is the type of a player command.
type CommandKind uint8

const (
	CmdMove       CommandKind = iota // move to Point, ignoring enemies
	CmdAttackMove                    // move to Point, engaging enemies on the way
	CmdAttack                        // attack Target
	CmdStop                          // drop all orders
	CmdHold                          // drop all orders and hold position
	CmdSpawn                         // debug: spawn Count × Type at Point
	CmdGather                        // workers: gather Res
	CmdBuild                         // one worker: build Type at tile (TX, TY)
	CmdTrain                         // buildings: queue Type at the least busy one
	CmdCancel                        // first building: cancel construction or queue entry Index (-1 = last); else all cocoons
	CmdRally                         // buildings: rally new units to Point (or Res)
	CmdAssist                        // workers: construct/repair Target or bank cargo there; others move to it
	CmdShift                         // toggle SHIFT modes (Index -1), or set mode Index (0 base, 1 shifted)
	CmdMorph                         // Brood: re-morph one selected unit into Type
)

// Command is a player instruction. Commands are the only way input reaches
// the sim, so they are what a replay or AI would record or emit.
type Command struct {
	Kind   CommandKind
	Player PlayerID
	Units  []UnitID
	Point  geom.Vec
	Target UnitID
	// Queue appends the order instead of replacing current orders.
	Queue bool

	Res    ResourceID
	Type   faction.UnitType
	Count  int
	TX, TY int
	Index  int
}

func (w *World) apply(c *Command) {
	if c.Kind == CmdSpawn {
		if c.Count > 0 && c.Type < faction.UnitTypeCount {
			w.SpawnSquad(c.Player, c.Type, c.Count, w.Map.Clamp(c.Point, 0.5))
		}
		return
	}
	all := w.owned(c)
	if len(all) == 0 {
		return
	}
	switch c.Kind {
	case CmdTrain:
		if c.Type < faction.UnitTypeCount {
			w.train(c.Player, all, c.Type)
		}
		return
	case CmdCancel:
		for _, b := range all {
			if b.Def.Building {
				w.cancel(c.Player, b, c.Index)
				return
			}
		}
		// No building selected: every selected cocoon reverts to Spawn.
		for _, u := range all {
			if u.Morphing() {
				w.cancel(c.Player, u, 0)
			}
		}
		return
	case CmdShift:
		w.shift(c, all)
		return
	case CmdMorph:
		if c.Type < faction.UnitTypeCount {
			w.remorphOne(c.Player, all, c.Type)
		}
		return
	case CmdRally:
		for _, b := range all {
			if b.Def.Building && b.Mode == 0 {
				b.HasRally, b.Rally, b.RallyRes = true, w.Map.Clamp(c.Point, 0.5), 0
				if r := w.resByID[c.Res]; r != nil {
					b.Rally, b.RallyRes = r.Pos, r.ID
				}
			}
		}
		return
	}
	// The remaining commands only move mobile units (and packed buildings,
	// which fly wherever they are sent). Units in an immobile mode only
	// take stop, hold, and attack orders.
	units := all[:0:0]
	for _, u := range all {
		switch {
		case u.Def.Building && u.Mode == 0:
		case u.Def.Speed == 0 && c.Kind != CmdStop && c.Kind != CmdHold && c.Kind != CmdAttack:
		default:
			units = append(units, u)
		}
	}
	if len(units) == 0 {
		return
	}
	switch c.Kind {
	case CmdGather:
		r := w.resByID[c.Res]
		if r == nil {
			return
		}
		if r.Kind == world.Flux && !w.mineable(c.Player, r) {
			w.notice(c.Player, NoticeNeedVent, faction.Get(w.Players[c.Player].Faction).Extractor)
			return
		}
		for _, u := range units {
			if u.Def.Worker {
				w.give(u, Order{Kind: OrderGather, Res: r.ID}, c.Queue)
			} else {
				w.give(u, Order{Kind: OrderMove, Point: w.approach(u.Pos, u.Def.Radius, r.Rect())}, c.Queue)
			}
		}
		return
	case CmdBuild:
		w.orderBuild(c, units)
		return
	case CmdAssist:
		t := w.byID[c.Target]
		if t == nil || t.Owner != c.Player {
			return
		}
		for _, u := range units {
			switch {
			case u.Def.Worker && t.Def.Building && !t.Complete && !t.Def.SelfBuild:
				w.give(u, Order{Kind: OrderConstruct, Target: t.ID}, c.Queue)
			case u.Def.Worker && t.Def.DropOff && u.Cargo > 0:
				w.give(u, Order{Kind: OrderReturn}, c.Queue)
			case u.Def.Worker && t.Def.Mech && t.HP < t.Def.HP && t != u:
				w.give(u, Order{Kind: OrderRepair, Target: t.ID}, c.Queue)
			default:
				w.give(u, Order{Kind: OrderMove, Point: w.approach(u.Pos, u.Def.Radius, t.Rect())}, c.Queue)
			}
		}
		return
	}
	switch c.Kind {
	case CmdStop, CmdHold:
		for _, u := range units {
			u.clearOrders()
			u.mining = 0
			u.Hold = c.Kind == CmdHold
		}
	case CmdAttack:
		t := w.byID[c.Target]
		if t == nil || t.Owner == c.Player || !w.VisibleTo(c.Player, t) {
			return
		}
		for _, u := range units {
			if u.Def.Armed() {
				w.give(u, Order{Kind: OrderAttack, Target: t.ID}, c.Queue)
			} else if u.Def.Speed > 0 {
				w.give(u, Order{Kind: OrderMove, Point: t.Pos}, c.Queue)
			}
		}
	case CmdMove, CmdAttackMove:
		kind := OrderMove
		if c.Kind == CmdAttackMove {
			kind = OrderAttackMove
		}
		p := w.Map.SnapPassable(w.Map.Clamp(c.Point, 0.5))
		var g *moveGroup
		if len(units) > 1 {
			area := 0.0
			for _, u := range units {
				area += u.Def.Radius * u.Def.Radius
			}
			g = &moveGroup{
				size:   len(units),
				radius: math.Sqrt(area/0.6) + 0.2,
				flow:   w.Map.NewFlowField(p),
			}
		}
		for _, u := range units {
			if u.Def.Building {
				// Packed buildings fly over anything.
				w.give(u, Order{Kind: OrderMove, Point: w.Map.Clamp(c.Point, float64(u.Def.Size)/2)}, c.Queue)
				continue
			}
			w.give(u, Order{Kind: kind, Point: p, group: g}, c.Queue)
		}
	}
}

// orderBuild sends the worker closest to the site to build there, after
// checking the site and funds so the player gets immediate feedback.
func (w *World) orderBuild(c *Command, units []*Unit) {
	def := faction.Unit(c.Type)
	if c.Type >= faction.UnitTypeCount || !def.Building {
		return
	}
	if n := w.CanPlace(c.Player, c.Type, c.TX, c.TY); n != NoticeNone {
		missing, _ := w.Missing(c.Player, c.Type)
		w.notice(c.Player, n, missing)
		return
	}
	site := world.TileRect(c.TX, c.TY, def.Size)
	var best *Unit
	bestD := math.Inf(1)
	for _, u := range units {
		if !u.Def.Worker || u.Def.Faction != def.Faction {
			continue
		}
		if d := rectDist(site, u.Pos); d < bestD {
			best, bestD = u, d
		}
	}
	if best == nil {
		return
	}
	n := float64(def.Size) / 2
	w.give(best, Order{Kind: OrderBuild, Build: c.Type, TX: c.TX, TY: c.TY,
		Point: geom.V(float64(c.TX)+n, float64(c.TY)+n)}, c.Queue)
}

// owned resolves the command's unit list to live units belonging to the
// issuing player, dropping duplicates.
func (w *World) owned(c *Command) []*Unit {
	out := make([]*Unit, 0, len(c.Units))
	for _, id := range c.Units {
		u := w.byID[id]
		if u == nil || u.Owner != c.Player {
			continue
		}
		dup := false
		for _, o := range out {
			if o == u {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, u)
		}
	}
	return out
}

func (w *World) give(u *Unit, o Order, queue bool) {
	if len(u.Orders) > 0 && u.Orders[0].Kind == OrderGather {
		u.lastRes = u.Orders[0].Res
	}
	if !queue || u.Hold {
		u.clearOrders()
	}
	u.Hold = false
	u.arrived = nil
	if len(u.Orders) == 0 {
		u.resetNav()
	}
	u.Orders = append(u.Orders, o)
}
