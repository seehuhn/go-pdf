// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package graphics

import (
	"errors"
	"fmt"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/matrix"
	"seehuhn.de/go/pdf/internal/float"
)

// This file implements the operators in the "General Graphics State" and
// "Special graphics state" categories.  These operators are defined
// in table 56 of ISO 32000-2:2020.

// PushGraphicsState saves the current graphics state.
//
// This implementes the PDF graphics operator "q".
func (w *Writer) PushGraphicsState() {
	// This operator was classed as "Special graphics state" until PDF 1.7,
	// and as "General Graphics State" in PDF 2.0.
	allowedStates := ifelse(w.Version >= pdf.V2_0, objPage|objText, objPage)
	if !w.isValid("PushGraphicsState", allowedStates) {
		return
	}

	w.nesting = append(w.nesting, pairTypeQ)

	w.stack = append(w.stack, State{
		Parameters: w.State.Parameters.Clone(),
		Set:        w.State.Set,
	})

	_, w.Err = fmt.Fprintln(w.Content, "q")
}

// PopGraphicsState restores the previous graphics state.
//
// This implementes the PDF graphics operator "Q".
func (w *Writer) PopGraphicsState() {
	// This operator was classed as "Special graphics state" until PDF 1.7,
	// and as "General Graphics State" in PDF 2.0.
	allowedStates := ifelse(w.Version >= pdf.V2_0, objPage|objText, objPage)
	if !w.isValid("PopGraphicsState", allowedStates) {
		return
	}

	if len(w.nesting) == 0 || w.nesting[len(w.nesting)-1] != pairTypeQ {
		w.Err = errors.New("PopGraphicsState: no matching PushGraphicsState")
		return
	}
	w.nesting = w.nesting[:len(w.nesting)-1]

	n := len(w.stack) - 1
	savedState := w.stack[n]
	w.stack = w.stack[:n]
	w.State = savedState

	_, w.Err = fmt.Fprintln(w.Content, "Q")
}

// Transform applies a transformation matrix to the coordinate system.
// This function modifies the current transformation matrix, so that
// the new, additional transformation is applied to the user coordinates
// first, followed by the existing transformation.
//
// This implementes the PDF graphics operator "cm".
func (w *Writer) Transform(extraTrfm matrix.Matrix) {
	if !w.isValid("Transform", objPage) { // special graphics state
		return
	}

	w.CTM = extraTrfm.Mul(w.CTM)

	_, w.Err = fmt.Fprintln(w.Content,
		float.Format(extraTrfm[0], 3), float.Format(extraTrfm[1], 3),
		float.Format(extraTrfm[2], 3), float.Format(extraTrfm[3], 3),
		float.Format(extraTrfm[4], 3), float.Format(extraTrfm[5], 3), "cm")
}

// SetLineWidth sets the line width.
//
// This implementes the PDF graphics operator "w".
func (w *Writer) SetLineWidth(width float64) {
	if !w.isValid("SetLineWidth", objPage|objText) {
		return
	}
	if width < 0 {
		w.Err = fmt.Errorf("SetLineWidth: negative width %f", width)
		return
	}
	if w.isSet(StateLineWidth) && nearlyEqual(width, w.LineWidth) {
		return
	}

	w.LineWidth = width
	w.Set |= StateLineWidth

	_, w.Err = fmt.Fprintln(w.Content, w.coord(width), "w")
}

// SetLineCap sets the line cap style.
//
// This implementes the PDF graphics operator "J".
func (w *Writer) SetLineCap(cap LineCapStyle) {
	if !w.isValid("SetLineCap", objPage|objText) {
		return
	}
	if LineCapStyle(cap) > 2 {
		w.Err = fmt.Errorf("SetLineCap: invalid line cap style %d", cap)
	}
	if w.isSet(StateLineCap) && cap == w.LineCap {
		return
	}

	w.LineCap = cap
	w.Set |= StateLineCap

	_, w.Err = fmt.Fprintln(w.Content, int(cap), "J")
}

// SetLineJoin sets the line join style.
//
// This implementes the PDF graphics operator "j".
func (w *Writer) SetLineJoin(join LineJoinStyle) {
	if !w.isValid("SetLineJoin", objPage|objText) {
		return
	}
	if LineJoinStyle(join) > 2 {
		w.Err = fmt.Errorf("SetLineJoin: invalid line join style %d", join)
	}
	if w.isSet(StateLineJoin) && join == w.LineJoin {
		return
	}

	w.LineJoin = join
	w.Set |= StateLineJoin

	_, w.Err = fmt.Fprintln(w.Content, int(join), "j")
}

// SetMiterLimit sets the miter limit.
//
// This implementes the PDF graphics operator "M".
func (w *Writer) SetMiterLimit(limit float64) {
	if !w.isValid("SetMiterLimit", objPage|objText) {
		return
	}
	if limit < 1 {
		w.Err = fmt.Errorf("SetMiterLimit: invalid miter limit %f", limit)
		return
	}
	if w.isSet(StateMiterLimit) && nearlyEqual(limit, w.MiterLimit) {
		return
	}

	w.MiterLimit = limit
	w.Set |= StateMiterLimit

	_, w.Err = fmt.Fprintln(w.Content, float.Format(limit, 4), "M")
}

// SetLineDash sets the line dash pattern.
//
// This implementes the PDF graphics operator "d".
func (w *Writer) SetLineDash(pattern []float64, phase float64) {
	if !w.isValid("SetLineDash", objPage|objText) {
		return
	}
	if w.isSet(StateLineDash) &&
		sliceNearlyEqual(pattern, w.DashPattern) &&
		nearlyEqual(phase, w.DashPhase) {
		return
	}

	w.DashPattern = pattern
	w.DashPhase = phase
	w.Set |= StateLineDash

	_, w.Err = fmt.Fprint(w.Content, "[")
	if w.Err != nil {
		return
	}
	sep := ""
	for _, x := range pattern {
		_, w.Err = fmt.Fprint(w.Content, sep, float.Format(x, 3))
		if w.Err != nil {
			return
		}
		sep = " "
	}
	_, w.Err = fmt.Fprint(w.Content, "] ", float.Format(phase, 3), " d\n")
}

// SetRenderingIntent sets the rendering intent.
//
// This implementes the PDF graphics operator "ri".
func (w *Writer) SetRenderingIntent(intent RenderingIntent) {
	if !w.isValid("SetRenderingIntent", objPage|objText) {
		return
	}
	if w.Version < pdf.V1_1 {
		w.Err = &pdf.VersionError{Operation: "SetRenderingIntent", Earliest: pdf.V1_1}
	}
	if w.isSet(StateRenderingIntent) && intent == w.RenderingIntent {
		return
	}

	w.RenderingIntent = intent
	w.Set |= StateRenderingIntent

	err := pdf.Name(intent).PDF(w.Content)
	if err != nil {
		w.Err = err
		return
	}
	_, w.Err = fmt.Fprintln(w.Content, " ri")
}

// SetFlatnessTolerance sets the flatness tolerance.
//
// This implementes the PDF graphics operator "i".
func (w *Writer) SetFlatnessTolerance(flatness float64) {
	if !w.isValid("SetFlatness", objPage|objText) {
		return
	}
	if flatness < 0 || flatness > 100 {
		w.Err = fmt.Errorf("SetFlatnessTolerance: invalid flatness tolerance %f", flatness)
	}
	if w.isSet(StateFlatnessTolerance) && nearlyEqual(flatness, w.FlatnessTolerance) {
		return
	}

	w.FlatnessTolerance = flatness
	w.Set |= StateFlatnessTolerance

	_, w.Err = fmt.Fprintln(w.Content, float.Format(flatness, 3), "i")
}

// SetExtGState sets selected graphics state parameters.
//
// This implements the "gs" graphics operator.
func (w *Writer) SetExtGState(s *ExtGState) {
	if !w.isValid("SetExtGState", objPage|objText) {
		return
	}
	if w.Version < pdf.V1_2 {
		w.Err = &pdf.VersionError{Operation: "SetExtGState", Earliest: pdf.V1_2}
	}

	s.Value.CopyTo(&w.State)

	name := w.getResourceName(catExtGState, s)
	err := name.PDF(w.Content)
	if err != nil {
		w.Err = err
		return
	}
	_, w.Err = fmt.Fprintln(w.Content, " gs")
}

func ifelse[T any](c bool, a, b T) T {
	if c {
		return a
	}
	return b
}

func nearlyEqual(a, b float64) bool {
	const ε = 1e-6
	return math.Abs(a-b) < ε
}

func sliceNearlyEqual(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i, x := range a {
		if nearlyEqual(x, b[i]) {
			return false
		}
	}
	return true
}
