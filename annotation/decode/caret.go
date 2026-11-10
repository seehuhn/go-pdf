// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package decode

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
)

func decodeCaret(c pdf.Cursor, dict pdf.Dict) (*annotation.Caret, error) {
	caret := &annotation.Caret{}

	if err := decodeCommon(c, &caret.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(c, dict, &caret.Markup); err != nil {
		return nil, err
	}

	// Extract caret-specific fields
	// RD (optional)
	if rd, err := c.FloatArray(dict["RD"]); err == nil && len(rd) == 4 {
		caret.Margin = rd
	} else if pdf.IsReadError(err) {
		return nil, err
	}

	// Sy (optional)
	if sy, err := c.Name(dict["Sy"]); err == nil && sy != "" {
		caret.Symbol = sy
	} else if pdf.IsReadError(err) {
		return nil, err
	}
	if caret.Symbol == "" {
		caret.Symbol = "None"
	}

	return caret, nil
}
