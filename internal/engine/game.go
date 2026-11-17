// Package engine owns the main loop, scenes, camera, and input, and bridges
// Ebitengine's update/draw callbacks to the fixed-rate sim.
package engine

import (
	"fmt"
	"image"
	"image/png"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
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
	// AutoShiftUpdates morphs the title emblem every N updates (0 = off).
	AutoShiftUpdates int
	// StartMatch skips the title screen.
	StartMatch bool
	// Demo selects the player's army and attack-moves it into the enemy
	// forward squad (implies StartMatch).
	Demo bool
	// SkipSeconds fast-forwards a new match by this much game time.
	SkipSeconds int
	// Reveal starts the match with fog of war off.
	Reveal bool
	// Faction is the player's starting faction choice on the title screen.
	Faction faction.ID
}

// Game implements ebiten.Game.
type Game struct {
	opts     Options
	title    *ui.Title
	match    *Match
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
	g := &Game{opts: opts, title: ui.NewTitle(), renderer: r}
	if faction.Get(opts.Faction).Playable {
		g.title.Form, g.title.PrevForm = opts.Faction, opts.Faction
	}
	if opts.StartMatch || opts.Demo {
		g.startMatch()
	}
	return g, nil
}

func (g *Game) startMatch() {
	g.match = NewMatch(g.opts.Demo, g.title.Form)
	if g.opts.Demo {
		m := g.match
		m.setSelection(m.onScreen(func(*sim.Unit) bool { return true }))
		m.showHelp, m.follow = false, true
		m.commandAt(geom.V(46, 34), nil, true)
	}
	g.match.revealAll = g.opts.Reveal
	g.match.fastForward(g.opts.SkipSeconds * sim.TicksPerSecond)
}

func (g *Game) Update() error {
	if g.err != nil {
		return g.err
	}
	if g.opts.QuitAfterUpdates > 0 && g.updates >= g.opts.QuitAfterUpdates {
		return ebiten.Termination
	}
	g.updates++

	if g.match != nil {
		g.match.Update()
		if g.match.Done() {
			g.match = nil
		}
		return nil
	}
	shift := inpututil.IsKeyJustPressed(ebiten.KeyShiftLeft) || inpututil.IsKeyJustPressed(ebiten.KeyShiftRight)
	if n := g.opts.AutoShiftUpdates; n > 0 && g.updates%n == 0 {
		shift = true
	}
	g.title.Update(shift)
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		g.startMatch()
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	if g.match != nil {
		g.match.takeFX(g.renderer)
		g.renderer.DrawMatch(screen, g.match.View())
	} else {
		g.renderer.DrawTitle(screen, g.title, ebiten.IsFocused())
	}
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
