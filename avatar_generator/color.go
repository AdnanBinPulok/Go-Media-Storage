package avatar_generator

import (
	"image/color"
	"math"
)

// Soft off-black and warm off-white. Pure #000 on a pastel face looks like a
// hole; a warm charcoal keeps the contrast ratio while reading as gentle.
var (
	eyeDark  = color.RGBA{R: 58, G: 54, B: 64, A: 255}    // #3A3640
	eyeLight = color.RGBA{R: 255, G: 252, B: 247, A: 255} // #FFFCF7
)

func hslToRGB(h, s, l float64) color.RGBA {
	h = math.Mod(math.Mod(h, 360)+360, 360)

	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2

	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}

	return color.RGBA{
		R: uint8(clamp01(r+m) * 255),
		G: uint8(clamp01(g+m) * 255),
		B: uint8(clamp01(b+m) * 255),
		A: 255,
	}
}

func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}

// relativeLuminance is the WCAG measure, including gamma expansion.
func relativeLuminance(c color.RGBA) float64 {
	lin := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.04045 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

// contrastColor picks an eye color from measured luminance rather than HSL
// lightness: pure yellow and pure blue share L=0.5 but read very differently.
func contrastColor(bg color.RGBA) color.RGBA {
	if relativeLuminance(bg) > 0.34 {
		return eyeDark
	}
	return eyeLight
}
