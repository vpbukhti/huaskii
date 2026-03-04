package renderer

import (
	"image/color"
	"math"
	"sort"

	"github.com/vpbukhti/huaskii/geom"
)

// Edge represents an edge for scanline rasterization
type Edge struct {
	X0, Y0, X1, Y1 float64
	Dir            int
	// Precomputed for scanline intersection
	dxdy float64 // (x1-x0)/(y1-y0)
}

// Rasterizer uses scanline fill with winding rule
type Rasterizer struct {
	Canvas *Canvas
	Edges  []Edge
}

// NewRasterizer creates a new rasterizer for the given canvas
func NewRasterizer(canvas *Canvas) *Rasterizer {
	return &Rasterizer{Canvas: canvas}
}

// AddLine adds a line segment to the rasterizer
func (r *Rasterizer) AddLine(p0, p1 geom.Point) {
	if math.Abs(p0.Y-p1.Y) < 0.0001 {
		return
	}
	if p0.Close(p1) {
		return
	}
	dir := 1
	if p0.Y > p1.Y {
		dir = -1
		p0, p1 = p1, p0
	}
	dxdy := (p1.X - p0.X) / (p1.Y - p0.Y)
	r.Edges = append(r.Edges, Edge{p0.X, p0.Y, p1.X, p1.Y, dir, dxdy})
}

// AddQuadBezier adds a quadratic bezier curve as line segments
func (r *Rasterizer) AddQuadBezier(p0, p1, p2 geom.Point, steps int) {
	prev := p0
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		curr := geom.EvalQuadBezier(p0, p1, p2, t)
		r.AddLine(prev, curr)
		prev = curr
	}
}

// AddCubicBezier adds a cubic bezier curve as line segments
func (r *Rasterizer) AddCubicBezier(p0, p1, p2, p3 geom.Point, steps int) {
	prev := p0
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		curr := geom.EvalCubicBezier(p0, p1, p2, p3, t)
		r.AddLine(prev, curr)
		prev = curr
	}
}

// activeEdge tracks an edge currently crossing the scanline
type activeEdge struct {
	edge *Edge
	x    float64 // current x intersection
}

// Fill rasterizes the accumulated edges using non-zero winding rule
// Uses sorted active edge list for efficiency
func (r *Rasterizer) Fill(col color.RGBA) {
	if len(r.Edges) == 0 {
		return
	}

	// Sort edges by Y0
	sort.Slice(r.Edges, func(i, j int) bool {
		return r.Edges[i].Y0 < r.Edges[j].Y0
	})

	// Find bounding box
	minX, maxX := r.Edges[0].X0, r.Edges[0].X0
	minY, maxY := r.Edges[0].Y0, r.Edges[0].Y1
	for i := range r.Edges {
		e := &r.Edges[i]
		minX = math.Min(minX, math.Min(e.X0, e.X1))
		maxX = math.Max(maxX, math.Max(e.X0, e.X1))
		minY = math.Min(minY, e.Y0)
		maxY = math.Max(maxY, e.Y1)
	}

	startX := int(math.Max(0, minX-1))
	endX := int(math.Min(float64(r.Canvas.Width), maxX+2))
	startY := int(math.Max(0, minY))
	endY := int(math.Min(float64(r.Canvas.Height), maxY+1))

	// Active edge list
	var active []activeEdge
	edgeIdx := 0

	for y := startY; y <= endY; y++ {
		scanY := float64(y) + 0.5

		// Add new edges that start at this scanline
		for edgeIdx < len(r.Edges) && r.Edges[edgeIdx].Y0 <= scanY {
			e := &r.Edges[edgeIdx]
			if e.Y1 > scanY {
				x := e.X0 + (scanY-e.Y0)*e.dxdy
				active = append(active, activeEdge{edge: e, x: x})
			}
			edgeIdx++
		}

		// Remove edges that end before this scanline
		n := 0
		for i := range active {
			if active[i].edge.Y1 > scanY {
				active[n] = active[i]
				n++
			}
		}
		active = active[:n]

		if len(active) == 0 {
			continue
		}

		// Sort active edges by x
		sort.Slice(active, func(i, j int) bool {
			return active[i].x < active[j].x
		})

		// Fill pixels using winding rule
		winding := 0
		activeIdx := 0

		for x := startX; x <= endX; x++ {
			px := float64(x) + 0.5

			// Process all edges to the left of this pixel
			for activeIdx < len(active) && active[activeIdx].x < px {
				winding += active[activeIdx].edge.Dir
				activeIdx++
			}

			if winding != 0 {
				r.Canvas.BlendPixel(x, y, col, 1.0)
			}
		}

		// Update x positions for next scanline
		for i := range active {
			active[i].x += active[i].edge.dxdy
		}
	}
}

// Clear removes all edges
func (r *Rasterizer) Clear() {
	r.Edges = r.Edges[:0]
}
