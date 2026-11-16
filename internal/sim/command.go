package sim

import (
	"math"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
)

// CommandKind is the type of a player command.
type CommandKind uint8

const (
	CmdMove       CommandKind = iota // move to Point, ignoring enemies
	CmdAttackMove                    // move to Point, engaging enemies on the way
	CmdAttack                        // attack Target
	CmdStop                          // drop all orders
	CmdHold                          // drop all orders and hold position
	CmdSpawn                         // debug: spawn SpawnCount × SpawnType at Point
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

	SpawnType  faction.UnitType
	SpawnCount int
}

func (w *World) apply(c *Command) {
	if c.Kind == CmdSpawn {
		if c.SpawnCount > 0 && c.SpawnType < faction.UnitTypeCount {
			w.SpawnSquad(c.Player, c.SpawnType, c.SpawnCount, w.Map.Clamp(c.Point, 0.5))
		}
		return
	}
	units := w.owned(c)
	if len(units) == 0 {
		return
	}
	switch c.Kind {
	case CmdStop, CmdHold:
		for _, u := range units {
			u.clearOrders()
			u.Hold = c.Kind == CmdHold
		}
	case CmdAttack:
		t := w.byID[c.Target]
		if t == nil || t.Owner == c.Player || !w.VisibleTo(c.Player, t) {
			return
		}
		for _, u := range units {
			w.give(u, Order{Kind: OrderAttack, Target: t.ID}, c.Queue)
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
			w.give(u, Order{Kind: kind, Point: p, group: g}, c.Queue)
		}
	}
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
