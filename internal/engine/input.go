package engine

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/eprice555/shift-command/internal/sim"
)

// pollCommands appends sim commands for input that happened this update.
func pollCommands(dst []sim.Command) []sim.Command {
	if inpututil.IsKeyJustPressed(ebiten.KeyShiftLeft) || inpututil.IsKeyJustPressed(ebiten.KeyShiftRight) {
		dst = append(dst, sim.CmdShift)
	}
	return dst
}
