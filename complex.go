package pdf

// This file contains more complex PDF data structures, which are composed
// of the elementary types from "objects.go".

import (
	"io"
	"math"
)

// A Number is either an Integer or a Real.
type Number float64

// PDF implements the Object interface.
func (x Number) PDF(w io.Writer) error {
	var obj Object
	if i := Integer(x); Number(i) == x {
		obj = i
	} else {
		obj = Real(x)
	}
	return obj.PDF(w)
}

// Rectangle represents a PDF rectangle, given by the coordinates of
// two diagonally opposite corners in a PDF Array.
type Rectangle struct {
	LLx, LLy, URx, URy float64
}

// PDF implements the Object interface.
func (rect *Rectangle) PDF(w io.Writer) error {
	res := Array{}
	for _, x := range []float64{rect.LLx, rect.LLy, rect.URx, rect.URy} {
		res = append(res, Number(x))
	}
	return res.PDF(w)
}

// NearlyEqual reports whether the corner coordinates of two rectangles
// differ by less than `eps`.
func (rect *Rectangle) NearlyEqual(other *Rectangle, eps float64) bool {
	return (math.Abs(rect.LLx-other.LLx) < eps &&
		math.Abs(rect.LLy-other.LLy) < eps &&
		math.Abs(rect.URx-other.URx) < eps &&
		math.Abs(rect.URy-other.URy) < eps)
}
