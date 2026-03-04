package renderer

import (
	"image"
	"image/color"
	"math"
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
