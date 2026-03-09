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
	const size = 16
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	// Draw a simple colored square as placeholder icon
	iconColor := color.RGBA{R: 66, G: 135, B: 245, A: 255} // Blue
	for y := 2; y < size-2; y++ {
		for x := 2; x < size-2; x++ {
			img.Set(x, y, iconColor)
		}
	}

	// Draw a T in white
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	// Top bar of T
	for x := 4; x < 12; x++ {
		img.Set(x, 4, white)
		img.Set(x, 5, white)
	}
	// Stem of T
	for y := 4; y < 12; y++ {
		img.Set(7, y, white)
		img.Set(8, y, white)
	}

	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}
