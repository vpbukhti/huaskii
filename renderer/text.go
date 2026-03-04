package renderer

import (
	"image"
	"image/color"
	"math"

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

	return &glyphCacheEntry{
		img:     img,
		advance: fixedToFloat(advance),
		originX: -minX,
		originY: -minY,
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
	StrokeWidth    float64 // Width of the stroke around curves
	FillScale      float64 // Filler text height = StrokeWidth * FillScale (0.05 to 1.0)
	FillerText     string  // Text to repeat along the paths
	MainText       string  // Text to render
	FontSize       float64 // Size of the main text
	NumRows        int     // Number of rows to fill (0 = auto-calculate from FillScale)
	DrawBackground bool    // Draw white background behind each filler letter
	LetterPadding  float64 // Padding around letters for background (0 = no padding)
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

// renderFillerGlyph renders a single filler glyph at a position with rotation
// If drawBg is true, draws a background rectangle first
func (tr *TextRenderer) renderFillerGlyph(r rune, pos geom.Point, tangent geom.Point, scale float64, drawBg bool, letterPadding float64, bgColor, fgColor color.RGBA) float64 {
	// Get pre-rendered glyph from cache
	entry := tr.GlyphCache.Get(r, scale)
	if entry.img == nil {
		return entry.advance
	}

	// Rotate to follow tangent, +π to flip right-side up since we traverse paths backwards
	angle := math.Atan2(tangent.Y, tangent.X) + math.Pi

	bounds := entry.img.Bounds()
	w := float64(bounds.Dx())
	h := float64(bounds.Dy())

	// If drawing background, calculate transformed bounding box corners and draw rect
	if drawBg {
		pad := letterPadding
		// Corners relative to glyph origin (which is at originX, originY in the image)
		corners := []geom.Point{
			{X: -pad, Y: -float64(entry.originY) - pad},
			{X: w - float64(entry.originX) + pad, Y: -float64(entry.originY) - pad},
			{X: w - float64(entry.originX) + pad, Y: h - float64(entry.originY) + pad},
			{X: -pad, Y: h - float64(entry.originY) + pad},
		}
		for i, c := range corners {
			corners[i] = c.Rotate(angle).Add(pos)
		}
		tr.Rasterizer.AddLine(corners[0], corners[1])
		tr.Rasterizer.AddLine(corners[1], corners[2])
		tr.Rasterizer.AddLine(corners[2], corners[3])
		tr.Rasterizer.AddLine(corners[3], corners[0])
		tr.Rasterizer.Fill(bgColor)
		tr.Rasterizer.Clear()
	}

	// Composite the rotated glyph onto the canvas
	// The glyph origin is at (entry.originX, entry.originY) in the image
	// We want to place this origin at pos
	tr.Canvas.DrawRotatedImage(entry.img, pos.X, pos.Y, angle, entry.originX, entry.originY, fgColor)

	// Return visual width of the glyph
	return w - float64(entry.originX)
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

	cursorX := startX
	fillerRunes := []rune(settings.FillerText)
	if len(fillerRunes) == 0 {
		return
	}

	fillerIdx := 0

	for _, mainRune := range settings.MainText {
		mainSegments, advance, err := tr.getGlyphSegments(mainRune, settings.FontSize)
		if err != nil {
			continue
		}

		for _, seg := range mainSegments {
			segLength := seg.EstimateLength()
			if segLength < 1.0 {
				continue
			}

			for row := 0; row < numRows; row++ {
				// Pack rows tightly based on filler height, centered on curve
				rowOffset := 0.0
				if numRows > 1 {
					// Rows span (numRows-1)*fillerHeight, centered
					totalSpan := float64(numRows-1) * fillerHeight
					rowOffset = (float64(row)/float64(numRows-1)-0.5)*totalSpan
				}

				dist := 0.0
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
					reverseIdx := len(fillerRunes) - 1 - (fillerIdx % len(fillerRunes))
					fillerRune := fillerRunes[reverseIdx]
					bgColor := color.RGBA{255, 255, 255, 255} // white background
					charAdvance := tr.renderFillerGlyph(fillerRune, canvasPos, tangent, fillerHeight, settings.DrawBackground, settings.LetterPadding, bgColor, col)

					if charAdvance < 1 {
						charAdvance = fillerHeight * 0.6
					}
					dist += charAdvance * 1.1
					fillerIdx++
				}
			}
		}

		cursorX += advance
	}
}
