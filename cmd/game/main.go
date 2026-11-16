// Command game runs Shift Command natively or as a WASM build.
package main

import (
	"flag"
	"log"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/eprice555/shift-command/internal/engine"
)

func main() {
	var opts engine.Options
	flag.IntVar(&opts.QuitAfterUpdates, "updates", 0, "quit after N updates (smoke testing; 0 = run forever)")
	flag.StringVar(&opts.ScreenshotPath, "screenshot", "", "save a PNG of the final frame (requires -updates)")
	flag.IntVar(&opts.AutoShiftTicks, "autoshift", 0, "press Shift automatically every N sim ticks (demo)")
	flag.Parse()

	ebiten.SetWindowSize(engine.ScreenWidth, engine.ScreenHeight)
	ebiten.SetWindowTitle("Shift Command")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetTPS(engine.UpdateTPS)

	g, err := engine.NewGame(opts)
	if err != nil {
		log.Fatal(err)
	}
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
