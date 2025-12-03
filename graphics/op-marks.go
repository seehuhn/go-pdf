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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/property"
)

// MarkedContent represents a marked-content point or sequence.
type MarkedContent struct {
	// Tag specifies the role or significance of the point/sequence.
	Tag pdf.Name

	// Properties is an optional property list providing additional data.
	// Set to nil for marked content without properties (MP/BMC operators).
	Properties property.List

	// Inline controls whether the property list is embedded inline in the
	// content stream (true) or referenced via the Properties resource
	// dictionary (false). Only relevant if Properties is not nil.
	// Property lists can only be inlined if Properties.IsDirect() returns true.
	Inline bool
}

// MarkedContentPoint adds a marked-content point to the content stream.
//
// This implements the PDF graphics operators "MP" (without properties)
// and "DP" (with properties).
func (w *Writer) MarkedContentPoint(mc *MarkedContent) {
	if !w.isValid("MarkedContentPoint", objPage|objText) {
		return
	}
	if err := pdf.CheckVersion(w.RM.Out, "marked content", pdf.V1_2); err != nil {
		w.Err = err
		return
	}

	if mc.Properties == nil {
		w.writeObjects(mc.Tag, pdf.Operator("MP"))
		return
	}

	prop := w.getProperties(mc)
	if w.Err != nil {
		return
	}
	w.writeObjects(mc.Tag, prop, pdf.Operator("DP"))
}

// MarkedContentStart begins a marked-content sequence.  The sequence is
// terminated by a call to [Writer.MarkedContentEnd].
//
// This implements the PDF graphics operators "BMC" and "BDC".
func (w *Writer) MarkedContentStart(mc *MarkedContent) {
	if !w.isValid("MarkedContentStart", objPage|objText) {
		return
	}
	if err := pdf.CheckVersion(w.RM.Out, "marked content", pdf.V1_2); err != nil {
		w.Err = err
		return
	}

	w.nesting = append(w.nesting, pairTypeBMC)
	w.markedContent = append(w.markedContent, mc)

	if mc.Properties == nil {
		w.writeObjects(mc.Tag, pdf.Operator("BMC"))
		return
	}

	prop := w.getProperties(mc)
	if w.Err != nil {
		return
	}
	w.writeObjects(mc.Tag, prop, pdf.Operator("BDC"))
}

func (w *Writer) getProperties(mc *MarkedContent) pdf.Object {
	if mc.Inline {
		if !mc.Properties.IsDirect() {
			w.Err = ErrNotDirect
			return nil
		}
		// Embed inline - will return the direct dict
		embedded, err := w.RM.Embed(mc.Properties)
		if err != nil {
			w.Err = err
			return nil
		}
		return embedded
	}

	name, err := writerGetResourceName(w, catProperties, mc.Properties)
	if err != nil {
		w.Err = err
		return nil
	}
	return name
}

// MarkedContentEnd ends a marked-content sequence.
// This must be matched with a preceding call to [Writer.MarkedContentStart].
func (w *Writer) MarkedContentEnd() {
	if len(w.nesting) == 0 || w.nesting[len(w.nesting)-1] != pairTypeBMC {
		w.Err = errors.New("MarkedContentEnd: no matching MarkedContentStart")
		return
	}

	w.nesting = w.nesting[:len(w.nesting)-1]
	w.markedContent = w.markedContent[:len(w.markedContent)-1]

	_, w.Err = fmt.Fprintln(w.Content, "EMC")
}

// ErrNotDirect is returned when attempting to inline a property list
// that cannot be embedded inline in the content stream.
var ErrNotDirect = errors.New("property list cannot be inlined in content stream")
