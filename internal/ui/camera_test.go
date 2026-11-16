package ui

import (
	"math"
	"testing"

	"github.com/eprice555/shift-command/internal/geom"
)

func testCam() *Camera {
	c := &Camera{Zoom: 1, ViewW: 1280, ViewH: 720, ViewBottom: 200, MapW: 80, MapH: 80}
	c.CenterOn(geom.V(40, 40))
	return c
}

func near(a, b geom.Vec, eps float64) bool { return a.Dist(b) <= eps }

func TestCameraRoundTrip(t *testing.T) {
	c := testCam()
	for _, p := range []geom.Vec{geom.V(40, 40), geom.V(31.25, 47.5), geom.V(50, 30)} {
		sx, sy := c.ToScreen(p)
		if got := c.ToWorld(sx, sy); !near(got, p, 1e-9) {
			t.Errorf("round trip %v -> %v", p, got)
		}
	}
	if got := c.Center(); !near(got, geom.V(40, 40), 0.05) {
		t.Errorf("center = %v", got)
	}
}

func TestCameraZoomKeepsCursorPoint(t *testing.T) {
	c := testCam()
	sx, sy := 300.0, 200.0
	before := c.ToWorld(sx, sy)
	c.ZoomAt(sx, sy, 1.25)
	if c.Zoom != 1.25 {
		t.Fatalf("zoom = %v", c.Zoom)
	}
	if got := c.ToWorld(sx, sy); !near(got, before, 0.05) {
		t.Errorf("point under cursor moved %v -> %v", before, got)
	}
	c.ZoomAt(sx, sy, 100)
	if c.Zoom != MaxZoom {
		t.Errorf("zoom not clamped: %v", c.Zoom)
	}
	c.ZoomAt(sx, sy, 0.001)
	if c.Zoom != MinZoom {
		t.Errorf("zoom not clamped: %v", c.Zoom)
	}
}

func TestCameraClampsToMap(t *testing.T) {
	c := testCam()
	c.Pan(-1e6, -1e6)
	if tl := c.ToWorld(0, 0); tl.X < -2.01 || tl.Y < -2.01 {
		t.Errorf("scrolled too far up-left: %v", tl)
	}
	c.Pan(1e6, 1e6)
	br := c.ToWorld(c.ViewW, c.ViewH-c.ViewBottom)
	if br.X > 82.01 || br.Y > 82.01 || math.IsNaN(br.X) {
		t.Errorf("scrolled too far down-right: %v", br)
	}
}
