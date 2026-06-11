// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package acroform

import "seehuhn.de/go/pdf"

// shared array write helpers for the signature lock and seed value
// dictionaries. each writer omits the key for an empty slice.

func writeNameArray(dict pdf.Dict, key pdf.Name, vals []pdf.Name) {
	if len(vals) == 0 {
		return
	}
	arr := make(pdf.Array, len(vals))
	for i, v := range vals {
		arr[i] = v
	}
	dict[key] = arr
}

func writeTextStringArray(dict pdf.Dict, key pdf.Name, vals []string) {
	if len(vals) == 0 {
		return
	}
	arr := make(pdf.Array, len(vals))
	for i, s := range vals {
		arr[i] = pdf.TextString(s)
	}
	dict[key] = arr
}

func writeASCIIStringArray(dict pdf.Dict, key pdf.Name, vals []string) {
	if len(vals) == 0 {
		return
	}
	arr := make(pdf.Array, len(vals))
	for i, s := range vals {
		arr[i] = pdf.String(s)
	}
	dict[key] = arr
}

func writeByteStringArray(dict pdf.Dict, key pdf.Name, vals [][]byte) {
	if len(vals) == 0 {
		return
	}
	arr := make(pdf.Array, len(vals))
	for i, b := range vals {
		arr[i] = pdf.String(b)
	}
	dict[key] = arr
}
