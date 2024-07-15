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
	// Properties is a property list.  The value can either be nil, or a
	// [pdf.Dict], or a [pdf.Resource] representing a [pdf.Dict].
	Properties pdf.Dict

	// Tag specifies the role or significance of the sequence.
	Tag pdf.Name

	Inline bool

	DefName pdf.Name
}

// PDFObject implements the [pdf.Resource] interface.
func (mc *MarkedContent) PDFObject() pdf.Object {
	return mc.Properties
}

// MarkedContentPoint adds a marked-content point to the content stream.
//
// The tag parameter specifies the role or significance of the point.
// The properties parameter is a property list.  Properties can either be
// nil, or a [pdf.Dict], or a [pdf.Resource] representing a [pdf.Dict].
//
// This implements the PDF graphics operators "MP" and "DP".
func (w *Writer) MarkedContentPoint(mc *MarkedContent) {
	if !w.isValid("MarkedContentPoint", objPage|objText) {
		return
	}
	if w.Version < pdf.V1_2 {
		w.Err = &pdf.VersionError{
			Operation: "marked content",
			Earliest:  pdf.V1_2,
		}
		return
	}

	if mc.Properties == nil {
		w.Err = mc.Tag.PDF(w.Content)
		if w.Err == nil {
			_, w.Err = fmt.Fprintln(w.Content, "MP")
		}
		return
	}

	var prop pdf.Object = mc.Properties
	if mc.Inline {
		w.Err = checkNoReferences(prop)
		if w.Err != nil {
			return
		}
	} else {
		prop = w.getResourceNameOld(catProperties, mc)
	}
	w.Err = prop.PDF(w.Content)
	if w.Err == nil {
		_, w.Err = fmt.Fprintln(w.Content, " DP")
	}
}

// MarkedContentStart begins a marked-content sequence.  The sequence is
// terminated by a call to [Writer.MarkedContentEnd].
//
// This implements the PDF graphics operators "BMC" and "BDC".
func (w *Writer) MarkedContentStart(mc *MarkedContent) {
	if !w.isValid("MarkedContentStart", objPage|objText) {
		return
	}
	if w.Version < pdf.V1_2 {
		w.Err = &pdf.VersionError{
			Operation: "marked content",
			Earliest:  pdf.V1_2,
		}
		return
	}

	w.nesting = append(w.nesting, pairTypeBMC)
	w.markedContent = append(w.markedContent, mc)

	if mc.Properties == nil {
		w.Err = mc.Tag.PDF(w.Content)
		if w.Err == nil {
			_, w.Err = fmt.Fprintln(w.Content, "BMC")
		}
		return
	}

	var prop pdf.Object = mc.Properties
	if mc.Inline {
		w.Err = checkNoReferences(prop)
		if w.Err != nil {
			return
		}
	} else {
		prop = w.getResourceNameOld(catProperties, mc)
	}
	w.Err = prop.PDF(w.Content)
	if w.Err == nil {
		_, w.Err = fmt.Fprintln(w.Content, " BDC")
	}
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

}

func checkNoReferences(obj pdf.Object) error {
	switch obj := obj.(type) {
	case pdf.Reference:
		return errors.New("properties cannot be inlined")
	case pdf.Dict:
		for _, v := range obj {
			if err := checkNoReferences(v); err != nil {
				return err
			}
		}
	case pdf.Array:
		for _, v := range obj {
			if err := checkNoReferences(v); err != nil {
				return err
			}
		}
	}
	return nil
}
