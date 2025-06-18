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
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/sfnt/os2"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestTrueTypeRoundtrip(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for i, d := range ttDicts {
			if v >= pdf.V2_0 && d.Name != "" {
				continue
			}

			t.Run(fmt.Sprintf("D%dv%s-%s", i, v, d.PostScriptName), func(t *testing.T) {
				checkRoundtripTT(t, d, v)
			})
		}
	}
}

func FuzzTrueTypeDict(f *testing.F) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for _, d := range ttDicts {
			if v >= pdf.V2_0 && d.Name != "" {
				continue
			}

			out := memfile.New()
			opt := &pdf.WriterOptions{
				HumanReadable: true,
			}
			w, err := pdf.NewWriter(out, v, opt)
			if err != nil {
				f.Fatal(err)
			}
			rm := pdf.NewResourceManager(w)

			ref := w.Alloc()
			d := clone(d)

			err = d.WriteToPDF(rm, ref)
			if err != nil {
				f.Fatal(err)
			}
			err = rm.Close()
			if err != nil {
				f.Fatal(err)
			}

			w.GetMeta().Trailer["Quir:X"] = ref

			err = w.Close()
			if err != nil {
				f.Fatal(err)
			}

			f.Add(out.Data)
		}
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		// Get a "random" TrueTypeDict from the PDF file.

		// Make sure we don't panic on random input.
		opt := &pdf.ReaderOptions{
			ErrorHandling: pdf.ErrorHandlingReport,
		}
		r, err := pdf.NewReader(bytes.NewReader(fileData), opt)
		if err != nil {
			t.Skip("broken PDF: " + err.Error())
		}
		obj := r.GetMeta().Trailer["Quir:X"]
		if obj == nil {
			pdf.Format(os.Stdout, pdf.OptPretty, r.GetMeta().Trailer)
			t.Skip("broken reference")
		}
		d, err := ReadTrueType(r, obj)
		if err != nil {
			t.Skip("broken TrueTypeDict")
		}

		// Make sure we can write the dict, and read it back.
		checkRoundtripTT(t, d, pdf.GetVersion(r))
	})
}

func checkRoundtripTT(t *testing.T, d1 *TrueType, v pdf.Version) {
	d1 = clone(d1)

	w, _ := memfile.NewPDFWriter(v, nil)
	rm := pdf.NewResourceManager(w)

	// == Write ==

	ref := w.Alloc()
	if d1.FontRef != 0 {
		d1.FontRef = w.Alloc()
		// write a fake font data stream
		var subtype pdf.Object
		switch d1.FontType {
		case glyphdata.OpenTypeGlyf:
			subtype = pdf.Name("OpenType")
		}
		stm, err := w.OpenStream(d1.FontRef, pdf.Dict{"Subtype": subtype})
		if err != nil {
			t.Fatal(err)
		}
		err = stm.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	err := d1.WriteToPDF(rm, ref)
	if err != nil {
		t.Fatal(err)
	}
	err = rm.Close()
	if err != nil {
		t.Fatal(err)
	}

	// == Read ==

	d2, err := ReadTrueType(w, ref)
	if err != nil {
		t.Fatal(err)
	}

	// == Compare ==

	// Text and width for unused codes are arbitrary after roundtrip. We
	// compare these manually here, and zero the values before the comparison
	// below.
	text1 := simpleTextMap(d1.PostScriptName, d1.Encoding, d1.ToUnicode)
	text2 := simpleTextMap(d2.PostScriptName, d2.Encoding, d2.ToUnicode)
	for i := range 256 {
		code := byte(i)
		if d1.Encoding(code) != "" {
			if d1.Encoding(code) != d2.Encoding(code) {
				t.Errorf("glyphName[%d]: %q != %q", code, d1.Encoding(code), d2.Encoding(code))
			}
			if text1[code] != text2[code] {
				t.Errorf("text[%d]: %q != %q", code, text1[code], text2[code])
			}
			if d1.Width[code] != d2.Width[code] {
				t.Errorf("width[%d]: %f != %f", code, d1.Width[code], d2.Width[code])
			}
		}

		d1.Width[code] = 0
		d2.Width[code] = 0
	}
	d1.Encoding = nil
	d2.Encoding = nil

	if d := cmp.Diff(d1, d2); d != "" {
		t.Fatal(d)
	}
}

var ttDicts = []*TrueType{
	{
		PostScriptName: "Test",
		Descriptor: &font.Descriptor{
			FontName:     "Test",
			IsFixedPitch: true,
			IsSerif:      false,
			IsSymbolic:   true,
			IsScript:     false,
			IsItalic:     true,
			IsAllCap:     false,
			IsSmallCap:   true,
			ForceBold:    false,
			FontBBox: rect.Rect{
				LLx: 0,
				LLy: -100,
				URx: 200,
				URy: 300,
			},
			ItalicAngle: 10,
			Ascent:      250,
			Descent:     -50,
			Leading:     450,
			CapHeight:   150,
			XHeight:     50,
			StemV:       75,
			StemH:       25,
		},
		Encoding: func(code byte) string {
			switch code {
			case 65:
				return "A"
			default:
				return ""
			}
		},
		Width: makeTestWidth(65, 100.0),
	},
	{
		PostScriptName: "Troubadour-Bold",
		SubsetTag:      "ABCDEF",
		Name:           "Q",
		Descriptor: &font.Descriptor{
			FontName:     "ABCDEF+Troubadour-Bold",
			FontFamily:   "Toast",
			FontStretch:  os2.WidthCondensed,
			FontWeight:   os2.WeightBold,
			IsFixedPitch: true,
			FontBBox:     rect.Rect{LLx: 10, LLy: 20, URx: 30, URy: 40},
			MissingWidth: 666,
		},
		Encoding: func(c byte) string { return pdfenc.Standard.Encoding[c] },
		Width:    makeConstWidth(666),
		FontType: glyphdata.OpenTypeGlyf,
		FontRef:  pdf.NewReference(999, 0),
	},
}
