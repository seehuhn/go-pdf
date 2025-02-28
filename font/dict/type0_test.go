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
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestType0RoundTrip(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for i, d := range type0Dicts {
			t.Run(fmt.Sprintf("D%dv%s-%s", i, v, d.PostScriptName), func(t *testing.T) {
				w, buf := memfile.NewPDFWriter(pdf.V2_0, nil)
				rm := pdf.NewResourceManager(w)
				ref := w.Alloc()

				// == Write ==

				d1 := clone(d)
				d1.Ref = ref

				// If font data is included, write an empty stream with the
				// correct subtype.
				switch d1.FontType {
				case glyphdata.CFF:
					fontRef := w.Alloc()
					d1.FontRef = fontRef
					dict := pdf.Dict{
						"Subtype": pdf.Name("CIDFontType0C"),
					}
					stm, err := w.OpenStream(fontRef, dict)
					if err != nil {
						t.Fatal(err)
					}
					err = stm.Close()
					if err != nil {
						t.Fatal(err)
					}
				case glyphdata.OpenTypeCFF:
					fontRef := w.Alloc()
					d1.FontRef = fontRef
					dict := pdf.Dict{
						"Subtype": pdf.Name("OpenType"),
					}
					stm, err := w.OpenStream(fontRef, dict)
					if err != nil {
						t.Fatal(err)
					}
					err = stm.Close()
					if err != nil {
						t.Fatal(err)
					}
				}

				err := d1.WriteToPDF(rm)
				if err != nil {
					t.Fatal(err)
				}
				err = rm.Close()
				if err != nil {
					t.Fatal(err)
				}
				err = w.Close()
				if err != nil {
					t.Fatal(err)
				}

				_ = buf
				// os.WriteFile("debug.pdf", buf.Data, 0666)

				// == Read ==

				d2, err := ExtractCIDFontType0(w, ref)
				if err != nil {
					t.Fatal(err)
				}

				// == Compare ==

				if d := cmp.Diff(d1, d2); d != "" {
					t.Errorf("diff: %s", d)
				}
			})
		}
	}
}

func FuzzType0Dict(f *testing.F) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for _, d := range type0Dicts {
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
			d.Ref = ref

			// If font data is included, write an empty stream with the
			// correct subtype.
			switch d.FontType {
			case glyphdata.CFF:
				fontRef := w.Alloc()
				d.FontRef = fontRef
				dict := pdf.Dict{
					"Subtype": pdf.Name("CIDFontType0C"),
				}
				stm, err := w.OpenStream(fontRef, dict)
				if err != nil {
					f.Fatal(err)
				}
				err = stm.Close()
				if err != nil {
					f.Fatal(err)
				}
			case glyphdata.OpenTypeCFF:
				fontRef := w.Alloc()
				d.FontRef = fontRef
				dict := pdf.Dict{
					"Subtype": pdf.Name("OpenType"),
				}
				stm, err := w.OpenStream(fontRef, dict)
				if err != nil {
					f.Fatal(err)
				}
				err = stm.Close()
				if err != nil {
					f.Fatal(err)
				}
			}

			err = d.WriteToPDF(rm)
			if err != nil {
				f.Fatal(err)
			}
			err = rm.Close()
			if err != nil {
				f.Fatal(err)
			}

			w.GetMeta().Trailer["Seeh:X"] = ref

			err = w.Close()
			if err != nil {
				f.Fatal(err)
			}

			f.Add(out.Data)
		}
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		// Get "random" Type1Dict from PDF.
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
		d1, err := ExtractCIDFontType0(r, obj)
		if err != nil {
			t.Skip("broken Type1Dict")
		}

		// Write the Type1Dict back to a new PDF file.
		// Make sure we can write arbitrary Type1Dicts.
		w, _ := memfile.NewPDFWriter(r.GetMeta().Version, nil)
		rm := pdf.NewResourceManager(w)
		ref := w.Alloc()

		d1.Ref = ref

		// If font data is included, write an empty stream with the
		// correct subtype.
		switch d1.FontType {
		case glyphdata.CFF:
			fontRef := w.Alloc()
			d1.FontRef = fontRef
			dict := pdf.Dict{
				"Subtype": pdf.Name("CIDFontType0C"),
			}
			stm, err := w.OpenStream(fontRef, dict)
			if err != nil {
				t.Fatal(err)
			}
			err = stm.Close()
			if err != nil {
				t.Fatal(err)
			}
		case glyphdata.OpenTypeCFF:
			fontRef := w.Alloc()
			d1.FontRef = fontRef
			dict := pdf.Dict{
				"Subtype": pdf.Name("OpenType"),
			}
			stm, err := w.OpenStream(fontRef, dict)
			if err != nil {
				t.Fatal(err)
			}
			err = stm.Close()
			if err != nil {
				t.Fatal(err)
			}
		}

		err = d1.WriteToPDF(rm)
		if err != nil {
			t.Fatal(err)
		}
		err = rm.Close()
		if err != nil {
			t.Fatal(err)
		}

		// Read back the data.
		// Make sure we get the same Type1Dict back.
		d2, err := ExtractCIDFontType0(w, ref)
		if err != nil {
			t.Fatal(err)
		}

		if d := cmp.Diff(d1, d2); d != "" {
			t.Fatal(d)
		}
	})
}

var ros = &cmap.CIDSystemInfo{
	Registry:   "Quire",
	Ordering:   "Test",
	Supplement: 2,
}

var type0Dicts = []*CIDFontType0{
	{
		PostScriptName: "Test",
		Descriptor:     &font.Descriptor{},
		ROS:            ros,
		Encoding: &cmap.File{
			Name:           "Test-cmap",
			ROS:            ros,
			WMode:          font.Vertical,
			CodeSpaceRange: charcode.Simple,
			CIDSingles: []cmap.Single{
				{
					Code:  []byte{' '},
					Value: 1,
				},
			},
		},
		Width: map[cmap.CID]float64{
			0: 1000,
			1: 500,
		},
		DefaultWidth: 750,
		Text: &cmap.ToUnicodeFile{
			CodeSpaceRange: charcode.Simple,
			Singles: []cmap.ToUnicodeSingle{
				{
					Code:  []byte{' '},
					Value: " ",
				},
			},
		},
		FontType: glyphdata.None,
	},
	{
		PostScriptName: "Test",
		SubsetTag:      "ABCDEF",
		Descriptor:     &font.Descriptor{},
		ROS:            ros,
		Encoding:       cmap.Predefined("Identity-H"),
		Width: map[cmap.CID]float64{
			0: 1000,
			1: 500,
		},
		DefaultWidth: 750,
		Text: &cmap.ToUnicodeFile{
			CodeSpaceRange: charcode.Simple,
			Singles: []cmap.ToUnicodeSingle{
				{
					Code:  []byte{'A'},
					Value: "A",
				},
			},
		},
		FontType: glyphdata.OpenTypeCFF,
	},
	{
		PostScriptName: "Test",
		Descriptor:     &font.Descriptor{},
		ROS:            ros,
		Encoding: &cmap.File{
			Name:           "Test-cmap",
			ROS:            ros,
			WMode:          font.Vertical,
			CodeSpaceRange: charcode.Simple,
			CIDRanges: []cmap.Range{
				{
					First: []byte{'0'},
					Last:  []byte{'9'},
					Value: 1,
				},
				{
					First: []byte{'A'},
					Last:  []byte{'Z'},
					Value: 11,
				},
			},
		},
		Width: map[cmap.CID]float64{
			0:  1000,
			1:  100,
			2:  200,
			3:  300,
			4:  400,
			5:  500,
			6:  600,
			7:  700,
			8:  800,
			10: 1000,
			11: 1100,
			12: 1200,
		},
		DefaultWidth: 900,
		FontType:     glyphdata.CFF,
	},
}
