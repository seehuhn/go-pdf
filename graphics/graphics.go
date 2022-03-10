package graphics

type State struct {
	CTM Matrix
	// Clipping Path
	// Color Space
	// Color
	// Text State
	LineWidth  float64
	LineCap    LineCapStyle
	LineJoin   LineJoinStyle
	MiterLimit float64
	// DashPattern
	// Rendering Intent
	StrokeAdjustment bool
	// Blend Mode
	// Soft Mask
	// Alpha Constant
	// Alpha Source

	// Also Table 53 – Device-Dependent Graphics State Parameters (page 123)
}

type Matrix [6]float64

type LineCapStyle uint8

const (
	LineCapButt   LineCapStyle = 0
	LineCapRound  LineCapStyle = 1
	LineCapSquare LineCapStyle = 2
)

type LineJoinStyle uint8

const (
	LineJoinMiter LineJoinStyle = 0
	LineJoinRound LineJoinStyle = 1
	LineJoinBevel LineJoinStyle = 2
)
