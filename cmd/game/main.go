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
	flag.IntVar(&opts.AutoShiftUpdates, "autoshift", 0, "title screen: press Shift automatically every N updates")
	flag.BoolVar(&opts.StartMatch, "match", false, "skip the title screen and start the sandbox match")
	flag.BoolVar(&opts.Demo, "demo", false, "start the match with the army attack-moving into the enemy (implies -match)")
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
