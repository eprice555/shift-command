// Package assets provides code-generated and embedded resources. The game
// ships no external art or audio; fonts come from the Go font family and
// audio will be synthesized at startup.
package assets

import (
	"bytes"
	"sync"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomonobold"
)

var (
	fontsOnce        sync.Once
	boldSrc, monoSrc *text.GoTextFaceSource
	fontsErr         error
)

func loadFonts() {
	if boldSrc, fontsErr = text.NewGoTextFaceSource(bytes.NewReader(gobold.TTF)); fontsErr != nil {
		return
	}
	monoSrc, fontsErr = text.NewGoTextFaceSource(bytes.NewReader(gomonobold.TTF))
}

// BoldFont returns the display font (Go Bold).
func BoldFont() (*text.GoTextFaceSource, error) {
	fontsOnce.Do(loadFonts)
	return boldSrc, fontsErr
}

// MonoFont returns the HUD font (Go Mono Bold).
func MonoFont() (*text.GoTextFaceSource, error) {
	fontsOnce.Do(loadFonts)
	return monoSrc, fontsErr
}
