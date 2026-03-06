package renderer

import (
	"image"
	"image/color"
	"math"
	"math/rand"
	"unicode"

	"github.com/vpbukhti/huaskii/geom"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// glyphKey identifies a cached glyph by rune and size
type glyphKey struct {
	r    rune
	size int // size in pixels (rounded)
}

// glyphCacheEntry holds the pre-rendered glyph and its metrics
type glyphCacheEntry struct {
	img     *image.Alpha
	advance float64 // advance width in pixels
	originX int     // x offset from left edge to glyph origin
	originY int     // y offset from top edge to baseline
	centerX int     // x offset from left edge to horizontal center of mass
	width   int     // tight width of the glyph (after cropping)
}

// GlyphCache holds pre-rendered glyph bitmaps
type GlyphCache struct {
	cache map[glyphKey]*glyphCacheEntry
	font  *sfnt.Font
}

// NewGlyphCache creates a new glyph cache
func NewGlyphCache(f *sfnt.Font) *GlyphCache {
	return &GlyphCache{
		cache: make(map[glyphKey]*glyphCacheEntry),
		font:  f,
	}
}

// Get returns a pre-rendered glyph, rasterizing and caching if needed
func (gc *GlyphCache) Get(r rune, size float64) *glyphCacheEntry {
	key := glyphKey{r, int(size + 0.5)}
	if entry, ok := gc.cache[key]; ok {
		return entry
	}

	entry := gc.rasterize(r, size)
	gc.cache[key] = entry
	return entry
}

// PreloadAndGetMaxWidth pre-renders all runes and returns the maximum width
// This ensures uniform spacing when placing characters along curves
func (gc *GlyphCache) PreloadAndGetMaxWidth(runes []rune, size float64) int {
	maxWidth := 0
	for _, r := range runes {
		entry := gc.Get(r, size)
		if entry.width > maxWidth {
			maxWidth = entry.width
		}
	}
	return maxWidth
}

// rasterize renders a glyph using the standard font rasterizer
func (gc *GlyphCache) rasterize(r rune, size float64) *glyphCacheEntry {
	face, err := opentype.NewFace(gc.font, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return &glyphCacheEntry{}
	}
	defer face.Close()

	// Get glyph metrics
	bounds, advance, ok := face.GlyphBounds(r)
	if !ok {
		return &glyphCacheEntry{}
	}

	// Convert fixed-point bounds to pixels
	minX := bounds.Min.X.Floor()
	minY := bounds.Min.Y.Floor()
	maxX := bounds.Max.X.Ceil()
	maxY := bounds.Max.Y.Ceil()

	width := maxX - minX
	height := maxY - minY
	if width <= 0 || height <= 0 {
		return &glyphCacheEntry{advance: fixedToFloat(advance)}
	}

	// Create alpha image for the glyph
	img := image.NewAlpha(image.Rect(0, 0, width, height))

	// Create drawer
	drawer := &font.Drawer{
		Dst:  img,
		Src:  image.White,
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.I(-minX), Y: fixed.I(-minY)},
	}

	// Draw the glyph
	drawer.DrawString(string(r))

	// Find tight bounds and center of mass
	tightMinX, tightMaxX := width, 0
	weightedSumX := 0.0
	totalWeight := 0.0

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			alpha := float64(img.AlphaAt(x, y).A)
			if alpha > 0 {
				if x < tightMinX {
					tightMinX = x
				}
				if x > tightMaxX {
					tightMaxX = x
				}
				weightedSumX += float64(x) * alpha
				totalWeight += alpha
			}
		}
	}

	// If no pixels found, return empty
	if totalWeight == 0 || tightMaxX < tightMinX {
		return &glyphCacheEntry{advance: fixedToFloat(advance)}
	}

	// Calculate center of mass
	centerOfMassX := weightedSumX / totalWeight

	// Crop to tight horizontal bounds
	tightWidth := tightMaxX - tightMinX + 1
	croppedImg := image.NewAlpha(image.Rect(0, 0, tightWidth, height))
	for y := 0; y < height; y++ {
		for x := 0; x < tightWidth; x++ {
			croppedImg.SetAlpha(x, y, img.AlphaAt(x+tightMinX, y))
		}
	}

	// Adjust center of mass and origin for cropped image
	centerX := int(math.Round(centerOfMassX)) - tightMinX
	originX := -minX - tightMinX

	return &glyphCacheEntry{
		img:     croppedImg,
		advance: fixedToFloat(advance),
		originX: originX,
		originY: -minY,
		centerX: centerX,
		width:   tightWidth,
	}
}

// TextRenderer renders text using filler text along paths
type TextRenderer struct {
	Font       *sfnt.Font
	buf        sfnt.Buffer
	Canvas     *Canvas
	Rasterizer *Rasterizer
	GlyphCache *GlyphCache
}

// NewTextRenderer creates a new text renderer
func NewTextRenderer(font *sfnt.Font, canvas *Canvas) *TextRenderer {
	return &TextRenderer{
		Font:       font,
		Canvas:     canvas,
		Rasterizer: NewRasterizer(canvas),
		GlyphCache: NewGlyphCache(font),
	}
}

// RenderSettings configures the text-on-path rendering
type RenderSettings struct {
	StrokeWidth     float64    // Width of the stroke around curves
	FillScale       float64    // Filler text height = StrokeWidth * FillScale (0.05 to 1.0)
	FillerText      string     // Text to repeat along the paths
	MainText        string     // Text to render
	FontSize        float64    // Size of the main text
	NumRows         int        // Number of rows to fill (0 = auto-calculate from FillScale)
	DrawBackground  bool       // Draw white background behind each filler letter
	FillerSpacing   float64    // Horizontal spacing between filler letters in pixels (can be negative)
	RowSpacing      float64    // Vertical spacing between rows in pixels (can be negative for overlap)
	BackgroundColor color.RGBA // Solid background fill for main text letters (zero alpha = disabled)
}

// fixedToFloat converts fixed.Int26_6 to float64
func fixedToFloat(f fixed.Int26_6) float64 {
	return float64(f) / 64.0
}

// MeasureText returns the total advance width of a string at a given font size
func MeasureText(font *sfnt.Font, text string, size float64) float64 {
	var buf sfnt.Buffer
	ppem := fixed.Int26_6(size * 64)
	totalWidth := 0.0

	for _, r := range text {
		glyphIndex, err := font.GlyphIndex(&buf, r)
		if err != nil {
			continue
		}
		adv, err := font.GlyphAdvance(&buf, glyphIndex, ppem, 0)
		if err != nil {
			continue
		}
		totalWidth += fixedToFloat(adv)
	}

	return totalWidth
}

// getGlyphSegments returns the path segments for a glyph
func (tr *TextRenderer) getGlyphSegments(r rune, size float64) ([]geom.PathSegment, float64, error) {
	ppem := fixed.Int26_6(size * 64)

	glyphIndex, err := tr.Font.GlyphIndex(&tr.buf, r)
	if err != nil {
		return nil, 0, err
	}

	segments, err := tr.Font.LoadGlyph(&tr.buf, glyphIndex, ppem, nil)
	if err != nil {
		return nil, 0, err
	}

	adv, err := tr.Font.GlyphAdvance(&tr.buf, glyphIndex, ppem, 0)
	if err != nil {
		return nil, 0, err
	}

	var result []geom.PathSegment
	var currentPos geom.Point
	var subpathStart geom.Point
	hasSubpath := false

	for _, seg := range segments {
		switch seg.Op {
		case sfnt.SegmentOpMoveTo:
			// Close previous subpath if exists
			if hasSubpath && !currentPos.Close(subpathStart) {
				result = append(result, geom.PathSegment{
					Type:   0,
					Points: []geom.Point{currentPos, subpathStart},
				})
			}
			currentPos = geom.Point{
				X: fixedToFloat(seg.Args[0].X),
				Y: fixedToFloat(seg.Args[0].Y),
			}
			subpathStart = currentPos
			hasSubpath = true
		case sfnt.SegmentOpLineTo:
			pt := geom.Point{
				X: fixedToFloat(seg.Args[0].X),
				Y: fixedToFloat(seg.Args[0].Y),
			}
			result = append(result, geom.PathSegment{
				Type:   0,
				Points: []geom.Point{currentPos, pt},
			})
			currentPos = pt
		case sfnt.SegmentOpQuadTo:
			p1 := geom.Point{X: fixedToFloat(seg.Args[0].X), Y: fixedToFloat(seg.Args[0].Y)}
			p2 := geom.Point{X: fixedToFloat(seg.Args[1].X), Y: fixedToFloat(seg.Args[1].Y)}
			result = append(result, geom.PathSegment{
				Type:   1,
				Points: []geom.Point{currentPos, p1, p2},
			})
			currentPos = p2
		case sfnt.SegmentOpCubeTo:
			p1 := geom.Point{X: fixedToFloat(seg.Args[0].X), Y: fixedToFloat(seg.Args[0].Y)}
			p2 := geom.Point{X: fixedToFloat(seg.Args[1].X), Y: fixedToFloat(seg.Args[1].Y)}
			p3 := geom.Point{X: fixedToFloat(seg.Args[2].X), Y: fixedToFloat(seg.Args[2].Y)}
			result = append(result, geom.PathSegment{
				Type:   2,
				Points: []geom.Point{currentPos, p1, p2, p3},
			})
			currentPos = p3
		}
	}

	// Close final subpath
	if hasSubpath && !currentPos.Close(subpathStart) {
		result = append(result, geom.PathSegment{
			Type:   0,
			Points: []geom.Point{currentPos, subpathStart},
		})
	}

	return result, fixedToFloat(adv), nil
}

// renderFillerGlyphToCanvas renders a single filler glyph to a target canvas
// The glyph is centered at pos using its horizontal center of mass
func (tr *TextRenderer) renderFillerGlyphToCanvas(r rune, pos geom.Point, tangent geom.Point, scale float64, fgColor color.RGBA, target *Canvas) float64 {
	entry := tr.GlyphCache.Get(r, scale)
	if entry.img == nil {
		return float64(entry.width)
	}

	// Rotate to follow tangent, +π to flip right-side up since we traverse paths backwards
	angle := math.Atan2(tangent.Y, tangent.X) + math.Pi

	// Composite the rotated glyph onto the target canvas
	target.DrawRotatedImage(entry.img, pos.X, pos.Y, angle, entry.centerX, entry.originY, fgColor)

	return float64(entry.width)
}

// renderFillerBackgroundToCanvas renders a background rectangle for a glyph to a target canvas
// padding is the extra space around the glyph bounding box
func (tr *TextRenderer) renderFillerBackgroundToCanvas(r rune, pos geom.Point, tangent geom.Point, scale float64, padding float64, bgColor color.RGBA, target *Canvas) {
	entry := tr.GlyphCache.Get(r, scale)
	if entry.img == nil {
		return
	}

	// Rotate to follow tangent, +π to flip right-side up since we traverse paths backwards
	angle := math.Atan2(tangent.Y, tangent.X) + math.Pi

	bounds := entry.img.Bounds()
	w := float64(bounds.Dx())
	h := float64(bounds.Dy())

	// Create a rasterizer for the target canvas
	rast := NewRasterizer(target)

	// Corners relative to center of mass with padding
	pad := padding
	corners := []geom.Point{
		{X: -float64(entry.centerX) - pad, Y: -float64(entry.originY) - pad},
		{X: w - float64(entry.centerX) + pad, Y: -float64(entry.originY) - pad},
		{X: w - float64(entry.centerX) + pad, Y: h - float64(entry.originY) + pad},
		{X: -float64(entry.centerX) - pad, Y: h - float64(entry.originY) + pad},
	}
	for i, c := range corners {
		corners[i] = c.Rotate(angle).Add(pos)
	}
	rast.AddLine(corners[0], corners[1])
	rast.AddLine(corners[1], corners[2])
	rast.AddLine(corners[2], corners[3])
	rast.AddLine(corners[3], corners[0])
	rast.Fill(bgColor)
}

// RenderTextMasked renders main text as a mask filled with rows of small repeating text
// Simple approach: big letters = white background mask, small text fills it, rest is transparent
func (tr *TextRenderer) RenderTextMasked(settings RenderSettings, startX, baseline float64, bgColor, fgColor color.RGBA) {
	fillerHeight := settings.StrokeWidth * settings.FillScale
	fillerRunes := []rune(settings.FillerText)
	if len(fillerRunes) == 0 {
		return
	}

	// Step 1: Render main text as solid fill (this becomes our mask)
	maskCanvas := NewCanvas(tr.Canvas.Width, tr.Canvas.Height)
	tr.renderMainTextFilled(settings, startX, baseline, maskCanvas)

	// Step 2: Render rows of repeating small text across entire canvas
	textCanvas := NewCanvas(tr.Canvas.Width, tr.Canvas.Height)
	tr.renderFillerRows(fillerRunes, fillerHeight, settings.FillerSpacing, settings.RowSpacing, fgColor, textCanvas)

	// Step 3: Apply mask and composite onto main canvas
	// Where mask is set: show bgColor background + fgColor text
	// Where mask is not set: transparent
	for y := 0; y < tr.Canvas.Height; y++ {
		for x := 0; x < tr.Canvas.Width; x++ {
			maskAlpha := maskCanvas.Img.RGBAAt(x, y).A
			if maskAlpha == 0 {
				continue // transparent area
			}

			// Get the small text pixel
			textPx := textCanvas.Img.RGBAAt(x, y)

			// Composite: background color first, then text on top
			a := float64(maskAlpha) / 255.0
			if textPx.A > 0 {
				// Text pixel - blend fgColor
				textA := float64(textPx.A) / 255.0
				finalR := uint8(float64(bgColor.R)*(1-textA) + float64(fgColor.R)*textA)
				finalG := uint8(float64(bgColor.G)*(1-textA) + float64(fgColor.G)*textA)
				finalB := uint8(float64(bgColor.B)*(1-textA) + float64(fgColor.B)*textA)
				tr.Canvas.Img.Set(x, y, color.RGBA{finalR, finalG, finalB, uint8(255 * a)})
			} else {
				// No text - just background
				tr.Canvas.Img.Set(x, y, color.RGBA{bgColor.R, bgColor.G, bgColor.B, uint8(255 * a)})
			}
		}
	}
}

// renderMainTextFilled renders the main text as solid white fill
func (tr *TextRenderer) renderMainTextFilled(settings RenderSettings, startX, baseline float64, target *Canvas) {
	face, err := opentype.NewFace(tr.Font, &opentype.FaceOptions{
		Size:    settings.FontSize,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return
	}
	defer face.Close()

	drawer := &font.Drawer{
		Dst:  target.Img,
		Src:  image.White,
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.I(int(startX)), Y: fixed.I(int(baseline))},
	}
	drawer.DrawString(settings.MainText)
}

// renderFillerRows renders rows of repeating small text across the entire canvas
func (tr *TextRenderer) renderFillerRows(fillerRunes []rune, fillerHeight, spacing, rowSpacing float64, col color.RGBA, target *Canvas) {
	face, err := opentype.NewFace(tr.Font, &opentype.FaceOptions{
		Size:    fillerHeight,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return
	}
	defer face.Close()

	// Calculate line height
	lineHeight := fillerHeight + rowSpacing

	// Render rows from top to bottom
	y := fillerHeight // start at first line baseline
	runeIdx := 0

	for y < float64(target.Height)+fillerHeight {
		x := 0.0
		for x < float64(target.Width) {
			r := fillerRunes[runeIdx%len(fillerRunes)]
			runeIdx++

			// Skip spaces for visual clarity but still advance
			if r == ' ' {
				// Get advance width for space
				if idx, err := tr.Font.GlyphIndex(&tr.buf, r); err == nil {
					if adv, err := tr.Font.GlyphAdvance(&tr.buf, idx, fixed.Int26_6(fillerHeight*64), 0); err == nil {
						x += fixedToFloat(adv) + spacing
					}
				}
				continue
			}

			drawer := &font.Drawer{
				Dst:  target.Img,
				Src:  image.NewUniform(col),
				Face: face,
				Dot:  fixed.Point26_6{X: fixed.I(int(x)), Y: fixed.I(int(y))},
			}
			drawer.DrawString(string(r))

			// Get advance width
			if idx, err := tr.Font.GlyphIndex(&tr.buf, r); err == nil {
				if adv, err := tr.Font.GlyphAdvance(&tr.buf, idx, fixed.Int26_6(fillerHeight*64), 0); err == nil {
					x += fixedToFloat(adv) + spacing
				}
			}
		}
		y += lineHeight
	}
}

// RenderTextWithFiller renders main text using filler text along the curves
func (tr *TextRenderer) RenderTextWithFiller(settings RenderSettings, startX, baseline float64, col color.RGBA) {
	fillerHeight := settings.StrokeWidth * settings.FillScale
	numRows := settings.NumRows
	if numRows <= 0 {
		// Auto-calculate from FillScale
		numRows = int(math.Ceil(settings.StrokeWidth / fillerHeight))
		if numRows < 1 {
			numRows = 1
		}
	}

	fillerRunes := []rune(settings.FillerText)
	if len(fillerRunes) == 0 {
		return
	}

	// Pre-render all filler glyphs and get uniform width for consistent spacing
	uniformWidth := tr.GlyphCache.PreloadAndGetMaxWidth(fillerRunes, fillerHeight)

	bgColor := color.RGBA{255, 255, 255, 255} // white background
	bgPadding := 5.0                          // 5px padding around each letter's bbox

	// PASS 1: Render solid background fill for main text letters
	if settings.BackgroundColor.A > 0 {
		// Render main text to mask
		maskCanvas := NewCanvas(tr.Canvas.Width, tr.Canvas.Height)
		tr.renderMainTextFilled(settings, startX, baseline, maskCanvas)

		// Calculate expansion based on filler rows (extra padding for fatter look)
		rowStep := fillerHeight + settings.RowSpacing
		totalSpan := float64(numRows-1) * rowStep
		expansion := int(math.Ceil(totalSpan/2+fillerHeight/2+bgPadding)) + 20 // extra fat

		w, h := tr.Canvas.Width, tr.Canvas.Height

		// Precompute circular kernel offsets for rounded dilation
		var circleOffsets [][2]int
		for dy := -expansion; dy <= expansion; dy++ {
			for dx := -expansion; dx <= expansion; dx++ {
				if dx*dx+dy*dy <= expansion*expansion {
					circleOffsets = append(circleOffsets, [2]int{dx, dy})
				}
			}
		}

		// Dilate mask with circular kernel
		dilatedMask := make([][]uint8, h)
		for y := 0; y < h; y++ {
			dilatedMask[y] = make([]uint8, w)
		}

		// For each mask pixel, expand to all circle offsets
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				a := maskCanvas.Img.RGBAAt(x, y).A
				if a > 0 {
					for _, off := range circleOffsets {
						nx, ny := x+off[0], y+off[1]
						if nx >= 0 && nx < w && ny >= 0 && ny < h {
							if a > dilatedMask[ny][nx] {
								dilatedMask[ny][nx] = a
							}
						}
					}
				}
			}
		}

		// Composite dilated mask
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				maxAlpha := dilatedMask[y][x]
				if maxAlpha > 0 {
					a := float64(maxAlpha) / 255.0
					existing := tr.Canvas.Img.RGBAAt(x, y)
					newR := uint8(float64(existing.R)*(1-a) + float64(settings.BackgroundColor.R)*a)
					newG := uint8(float64(existing.G)*(1-a) + float64(settings.BackgroundColor.G)*a)
					newB := uint8(float64(existing.B)*(1-a) + float64(settings.BackgroundColor.B)*a)
					newA := uint8(math.Min(255, float64(existing.A)+float64(settings.BackgroundColor.A)*a))
					tr.Canvas.Img.Set(x, y, color.RGBA{newR, newG, newB, newA})
				}
			}
		}
	}

	// PASS 2: Render filler text along the curves
	cursorX := startX

	for _, mainRune := range settings.MainText {
		mainSegments, advance, err := tr.getGlyphSegments(mainRune, settings.FontSize)
		if err != nil {
			continue
		}

		for _, seg := range mainSegments {
			// Normalize curve direction based on average normal
			// If average normal points upward, reverse the curve
			if seg.ShouldReverse() {
				seg = seg.Reversed()
			}

			segLength := seg.EstimateLength()
			if segLength < 1.0 {
				continue
			}

			// Create temporary canvases for this segment
			// bgCanvas: white background rectangles (clears space for letters)
			// letterCanvas: the actual filler letters
			bgCanvas := NewCanvas(tr.Canvas.Width, tr.Canvas.Height)
			letterCanvas := NewCanvas(tr.Canvas.Width, tr.Canvas.Height)

			for row := 0; row < numRows; row++ {
				// Pack rows with configurable spacing, centered on curve
				rowOffset := 0.0
				if numRows > 1 {
					// Row step = fillerHeight + RowSpacing (negative = tighter)
					rowStep := fillerHeight + settings.RowSpacing
					totalSpan := float64(numRows-1) * rowStep
					rowOffset = (float64(row)/float64(numRows-1) - 0.5) * totalSpan
				}

				// Randomize starting position to avoid striping
				// Pick a random starting index, skip whitespace
				startIdx := rand.Intn(len(fillerRunes))
				for i := 0; i < len(fillerRunes); i++ {
					if !unicode.IsSpace(fillerRunes[(startIdx+i)%len(fillerRunes)]) {
						startIdx = (startIdx + i) % len(fillerRunes)
						break
					}
				}
				rowFillerIdx := startIdx

				// Random distance offset to prevent vertical alignment
				dist := rand.Float64() * fillerHeight * 0.5

				for dist < segLength {
					t := dist / segLength
					if t > 1 {
						break
					}

					pos, tangent := seg.GetPointAndTangent(t)
					perp := tangent.Perpendicular()
					pos = pos.Add(perp.Scale(rowOffset))

					canvasPos := geom.Point{
						X: pos.X + cursorX,
						Y: baseline + pos.Y,
					}

					// Use runes in reverse order to counteract backwards path traversal
					reverseIdx := len(fillerRunes) - 1 - (rowFillerIdx % len(fillerRunes))
					fillerRune := fillerRunes[reverseIdx]

					// Draw background rectangle to bgCanvas
					tr.renderFillerBackgroundToCanvas(fillerRune, canvasPos, tangent, fillerHeight, bgPadding, bgColor, bgCanvas)

					// Draw letter to letterCanvas
					tr.renderFillerGlyphToCanvas(fillerRune, canvasPos, tangent, fillerHeight, col, letterCanvas)

					// Use uniform width for consistent spacing
					dist += float64(uniformWidth) + settings.FillerSpacing
					rowFillerIdx++
				}
			}

			// Composite this segment's layers onto main canvas:
			// 1. First apply white backgrounds (clears space)
			// 2. Then apply letters on top
			tr.Canvas.CompositeOver(bgCanvas)
			tr.Canvas.CompositeOver(letterCanvas)
		}

		cursorX += advance
	}
}
