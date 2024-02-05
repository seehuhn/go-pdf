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

package type1_test

import (
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/internal/testfont"
)

// TestEncoding checks that the encoding of a Type 1 font is the standard
// encoding, if the set of included characters is in the standard encoding.
func TestEncoding(t *testing.T) {
	t1, err := testfont.MakeType1()
	if err != nil {
		t.Fatal(err)
	}
	metrics, err := testfont.MakeAFM()
	if err != nil {
		t.Fatal(err)
	}
	F, err := type1.New(t1, metrics)
	if err != nil {
		t.Fatal(err)
	}

	// Embed the font
	data := pdf.NewData(pdf.V1_7)
	E, err := F.Embed(data, &font.Options{ResName: "F"})
	if err != nil {
		t.Fatal(err)
	}
	gg := E.Layout(10, ".MiAbc")
	for _, g := range gg.Seq {
		E.CodeAndWidth(nil, g.GID, g.Text) // allocate codes
	}
	err = E.Close()
	if err != nil {
		t.Fatal(err)
	}

	dicts, err := font.ExtractDicts(data, E.PDFObject())
	if err != nil {
		t.Fatal(err)
	}
	info, err := type1.Extract(data, dicts)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 256; i++ {
		if info.Encoding[i] != ".notdef" && info.Encoding[i] != pdfenc.StandardEncoding[i] {
			t.Error(i, info.Encoding[i])
		}
	}
}

func TestRoundTrip(t *testing.T) {
	t1, err := testfont.MakeType1()
	if err != nil {
		t.Fatal(err)
	}

	encoding := make([]string, 256)
	for i := range encoding {
		encoding[i] = ".notdef"
	}
	encoding[65] = "A"
	encoding[66] = "B"

	m := map[charcode.CharCode][]rune{
		65: {'A'},
		66: {'B'},
	}
	toUnicode := cmap.NewToUnicode(charcode.Simple, m)

	info1 := &type1.EmbedInfo{
		Font:      t1,
		SubsetTag: "UVWXYZ",
		Encoding:  encoding,
		ToUnicode: toUnicode,
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
	info2, err := type1.Extract(rw, dicts)
	if err != nil {
		t.Fatal(err)
	}

	// Compare encodings:
	if len(info1.Encoding) != len(info2.Encoding) {
		t.Fatalf("len(info1.Encoding) != len(info2.Encoding): %d != %d", len(info1.Encoding), len(info2.Encoding))
	}
	for i := range info1.Encoding {
		if info1.Encoding[i] != ".notdef" && info1.Encoding[i] != info2.Encoding[i] {
			t.Fatalf("info1.Encoding[%d] != info2.Encoding[%d]: %q != %q", i, i, info1.Encoding[i], info2.Encoding[i])
		}
	}

	for _, info := range []*type1.EmbedInfo{info1, info2} {
		info.Encoding = nil // already compared above
		info.Metrics = nil  // TODO(voss): enable this once it works
	}

	cmpFloat := cmp.Comparer(func(x, y float64) bool {
		return math.Abs(x-y) < 1/65536.
	})
	if d := cmp.Diff(info1, info2, cmpFloat); d != "" {
		t.Errorf("info mismatch (-want +got):\n%s", d)
	}
}
