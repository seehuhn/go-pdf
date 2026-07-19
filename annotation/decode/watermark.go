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

func decodeWatermark(c pdf.Cursor, dict pdf.Dict) (*annotation.Watermark, error) {
	watermark := &annotation.Watermark{}

	if err := decodeCommon(c, &watermark.Common, dict); err != nil {
		return nil, err
	}

	// FixedPrint (optional)
	if fp, err := pdf.DecodeOptional(c, dict["FixedPrint"], annotation.ExtractFixedPrint); err != nil {
		return nil, err
	} else {
		watermark.FixedPrint = fp
	}

	return watermark, nil
}
