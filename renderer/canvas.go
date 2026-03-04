package renderer

import (
	"image"
	"image/color"
	"math"

	"golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
)

// Canvas is a simple rasterization target
type Canvas struct {
	Img    *image.RGBA
	Width  int
	Height int
}

// NewCanvas creates a new canvas with the given dimensions
func NewCanvas(width, height int) *Canvas {
	return &Canvas{
		Img:    image.NewRGBA(image.Rect(0, 0, width, height)),
		Width:  width,
		Height: height,
	}
}

// Fill fills the entire canvas with a color
func (c *Canvas) Fill(col color.Color) {
	for y := 0; y < c.Height; y++ {
		for x := 0; x < c.Width; x++ {
			c.Img.Set(x, y, col)
		}
	}
}

// BlendPixel blends a pixel with alpha for antialiasing
func (c *Canvas) BlendPixel(x, y int, col color.RGBA, alpha float64) {
	if x < 0 || x >= c.Width || y < 0 || y >= c.Height {
		return
	}
	existing := c.Img.RGBAAt(x, y)
	newR := uint8(float64(existing.R)*(1-alpha) + float64(col.R)*alpha)
	newG := uint8(float64(existing.G)*(1-alpha) + float64(col.G)*alpha)
	newB := uint8(float64(existing.B)*(1-alpha) + float64(col.B)*alpha)
	newA := uint8(math.Min(255, float64(existing.A)+float64(col.A)*alpha))
	c.Img.Set(x, y, color.RGBA{newR, newG, newB, newA})
}

// DrawRotatedImage composites a rotated alpha image onto the canvas.
// (x, y) is where the image origin (originX, originY within the image) should be placed on the canvas.
// The image is rotated by angle radians around the origin point.
func (c *Canvas) DrawRotatedImage(img *image.Alpha, x, y float64, angle float64, originX, originY int, col color.RGBA) {
	if img == nil {
		return
	}

	bounds := img.Bounds()
	w := float64(bounds.Dx())
	h := float64(bounds.Dy())

	// Origin point in image coordinates
	ox := float64(originX)
	oy := float64(originY)

	cos := math.Cos(angle)
	sin := math.Sin(angle)

	// Compute transformed corners to find bounding box
	// Corners relative to the origin point
	corners := [][2]float64{
		{-ox, -oy},
		{w - ox, -oy},
		{w - ox, h - oy},
		{-ox, h - oy},
	}
	minX, maxX := math.MaxFloat64, -math.MaxFloat64
	minY, maxY := math.MaxFloat64, -math.MaxFloat64
	for _, c := range corners {
		rx := c[0]*cos - c[1]*sin
		ry := c[0]*sin + c[1]*cos
		minX = math.Min(minX, rx)
		maxX = math.Max(maxX, rx)
		minY = math.Min(minY, ry)
		maxY = math.Max(maxY, ry)
	}

	// Create destination image for the rotated glyph
	dstW := int(math.Ceil(maxX-minX)) + 2
	dstH := int(math.Ceil(maxY-minY)) + 2
	if dstW <= 0 || dstH <= 0 {
		return
	}
	dst := image.NewAlpha(image.Rect(0, 0, dstW, dstH))

	// Transform matrix: maps source coords to destination coords
	// dst_x = (src_x - ox)*cos - (src_y - oy)*sin - minX + 1
	// dst_y = (src_x - ox)*sin + (src_y - oy)*cos - minY + 1
	transform := f64.Aff3{
		cos, -sin, -ox*cos + oy*sin - minX + 1,
		sin, cos, -ox*sin - oy*cos - minY + 1,
	}

	draw.BiLinear.Transform(dst, transform, img, bounds, draw.Over, nil)

	// Composite onto canvas
	// The origin in dst is at (-minX+1, -minY+1), which should map to (x, y) on canvas
	dstX := int(math.Round(x + minX - 1))
	dstY := int(math.Round(y + minY - 1))

	for py := range dstH {
		for px := range dstW {
			alpha := dst.AlphaAt(px, py).A
			if alpha == 0 {
				continue
			}
			canvasX := dstX + px
			canvasY := dstY + py
			if canvasX < 0 || canvasX >= c.Width || canvasY < 0 || canvasY >= c.Height {
				continue
			}
			a := float64(alpha) / 255.0
			c.BlendPixel(canvasX, canvasY, col, a)
		}
	}
}

// CompositeOver blends another canvas onto this canvas using alpha compositing
func (c *Canvas) CompositeOver(src *Canvas) {
	for y := 0; y < c.Height && y < src.Height; y++ {
		for x := 0; x < c.Width && x < src.Width; x++ {
			srcPx := src.Img.RGBAAt(x, y)
			if srcPx.A == 0 {
				continue
			}
			alpha := float64(srcPx.A) / 255.0
			c.BlendPixel(x, y, srcPx, alpha)
		}
	}
}
