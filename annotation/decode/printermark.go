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
)

func decodePrinterMark(c pdf.Cursor, dict pdf.Dict) (*annotation.PrinterMark, error) {
	printerMark := &annotation.PrinterMark{}

	// Extract common annotation fields
	if err := decodeCommon(c, &printerMark.Common, dict); err != nil {
		return nil, err
	}

	// The Print and ReadOnly flags are required, and all others must be
	// clear.  Snap the flags so that malformed input stays writable.
	printerMark.Flags = annotation.FlagPrint | annotation.FlagReadOnly

	// Extract printer's mark-specific fields
	// MN (optional)
	if mn, err := pdf.Optional(c.Name(dict["MN"])); err != nil {
		return nil, err
	} else if mn != "" {
		printerMark.MN = mn
	}

	return printerMark, nil
}
