package geom

// EvalQuadBezier evaluates a quadratic Bezier curve at parameter t
func EvalQuadBezier(p0, p1, p2 Point, t float64) Point {
	mt := 1 - t
	return Point{
		X: mt*mt*p0.X + 2*mt*t*p1.X + t*t*p2.X,
		Y: mt*mt*p0.Y + 2*mt*t*p1.Y + t*t*p2.Y,
	}
}

// EvalQuadBezierTangent returns the tangent (derivative) at parameter t
func EvalQuadBezierTangent(p0, p1, p2 Point, t float64) Point {
	mt := 1 - t
	return Point{
		X: 2*mt*(p1.X-p0.X) + 2*t*(p2.X-p1.X),
		Y: 2*mt*(p1.Y-p0.Y) + 2*t*(p2.Y-p1.Y),
	}
}

// EvalCubicBezier evaluates a cubic Bezier curve at parameter t
func EvalCubicBezier(p0, p1, p2, p3 Point, t float64) Point {
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

// EvalCubicBezierTangent returns the tangent at parameter t
func EvalCubicBezierTangent(p0, p1, p2, p3 Point, t float64) Point {
	mt := 1 - t
	mt2 := mt * mt
	t2 := t * t
	return Point{
		X: 3*mt2*(p1.X-p0.X) + 6*mt*t*(p2.X-p1.X) + 3*t2*(p3.X-p2.X),
		Y: 3*mt2*(p1.Y-p0.Y) + 6*mt*t*(p2.Y-p1.Y) + 3*t2*(p3.Y-p2.Y),
	}
}

// PathSegment represents a segment of a path
type PathSegment struct {
	Type   int // 0=line, 1=quad, 2=cubic
	Points []Point
}

// EstimateLength approximates the arc length of a path segment
func (seg PathSegment) EstimateLength() float64 {
	switch seg.Type {
	case 0: // line
		return seg.Points[0].Sub(seg.Points[1]).Length()
	case 1: // quad bezier
		length := 0.0
		prev := seg.Points[0]
		for i := 1; i <= 16; i++ {
			t := float64(i) / 16.0
			curr := EvalQuadBezier(seg.Points[0], seg.Points[1], seg.Points[2], t)
			length += curr.Sub(prev).Length()
			prev = curr
		}
		return length
	case 2: // cubic bezier
		length := 0.0
		prev := seg.Points[0]
		for i := 1; i <= 16; i++ {
			t := float64(i) / 16.0
			curr := EvalCubicBezier(seg.Points[0], seg.Points[1], seg.Points[2], seg.Points[3], t)
			length += curr.Sub(prev).Length()
			prev = curr
		}
		return length
	}
	return 0
}

// GetPointAndTangent returns position and tangent at parameter t
func (seg PathSegment) GetPointAndTangent(t float64) (Point, Point) {
	switch seg.Type {
	case 0: // line
		p := seg.Points[0].Add(seg.Points[1].Sub(seg.Points[0]).Scale(t))
		tangent := seg.Points[1].Sub(seg.Points[0]).Normalize()
		return p, tangent
	case 1: // quad
		p := EvalQuadBezier(seg.Points[0], seg.Points[1], seg.Points[2], t)
		tangent := EvalQuadBezierTangent(seg.Points[0], seg.Points[1], seg.Points[2], t).Normalize()
		return p, tangent
	case 2: // cubic
		p := EvalCubicBezier(seg.Points[0], seg.Points[1], seg.Points[2], seg.Points[3], t)
		tangent := EvalCubicBezierTangent(seg.Points[0], seg.Points[1], seg.Points[2], seg.Points[3], t).Normalize()
		return p, tangent
	}
	return Point{}, Point{1, 0}
}
