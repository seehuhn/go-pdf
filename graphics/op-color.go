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
	"fmt"
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/float"
)

// This file implements functions to set the stroke and fill colors.
// The operators used here are defined in table 73 of ISO 32000-2:2020.

// SetStrokeColor sets the color to use for stroking operations.
func (w *Writer) SetStrokeColor(c color.Color) {
	if !w.isValid("SetStrokeColor", objPage|objText) {
		return
	}

	cs := c.ColorSpace()
	if err := color.CheckVersion(cs, w.Version); err != nil {
		w.Err = err
		return
	}

	var cur color.Color
	if w.isSet(StateStrokeColor) {
		cur = w.StrokeColor
	}
	needsColorSpace, needsColor := color.CheckCurrent(cur, c)

	w.StrokeColor = c
	w.Set |= StateStrokeColor

	if needsColorSpace {
		name := w.getResourceName(catColorSpace, cs)
		w.Err = name.PDF(w.Content)
		if w.Err != nil {
			return
		}
		_, w.Err = fmt.Fprintln(w.Content, " CS")
		if w.Err != nil {
			return
		}
	}

	if needsColor {
		values, pattern, op := color.Operator(c)
		for _, val := range values {
			valString := float.Format(val, 3)
			_, w.Err = fmt.Fprint(w.Content, valString, " ")
			if w.Err != nil {
				return
			}
		}
		if pattern != nil {
			name := w.getResourceName(catPattern, pattern)
			w.Err = name.PDF(w.Content)
			if w.Err != nil {
				return
			}
			_, w.Err = fmt.Fprint(w.Content, " ")
			if w.Err != nil {
				return
			}
		}
		_, w.Err = fmt.Fprintln(w.Content, op)
		if w.Err != nil {
			return
		}
	}
}

// SetFillColor sets the color to use for non-stroking operations.
func (w *Writer) SetFillColor(c color.Color) {
	if !w.isValid("SetFillColor", objPage|objText) {
		return
	}

	cs := c.ColorSpace()
	if err := color.CheckVersion(cs, w.Version); err != nil {
		w.Err = err
		return
	}

	var cur color.Color
	if w.isSet(StateFillColor) {
		cur = w.FillColor
	}
	needsColorSpace, needsColor := color.CheckCurrent(cur, c)

	w.FillColor = c
	w.Set |= StateFillColor

	if needsColorSpace {
		name := w.getResourceName(catColorSpace, cs)
		w.Err = name.PDF(w.Content)
		if w.Err != nil {
			return
		}
		_, w.Err = fmt.Fprintln(w.Content, " cs")
		if w.Err != nil {
			return
		}
	}

	if needsColor {
		values, pattern, op := color.Operator(c)
		for _, val := range values {
			valString := float.Format(val, 3)
			_, w.Err = fmt.Fprint(w.Content, valString, " ")
			if w.Err != nil {
				return
			}
		}
		if pattern != nil {
			name := w.getResourceName(catPattern, pattern)
			w.Err = name.PDF(w.Content)
			if w.Err != nil {
				return
			}
			_, w.Err = fmt.Fprint(w.Content, " ")
			if w.Err != nil {
				return
			}
		}
		_, w.Err = fmt.Fprintln(w.Content, strings.ToLower(op))
		if w.Err != nil {
			return
		}
	}
}

type Shading struct {
	pdf.Res
}

// DrawShading paints the given shading, subject to the current clipping path.
// The current colour in the graphics state is neither used nor altered.
//
// All coordinates in the shading dictionary are interpreted relative to the
// current user space. The "Background" entry in the shading pattern (if any)
// is ignored.
//
// This implements the "sh" graphics operator.
func (w *Writer) DrawShading(shading *Shading) {
	if !w.isValid("DrawShading", objPage) {
		return
	}
	if w.Version < pdf.V1_3 {
		w.Err = &pdf.VersionError{
			Operation: "shading objects",
			Earliest:  pdf.V1_3,
		}
		return
	}

	name := w.getResourceName(catShading, shading)
	w.Err = name.PDF(w.Content)
	if w.Err != nil {
		return
	}
	_, w.Err = fmt.Fprintln(w.Content, " sh")
}
