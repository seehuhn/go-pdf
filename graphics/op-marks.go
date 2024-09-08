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
)

// MarkedContent represents a marked-content point or sequence.
type MarkedContent struct {
	// Tag specifies the role or significance of the sequence.
	Tag pdf.Name

	// Properties is a property list.
	Properties pdf.Dict

	Inline    bool
	SingleUse bool
}

// Embed adds the MarkedContent properties dict to a PDF file.
// This implements the [pdf.Embedder] interface.
func (mc *MarkedContent) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	if mc.SingleUse {
		return mc.Properties, zero, nil
	}

	ref := rm.Out.Alloc()
	err := rm.Out.Put(ref, mc.Properties)
	if err != nil {
		return nil, zero, err
	}
	return ref, zero, nil
}

// MarkedContentPoint adds a marked-content point to the content stream.
//
// The tag parameter specifies the role or significance of the point.
// The properties parameter is a property list.  Properties can either be
// nil, or a [pdf.Dict], or a [pdf.Reference] representing a [pdf.Dict].
//
// This implements the PDF graphics operators "MP" and "DP".
func (w *Writer) MarkedContentPoint(mc *MarkedContent) {
	if !w.isValid("MarkedContentPoint", objPage|objText) {
		return
	}
	if err := pdf.CheckVersion(w.RM.Out, "marked content", pdf.V1_2); err != nil {
		w.Err = err
		return
	}

	w.writeObject(mc.Tag)
	if w.Err != nil {
		return
	}

	if mc.Properties == nil {
		_, w.Err = fmt.Fprintln(w.Content, " MP")
		return
	}

	w.writeProperties(mc, "DP")
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

	w.writeObject(mc.Tag)
	if w.Err != nil {
		return
	}

	if mc.Properties == nil {
		_, w.Err = fmt.Fprintln(w.Content, " BMC")
		return
	}

	w.writeProperties(mc, "BDC")
}

func (w *Writer) writeProperties(mc *MarkedContent, op string) {
	var prop pdf.Object
	if mc.Inline {
		if !pdf.IsDirect(mc.Properties) {
			w.Err = ErrNotDirect
			return
		}
		prop = mc.Properties
	} else {
		name, _, err := writerGetResourceName(w, catProperties, mc)
		if err != nil {
			w.Err = err
			return
		}
		prop = name
	}

	_, err := w.Content.Write([]byte(" "))
	if err != nil {
		w.Err = err
		return
	}
	w.writeObject(prop)
	if w.Err != nil {
		return
	}
	_, w.Err = fmt.Fprintln(w.Content, " "+op)
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

// ErrNotDirect is returned by [Writer.MarkedContentStart] if the properties
// object is not a direct object, and the Inline property is set.
var ErrNotDirect = errors.New("MarkedContent: indirect object in inline property list")
