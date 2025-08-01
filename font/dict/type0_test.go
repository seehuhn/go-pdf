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

	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/postscript/cid"

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
		for i, d := range t0Dicts {
			t.Run(fmt.Sprintf("D%dv%s-%s", i, v, d.PostScriptName), func(t *testing.T) {
				checkRoundtripT0(t, d, v)
			})
		}
	}
}

func FuzzType0Dict(f *testing.F) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for _, d := range t0Dicts {
			out := memfile.New()
			opt := &pdf.WriterOptions{
				HumanReadable: true,
			}
			w, err := pdf.NewWriter(out, v, opt)
			if err != nil {
				f.Fatal(err)
			}
			rm := pdf.NewResourceManager(w)
			fontDictRef := w.Alloc()

			d := clone(d)
			if d.FontRef != 0 {
				d.FontRef = w.Alloc()
				// write a fake font data stream
				var subtype pdf.Object
				switch d.FontType {
				case glyphdata.CFF:
					subtype = pdf.Name("CIDFontType0C")
				case glyphdata.OpenTypeCFF:
					subtype = pdf.Name("OpenType")
				}
				stm, err := w.OpenStream(d.FontRef, pdf.Dict{"Subtype": subtype})
				if err != nil {
					f.Fatal(err)
				}
				err = stm.Close()
				if err != nil {
					f.Fatal(err)
				}
			}
			err = d.WriteToPDF(rm, fontDictRef)
			if err != nil {
				f.Fatal(err)
			}
			err = rm.Close()
			if err != nil {
				f.Fatal(err)
			}

			w.GetMeta().Trailer["Quir:X"] = fontDictRef

			err = w.Close()
			if err != nil {
				f.Fatal(err)
			}

			f.Add(out.Data)
		}
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		// Get a "random" Type1Dict from the PDF file.

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
		d, err := ReadCIDFontType0(r, obj)
		if err != nil {
			t.Skip("no valid CIDFontType0 dict")
		}

		// We need at version 1.6, in case an OpenType font is used.
		v := max(pdf.GetVersion(r), pdf.V1_6)

		// Make sure we can write the dict, and read it back.
		checkRoundtripT0(t, d, v)
	})
}

func checkRoundtripT0(t *testing.T, d1 *CIDFontType0, v pdf.Version) {
	d1 = clone(d1)

	w, _ := memfile.NewPDFWriter(v, nil)
	rm := pdf.NewResourceManager(w)
	fontDictRef := w.Alloc()

	// == Write ==

	if d1.FontRef != 0 {
		d1.FontRef = w.Alloc()
		// write a fake font data stream
		var subtype pdf.Object
		switch d1.FontType {
		case glyphdata.CFF:
			subtype = pdf.Name("CIDFontType0C")
		case glyphdata.OpenTypeCFF:
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
	err := d1.WriteToPDF(rm, fontDictRef)
	if err != nil {
		t.Fatal(err)
	}
	err = rm.Close()
	if err != nil {
		t.Fatal(err)
	}

	// == Read ==

	d2, err := ReadCIDFontType0(w, fontDictRef)
	if err != nil {
		t.Fatal(err)
	}

	// == Compare ==

	if d := cmp.Diff(d1, d2); d != "" {
		t.Fatal(d)
	}
}

var ros = &cid.SystemInfo{
	Registry:   "Quire",
	Ordering:   "Test",
	Supplement: 2,
}

var t0Dicts = []*CIDFontType0{
	{
		PostScriptName: "Test",
		Descriptor: &font.Descriptor{
			FontName: "Test",
		},
		ROS: ros,
		CMap: &cmap.File{
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
		VMetrics: map[cmap.CID]VMetrics{
			0: {OffsY: 800, DeltaY: -1000},
			1: {OffsY: 880, DeltaY: -900},
			2: {OffsY: 880, DeltaY: -900},
		},
		DefaultVMetrics: DefaultVMetrics{800, -800},
		ToUnicode: &cmap.ToUnicodeFile{
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
		Descriptor: &font.Descriptor{
			FontName: "ABCDEF+Test",
		},
		ROS:  ros,
		CMap: func() *cmap.File { c, _ := cmap.Predefined("Identity-H"); return c }(),
		Width: map[cmap.CID]float64{
			0: 1000,
			1: 500,
		},
		DefaultWidth: 750,
		ToUnicode: &cmap.ToUnicodeFile{
			CodeSpaceRange: charcode.Simple,
			Singles: []cmap.ToUnicodeSingle{
				{
					Code:  []byte{'A'},
					Value: "A",
				},
			},
		},
		FontType: glyphdata.OpenTypeCFF,
		FontRef:  pdf.NewReference(999, 0),
	},
	{
		PostScriptName: "Test",
		Descriptor: &font.Descriptor{
			FontName: "Test",
		},
		ROS: ros,
		CMap: &cmap.File{
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
		FontRef:      pdf.NewReference(999, 0),
	},
	{
		PostScriptName: "Nil-Width",
		Descriptor: &font.Descriptor{
			FontName:     "Nil-Width",
			IsFixedPitch: true,
			FontBBox: rect.Rect{
				LLx: 0,
				LLy: 0,
				URx: 1000,
				URy: 1000,
			},
			Ascent:    1000,
			Descent:   0,
			CapHeight: 1000,
		},
		ROS:          ros,
		CMap:         func() *cmap.File { c, _ := cmap.Predefined("Identity-H"); return c }(),
		DefaultWidth: 1000,
		FontType:     glyphdata.None,
	},
}
