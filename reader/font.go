// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package reader

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"

	"seehuhn.de/go/pdf/font/dict"
	_ "seehuhn.de/go/pdf/font/dict" // import all font readers
)

// ReadFont extracts a font from a PDF file.
func (r *Reader) ReadFont(ref pdf.Object) (F font.FromFile, err error) {
	if ref, ok := ref.(pdf.Reference); ok {
		if res, ok := r.fontCache[ref]; ok {
			return res, nil
		}
		defer func() {
			if err == nil {
				r.fontCache[ref] = F
			}
		}()
	}

	dict, err := dict.Read(r.R, ref)
	if err != nil {
		return nil, err
	}
	return dict.MakeFont()
}
