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

const (
	iconSize = 64
	fontScale = 2 // each font pixel becomes 2x2
	glyphW   = 5 * fontScale
	glyphH   = 7 * fontScale
	glyphGap = 1 * fontScale
)

// levelColor returns the background color for a given remaining-% value.
// green >= 50%, amber 20-49%, red < 20%.
func levelColor(remaining int) color.RGBA {
	switch {
	case remaining >= 50:
		return color.RGBA{R: 0x2e, G: 0xcc, B: 0x71, A: 0xff} // green
	case remaining >= 20:
		return color.RGBA{R: 0xf3, G: 0x9c, B: 0x12, A: 0xff} // amber
	default:
		return color.RGBA{R: 0xe7, G: 0x4c, B: 0x3c, A: 0xff} // red
	}
}

// textWidth returns the pixel width of s rendered with the scaled bitmap font.
func textWidth(s string) int {
	if len(s) == 0 {
		return 0
	}
	return len(s)*(glyphW+glyphGap) - glyphGap
}

// startXInHalf returns the x offset to center text in a half of the icon.
func startXInHalf(halfW int, s string) int {
	x := (halfW - textWidth(s)) / 2
	if x < 0 {
		x = 0
	}
	return x
}

// drawTextOutlined renders s onto img at (x, y) with a dark outline for contrast.
// Draws dark outline at 4 cardinal offsets, then white text on top.
func drawTextOutlined(img *image.RGBA, s string, x, y int) {
	outline := color.RGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xc0}
	white := color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}

	// Outline offsets (N, S, E, W)
	offsets := [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}
	for _, off := range offsets {
		drawTextRaw(img, s, x+off[0], y+off[1], outline)
	}
	// White foreground
	drawTextRaw(img, s, x, y, white)
}

// drawTextRaw renders s onto img at (x, y) using the given color and 2x scale.
func drawTextRaw(img *image.RGBA, s string, x, y int, c color.RGBA) {
	cx := x
	for _, ch := range s {
		glyph, ok := digitFont[ch]
		if !ok {
			cx += glyphW + glyphGap
			continue
		}
		for row, bits := range glyph {
			for col := 0; col < 5; col++ {
				if bits&(1<<uint(4-col)) != 0 {
					// Draw 2x2 block for each font pixel
					for dy := 0; dy < fontScale; dy++ {
						for dx := 0; dx < fontScale; dx++ {
							px := cx + col*fontScale + dx
							py := y + row*fontScale + dy
							if px >= 0 && px < iconSize && py >= 0 && py < iconSize {
								img.SetRGBA(px, py, c)
							}
						}
					}
				}
			}
		}
		cx += glyphW + glyphGap
	}
}

// formatPct formats a remaining percentage for display.
// 0-99 -> "N%", 100 -> "100" (no % to save space).
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

// makeIcon generates a 64x64 icon showing session and weekly remaining percentages.
// Left half = sessionRemaining, right half = weeklyRemaining.
// Colors: green >= 50%, amber 20-49%, red < 20%.
// Text is rendered with a dark outline for readability.
func makeIcon(sessionRemaining, weeklyRemaining int) []byte {
	const half = iconSize / 2

	img := image.NewRGBA(image.Rect(0, 0, iconSize, iconSize))

	sessionColor := levelColor(sessionRemaining)
	weeklyColor := levelColor(weeklyRemaining)

	// Fill background halves
	for y := 0; y < iconSize; y++ {
		for x := 0; x < half; x++ {
			img.SetRGBA(x, y, sessionColor)
		}
		for x := half; x < iconSize; x++ {
			img.SetRGBA(x, y, weeklyColor)
		}
	}

	// Draw 1px dark border around the icon
	border := color.RGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x80}
	for i := 0; i < iconSize; i++ {
		img.SetRGBA(i, 0, border)              // top
		img.SetRGBA(i, iconSize-1, border)      // bottom
		img.SetRGBA(0, i, border)              // left
		img.SetRGBA(iconSize-1, i, border)      // right
	}

	// Draw 1px vertical divider in semi-transparent black
	divider := color.RGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x60}
	for y := 0; y < iconSize; y++ {
		img.SetRGBA(half-1, y, divider)
		img.SetRGBA(half, y, divider)
	}

	// Render text centered vertically: (64 - 14) / 2 = 25
	textY := (iconSize - glyphH) / 2
	sessionStr := formatPct(sessionRemaining)
	weeklyStr := formatPct(weeklyRemaining)

	drawTextOutlined(img, sessionStr, 1+startXInHalf(half-2, sessionStr), textY)
	drawTextOutlined(img, weeklyStr, half+1+startXInHalf(half-2, weeklyStr), textY)

	var pngBuf bytes.Buffer
	png.Encode(&pngBuf, img)
	if runtime.GOOS == "windows" {
		return wrapInICO(pngBuf.Bytes(), iconSize, iconSize)
	}
	return pngBuf.Bytes()
}

// makeGrayIcon returns a 64x64 solid gray icon used for loading/error states.
func makeGrayIcon() []byte {
	img := image.NewRGBA(image.Rect(0, 0, iconSize, iconSize))
	gray := color.RGBA{R: 0x80, G: 0x80, B: 0x80, A: 0xff}
	for y := 0; y < iconSize; y++ {
		for x := 0; x < iconSize; x++ {
			img.SetRGBA(x, y, gray)
		}
	}
	// Dark border
	border := color.RGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x80}
	for i := 0; i < iconSize; i++ {
		img.SetRGBA(i, 0, border)
		img.SetRGBA(i, iconSize-1, border)
		img.SetRGBA(0, i, border)
		img.SetRGBA(iconSize-1, i, border)
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	if runtime.GOOS == "windows" {
		return wrapInICO(buf.Bytes(), iconSize, iconSize)
	}
	return buf.Bytes()
}
