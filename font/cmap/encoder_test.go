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

package cmap

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
)

func TestUTF8(t *testing.T) {
	// verify that the encoding equals standard UTF-8 encoding

	enc := NewCIDEncoderUTF8(NewGIDToCIDIdentity())

	var buf pdf.String
	for r := rune(0); r <= 0x10_FFFF; r++ {
		if r >= 0xD800 && r <= 0xDFFF || r == 0xFFFD {
			continue
		}

		buf = enc.AppendEncoded(buf[:0], 0, []rune{r})
		buf2 := []byte(string(r))
		if !bytes.Equal(buf, buf2) {
			t.Fatalf("AppendEncoded(0x%04x) = %v, want %v", r, buf, buf2)
		}
	}
}
