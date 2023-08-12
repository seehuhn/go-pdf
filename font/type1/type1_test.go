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

package type1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/gofont"
)

func TestRoundTrip(t *testing.T) {
	t1, err := gofont.Type1(gofont.GoRegular)
	if err != nil {
		t.Fatal(err)
	}

	encoding := make([]string, 256)
	for i := range encoding {
		encoding[i] = ".notdef"
	}
	encoding[65] = "A"
	encoding[66] = "B"

	toUnicode := map[charcode.CharCode][]rune{
		65: {'A'},
		66: {'B'},
	}

	info := &EmbedInfo{
		PSFont:    t1,
		SubsetTag: "UVWXYZ",
		Encoding:  encoding,
		ToUnicode: toUnicode,
	}

	rw := pdf.NewData(pdf.V1_7)
	ref := rw.Alloc()
	err = info.Embed(rw, ref)
	if err != nil {
		t.Fatal(err)
	}

	info2, err := ExtractEmbedInfo(rw, ref)
	if err != nil {
		t.Fatal(err)
	}

	if d := cmp.Diff(info, info2); d != "" {
		t.Errorf("info mismatch (-want +got):\n%s", d)
	}
}
