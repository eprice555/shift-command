package ui

import (
	"math"

	"github.com/eprice555/shift-command/internal/geom"
)

// TilePx is the size of one map tile in world pixels at zoom 1.
const TilePx = 32

// Zoom limits.
const (
	MinZoom = 0.5
	MaxZoom = 1.5
)

// Camera maps between world tiles and screen pixels. X, Y is the world-pixel
// position of the screen's top-left corner.
type Camera struct {
	X, Y, Zoom float64
	// ViewW, ViewH is the screen size; ViewBottom is how much of the bottom
	// is covered by the HUD panel (the camera may scroll past it).
	ViewW, ViewH, ViewBottom float64
	// MapW, MapH is the map size in tiles.
	MapW, MapH float64
}

// ToScreen converts a world point (tiles) to screen pixels.
func (c *Camera) ToScreen(p geom.Vec) (float64, float64) {
	return (p.X*TilePx - c.X) * c.Zoom, (p.Y*TilePx - c.Y) * c.Zoom
}

// ToWorld converts screen pixels to a world point (tiles).
func (c *Camera) ToWorld(sx, sy float64) geom.Vec {
	return geom.V((sx/c.Zoom+c.X)/TilePx, (sy/c.Zoom+c.Y)/TilePx)
}

// Scale is the number of screen pixels per tile.
func (c *Camera) Scale() float64 { return TilePx * c.Zoom }

// Visible returns the world rect (tiles) currently on screen.
func (c *Camera) Visible() geom.Rect {
	return geom.RectFrom(c.ToWorld(0, 0), c.ToWorld(c.ViewW, c.ViewH))
}

// Center returns the world point at the middle of the playable view.
func (c *Camera) Center() geom.Vec {
	return c.ToWorld(c.ViewW/2, (c.ViewH-c.ViewBottom)/2)
}

// CenterOn moves the camera so p is in the middle of the playable view.
func (c *Camera) CenterOn(p geom.Vec) {
	cur := c.Center()
	c.Pan((p.X-cur.X)*c.Scale(), (p.Y-cur.Y)*c.Scale())
}

// Pan scrolls by a screen-pixel offset.
func (c *Camera) Pan(dx, dy float64) {
	c.X += dx / c.Zoom
	c.Y += dy / c.Zoom
	c.Clamp()
}

// ZoomAt multiplies the zoom by k, keeping the world point under (sx, sy)
// fixed on screen.
func (c *Camera) ZoomAt(sx, sy, k float64) {
	before := c.ToWorld(sx, sy)
	c.Zoom = min(max(c.Zoom*k, MinZoom), MaxZoom)
	c.X = before.X*TilePx - sx/c.Zoom
	c.Y = before.Y*TilePx - sy/c.Zoom
	c.Clamp()
}

// Clamp keeps the map on screen, letting the camera scroll a little past
// the edges so edge units aren't hidden under the HUD.
func (c *Camera) Clamp() {
	const pad = 2.0 * TilePx
	clamp := func(v, mapPx, viewPx, extra float64) float64 {
		lo, hi := float64(-pad), mapPx+pad+extra-viewPx/c.Zoom
		if hi < lo {
			return (lo + hi) / 2
		}
		return min(max(v, lo), hi)
	}
	c.X = clamp(c.X, c.MapW*TilePx, c.ViewW, 0)
	c.Y = clamp(c.Y, c.MapH*TilePx, c.ViewH, c.ViewBottom/c.Zoom)
	c.X, c.Y = math.Round(c.X*c.Zoom)/c.Zoom, math.Round(c.Y*c.Zoom)/c.Zoom
}
