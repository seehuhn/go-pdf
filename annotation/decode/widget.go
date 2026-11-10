// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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

	if err := decodeCommon(c, &widget.Common, dict); err != nil {
		return nil, err
	}

	// H (optional)
	if h, err := decodeHighlight(c, dict["H"]); err != nil {
		return nil, err
	} else {
		widget.Highlight = h
	}

	// MK (optional)
	if mk, err := c.DecodeOptional(dict["MK"], appearance.ExtractCharacteristics); err != nil {
		return nil, err
	} else {
		widget.Style = mk
	}

	// A (optional)
	if a, err := c.DecodeOptional(dict["A"], action.Decode); err != nil {
		return nil, err
	} else {
		widget.Action = a
	}

	// AA (optional)
	if aa, err := c.DecodeOptional(dict["AA"], triggers.DecodeAnnotation); err != nil {
		return nil, err
	} else {
		widget.AA = aa
	}

	// BS (optional)
	if bs, err := c.DecodeOptional(dict["BS"], annotation.ExtractBorderStyle); err != nil {
		return nil, err
	} else {
		widget.BorderStyle = bs
		if bs != nil {
			// per PDF spec, Border is ignored when BS is present
			widget.Common.Border = nil
		}
	}

	// /Parent is intentionally not read here: the field tree owns the linkage
	// and sets Widget.Field when the form is decoded.

	// the widget half of a shared /AA keeps only the annotation-level triggers;
	// drop an empty remnant left by the field/widget split
	if widget.AA != nil && widget.AA.IsEmpty() {
		widget.AA = nil
	}

	return widget, nil
}
