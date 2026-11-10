// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package decode

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
)

func decodePrinterMark(c pdf.Cursor, dict pdf.Dict) (*annotation.PrinterMark, error) {
	printerMark := &annotation.PrinterMark{}

	if err := decodeCommon(c, &printerMark.Common, dict); err != nil {
		return nil, err
	}

	// flag values for printer's mark annotations are prescribed by the spec:
	printerMark.Flags = annotation.FlagPrint | annotation.FlagReadOnly

	// Extract printer's mark-specific fields

	// MN (optional)
	if mn, err := pdf.Optional(c.Name(dict["MN"])); err != nil {
		return nil, err
	} else {
		printerMark.MarkName = mn
	}

	return printerMark, nil
}
