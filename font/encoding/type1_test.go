// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package encoding

import (
	"testing"

	"seehuhn.de/go/pdf/font/pdfenc"
)

func TestDescribeEncoding(t *testing.T) {
	funnyEncoding := make([]string, 256)
	for i := range funnyEncoding {
		funnyEncoding[i] = ".notdef"
	}
	funnyEncoding[0o001] = "funny"  // non-standard name
	funnyEncoding[0o101] = "A"      // common to all encodings
	funnyEncoding[0o102] = "C"      // clashes with all encodings
	funnyEncoding[0o103] = "B"      // clashes with all encodings
	funnyEncoding[0o104] = "D"      // common to all encodings
	funnyEncoding[0o142] = "Bsmall" // only in MacExpertEncoding
	funnyEncoding[0o201] = "A"      // double encode some characters
	funnyEncoding[0o202] = "B"      // double encode some characters
	funnyEncoding[0o203] = "C"      // double encode some characters
	funnyEncoding[0o204] = "D"      // double encode some characters
	funnyEncoding[0o214] = "OE"     // only in WinAnsiEncoding
	funnyEncoding[0o227] = "Scaron" // only in PdfDocEncoding
	funnyEncoding[0o341] = "AE"     // only in StandardEncoding
	funnyEncoding[0o347] = "Aacute" // only in MacRomanEncoding

	encodings := [][]string{
		pdfenc.Standard.Encoding[:],
		pdfenc.MacRoman.Encoding[:],
		pdfenc.MacExpert.Encoding[:],
		pdfenc.WinAnsi.Encoding[:],
		pdfenc.PDFDoc.Encoding[:],
		funnyEncoding,
	}

	for i, enc := range encodings {
		for j, builtin := range encodings {
			desc := DescribeEncodingType1(enc, builtin)
			if i == j {
				if desc != nil {
					t.Errorf("DescribeEncoding(%d, %d) = %v", i, j, desc)
				}
			}

			enc2, err := UndescribeEncodingType1(nil, desc, builtin)
			if err != nil {
				t.Error(err)
				continue
			}

			for c, name := range enc {
				if name == ".notdef" {
					continue
				}
				if enc2[c] != name {
					t.Errorf("UndescribeEncoding(%d, %d) = %v", i, j, enc2)
					break
				}
			}
		}
	}
}
