//go:build systray

package systray

// iconData is a minimal 16x16 PNG icon (a simple clock shape).
// Generated as a tiny valid PNG to avoid needing external icon files.
// Replace with a proper icon file later if desired.
//
//go:generate echo "Use a proper icon file for production"

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

func generateIcon() []byte {
	const size = 22
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	// Transparent background (default RGBA is all zeros = transparent)

	// Draw a white T
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}

	// Top bar of T
	for x := 3; x < 19; x++ {
		for y := 3; y < 7; y++ {
			img.Set(x, y, white)
		}
	}
	// Stem of T
	for y := 7; y < 19; y++ {
		for x := 8; x < 14; x++ {
			img.Set(x, y, white)
		}
	}

	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}
