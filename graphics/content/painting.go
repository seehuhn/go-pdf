// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package content

// PaintKind is the kind of graphics object an operator paints.
type PaintKind uint8

const (
	// PaintNone is the kind of all operators which do not paint.
	PaintNone PaintKind = iota

	// PaintPath is the kind of the path-painting operators.  This includes
	// n, which ends the path without painting it.
	PaintPath

	// PaintText is the kind of the text-showing operators
	// Tj, TJ, ' and ".
	PaintText

	// PaintShading is the kind of the sh operator.
	PaintShading

	// PaintXObject is the kind of the Do operator.  The operator name does
	// not say whether the object is an image, an image mask or a form.
	PaintXObject

	// PaintInlineImage is the kind of an inline image.
	PaintInlineImage
)

// PaintKind returns the kind of graphics object the operator paints.
func (name OpName) PaintKind() PaintKind {
	if _, ok := PathPainting(name); ok {
		return PaintPath
	}
	switch name {
	case OpTextShow, OpTextShowArray, OpTextShowMoveNextLine,
		OpTextShowMoveNextLineSetSpacing:
		return PaintText
	case OpShading:
		return PaintShading
	case OpXObject:
		return PaintXObject
	case OpInlineImage:
		return PaintInlineImage
	}
	return PaintNone
}

// PathPaint describes a path-painting operator.
type PathPaint struct {
	// Fill is true if the operator fills the path.
	Fill bool

	// Stroke is true if the operator strokes the path.
	Stroke bool

	// EvenOdd is true if the operator fills using the even-odd rule.
	EvenOdd bool

	// Close is true if the operator closes the path first.
	Close bool
}

// PathPainting returns what a path-painting operator does.  For n, which
// ends the path without painting it, all fields are false.  The second
// result is false for operators which do not paint paths.
func PathPainting(name OpName) (PathPaint, bool) {
	switch name {
	case OpStroke:
		return PathPaint{Stroke: true}, true
	case OpCloseAndStroke:
		return PathPaint{Stroke: true, Close: true}, true
	case OpFill, OpFillCompat:
		return PathPaint{Fill: true}, true
	case OpFillEvenOdd:
		return PathPaint{Fill: true, EvenOdd: true}, true
	case OpFillAndStroke:
		return PathPaint{Fill: true, Stroke: true}, true
	case OpFillAndStrokeEvenOdd:
		return PathPaint{Fill: true, Stroke: true, EvenOdd: true}, true
	case OpCloseFillAndStroke:
		return PathPaint{Fill: true, Stroke: true, Close: true}, true
	case OpCloseFillAndStrokeEvenOdd:
		return PathPaint{Fill: true, Stroke: true, EvenOdd: true, Close: true}, true
	case OpEndPath:
		return PathPaint{}, true
	}
	return PathPaint{}, false
}
