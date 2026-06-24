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

	// Extract common annotation fields
	if err := decodeCommon(c, &screen.Common, dict); err != nil {
		return nil, err
	}

	// Extract screen-specific fields
	// T (optional)
	if t, err := pdf.Optional(c.TextString(dict["T"])); err != nil {
		return nil, err
	} else {
		screen.Title = string(t)
	}

	// MK (optional)
	if mk, err := pdf.DecodeOptional(c, dict["MK"], appearance.ExtractCharacteristics); err != nil {
		return nil, err
	} else {
		screen.Style = mk
	}

	// A (optional)
	if act, err := pdf.DecodeOptional(c, dict["A"], action.Decode); err != nil {
		return nil, err
	} else {
		screen.Action = act
	}

	// AA (optional)
	if aa, err := pdf.DecodeOptional(c, dict["AA"], triggers.DecodeAnnotation); err != nil {
		return nil, err
	} else {
		screen.AA = aa
	}

	return screen, nil
}
