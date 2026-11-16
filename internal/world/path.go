package world

import (
	"math"

	"github.com/eprice555/shift-command/internal/geom"
)

// Neighbor offsets: 4 orthogonal first, then 4 diagonal.
var (
	nbrDX = [8]int{1, -1, 0, 0, 1, 1, -1, -1}
	nbrDY = [8]int{0, 0, 1, -1, 1, -1, 1, -1}
)

const diagCost = math.Sqrt2

// canStep reports whether a unit can move from (x, y) to its neighbor
// (x+dx, y+dy). Diagonal steps may not cut blocked corners.
func (g *Grid) canStep(x, y, dx, dy int) bool {
	if g.Blocked(x+dx, y+dy) {
		return false
	}
	if dx != 0 && dy != 0 {
		return g.Passable(x+dx, y) && g.Passable(x, y+dy)
	}
	return true
}

type heapItem struct {
	f   float32
	idx int32
}

// minHeap is a binary min-heap keyed on f, with idx as a tie-breaker so
// results are deterministic.
type minHeap []heapItem

func (h minHeap) less(i, j int) bool {
	if h[i].f != h[j].f {
		return h[i].f < h[j].f
	}
	return h[i].idx < h[j].idx
}

func (h *minHeap) push(it heapItem) {
	*h = append(*h, it)
	s := *h
	for i := len(s) - 1; i > 0; {
		p := (i - 1) / 2
		if !s.less(i, p) {
			break
		}
		s[i], s[p] = s[p], s[i]
		i = p
	}
}

func (h *minHeap) pop() heapItem {
	s := *h
	top := s[0]
	n := len(s) - 1
	s[0] = s[n]
	s = s[:n]
	for i := 0; ; {
		l, r, m := 2*i+1, 2*i+2, i
		if l < n && s.less(l, m) {
			m = l
		}
		if r < n && s.less(r, m) {
			m = r
		}
		if m == i {
			break
		}
		s[i], s[m] = s[m], s[i]
		i = m
	}
	*h = s
	return top
}

type searchScratch struct {
	g     []float32
	came  []int32
	state []uint8 // 0 unseen, 1 open, 2 closed
	open  minHeap
}

func (g *Grid) getScratch() *searchScratch {
	n := g.W * g.H
	if g.scratch == nil {
		g.scratch = &searchScratch{g: make([]float32, n), came: make([]int32, n), state: make([]uint8, n)}
	}
	s := g.scratch
	clear(s.state)
	s.open = s.open[:0]
	return s
}

func octile(ax, ay, bx, by int) float32 {
	dx, dy := abs(ax-bx), abs(ay-by)
	return float32(max(dx, dy)-min(dx, dy)) + float32(diagCost)*float32(min(dx, dy))
}

// FindPath returns waypoints from `from` to `to` for a body of radius r,
// excluding the start. If `to` is blocked, the nearest passable tile is used.
// If the goal is unreachable, the path ends at the closest reachable tile.
// It returns nil only if `from` has no passable tile at all.
func (g *Grid) FindPath(from, to geom.Vec, r float64) []geom.Vec {
	sx, sy := TileOf(from)
	if g.Blocked(sx, sy) {
		var ok bool
		if sx, sy, ok = g.NearestPassable(sx, sy); !ok {
			return nil
		}
	}
	gx, gy := TileOf(to)
	goal := to
	if g.Blocked(gx, gy) {
		gx, gy, _ = g.NearestPassable(gx, gy)
		goal = TileCenter(gx, gy)
	}
	if sx == gx && sy == gy {
		return []geom.Vec{goal}
	}

	s := g.getScratch()
	w := g.W
	start, target := int32(sy*w+sx), int32(gy*w+gx)
	s.g[start] = 0
	s.came[start] = -1
	s.state[start] = 1
	s.open.push(heapItem{octile(sx, sy, gx, gy), start})
	best, bestH := start, octile(sx, sy, gx, gy)
	found := false
	for len(s.open) > 0 {
		cur := s.open.pop().idx
		if s.state[cur] == 2 {
			continue
		}
		s.state[cur] = 2
		if cur == target {
			found = true
			break
		}
		cx, cy := int(cur)%w, int(cur)/w
		if h := octile(cx, cy, gx, gy); h < bestH {
			best, bestH = cur, h
		}
		for i := range 8 {
			dx, dy := nbrDX[i], nbrDY[i]
			if !g.canStep(cx, cy, dx, dy) {
				continue
			}
			nx, ny := cx+dx, cy+dy
			ni := int32(ny*w + nx)
			if s.state[ni] == 2 {
				continue
			}
			step := float32(1)
			if i >= 4 {
				step = float32(diagCost)
			}
			ng := s.g[cur] + step
			if s.state[ni] == 1 && ng >= s.g[ni] {
				continue
			}
			s.g[ni] = ng
			s.came[ni] = cur
			s.state[ni] = 1
			s.open.push(heapItem{ng + octile(nx, ny, gx, gy), ni})
		}
	}

	end := target
	if !found {
		end = best
		goal = TileCenter(int(best)%w, int(best)/w)
		if best == start {
			return []geom.Vec{goal}
		}
	}
	var tiles []geom.Vec
	for i := end; i != -1 && i != start; i = s.came[i] {
		tiles = append(tiles, TileCenter(int(i)%w, int(i)/w))
	}
	for i, j := 0, len(tiles)-1; i < j; i, j = i+1, j-1 {
		tiles[i], tiles[j] = tiles[j], tiles[i]
	}
	tiles[len(tiles)-1] = goal
	return g.smooth(from, tiles, r)
}

// smooth drops waypoints that can be skipped with a clear straight line.
func (g *Grid) smooth(from geom.Vec, pts []geom.Vec, r float64) []geom.Vec {
	out := make([]geom.Vec, 0, 8)
	cur := from
	for i := 0; i < len(pts); {
		j := i
		for j+1 < len(pts) && g.ClearPath(cur, pts[j+1], r) {
			j++
		}
		out = append(out, pts[j])
		cur = pts[j]
		i = j + 1
	}
	return out
}

// FlowField is a goal-distance field shared by every unit in a group move.
// Each unit descends it from its own tile, so one search serves the group.
type FlowField struct {
	W, H int
	Goal geom.Vec
	dist []float32
}

// NewFlowField builds a flow field toward goal (snapped to a passable tile).
func (g *Grid) NewFlowField(goal geom.Vec) *FlowField {
	gx, gy := TileOf(goal)
	if g.Blocked(gx, gy) {
		gx, gy, _ = g.NearestPassable(gx, gy)
		goal = TileCenter(gx, gy)
	}
	f := &FlowField{W: g.W, H: g.H, Goal: goal, dist: make([]float32, g.W*g.H)}
	inf := float32(math.Inf(1))
	for i := range f.dist {
		f.dist[i] = inf
	}
	w := g.W
	t := int32(gy*w + gx)
	f.dist[t] = 0
	open := minHeap{{0, t}}
	for len(open) > 0 {
		it := open.pop()
		if it.f > f.dist[it.idx] {
			continue
		}
		cx, cy := int(it.idx)%w, int(it.idx)/w
		for i := range 8 {
			dx, dy := nbrDX[i], nbrDY[i]
			if !g.canStep(cx, cy, dx, dy) {
				continue
			}
			ni := int32((cy+dy)*w + cx + dx)
			step := float32(1)
			if i >= 4 {
				step = float32(diagCost)
			}
			if nd := it.f + step; nd < f.dist[ni] {
				f.dist[ni] = nd
				open.push(heapItem{nd, ni})
			}
		}
	}
	return f
}

// DistAt returns the path distance from tile (x, y) to the goal, or +Inf if
// unreachable.
func (f *FlowField) DistAt(x, y int) float32 {
	if x < 0 || y < 0 || x >= f.W || y >= f.H {
		return float32(math.Inf(1))
	}
	return f.dist[y*f.W+x]
}

// Reachable reports whether p's tile can reach the goal.
func (f *FlowField) Reachable(p geom.Vec) bool {
	return !math.IsInf(float64(f.DistAt(TileOf(p))), 1)
}

// flowLookahead is how many descent steps Next considers for shortcuts.
const flowLookahead = 6

// Next returns the point a body of radius r at p should steer toward. It
// heads straight for the goal when the way is clear; otherwise it follows
// the field and cuts toward the farthest visible point a few steps ahead.
// ok is false if p cannot reach the goal.
func (f *FlowField) Next(g *Grid, p geom.Vec, r float64) (geom.Vec, bool) {
	if g.ClearPath(p, f.Goal, r) {
		return f.Goal, true
	}
	x, y := TileOf(p)
	if g.Blocked(x, y) {
		x, y, _ = g.NearestPassable(x, y)
	}
	if math.IsInf(float64(f.DistAt(x, y)), 1) {
		return geom.Vec{}, false
	}
	var steps [flowLookahead]geom.Vec
	n := 0
	for n < flowLookahead {
		bd, bx, by := f.DistAt(x, y), -1, -1
		for i := range 8 {
			dx, dy := nbrDX[i], nbrDY[i]
			if !g.canStep(x, y, dx, dy) {
				continue
			}
			if d := f.DistAt(x+dx, y+dy); d < bd {
				bd, bx, by = d, x+dx, y+dy
			}
		}
		if bx < 0 {
			break // at the goal tile
		}
		x, y = bx, by
		steps[n] = TileCenter(x, y)
		n++
	}
	if n == 0 {
		return f.Goal, true
	}
	for i := n - 1; i > 0; i-- {
		if g.ClearPath(p, steps[i], r) {
			return steps[i], true
		}
	}
	return steps[0], true
}
