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

package main

import (
	"fmt"
	"testing"

	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/postscript/type1/names"
)

func TestEncodings(t *testing.T) {
	for _, code := range markerString {
		name := pdfenc.WinAnsiEncoding[code]
		rr := names.ToUnicode(name, false)
		if len(rr) != 1 {
			t.Errorf("expected 1 rune for %s, got %d", name, len(rr))
			continue
		}
		fmt.Printf("%d -> %s -> 0x%04x %d %q\n", code, name, rr[0], rr[0], rr[0])
	}
}
