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

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/postscript/cid"
)

func TestType3Roundtrip(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for i, d := range type3Dicts {
			t.Run(fmt.Sprintf("D%dv%s-%s", i, v, d.Name), func(t *testing.T) {
				w, _ := memfile.NewPDFWriter(v, nil)
				rm := pdf.NewResourceManager(w)
				fontDictRef := w.Alloc()

				// == Write ==

				d1 := clone(d)

				err := d1.WriteToPDF(rm, fontDictRef)
				if err != nil {
					t.Fatal(err)
				}
				err = rm.Close()
				if err != nil {
					t.Fatal(err)
				}

				// == Read ==

				d2, err := ExtractType3(w, fontDictRef)
				if err != nil {
					t.Fatal(err)
				}

				// Text and glyph for unused codes are arbitrary after roundtrip.
				// We compare these manually here, and zero the values for the comparison
				// below.
				text1 := d1.TextMapping()
				text2 := d2.TextMapping()
				for code := range 256 {
					if d1.Encoding(byte(code)) != "" {
						if d1.Encoding(byte(code)) != d2.Encoding(byte(code)) {
							t.Errorf("glyphName[%d]: %q != %q", code, d1.Encoding(byte(code)), d2.Encoding(byte(code)))
						}
						cid := cid.CID(code) + 1
						if text1[cid] != text2[cid] {
							t.Errorf("text[%d]: %q != %q", code, text1[cid], text2[cid])
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
			})
		}
	}
}

func FuzzType3Dict(f *testing.F) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for _, d := range type3Dicts {
			out := memfile.New()
			opt := &pdf.WriterOptions{
				HumanReadable: true,
			}
			w, err := pdf.NewWriter(out, v, opt)
			if err != nil {
				f.Fatal(err)
			}

			fontDictRef := w.Alloc()
			d := clone(d)

			rm := pdf.NewResourceManager(w)
			err = d.WriteToPDF(rm, fontDictRef)
			if err != nil {
				f.Fatal(err)
			}
			err = rm.Close()
			if err != nil {
				f.Fatal(err)
			}

			w.GetMeta().Trailer["Seeh:X"] = fontDictRef

			err = w.Close()
			if err != nil {
				f.Fatal(err)
			}

			f.Add(out.Data)
		}
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		// Get "random" Type3Dict from PDF.
		// Make sure we don't panic on random input.
		opt := &pdf.ReaderOptions{
			ErrorHandling: pdf.ErrorHandlingReport,
		}
		r, err := pdf.NewReader(bytes.NewReader(fileData), opt)
		if err != nil {
			t.Skip("broken PDF: " + err.Error())
		}
		obj := r.GetMeta().Trailer["Seeh:X"]
		if obj == nil {
			pdf.Format(os.Stdout, pdf.OptPretty, r.GetMeta().Trailer)
			t.Skip("broken reference")
		}
		d1, err := ExtractType3(r, obj)
		if err != nil {
			t.Skip("broken Type3Dict")
		}

		// Write the Type3Dict back to a new PDF file.
		// Make sure we can write arbitrary Type3Dicts.
		w, _ := memfile.NewPDFWriter(r.GetMeta().Version, nil)
		rm := pdf.NewResourceManager(w)
		fontDictRef := w.Alloc()

		err = d1.WriteToPDF(rm, fontDictRef)
		if err != nil {
			t.Fatal(err)
		}
		err = rm.Close()
		if err != nil {
			t.Fatal(err)
		}

		// Read back the data.
		// Make sure we get the same Type3Dict back.
		d2, err := ExtractType3(w, fontDictRef)
		if err != nil {
			t.Fatal(err)
		}

		// Text and glyph for unused codes are arbitrary after roundtrip.
		// We compare these manually here, and zero the values for the comparison
		// below.
		text1 := d1.TextMapping()
		text2 := d2.TextMapping()
		for code := range 256 {
			if d1.Encoding(byte(code)) != "" {
				if d1.Encoding(byte(code)) != d2.Encoding(byte(code)) {
					t.Errorf("glyphName[%d]: %q != %q", code, d1.Encoding(byte(code)), d2.Encoding(byte(code)))
				}
				cid := cid.CID(code) + 1
				if text1[cid] != text2[cid] {
					t.Errorf("text[%d]: %q != %q", code, text1[cid], text2[cid])
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
	})
}

var type3Dicts = []*Type3{
	{
		Name: "Test1",
		Encoding: func(code byte) string {
			switch code {
			case 65:
				return "A"
			case 66:
				return "B"
			default:
				return ""
			}
		},
		Width: makeTestWidth(65, 100.0, 66, 100.0),
		CharProcs: map[pdf.Name]pdf.Reference{
			"A": pdf.NewReference(1, 0),
			"B": pdf.NewReference(2, 0),
		},
		FontMatrix: matrix.Scale(0.001, 0.001),
	},

	{
		Name: "Test2",
		Descriptor: &font.Descriptor{
			FontName:     "Test",
			IsFixedPitch: false,
			IsSerif:      true,
			IsSymbolic:   false,
			IsScript:     true,
			IsItalic:     false,
			IsAllCap:     true,
			IsSmallCap:   false,
			ForceBold:    true,
			ItalicAngle:  10,
			Ascent:       250,
			Descent:      -50,
			Leading:      450,
			CapHeight:    150,
			XHeight:      50,
			StemV:        75,
			StemH:        25,
		},
		Encoding: func(code byte) string {
			switch code {
			case 65:
				return "A"
			case 66:
				return "funny"
			case 67:
				return "C"
			case 68:
				return "D"
			case 69:
				return "E"
			case 70:
				return "F"
			default:
				return ""
			}
		},
		Width: makeTestWidth(65, 100.0, 66, 120.0, 67, 110.0, 68, 90.0, 69, 80.0, 70, 70.0),
		CharProcs: map[pdf.Name]pdf.Reference{
			"A":     pdf.NewReference(1, 0),
			"funny": pdf.NewReference(2, 0),
			"B":     pdf.NewReference(3, 0),
			"C":     pdf.NewReference(4, 0),
			"D":     pdf.NewReference(5, 0),
			"E":     pdf.NewReference(6, 0),
			"F":     pdf.NewReference(7, 0),
		},
		FontBBox:   &pdf.Rectangle{LLx: 0, LLy: -100, URx: 200, URy: 300},
		FontMatrix: matrix.Scale(0.001, 0.001),
		Resources: &pdf.Resources{
			Font: map[pdf.Name]pdf.Object{
				"F0": pdf.Name("Just for testing"),
			},
		},
	},
}
