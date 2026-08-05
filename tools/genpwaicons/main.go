// genpwaicons generates the static PWA icons for probakgo. Run it once when
// the brand colours or the icon design change. The icons use the same blue
// gradient and hdd-rack glyph as the sidebar, login page and favicon.
//
//	go run ./tools/genpwaicons
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"
)

const (
	brandTop    = "#3b82f6"
	brandBottom = "#1d4ed8"
	white       = "#ffffff"
)

func mustParseHex(s string) color.RGBA {
	var c color.RGBA
	c.A = 0xff
	if len(s) != 7 || s[0] != '#' {
		log.Fatalf("bad hex %q", s)
	}
	parse := func(i int) uint8 {
		var v uint8
		for _, ch := range s[i : i+2] {
			v <<= 4
			switch {
			case ch >= '0' && ch <= '9':
				v |= uint8(ch - '0')
			case ch >= 'a' && ch <= 'f':
				v |= uint8(ch-'a') + 10
			case ch >= 'A' && ch <= 'F':
				v |= uint8(ch-'A') + 10
			default:
				log.Fatalf("bad hex digit %q in %q", ch, s)
			}
		}
		return v
	}
	c.R, c.G, c.B = parse(1), parse(3), parse(5)
	return c
}

func main() {
	out := filepath.Join("web", "static", "icons")
	if err := os.MkdirAll(out, 0o755); err != nil {
		log.Fatal(err)
	}
	top := mustParseHex(brandTop)
	bottom := mustParseHex(brandBottom)
	fg := mustParseHex(white)

	for _, size := range []int{192, 512} {
		write(filepath.Join(out, "icon-"+itoa(size)+".png"), size, top, bottom, fg, false)
	}
	// The maskable variant uses a full-bleed background while the rack stays
	// inside the central safe area used by Android circles and squircles.
	write(filepath.Join(out, "icon-maskable-512.png"), 512, top, bottom, fg, true)
}

func write(path string, size int, top, bottom, fg color.RGBA, maskable bool) {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	background := image.Rect(size*8/100, size*8/100, size*92/100, size*92/100)
	radius := size * 15 / 100
	if maskable {
		background = img.Bounds()
		radius = 0
	}
	drawGradientRoundedRect(img, background, radius, top, bottom)

	// Same two-bay server rack represented by the existing hdd-rack-fill
	// logo and the inline favicon in base.html.
	glyphWidth := size * 50 / 100
	glyphHeight := size * 40 / 100
	left := (size - glyphWidth) / 2
	topY := (size - glyphHeight) / 2
	gap := size * 6 / 100
	bayHeight := (glyphHeight - gap) / 2
	bayRadius := size * 4 / 100
	for _, y := range []int{topY, topY + bayHeight + gap} {
		drawRoundedRect(img, image.Rect(left, y, left+glyphWidth, y+bayHeight), bayRadius, fg)
		dotRadius := max(2, size*2/100)
		dotY := y + bayHeight/2
		drawDisk(img, left+glyphWidth-size*11/100, dotY, dotRadius, bottom)
		drawDisk(img, left+glyphWidth-size*6/100, dotY, dotRadius, bottom)
	}

	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %s (%dx%d)", path, size, size)
}

func drawGradientRoundedRect(img *image.RGBA, rect image.Rectangle, radius int, top, bottom color.RGBA) {
	height := max(1, rect.Dy()-1)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if !insideRoundedRect(x, y, rect, radius) {
				continue
			}
			// The web logo uses a 135-degree gradient, so both coordinates
			// contribute to the colour transition.
			t := float64((x-rect.Min.X)+(y-rect.Min.Y)) / float64(max(1, rect.Dx()-1)+height)
			img.SetRGBA(x, y, mix(top, bottom, t))
		}
	}
}

func drawRoundedRect(img *image.RGBA, rect image.Rectangle, radius int, c color.RGBA) {
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if insideRoundedRect(x, y, rect, radius) {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func insideRoundedRect(x, y int, rect image.Rectangle, radius int) bool {
	if radius <= 0 || (x >= rect.Min.X+radius && x < rect.Max.X-radius) ||
		(y >= rect.Min.Y+radius && y < rect.Max.Y-radius) {
		return true
	}
	cx := rect.Min.X + radius
	if x >= rect.Max.X-radius {
		cx = rect.Max.X - radius - 1
	}
	cy := rect.Min.Y + radius
	if y >= rect.Max.Y-radius {
		cy = rect.Max.Y - radius - 1
	}
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= radius*radius
}

func mix(a, b color.RGBA, t float64) color.RGBA {
	return color.RGBA{
		R: uint8(float64(a.R)*(1-t) + float64(b.R)*t),
		G: uint8(float64(a.G)*(1-t) + float64(b.G)*t),
		B: uint8(float64(a.B)*(1-t) + float64(b.B)*t),
		A: 0xff,
	}
}

func drawDisk(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	r2 := r * r
	for y := -r; y <= r; y++ {
		for x := -r; x <= r; x++ {
			if x*x+y*y <= r2 {
				img.SetRGBA(cx+x, cy+y, c)
			}
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
