// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package annotation

import (
	"errors"

	"seehuhn.de/go/pdf"
)

// Widget represents a widget annotation used by interactive forms to represent
// the appearance of fields and to manage user interactions. When a field has
// only a single associated widget annotation, the contents of the field
// dictionary and the annotation dictionary may be merged into a single dictionary.
type Widget struct {
	Common

	// Highlight (optional) is the annotation's highlighting mode, the visual effect
	// that is used when the mouse button is pressed or held down inside
	// its active area. Valid values:
	// - "N" (None): No highlighting
	// - "I" (Invert): Invert the colors used to display the contents
	// - "O" (Outline): Stroke the colors used to display the annotation border
	// - "P" (Push): Display the annotation's down appearance
	// - "T" (Toggle): Same as P (which is preferred)
	// Default value: "I"
	Highlight pdf.Name

	// MK (optional) is an appearance characteristics dictionary that is
	// used in constructing a dynamic appearance stream specifying the
	// annotation's visual presentation on the page.
	MK pdf.Reference

	// A (optional; PDF 1.1) is an action that is performed when the
	// annotation is activated.
	A pdf.Reference

	// AA (optional; PDF 1.2) is an additional-actions dictionary defining
	// the annotation's behaviour in response to various trigger events.
	AA pdf.Reference

	// BorderStyle (optional; PDF 1.2) is a border style dictionary specifying
	// the width and dash pattern that is used in drawing the annotation's
	// border.
	//
	// If this field is set, the Common.Border field is ignored.
	//
	// This corresponds to the /BS entry in the PDF annotation dictionary.
	BorderStyle *BorderStyle

	// Parent (required if this widget annotation is one of multiple children
	// in a field; optional otherwise) is an indirect reference to the widget
	// annotation's parent field. A widget annotation may have at most one parent.
	Parent pdf.Reference
}

var _ Annotation = (*Widget)(nil)

// AnnotationType returns "Widget".
// This implements the [Annotation] interface.
func (w *Widget) AnnotationType() pdf.Name {
	return "Widget"
}

func decodeWidget(x *pdf.Extractor, dict pdf.Dict) (*Widget, error) {
	r := x.R
	widget := &Widget{}

	// Extract common annotation fields
	if err := decodeCommon(x, &widget.Common, dict); err != nil {
		return nil, err
	}

	// Extract widget-specific fields
	// H (optional) - default to "I" if not specified
	if h, err := pdf.GetName(r, dict["H"]); err == nil && h != "" {
		widget.Highlight = h
	} else {
		widget.Highlight = "I" // PDF default value
	}

	// MK (optional)
	if mk, ok := dict["MK"].(pdf.Reference); ok {
		widget.MK = mk
	}

	// A (optional)
	if a, ok := dict["A"].(pdf.Reference); ok {
		widget.A = a
	}

	// AA (optional)
	if aa, ok := dict["AA"].(pdf.Reference); ok {
		widget.AA = aa
	}

	// BS (optional)
	if bs, err := pdf.Optional(pdf.ExtractorGet(x, dict["BS"], ExtractBorderStyle)); err != nil {
		return nil, err
	} else {
		widget.BorderStyle = bs
		widget.Common.Border = nil
	}

	// Parent (optional)
	if parent, ok := dict["Parent"].(pdf.Reference); ok {
		widget.Parent = parent
	}

	return widget, nil
}

func (w *Widget) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "widget annotation", pdf.V1_2); err != nil {
		return nil, err
	}

	if w.Common.Border != nil && w.BorderStyle != nil {
		return nil, errors.New("conflicting border settings")
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("Widget"),
	}

	// Add common annotation fields
	if err := w.Common.fillDict(rm, dict, isMarkup(w)); err != nil {
		return nil, err
	}

	// Add widget-specific fields
	// H (optional) - only write if not the default value "I"
	if w.Highlight != "" && w.Highlight != "I" {
		dict["H"] = w.Highlight
	}

	// MK (optional)
	if w.MK != 0 {
		dict["MK"] = w.MK
	}

	// A (optional)
	if w.A != 0 {
		dict["A"] = w.A
	}

	// AA (optional)
	if w.AA != 0 {
		dict["AA"] = w.AA
	}

	// BS (optional)
	if w.BorderStyle != nil {
		bs, err := rm.Embed(w.BorderStyle)
		if err != nil {
			return nil, err
		}
		dict["BS"] = bs
		delete(dict, "Border")
	}

	// Parent (optional)
	if w.Parent != 0 {
		dict["Parent"] = w.Parent
	}

	return dict, nil
}
