package world

import (
	"strings"
	"testing"

	"github.com/eprice555/shift-command/internal/geom"
)

// gridFrom parses rows where '#' is blocked.
func gridFrom(rows ...string) *Grid {
	g := NewGrid(len(rows[0]), len(rows))
	for y, row := range rows {
		for x, c := range row {
			g.SetBlocked(x, y, c == '#')
		}
	}
	return g
}

func TestLineOfSight(t *testing.T) {
	g := gridFrom(
		"..........",
		"....#.....",
		"....#.....",
		"..........",
		"...#......",
		"....#.....",
	)
	cases := []struct {
		a, b geom.Vec
		want bool
	}{
		{geom.V(0.5, 0.5), geom.V(9.5, 0.5), true},
		{geom.V(0.5, 1.5), geom.V(9.5, 1.5), false},
		{geom.V(3.5, 3.5), geom.V(9.5, 3.5), true},
		{geom.V(3.5, 3.5), geom.V(3.5, 0.5), true},
		// Exactly through the corner between (3,4) and (4,5): both blocked.
		{geom.V(3, 5), geom.V(5, 3), false},
		{geom.V(2.5, 5.5), geom.V(2.5, 5.5), true},
	}
	for _, c := range cases {
		if got := g.LineOfSight(c.a, c.b); got != c.want {
			t.Errorf("LineOfSight(%v, %v) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func checkPath(t *testing.T, g *Grid, from geom.Vec, path []geom.Vec, r float64) {
	t.Helper()
	if len(path) == 0 {
		t.Fatal("empty path")
	}
	cur := from
	for _, p := range path {
		if !g.LineOfSight(cur, p) {
			t.Fatalf("path segment %v -> %v crosses a wall (path %v)", cur, p, path)
		}
		cur = p
	}
}

func TestFindPathAroundWall(t *testing.T) {
	g := gridFrom(
		"..........",
		"..........",
		"#######...",
		"..........",
		"..........",
	)
	from, to := geom.V(1.5, 0.5), geom.V(1.5, 4.5)
	path := g.FindPath(from, to, 0.3)
	checkPath(t, g, from, path, 0.3)
	if got := path[len(path)-1]; got != to {
		t.Fatalf("path ends at %v, want %v", got, to)
	}
	if len(path) > 4 {
		t.Errorf("path not smoothed: %v", path)
	}
}

func TestFindPathBlockedGoalAndUnreachable(t *testing.T) {
	g := gridFrom(
		".....#...",
		".....#.#.",
		".....#...",
		".....####",
		".........",
	)
	// Goal inside a wall snaps to the nearest open tile.
	path := g.FindPath(geom.V(0.5, 0.5), geom.V(5.5, 1.5), 0)
	end := path[len(path)-1]
	if x, y := TileOf(end); g.Blocked(x, y) || end.Dist(geom.V(5.5, 1.5)) > 1.5 {
		t.Fatalf("blocked goal: path ends at %v", end)
	}
	// Goal inside an enclosed pocket: path ends at the closest reachable tile.
	path = g.FindPath(geom.V(0.5, 4.5), geom.V(6.5, 0.5), 0)
	checkPath(t, g, geom.V(0.5, 4.5), path, 0)
	end = path[len(path)-1]
	if end.Dist(geom.V(6.5, 0.5)) > 2.3 {
		t.Fatalf("unreachable goal: path ends at %v, want near the pocket", end)
	}
}

func TestFlowFieldLeadsToGoal(t *testing.T) {
	m := NewSkirmishMap()
	goal := m.Starts[1]
	f := m.NewFlowField(goal)
	for _, start := range []geom.Vec{m.Starts[0], geom.V(5, 75), geom.V(40, 30)} {
		p := start
		const r, speed = 0.4, 0.3
		steps := 0
		for ; steps < 2000 && p.Dist(goal) > 0.5; steps++ {
			next, ok := f.Next(m.Grid, p, r)
			if !ok {
				t.Fatalf("from %v: unreachable at %v", start, p)
			}
			d := next.Sub(p)
			if d.Len() > speed {
				d = d.Norm().Scale(speed)
			}
			p = m.ResolveCircle(p.Add(d), r)
		}
		if p.Dist(goal) > 0.5 {
			t.Errorf("from %v: stuck at %v after %d steps", start, p, steps)
		}
	}
}

func TestResolveCircle(t *testing.T) {
	g := gridFrom(
		"...",
		".#.",
		"...",
	)
	p := g.ResolveCircle(geom.V(0.8, 1.5), 0.4)
	if p.X > 0.6+1e-9 {
		t.Errorf("not pushed out of wall: %v", p)
	}
	p = g.ResolveCircle(geom.V(1.6, 1.5), 0.3)
	if x, y := TileOf(p); g.Blocked(x, y) {
		t.Errorf("center left inside wall: %v", p)
	}
	p = g.ResolveCircle(geom.V(-1, 5), 0.3)
	if p != geom.V(0.3, 2.7) {
		t.Errorf("not clamped to map: %v", p)
	}
}

func TestFog(t *testing.T) {
	f := NewFog(10, 10)
	f.Reveal(geom.V(5, 5), 2)
	if !f.VisibleAt(geom.V(5.5, 5.5)) || f.VisibleAt(geom.V(8.5, 8.5)) {
		t.Fatal("reveal radius wrong")
	}
	f.Begin()
	if f.At(5, 5) != Explored {
		t.Fatalf("after Begin, revealed tile = %v, want Explored", f.At(5, 5))
	}
	if f.At(0, 0) != Unexplored || f.At(-1, 3) != Unexplored {
		t.Fatal("unrevealed tiles should be unexplored")
	}
}

func TestSkirmishMap(t *testing.T) {
	m := NewSkirmishMap()
	for y := range m.H {
		for x := range m.W {
			if m.Blocked(x, y) != m.Blocked(m.W-1-x, m.H-1-y) {
				t.Fatalf("map not symmetric at (%d,%d)", x, y)
			}
		}
	}
	for _, s := range m.Starts {
		if x, y := TileOf(s); m.Blocked(x, y) {
			t.Fatalf("start %v is blocked", s)
		}
	}
	// Every open tile must be reachable from player 1's start.
	f := m.NewFlowField(m.Starts[0])
	for y := range m.H {
		for x := range m.W {
			if m.Passable(x, y) && !f.Reachable(TileCenter(x, y)) {
				t.Fatalf("open tile (%d,%d) is unreachable", x, y)
			}
		}
	}
	if testing.Verbose() {
		var sb strings.Builder
		for y := range m.H {
			for x := range m.W {
				switch {
				case geom.V(float64(x)+0.5, float64(y)+0.5).Dist(m.Starts[0]) < 1, geom.V(float64(x)+0.5, float64(y)+0.5).Dist(m.Starts[1]) < 1:
					sb.WriteByte('S')
				case m.Blocked(x, y):
					sb.WriteByte('#')
				default:
					sb.WriteByte('.')
				}
			}
			sb.WriteByte('\n')
		}
		t.Log("\n" + sb.String())
	}
}
