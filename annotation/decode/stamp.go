// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package decode

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
)

func decodeStamp(c pdf.Cursor, dict pdf.Dict) (*annotation.Stamp, error) {
	stamp := &annotation.Stamp{}

	if err := decodeCommon(c, &stamp.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(c, dict, &stamp.Markup); err != nil {
		return nil, err
	}

	if stamp.Intent == "" {
		stamp.Intent = annotation.StampIntentStamp
	}

	if stamp.Intent == annotation.StampIntentStamp {
		if icon, err := pdf.Optional(c.Name(dict["Name"])); err != nil {
			return nil, err
		} else if icon != "" {
			stamp.Icon = annotation.StampIcon(icon)
		} else {
			stamp.Icon = annotation.StampIconDraft
		}
	}

	return stamp, nil
}
