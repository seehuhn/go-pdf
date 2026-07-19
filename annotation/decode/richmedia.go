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

func decodeRichMedia(c pdf.Cursor, dict pdf.Dict) (*annotation.RichMedia, error) {
	richMedia := &annotation.RichMedia{}

	if err := decodeCommon(c, &richMedia.Common, dict); err != nil {
		return nil, err
	}

	// RichMediaContent (required)
	if content, ok := dict["RichMediaContent"].(pdf.Reference); ok {
		richMedia.RichMediaContent = content
	}

	// RichMediaSettings (optional)
	if settings, ok := dict["RichMediaSettings"].(pdf.Reference); ok {
		richMedia.RichMediaSettings = settings
	}

	return richMedia, nil
}
