// Package engine owns the main loop, camera, and input, and bridges
// Ebitengine's update/draw callbacks to the fixed-rate sim.
package engine

import (
	"fmt"
	"image"
	"image/png"
	"os"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/eprice555/shift-command/internal/sim"
	"github.com/eprice555/shift-command/internal/ui"
)

const (
	ScreenWidth  = 1280
	ScreenHeight = 720
	// UpdateTPS is Ebitengine's update rate; input is polled at this rate
	// and the sim is stepped at sim.TicksPerSecond.
	UpdateTPS = 60
)

// Options are debug/automation switches, mainly for smoke tests.
type Options struct {
	// QuitAfterUpdates exits after this many updates (0 = run forever).
	QuitAfterUpdates int
	// ScreenshotPath, if set, saves the frame drawn at QuitAfterUpdates.
	ScreenshotPath string
	// AutoShiftTicks injects a Shift press every N sim ticks (0 = off).
	AutoShiftTicks int
}

// Game implements ebiten.Game.
type Game struct {
	opts     Options
	world    *sim.World
	clock    *FixedStep
	pending  []sim.Command
	renderer *ui.Renderer
	updates  int
	err      error
}

// NewGame builds a game ready for ebiten.RunGame.
func NewGame(opts Options) (*Game, error) {
	r, err := ui.NewRenderer()
	if err != nil {
		return nil, err
	}
	return &Game{
		opts:     opts,
		world:    sim.NewWorld(),
		clock:    NewFixedStep(sim.TicksPerSecond, UpdateTPS),
		renderer: r,
	}, nil
}

func (g *Game) Update() error {
	if g.err != nil {
		return g.err
	}
	if g.opts.QuitAfterUpdates > 0 && g.updates >= g.opts.QuitAfterUpdates {
		return ebiten.Termination
	}
	g.updates++

	g.pending = pollCommands(g.pending)
	for range g.clock.Advance() {
		if n := g.opts.AutoShiftTicks; n > 0 && g.world.Tick%uint64(n) == uint64(n-1) {
			g.pending = append(g.pending, sim.CmdShift)
		}
		g.world.Step(g.pending)
		g.pending = g.pending[:0]
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.renderer.Draw(screen, ui.Frame{World: g.world, Alpha: g.clock.Alpha()})
	if g.opts.ScreenshotPath != "" && g.updates == g.opts.QuitAfterUpdates {
		if err := saveScreenshot(screen, g.opts.ScreenshotPath); err != nil {
			g.err = err
		}
	}
}

func (g *Game) Layout(_, _ int) (int, int) { return ScreenWidth, ScreenHeight }

func saveScreenshot(screen *ebiten.Image, path string) error {
	b := screen.Bounds()
	img := image.NewRGBA(b)
	screen.ReadPixels(img.Pix)
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("screenshot: %w", err)
	}
	defer f.Close()
	return png.Encode(f, img)
}
