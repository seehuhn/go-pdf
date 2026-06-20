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
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/sound"
)

func decodeSound(c pdf.Cursor, dict pdf.Dict) (*annotation.Sound, error) {
	a := &annotation.Sound{}

	// Extract common annotation fields
	if err := decodeCommon(c, &a.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(c, dict, &a.Markup); err != nil {
		return nil, err
	}

	// Sound (required): without a usable Sound the annotation cannot be
	// written back, so reject it; the page decoder drops annotations that
	// fail to decode, matching the permissive-reader policy.
	s, err := pdf.Decode(c, dict["Sound"], sound.Extract)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, pdf.Error("sound annotation missing Sound")
	}
	a.Sound = s

	// Name (optional) - default to "Speaker" if not specified
	if name, err := c.Name(dict["Name"]); err == nil && name != "" {
		a.Icon = annotation.SoundIcon(name)
	} else {
		a.Icon = annotation.SoundIconSpeaker
	}

	return a, nil
}
