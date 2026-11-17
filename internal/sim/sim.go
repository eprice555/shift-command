// Package sim is the deterministic game simulation (units, combat, orders).
// It advances in fixed ticks and must not depend on rendering or wall-clock
// time, so it stays replayable and testable. Positions are in tile units.
package sim

import (
	"math"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
	"github.com/eprice555/shift-command/internal/world"
)

// TicksPerSecond is the fixed simulation rate.
const TicksPerSecond = 20

// NumPlayers is the number of sides in a match.
const NumPlayers = 2

// PlayerID identifies a side. Player 0 is the human in single-player.
type PlayerID uint8

// UnitID identifies a unit for the whole match. 0 means "no unit".
type UnitID uint32

// EventKind classifies sim events for the renderer and audio.
type EventKind uint8

const (
	EvShot         EventKind = iota // a unit fired: From → To
	EvDeath                         // a unit died at From
	EvNotice                        // a command was refused (Notice; Type for "Requires")
	EvUnitReady                     // a unit finished training at From
	EvBuildingDone                  // a building finished construction at From
	EvHeal                          // a healer pulsed: From → To
	EvShift                         // a unit switched SHIFT mode at From
	EvAura                          // a heal aura pulsed at From (radius Splash)
)

// Event is something that happened during a tick. The sim never reads its
// own events; they exist so presentation can react (muzzle lines, bursts).
type Event struct {
	Kind   EventKind
	Owner  PlayerID
	Type   faction.UnitType
	From   geom.Vec
	To     geom.Vec
	Attack faction.AttackType
	Notice Notice
	Splash float64 // shots: splash radius at To; auras: radius
}

// Ghost is a player's memory of an enemy building last seen in the fog.
type Ghost struct {
	Type     faction.UnitType
	Owner    PlayerID
	TileX    int
	TileY    int
	Complete bool
}

// World is the full simulation state.
type World struct {
	Tick  uint64
	Map   *world.Map
	Units []*Unit // live units in ID order
	Fog   [NumPlayers]*world.Fog
	// Events lists what happened during the most recent Step.
	Events []Event

	Players   [NumPlayers]Player
	Resources []*Resource
	// Seen holds each player's last sighting of enemy buildings.
	Seen [NumPlayers]map[UnitID]Ghost

	// Over is set once a side has lost all its buildings.
	Over   bool
	Winner PlayerID
	Draw   bool

	byID    map[UnitID]*Unit
	resByID map[ResourceID]*Resource
	nextID  UnitID
	nextRes ResourceID
	hits    []hit
	vel     []geom.Vec
}

type hit struct {
	target UnitID
	damage float64
}

// NewWorld returns a world on map m with its resource nodes and no units.
func NewWorld(m *world.Map) *World {
	w := &World{Map: m, byID: map[UnitID]*Unit{}, resByID: map[ResourceID]*Resource{}, nextID: 1, nextRes: 1}
	for i := range w.Fog {
		w.Fog[i] = world.NewFog(m.W, m.H)
		w.Seen[i] = map[UnitID]Ghost{}
	}
	w.addResources()
	return w
}

// Unit returns the live unit with id, or nil.
func (w *World) Unit(id UnitID) *Unit { return w.byID[id] }

// Spawn adds a unit at pos (nudged out of walls) and returns it. Buildings
// are placed complete with their footprint centered near pos.
func (w *World) Spawn(owner PlayerID, t faction.UnitType, pos geom.Vec) *Unit {
	def := faction.Unit(t)
	if def.Building {
		h := float64(def.Size) / 2
		x, y := world.TileOf(pos.Sub(geom.V(h, h)).Add(geom.V(0.5, 0.5)))
		return w.PlaceBuilding(owner, t, x, y, true)
	}
	pos = w.Map.ResolveCircle(w.Map.SnapPassable(pos), def.Radius)
	u := &Unit{
		ID: w.nextID, Owner: owner, Type: t, Def: def,
		Pos: pos, PrevPos: pos, HP: def.HP, Complete: true,
	}
	if owner == 1 {
		u.Facing = math.Pi
	}
	u.PrevFacing = u.Facing
	w.nextID++
	w.Units = append(w.Units, u)
	w.byID[u.ID] = u
	w.Fog[owner].Reveal(u.Pos, def.Sight)
	return u
}

// SpawnSquad spawns n units of type t in a compact block centered on c.
func (w *World) SpawnSquad(owner PlayerID, t faction.UnitType, n int, c geom.Vec) []*Unit {
	spacing := faction.Unit(t).Radius*2 + 0.25
	cols := 1
	for cols*cols < n {
		cols++
	}
	rows := (n + cols - 1) / cols
	out := make([]*Unit, 0, n)
	for i := range n {
		x := (float64(i%cols) - float64(cols-1)/2) * spacing
		y := (float64(i/cols) - float64(rows-1)/2) * spacing
		out = append(out, w.Spawn(owner, t, c.Add(geom.V(x, y))))
	}
	return out
}

// Count returns how many live units owner has.
func (w *World) Count(owner PlayerID) int {
	n := 0
	for _, u := range w.Units {
		if u.Owner == owner {
			n++
		}
	}
	return n
}

// VisibleTo reports whether player p can currently see unit u.
func (w *World) VisibleTo(p PlayerID, u *Unit) bool {
	return u.Owner == p || w.Fog[p].VisibleAt(u.Pos)
}

// Step applies cmds and advances the world by one tick.
func (w *World) Step(cmds []Command) {
	w.Events = w.Events[:0]
	for _, u := range w.Units {
		u.PrevPos, u.PrevFacing = u.Pos, u.Facing
	}
	w.recount()
	for i := range cmds {
		w.apply(&cmds[i])
	}
	w.countAssignments()

	w.vel = w.vel[:0]
	w.hits = w.hits[:0]
	for _, u := range w.Units {
		w.vel = append(w.vel, w.think(u))
	}
	// Units trained or buildings placed this tick start moving next tick.
	for len(w.vel) < len(w.Units) {
		w.vel = append(w.vel, geom.Vec{})
	}
	for i, u := range w.Units {
		v := w.vel[i]
		u.Pos = u.Pos.Add(v)
		u.Moving = v != (geom.Vec{})
		if u.Moving && u.target == 0 && !u.Def.Building {
			u.Facing = v.Angle()
		}
		if u.lifted {
			u.syncTiles()
		}
	}
	w.separate()
	for i, u := range w.Units {
		if u.Def.Building {
			continue
		}
		u.Pos = w.Map.ResolveCircle(u.Pos, u.Def.Radius)
		if want := w.vel[i].Len(); want > 0 && u.Pos.Dist(u.PrevPos) < want*0.2 {
			u.stuck++
		} else {
			u.stuck = 0
		}
	}

	// Damage lands after every unit has acted, so iteration order never
	// decides who shoots first.
	// Negative damage is healing or repair.
	for _, h := range w.hits {
		if t := w.byID[h.target]; t != nil {
			t.HP = min(t.HP-h.damage, max(t.HP, t.Def.HP))
		}
	}
	w.removeDead()
	w.updateFog()
	w.recount()
	w.checkOver()
	w.Tick++
}

func (w *World) removeDead() {
	live := w.Units[:0]
	for _, u := range w.Units {
		if u.HP > 0 {
			live = append(live, u)
			continue
		}
		delete(w.byID, u.ID)
		if u.Def.Building && !u.lifted {
			w.Map.Occupy(u.TileX, u.TileY, u.Def.Size, -1)
		}
		w.Events = append(w.Events, Event{Kind: EvDeath, Owner: u.Owner, Type: u.Type, From: u.Pos})
	}
	clear(w.Units[len(live):])
	w.Units = live
}

func (w *World) updateFog() {
	for _, f := range w.Fog {
		f.Begin()
	}
	for _, u := range w.Units {
		w.Fog[u.Owner].Reveal(u.Pos, u.Def.Sight)
	}
	for p := range w.Seen {
		seen, fog := w.Seen[p], w.Fog[p]
		for id, g := range seen {
			c := faction.Unit(g.Type)
			n := float64(c.Size) / 2
			if fog.VisibleAt(geom.V(float64(g.TileX)+n, float64(g.TileY)+n)) && w.byID[id] == nil {
				delete(seen, id)
			}
		}
		for _, u := range w.Units {
			if u.Def.Building && u.Owner != PlayerID(p) && fog.VisibleAt(u.Pos) {
				seen[u.ID] = Ghost{Type: u.Type, Owner: u.Owner, TileX: u.TileX, TileY: u.TileY, Complete: u.Complete}
			}
		}
	}
}

// separate pushes overlapping units apart. Busy units (holding, firing)
// yield less than idle ones, so a moving squad shoves idle units aside
// instead of getting stuck on them.
func (w *World) separate() {
	us := w.Units
	for i := range us {
		a := us[i]
		if a.Def.Building {
			continue
		}
		for j := i + 1; j < len(us); j++ {
			b := us[j]
			if b.Def.Building || a.Owner == b.Owner && a.gathering() && b.gathering() {
				continue
			}
			minD := a.Def.Radius + b.Def.Radius
			dx, dy := b.Pos.X-a.Pos.X, b.Pos.Y-a.Pos.Y
			if dx > minD || dx < -minD || dy > minD || dy < -minD {
				continue
			}
			d2 := dx*dx + dy*dy
			if d2 >= minD*minD {
				continue
			}
			var n geom.Vec
			d := 0.0
			if d2 == 0 {
				// Exactly stacked: split along a direction derived from IDs.
				n = geom.V(1, 0)
				if (a.ID+b.ID)%2 == 1 {
					n = geom.V(0, 1)
				}
			} else {
				d = math.Sqrt(d2)
				n = geom.V(dx/d, dy/d)
			}
			overlap := (minD - d) * 0.8
			wa, wb := a.yield(), b.yield()
			s := wa + wb
			a.Pos = a.Pos.Sub(n.Scale(overlap * wa / s))
			b.Pos = b.Pos.Add(n.Scale(overlap * wb / s))
		}
	}
}
