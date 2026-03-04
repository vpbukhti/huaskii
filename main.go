package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"

	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// Point represents a 2D point
type Point struct {
	X, Y float64
}

// Canvas is a simple rasterization target
type Canvas struct {
	img    *image.RGBA
	width  int
	height int
}

func NewCanvas(width, height int) *Canvas {
	return &Canvas{
		img:    image.NewRGBA(image.Rect(0, 0, width, height)),
		width:  width,
		height: height,
	}
}

func (c *Canvas) Fill(col color.Color) {
	for y := 0; y < c.height; y++ {
		for x := 0; x < c.width; x++ {
			c.img.Set(x, y, col)
		}
	}
}

func (c *Canvas) SetPixel(x, y int, col color.Color) {
	if x >= 0 && x < c.width && y >= 0 && y < c.height {
		c.img.Set(x, y, col)
	}
}

// BlendPixel blends a pixel with alpha for antialiasing
func (c *Canvas) BlendPixel(x, y int, col color.RGBA, alpha float64) {
	if x < 0 || x >= c.width || y < 0 || y >= c.height {
		return
	}
	existing := c.img.RGBAAt(x, y)
	newR := uint8(float64(existing.R)*(1-alpha) + float64(col.R)*alpha)
	newG := uint8(float64(existing.G)*(1-alpha) + float64(col.G)*alpha)
	newB := uint8(float64(existing.B)*(1-alpha) + float64(col.B)*alpha)
	newA := uint8(math.Min(255, float64(existing.A)+float64(col.A)*alpha))
	c.img.Set(x, y, color.RGBA{newR, newG, newB, newA})
}

// fixedToFloat converts fixed.Int26_6 to float64
func fixedToFloat(f fixed.Int26_6) float64 {
	return float64(f) / 64.0
}

// pointsClose checks if two points are approximately equal
func pointsClose(a, b Point) bool {
	const epsilon = 0.01
	return math.Abs(a.X-b.X) < epsilon && math.Abs(a.Y-b.Y) < epsilon
}

// evalQuadBezier evaluates a quadratic Bezier curve at parameter t
func evalQuadBezier(p0, p1, p2 Point, t float64) Point {
	mt := 1 - t
	return Point{
		X: mt*mt*p0.X + 2*mt*t*p1.X + t*t*p2.X,
		Y: mt*mt*p0.Y + 2*mt*t*p1.Y + t*t*p2.Y,
	}
}

// evalCubicBezier evaluates a cubic Bezier curve at parameter t
func evalCubicBezier(p0, p1, p2, p3 Point, t float64) Point {
	mt := 1 - t
	mt2 := mt * mt
	mt3 := mt2 * mt
	t2 := t * t
	t3 := t2 * t
	return Point{
		X: mt3*p0.X + 3*mt2*t*p1.X + 3*mt*t2*p2.X + t3*p3.X,
		Y: mt3*p0.Y + 3*mt2*t*p1.Y + 3*mt*t2*p2.Y + t3*p3.Y,
	}
}

// Rasterizer uses scanline fill with winding rule
type Rasterizer struct {
	canvas *Canvas
	edges  []Edge
}

type Edge struct {
	x0, y0, x1, y1 float64
	dir            int // +1 going down, -1 going up (before normalization)
}

func NewRasterizer(canvas *Canvas) *Rasterizer {
	return &Rasterizer{canvas: canvas}
}

func (r *Rasterizer) AddLine(p0, p1 Point) {
	// Skip horizontal lines (don't contribute to scanline fill)
	if math.Abs(p0.Y-p1.Y) < 0.001 {
		return
	}
	// Skip degenerate edges
	if pointsClose(p0, p1) {
		return
	}
	// Track original direction before normalizing
	dir := 1 // going down (Y increasing)
	if p0.Y > p1.Y {
		dir = -1 // was going up
		p0, p1 = p1, p0
	}
	r.edges = append(r.edges, Edge{p0.X, p0.Y, p1.X, p1.Y, dir})
}

func (r *Rasterizer) AddQuadBezier(p0, p1, p2 Point, steps int) {
	prev := p0
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		curr := evalQuadBezier(p0, p1, p2, t)
		r.AddLine(prev, curr)
		prev = curr
	}
}

func (r *Rasterizer) AddCubicBezier(p0, p1, p2, p3 Point, steps int) {
	prev := p0
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		curr := evalCubicBezier(p0, p1, p2, p3, t)
		r.AddLine(prev, curr)
		prev = curr
	}
}

// Fill rasterizes the accumulated edges using non-zero winding rule
func (r *Rasterizer) Fill(col color.RGBA) {
	if len(r.edges) == 0 {
		return
	}

	// Find bounding box
	minY, maxY := r.edges[0].y0, r.edges[0].y1
	for _, e := range r.edges {
		minY = math.Min(minY, math.Min(e.y0, e.y1))
		maxY = math.Max(maxY, math.Max(e.y0, e.y1))
	}

	// Scanline fill with supersampling for antialiasing
	subsamples := 4
	subStep := 1.0 / float64(subsamples)

	for y := int(minY); y <= int(maxY)+1 && y < r.canvas.height; y++ {
		if y < 0 {
			continue
		}
		for x := 0; x < r.canvas.width; x++ {
			coverage := 0.0
			for sy := 0; sy < subsamples; sy++ {
				scanY := float64(y) + float64(sy)*subStep + subStep/2
				for sx := 0; sx < subsamples; sx++ {
					sampleX := float64(x) + float64(sx)*subStep + subStep/2
					winding := 0
					for _, e := range r.edges {
						// Use half-open interval [y0, y1) to avoid double-counting vertices
						if scanY >= e.y0 && scanY < e.y1 {
							// Calculate x intersection
							t := (scanY - e.y0) / (e.y1 - e.y0)
							xInt := e.x0 + t*(e.x1-e.x0)
							if xInt < sampleX {
								winding += e.dir
							}
						}
					}
					if winding != 0 { // non-zero winding rule
						coverage += 1.0
					}
				}
			}
			coverage /= float64(subsamples * subsamples)
			if coverage > 0 {
				r.canvas.BlendPixel(x, y, col, coverage)
			}
		}
	}
}

func (r *Rasterizer) Clear() {
	r.edges = r.edges[:0]
}

func main() {
	// Load the font file
	fontData, err := os.ReadFile("assets/Roboto-VariableFont_wdth,wght.ttf")
	if err != nil {
		log.Fatalf("failed to read font: %v", err)
	}

	// Parse the font
	font, err := sfnt.Parse(fontData)
	if err != nil {
		log.Fatalf("failed to parse font: %v", err)
	}

	// Create canvas
	width, height := 800, 200
	canvas := NewCanvas(width, height)
	canvas.Fill(color.White)

	// Text to render
	text := "Hello, Bezier!"

	// Font size and positioning
	fontSize := 72.0
	ppem := fixed.Int26_6(fontSize * 64)

	// Get font metrics for baseline positioning
	var buf sfnt.Buffer
	metrics, err := font.Metrics(&buf, ppem, 0)
	if err != nil {
		log.Fatalf("failed to get metrics: %v", err)
	}

	// The sfnt package returns Y in image coordinates (Y+ is down)
	// Position so text is vertically centered
	textHeight := fixedToFloat(metrics.Ascent) + fixedToFloat(metrics.Descent)
	baseline := (float64(height) - textHeight) / 2

	rasterizer := NewRasterizer(canvas)
	cursorX := 50.0

	// Render each character
	for _, r := range text {
		// Get glyph index for this rune
		glyphIndex, err := font.GlyphIndex(&buf, r)
		if err != nil {
			log.Printf("no glyph for %c: %v", r, err)
			continue
		}

		// Load glyph segments
		segments, err := font.LoadGlyph(&buf, glyphIndex, ppem, nil)
		if err != nil {
			log.Printf("failed to load glyph for %c: %v", r, err)
			continue
		}

		// Process segments - LoadGlyph returns segments already scaled to ppem
		// Track contours to properly close them
		var currentPos Point
		var contourStart Point
		inContour := false

		for i, seg := range segments {
			switch seg.Op {
			case sfnt.SegmentOpMoveTo:
				// Close previous contour before starting new one
				if inContour {
					rasterizer.AddLine(currentPos, contourStart)
				}
				pt := Point{
					X: fixedToFloat(seg.Args[0].X) + cursorX,
					Y: baseline + fixedToFloat(seg.Args[0].Y),
				}
				currentPos = pt
				contourStart = pt
				inContour = true

			case sfnt.SegmentOpLineTo:
				pt := Point{
					X: fixedToFloat(seg.Args[0].X) + cursorX,
					Y: baseline + fixedToFloat(seg.Args[0].Y),
				}
				rasterizer.AddLine(currentPos, pt)
				currentPos = pt

			case sfnt.SegmentOpQuadTo:
				p1 := Point{
					X: fixedToFloat(seg.Args[0].X) + cursorX,
					Y: baseline + fixedToFloat(seg.Args[0].Y),
				}
				p2 := Point{
					X: fixedToFloat(seg.Args[1].X) + cursorX,
					Y: baseline + fixedToFloat(seg.Args[1].Y),
				}
				rasterizer.AddQuadBezier(currentPos, p1, p2, 32)
				currentPos = p2

			case sfnt.SegmentOpCubeTo:
				p1 := Point{
					X: fixedToFloat(seg.Args[0].X) + cursorX,
					Y: baseline + fixedToFloat(seg.Args[0].Y),
				}
				p2 := Point{
					X: fixedToFloat(seg.Args[1].X) + cursorX,
					Y: baseline + fixedToFloat(seg.Args[1].Y),
				}
				p3 := Point{
					X: fixedToFloat(seg.Args[2].X) + cursorX,
					Y: baseline + fixedToFloat(seg.Args[2].Y),
				}
				rasterizer.AddCubicBezier(currentPos, p1, p2, p3, 32)
				currentPos = p3
			}

			// Close contour at end of glyph
			if i == len(segments)-1 && inContour {
				rasterizer.AddLine(currentPos, contourStart)
			}
		}

		// Fill the glyph
		rasterizer.Fill(color.RGBA{0, 0, 0, 255})
		rasterizer.Clear()

		// Advance cursor
		adv, err := font.GlyphAdvance(&buf, glyphIndex, ppem, 0)
		if err != nil {
			log.Printf("failed to get advance for %c: %v", r, err)
			continue
		}
		cursorX += fixedToFloat(adv)
	}

	// Save to PNG
	outFile, err := os.Create("output/output.png")
	if err != nil {
		log.Fatalf("failed to create output file: %v", err)
	}
	defer outFile.Close()

	if err := png.Encode(outFile, canvas.img); err != nil {
		log.Fatalf("failed to encode PNG: %v", err)
	}

	log.Println("Rendered to output.png")
}
