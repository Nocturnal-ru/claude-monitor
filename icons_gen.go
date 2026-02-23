package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"runtime"
)

// digitFont maps digits '0'..'9' and '%' to a 5x7 pixel bitmap.
// Each [7]uint8 is 7 rows; within each row bit 4 = leftmost pixel.
var digitFont = map[rune][7]uint8{
	'0': {0b01110, 0b10001, 0b10011, 0b10101, 0b11001, 0b10001, 0b01110},
	'1': {0b00100, 0b01100, 0b00100, 0b00100, 0b00100, 0b00100, 0b01110},
	'2': {0b01110, 0b10001, 0b00001, 0b00110, 0b01000, 0b10000, 0b11111},
	'3': {0b11110, 0b00001, 0b00001, 0b01110, 0b00001, 0b00001, 0b11110},
	'4': {0b00010, 0b00110, 0b01010, 0b10010, 0b11111, 0b00010, 0b00010},
	'5': {0b11111, 0b10000, 0b10000, 0b11110, 0b00001, 0b00001, 0b11110},
	'6': {0b00110, 0b01000, 0b10000, 0b11110, 0b10001, 0b10001, 0b01110},
	'7': {0b11111, 0b00001, 0b00010, 0b00100, 0b01000, 0b01000, 0b01000},
	'8': {0b01110, 0b10001, 0b10001, 0b01110, 0b10001, 0b10001, 0b01110},
	'9': {0b01110, 0b10001, 0b10001, 0b01111, 0b00001, 0b00010, 0b01100},
	'%': {0b11000, 0b11001, 0b00010, 0b00100, 0b01000, 0b10011, 0b00011},
}

// levelColor returns the background color for a given remaining-% value.
// green ≥ 50%, yellow 20–49%, red < 20%.
func levelColor(remaining int) color.RGBA {
	switch {
	case remaining >= 50:
		return color.RGBA{R: 0x22, G: 0xc5, B: 0x5e, A: 0xff} // green
	case remaining >= 20:
		return color.RGBA{R: 0xf5, G: 0xa6, B: 0x23, A: 0xff} // amber
	default:
		return color.RGBA{R: 0xe5, G: 0x39, B: 0x35, A: 0xff} // red
	}
}

// textWidth returns the pixel width of s rendered with the bitmap font.
// Each glyph is 5px wide + 1px gap, minus trailing gap.
func textWidth(s string) int {
	if len(s) == 0 {
		return 0
	}
	return len(s)*6 - 1
}

// startXInHalf returns the x offset to center text in a half of the icon.
func startXInHalf(halfW int, s string) int {
	x := (halfW - textWidth(s)) / 2
	if x < 0 {
		x = 0
	}
	return x
}

// drawText renders s onto img starting at (x, y) using white pixels.
func drawText(img *image.RGBA, s string, x, y int) {
	white := color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	cx := x
	for _, ch := range s {
		glyph, ok := digitFont[ch]
		if !ok {
			cx += 6
			continue
		}
		for row, bits := range glyph {
			for col := 0; col < 5; col++ {
				if bits&(1<<uint(4-col)) != 0 {
					img.SetRGBA(cx+col, y+row, white)
				}
			}
		}
		cx += 6
	}
}

// formatPct formats a remaining percentage for display.
// 0–99 → "N%", 100 → "100" (no % to save space).
func formatPct(pct int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	if pct == 100 {
		return "100"
	}
	return fmt.Sprintf("%d%%", pct)
}

// wrapInICO wraps raw PNG bytes in a single-image ICO container.
// Windows Vista+ supports PNG-compressed ICO images.
func wrapInICO(pngData []byte, width, height int) []byte {
	const headerSize = 6 + 16 // ICONDIR + one ICONDIRENTRY

	buf := make([]byte, headerSize+len(pngData))

	// ICONDIR (6 bytes)
	buf[0] = 0 // reserved
	buf[1] = 0
	buf[2] = 1 // type = ICO
	buf[3] = 0
	buf[4] = 1 // count = 1 image
	buf[5] = 0

	// ICONDIRENTRY (16 bytes starting at offset 6)
	w := byte(width)
	h := byte(height)
	if width == 256 {
		w = 0
	}
	if height == 256 {
		h = 0
	}
	buf[6] = w  // width
	buf[7] = h  // height
	buf[8] = 0  // color count (0 = no palette)
	buf[9] = 0  // reserved
	buf[10] = 1 // planes (LE)
	buf[11] = 0
	buf[12] = 32 // bit count (LE)
	buf[13] = 0
	binary.LittleEndian.PutUint32(buf[14:18], uint32(len(pngData)))
	binary.LittleEndian.PutUint32(buf[18:22], uint32(headerSize))

	copy(buf[headerSize:], pngData)
	return buf
}

// makeIcon generates a 32×32 ICO showing session and weekly remaining percentages.
// Left half = sessionRemaining, right half = weeklyRemaining.
// Colors: green ≥50%, amber 20–49%, red <20%.
func makeIcon(sessionRemaining, weeklyRemaining int) []byte {
	const size = 32
	const half = size / 2

	img := image.NewRGBA(image.Rect(0, 0, size, size))

	sessionColor := levelColor(sessionRemaining)
	weeklyColor := levelColor(weeklyRemaining)

	for y := 0; y < size; y++ {
		for x := 0; x < half; x++ {
			img.SetRGBA(x, y, sessionColor)
		}
		for x := half; x < size; x++ {
			img.SetRGBA(x, y, weeklyColor)
		}
	}

	// Draw a 1px vertical divider in semi-transparent black
	divider := color.RGBA{R: 0, G: 0, B: 0, A: 0x60}
	for y := 0; y < size; y++ {
		img.SetRGBA(half-1, y, divider)
		img.SetRGBA(half, y, divider)
	}

	// Render text centered vertically: (32 - 7) / 2 = 12
	const textY = 12
	sessionStr := formatPct(sessionRemaining)
	weeklyStr := formatPct(weeklyRemaining)

	drawText(img, sessionStr, startXInHalf(half-1, sessionStr), textY)
	drawText(img, weeklyStr, half+1+startXInHalf(half-1, weeklyStr), textY)

	var pngBuf bytes.Buffer
	png.Encode(&pngBuf, img)
	if runtime.GOOS == "windows" {
		return wrapInICO(pngBuf.Bytes(), size, size)
	}
	return pngBuf.Bytes()
}

// makeGrayIcon returns a 32×32 solid gray ICO used for loading/error states.
func makeGrayIcon() []byte {
	const size = 32
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	gray := color.RGBA{R: 0x80, G: 0x80, B: 0x80, A: 0xff}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.SetRGBA(x, y, gray)
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	if runtime.GOOS == "windows" {
		return wrapInICO(buf.Bytes(), size, size)
	}
	return buf.Bytes()
}
