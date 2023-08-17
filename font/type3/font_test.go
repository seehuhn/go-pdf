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

package type3_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/gofont"
	"seehuhn.de/go/pdf/font/type3"
)

func TestRoundTrip(t *testing.T) {
	t3, err := gofont.Type3(gofont.GoItalic)
	if err != nil {
		t.Fatal(err)
	}

	encoding := make([]string, 256)
	encoding[65] = "A"
	encoding[66] = "C"

	toUnicode := map[charcode.CharCode][]rune{
		65: {'A'},
		66: {'C'},
	}

	descriptor := &font.Descriptor{
		IsFixedPitch: t3.IsFixedPitch,
		IsSerif:      t3.IsSerif,
		IsScript:     t3.IsScript,
		IsItalic:     t3.IsItalic,
		IsAllCap:     t3.IsAllCap,
		IsSmallCap:   t3.IsSmallCap,
		ForceBold:    t3.ForceBold,
		ItalicAngle:  t3.ItalicAngle,
		StemV:        -1,
	}

	info1 := &type3.EmbedInfo{
		Glyphs:     t3.Glyphs,
		FontMatrix: t3.FontMatrix,
		Resources:  t3.Resources,
		Encoding:   encoding,
		ToUnicode:  toUnicode,
		Descriptor: descriptor,
	}

	rw := pdf.NewData(pdf.V1_7)
	ref := rw.Alloc()
	err = info1.Embed(rw, ref)
	if err != nil {
		t.Fatal(err)
	}

	dicts, err := font.ExtractDicts(rw, ref)
	if err != nil {
		t.Fatal(err)
	}
	info2, err := type3.Extract(rw, dicts)
	if err != nil {
		t.Fatal(err)
	}

	if d := cmp.Diff(info1, info2); d != "" {
		t.Errorf("info mismatch (-want +got):\n%s", d)
	}
}
