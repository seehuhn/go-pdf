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

package extract_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestType0RoundTrip(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for i, d := range t0Dicts {
			t.Run(fmt.Sprintf("D%dv%s-%s", i, v, d.PostScriptName), func(t *testing.T) {
				checkRoundTripT0(t, d, v)
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
			if err := memfile.AddBlankPage(w); err != nil {
				f.Fatal(err)
			}
			rm := pdf.NewResourceManager(w)

			d := clone(d)
			if d.FontFile != nil {
				fontRef := w.Alloc()
				// write a fake font data stream
				var subtype pdf.Object
				switch d.FontFile.Type {
				case glyphdata.CFF:
					subtype = pdf.Name("CIDFontType0C")
				case glyphdata.OpenTypeCFF:
					subtype = pdf.Name("OpenType")
				}
				stm, err := w.OpenStream(fontRef, pdf.Dict{"Subtype": subtype})
				if err != nil {
					f.Fatal(err)
				}
				err = stm.Close()
				if err != nil {
					f.Fatal(err)
				}
				// Keep FontFile but simplify WriteTo for test
				d.FontFile = &glyphdata.Stream{
					Type: d.FontFile.Type,
					WriteTo: func(w io.Writer, length *glyphdata.Lengths) error {
						return nil // test stub
					},
				}
			}
			fontDictRef, err := rm.Embed(d)
			if err != nil {
				f.Fatal(err)
			}
			err = rm.Close()
			if err != nil {
				f.Fatal(err)
			}

			w.GetMeta().Trailer["Quir:E"] = fontDictRef

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
		r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)), opt)
		if err != nil {
			t.Skip("broken PDF: " + err.Error())
		}
		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			pdf.Format(os.Stdout, pdf.OptPretty, r.GetMeta().Trailer)
			t.Skip("broken reference")
		}
		x := pdf.NewExtractor(r)
		dictAny, err := extract.Dict(pdf.CursorAt(x, nil), obj, false)
		if err != nil {
			t.Skip("no valid CIDFontType0 dict")
		}
		d, ok := dictAny.(*dict.CIDFontType0)
		if !ok {
			t.Skip("not a CIDFontType0 font")
		}

		// Make sure we can write the dict, and read it back.
		checkRoundTripT0(t, d, pdf.GetVersion(r))
	})
}

func checkRoundTripT0(t *testing.T, d1 *dict.CIDFontType0, v pdf.Version) {
	d1 = clone(d1)

	w, _ := memfile.NewPDFWriter(t, v, nil)
	rm := pdf.NewResourceManager(w)

	// == Write ==

	if d1.FontFile != nil {
		fontRef := w.Alloc()
		// write a fake font data stream
		var subtype pdf.Object
		switch d1.FontFile.Type {
		case glyphdata.CFF:
			subtype = pdf.Name("CIDFontType0C")
		case glyphdata.OpenTypeCFF:
			subtype = pdf.Name("OpenType")
		}
		stm, err := w.OpenStream(fontRef, pdf.Dict{"Subtype": subtype})
		if err != nil {
			t.Fatal(err)
		}
		err = stm.Close()
		if err != nil {
			t.Fatal(err)
		}
		// Keep FontFile but simplify WriteTo for test
		d1.FontFile = &glyphdata.Stream{
			Type: d1.FontFile.Type,
			WriteTo: func(w io.Writer, length *glyphdata.Lengths) error {
				return nil // test stub
			},
		}
	}
	fontDictRef, err := rm.Embed(d1)
	if err != nil {
		if pdf.IsWrongVersion(err) {
			t.Skip("version not supported")
		}
		t.Fatal(err)
	}
	err = rm.Close()
	if err != nil {
		t.Fatal(err)
	}

	// == Read ==

	x := pdf.NewExtractor(w)
	dictAny, err := extract.Dict(pdf.CursorAt(x, nil), fontDictRef, false)
	if err != nil {
		t.Fatal(err)
	}
	d2 := dictAny.(*dict.CIDFontType0)

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

var t0Dicts = []*dict.CIDFontType0{
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
		Width: map[cid.CID]float64{
			0: 1000,
			1: 500,
		},
		DefaultWidth: 750,
		VMetrics: map[cid.CID]dict.VMetrics{
			0: {OffsY: 800, DeltaY: -1000},
			1: {OffsY: 880, DeltaY: -900},
			2: {OffsY: 880, DeltaY: -900},
		},
		DefaultVMetrics: dict.DefaultVMetrics{OffsY: 800, DeltaY: -800},
		ToUnicode: &cmap.ToUnicodeFile{
			CodeSpaceRange: charcode.Simple,
			Singles: []cmap.ToUnicodeSingle{
				{
					Code:  []byte{' '},
					Value: " ",
				},
			},
		},
		FontFile: nil, // external font
	},
	{
		PostScriptName: "Test",
		SubsetTag:      "ABCDEF",
		Descriptor: &font.Descriptor{
			FontName: "ABCDEF+Test",
		},
		ROS:  ros,
		CMap: func() *cmap.File { c, _ := cmap.Predefined("Identity-H"); return c }(),
		Width: map[cid.CID]float64{
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
		FontFile: &glyphdata.Stream{
			Type: glyphdata.OpenTypeCFF,
			WriteTo: func(w io.Writer, length *glyphdata.Lengths) error {
				return nil // test stub
			},
		},
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
		Width: map[cid.CID]float64{
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
		FontFile:     nil, // No font data needed for this test
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
		FontFile:     nil, // external font
	},
}

// TestType0EncodingProvenance regresses a bug where a composite font's
// indirect /Encoding CMap lost its provenance on decode, so re-embedding
// the font wrote a second copy of the CMap stream instead of reusing the
// original object.
func TestType0EncodingProvenance(t *testing.T) {
	w0, f := memfile.NewPDFWriter(t, pdf.V1_7, &pdf.WriterOptions{HumanReadable: true})

	cm := &cmap.File{
		Name:           "Test-cmap",
		ROS:            ros,
		CodeSpaceRange: charcode.Simple,
		CIDSingles: []cmap.Single{
			{Code: []byte{' '}, Value: 1},
		},
	}
	rm0 := pdf.NewResourceManager(w0)
	cmapRef, err := rm0.Embed(cm)
	if err != nil {
		t.Fatal(err)
	}
	if err := rm0.Close(); err != nil {
		t.Fatal(err)
	}

	cidFontDict := pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("CIDFontType0"),
		"BaseFont": pdf.Name("Test"),
		"CIDSystemInfo": pdf.Dict{
			"Registry":   pdf.String(ros.Registry),
			"Ordering":   pdf.String(ros.Ordering),
			"Supplement": pdf.Integer(ros.Supplement),
		},
	}
	cidFontRef := w0.Alloc()
	if err := w0.Put(cidFontRef, cidFontDict); err != nil {
		t.Fatal(err)
	}

	fontDict := pdf.Dict{
		"Type":            pdf.Name("Font"),
		"Subtype":         pdf.Name("Type0"),
		"BaseFont":        pdf.Name("Test"),
		"Encoding":        cmapRef,
		"DescendantFonts": pdf.Array{cidFontRef},
	}
	fontRef := w0.Alloc()
	if err := w0.Put(fontRef, fontDict); err != nil {
		t.Fatal(err)
	}
	if err := w0.Close(); err != nil {
		t.Fatal(err)
	}

	orig := bytes.Clone(f.Data)
	w, err := pdf.NewUpdater(f, int64(len(orig)), nil)
	if err != nil {
		t.Fatal(err)
	}
	c := pdf.NewCursor(w)
	dictAny, err := pdf.Decode(c, fontRef, extract.Dict)
	if err != nil {
		t.Fatal(err)
	}
	d2, ok := dictAny.(*dict.CIDFontType0)
	if !ok {
		t.Fatalf("decoded %T, want *dict.CIDFontType0", dictAny)
	}
	if got := w.Origin(d2.CMap); got == 0 {
		t.Fatal("decoded CMap has no provenance")
	}

	// force the parent's Embed method to actually run, by embedding a copy
	// with no provenance entry of its own; the CMap it shares with d2 keeps
	// whatever provenance the fix under test does or does not give it
	cp := *d2
	rm := pdf.NewResourceManager(w)
	if _, err := rm.Embed(&cp); err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	if n := bytes.Count(f.Data[len(orig):], []byte("/Test-cmap")); n != 0 {
		t.Errorf("CMap written again: found %d copies in the update", n)
	}
}
