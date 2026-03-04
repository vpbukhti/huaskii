package renderer

import (
	"image/color"
	"math"

	"github.com/vpbukhti/huaskii/geom"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// TextRenderer renders text using filler text along paths
type TextRenderer struct {
	Font       *sfnt.Font
	buf        sfnt.Buffer
	Canvas     *Canvas
	Rasterizer *Rasterizer
}

// NewTextRenderer creates a new text renderer
func NewTextRenderer(font *sfnt.Font, canvas *Canvas) *TextRenderer {
	return &TextRenderer{
		Font:       font,
		Canvas:     canvas,
		Rasterizer: NewRasterizer(canvas),
	}
}

// RenderSettings configures the text-on-path rendering
type RenderSettings struct {
	StrokeWidth float64 // Width of the stroke around curves
	FillScale   float64 // Filler text height = StrokeWidth * FillScale (0.05 to 1.0)
	FillerText  string  // Text to repeat along the paths
	MainText    string  // Text to render
	FontSize    float64 // Size of the main text
	NumRows     int     // Number of rows to fill (0 = auto-calculate from FillScale)
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
func (tr *TextRenderer) renderFillerGlyph(r rune, pos geom.Point, tangent geom.Point, scale float64) float64 {
	segments, advance, err := tr.getGlyphSegments(r, scale)
	if err != nil {
		return 0
	}

	// Calculate bounding box to find visual bounds
	minX := math.MaxFloat64
	maxX := -math.MaxFloat64
	for _, seg := range segments {
		for _, p := range seg.Points {
			if p.X < minX {
				minX = p.X
			}
			if p.X > maxX {
				maxX = p.X
			}
		}
	}
	if minX == math.MaxFloat64 {
		minX = 0
		maxX = advance
	}

	// Rotate to follow tangent, +π to flip right-side up since we traverse paths backwards
	angle := math.Atan2(tangent.Y, tangent.X) + math.Pi

	for _, seg := range segments {
		transformedPoints := make([]geom.Point, len(seg.Points))
		for i, p := range seg.Points {
			// Shift to remove left side bearing
			p.X -= minX
			rotated := p.Rotate(angle)
			transformedPoints[i] = rotated.Add(pos)
		}

		switch seg.Type {
		case 0:
			tr.Rasterizer.AddLine(transformedPoints[0], transformedPoints[1])
		case 1:
			tr.Rasterizer.AddQuadBezier(transformedPoints[0], transformedPoints[1], transformedPoints[2], 16)
		case 2:
			tr.Rasterizer.AddCubicBezier(transformedPoints[0], transformedPoints[1], transformedPoints[2], transformedPoints[3], 16)
		}
	}

	// Return actual visual width of the glyph
	return maxX - minX
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
					charAdvance := tr.renderFillerGlyph(fillerRune, canvasPos, tangent, fillerHeight)

					if charAdvance < 1 {
						charAdvance = fillerHeight * 0.6
					}
					dist += charAdvance * 1.1
					fillerIdx++
				}
			}
		}

		tr.Rasterizer.Fill(col)
		tr.Rasterizer.Clear()

		cursorX += advance
	}
}
