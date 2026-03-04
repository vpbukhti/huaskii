package geom

import "math"

// Point represents a 2D point
type Point struct {
	X, Y float64
}

// Add returns p + q
func (p Point) Add(q Point) Point {
	return Point{p.X + q.X, p.Y + q.Y}
}

// Sub returns p - q
func (p Point) Sub(q Point) Point {
	return Point{p.X - q.X, p.Y - q.Y}
}

// Scale returns p * s
func (p Point) Scale(s float64) Point {
	return Point{p.X * s, p.Y * s}
}

// Length returns the magnitude of the vector
func (p Point) Length() float64 {
	return math.Sqrt(p.X*p.X + p.Y*p.Y)
}

// Normalize returns the unit vector
func (p Point) Normalize() Point {
	l := p.Length()
	if l < 0.0001 {
		return Point{1, 0}
	}
	return Point{p.X / l, p.Y / l}
}

// Perpendicular returns the perpendicular vector (rotated 90 degrees CCW)
func (p Point) Perpendicular() Point {
	return Point{-p.Y, p.X}
}

// Rotate rotates the point by angle (radians)
func (p Point) Rotate(angle float64) Point {
	cos, sin := math.Cos(angle), math.Sin(angle)
	return Point{
		X: p.X*cos - p.Y*sin,
		Y: p.X*sin + p.Y*cos,
	}
}

// Close checks if two points are approximately equal
func (p Point) Close(q Point) bool {
	const epsilon = 0.001
	return math.Abs(p.X-q.X) < epsilon && math.Abs(p.Y-q.Y) < epsilon
}
