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

package dict

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
)

// ExtractFont extracts a font from a PDF file as an immutable font object.
// This combines dict.Read with MakeFont() for convenience.
func ExtractFont(x *pdf.Extractor, obj pdf.Object) (font.Instance, error) {
	dict, err := Read(x, obj)
	if err != nil {
		return nil, err
	}
	return dict.MakeFont(), nil
}
