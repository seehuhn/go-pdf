// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package decode

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/action/triggers"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
)

// decodeWidgetBody decodes the annotation-level half of a widget dictionary (the
// entries that pertain to a widget annotation, not to a form field). It never
// sets Parent: the field tree owns that linkage.
func decodeWidgetBody(c pdf.Cursor, dict pdf.Dict) (*annotation.Widget, error) {
	widget := &annotation.Widget{}

	// Extract common annotation fields
	if err := decodeCommon(c, &widget.Common, dict); err != nil {
		return nil, err
	}

	// H (optional)
	widget.Highlight = decodeHighlight(c, dict["H"])

	// MK (optional)
	if mk, err := pdf.DecodeOptional(c, dict["MK"], appearance.ExtractCharacteristics); err != nil {
		return nil, err
	} else {
		widget.MK = mk
	}

	// A (optional)
	if a, err := pdf.DecodeOptional(c, dict["A"], action.Decode); err != nil {
		return nil, err
	} else {
		widget.Action = a
	}

	// AA (optional)
	if aa, err := pdf.DecodeOptional(c, dict["AA"], triggers.DecodeAnnotation); err != nil {
		return nil, err
	} else {
		widget.AA = aa
	}

	// BS (optional)
	if bs, err := pdf.DecodeOptional(c, dict["BS"], annotation.ExtractBorderStyle); err != nil {
		return nil, err
	} else {
		widget.BorderStyle = bs
		if bs != nil {
			// per PDF spec, Border is ignored when BS is present
			widget.Common.Border = nil
		}
	}

	// /Parent is intentionally not read here: the field tree owns the linkage
	// and sets Widget.Parent when the form is decoded.

	// the widget half of a shared /AA keeps only the annotation-level triggers;
	// drop an empty remnant left by the field/widget split
	if widget.AA != nil && widget.AA.IsEmpty() {
		widget.AA = nil
	}

	return widget, nil
}
