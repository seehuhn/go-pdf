// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package decode

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/action/triggers"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
)

func decodeScreen(c pdf.Cursor, dict pdf.Dict) (*annotation.Screen, error) {
	screen := &annotation.Screen{}

	if err := decodeCommon(c, &screen.Common, dict); err != nil {
		return nil, err
	}

	// Extract screen-specific fields
	// T (optional)
	if t, err := pdf.Optional(c.TextString(dict["T"])); err != nil {
		return nil, err
	} else {
		screen.Title = t
	}

	// MK (optional)
	if mk, err := c.DecodeOptional(dict["MK"], appearance.ExtractCharacteristics); err != nil {
		return nil, err
	} else {
		screen.Style = mk
	}

	// A (optional)
	if act, err := c.DecodeOptional(dict["A"], action.Decode); err != nil {
		return nil, err
	} else {
		screen.Action = act
	}

	// AA (optional)
	if aa, err := c.DecodeOptional(dict["AA"], triggers.DecodeAnnotation); err != nil {
		return nil, err
	} else {
		screen.AA = aa
	}

	return screen, nil
}
