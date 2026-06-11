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

func decodeCaret(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict) (*annotation.Caret, error) {
	caret := &annotation.Caret{}

	// Extract common annotation fields
	if err := decodeCommon(x, path, &caret.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(x, path, dict, &caret.Markup); err != nil {
		return nil, err
	}

	// Extract caret-specific fields
	// RD (optional)
	if rd, err := pdf.GetFloatArray(x.R, dict["RD"]); err == nil && len(rd) == 4 {
		caret.Margin = rd
	}

	// Sy (optional)
	if sy, err := x.GetName(path, dict["Sy"]); err == nil && sy != "" {
		caret.Symbol = sy
	}
	if caret.Symbol == "" {
		caret.Symbol = "None"
	}

	return caret, nil
}
