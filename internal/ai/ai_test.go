package ai

import (
	"fmt"
	"hash/fnv"
	"testing"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
	"github.com/eprice555/shift-command/internal/sim"
)

// run steps w for n ticks with the given AIs playing.
func run(w *sim.World, n int, ais ...*AI) (notices map[sim.Notice]int) {
	notices = map[sim.Notice]int{}
	var cmds []sim.Command
	for range n {
		if w.Over {
			break
		}
		cmds = cmds[:0]
		for _, a := range ais {
			cmds = append(cmds, a.Think(w)...)
		}
		w.Step(cmds)
		for _, ev := range w.Events {
			for _, a := range ais {
				if ev.Kind == sim.EvNotice && ev.Owner == a.P {
					notices[ev.Notice]++
				}
			}
		}
	}
	return notices
}

func counts(w *sim.World, p sim.PlayerID) (c [faction.UnitTypeCount]int) {
	for _, u := range w.Units {
		if u.Owner == p {
			c[u.Type]++
		}
	}
	return c
}

const minute = 60 * sim.TicksPerSecond

// bad lists refusals that mean the AI misjudged a site or requirement.
var bad = []sim.Notice{sim.NoticeCantBuild, sim.NoticeNeedVent, sim.NoticeRequires, sim.NoticeQueueFull}

func checkNotices(t *testing.T, n map[sim.Notice]int) {
	t.Helper()
	for _, k := range bad {
		if n[k] > 0 {
			t.Errorf("AI got %d %q notices", n[k], k)
		}
	}
}

func TestOpeningBuildsBase(t *testing.T) {
	for p := range sim.PlayerID(sim.NumPlayers) {
		t.Run(fmt.Sprint("P", p), func(t *testing.T) {
			w := sim.NewSkirmish()
			a := New(w, p)
			checkNotices(t, run(w, 4*minute+30*sim.TicksPerSecond, a))
			c := counts(w, p)
			for _, b := range []faction.UnitType{faction.Generator, faction.DrillYard, faction.Extractor, faction.Foundry} {
				if c[b] == 0 {
					t.Errorf("no %s after 4.5 min: %v", faction.Unit(b).Name, c)
				}
			}
			if c[faction.CommandHub] < 2 {
				t.Errorf("did not expand: %v", c)
			}
			if c[faction.Rigger] < 16 {
				t.Errorf("only %d workers", c[faction.Rigger])
			}
			if army := c[faction.Trooper] + c[faction.Outrider] + c[faction.Strider] + c[faction.Bastion]; army < 4 {
				t.Errorf("army of %d", army)
			}
		})
	}
}

func TestBeatsPassivePlayer(t *testing.T) {
	for p := range sim.PlayerID(sim.NumPlayers) {
		t.Run(fmt.Sprint("P", p), func(t *testing.T) {
			w := sim.NewSkirmish()
			checkNotices(t, run(w, 12*minute, New(w, p)))
			if !w.Over || w.Winner != p {
				t.Fatalf("after %d s: over %v winner %d", w.Tick/sim.TicksPerSecond, w.Over, w.Winner)
			}
		})
	}
}

func TestDefendsBase(t *testing.T) {
	w := sim.NewSkirmish()
	a := New(w, 1)
	run(w, 4*minute, a)
	if a.s.armySupply < 3 {
		t.Fatalf("army too small to test: %v", a.s.armySupply)
	}
	raid := w.SpawnSquad(0, faction.Trooper, 3, w.Map.Starts[1].Add(geom.V(-3, 7)))
	run(w, 40*sim.TicksPerSecond, a)
	for _, u := range raid {
		if w.Unit(u.ID) != nil {
			t.Errorf("raider %d survived 40 s in the AI base", u.ID)
		}
	}
	if w.Over {
		t.Fatal("raid ended the game")
	}
}

func TestDeterministic(t *testing.T) {
	sum := func() uint64 {
		w := sim.NewSkirmish()
		run(w, 8*minute, New(w, 0), New(w, 1))
		h := fnv.New64a()
		for _, u := range w.Units {
			fmt.Fprintf(h, "%d %d %d %.6f %.6f %.6f %d;", u.ID, u.Owner, u.Type, u.Pos.X, u.Pos.Y, u.HP, len(u.Orders))
		}
		fmt.Fprint(h, w.Players)
		return h.Sum64()
	}
	if a, b := sum(), sum(); a != b {
		t.Fatalf("AI vs AI diverged: %x vs %x", a, b)
	}
}

func TestAIvsAIFinishes(t *testing.T) {
	w := sim.NewSkirmish()
	n := run(w, 25*minute, New(w, 0), New(w, 1))
	checkNotices(t, n)
	if !w.Over {
		c0, c1 := counts(w, 0), counts(w, 1)
		t.Fatalf("no winner after 25 min: %v vs %v", c0, c1)
	}
}
