// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package decode

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/sound"
)

func decodeSound(c pdf.Cursor, dict pdf.Dict) (*annotation.Sound, error) {
	a := &annotation.Sound{}

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
	s, err := c.Decode(dict["Sound"], sound.Extract)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, pdf.Error("sound annotation missing Sound")
	}
	a.Sound = s

	// Name (optional) - default to "Speaker" if not specified
	if name, err := pdf.Optional(c.Name(dict["Name"])); err != nil {
		return nil, err
	} else if name != "" {
		a.Icon = annotation.SoundIcon(name)
	} else {
		a.Icon = annotation.SoundIconSpeaker
	}

	return a, nil
}
