// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package printermark

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/mock"
)

// separation returns a Separation colour space for the given colorant.
func separation(colorant pdf.Name) *color.SpaceSeparation {
	space, err := color.Separation(colorant, color.SpaceDeviceCMYK,
		&function.Type2{
			XMin: 0,
			XMax: 1,
			C0:   []float64{0, 0, 0, 0},
			C1:   []float64{0, 1, 0, 0},
			N:    1,
		})
	if err != nil {
		panic(err)
	}
	return space
}

var testCases = []struct {
	name  string
	attrs *Attributes
}{
	{
		name:  "mark style only",
		attrs: &Attributes{MarkStyle: "Registration target"},
	},
	{
		name: "single colorant",
		attrs: &Attributes{
			Colorants: map[pdf.Name]*color.SpaceSeparation{
				"Spot1": separation("Spot1"),
			},
		},
	},
	{
		name: "mark style and colorants",
		attrs: &Attributes{
			MarkStyle: "Colour bar",
			Colorants: map[pdf.Name]*color.SpaceSeparation{
				"Spot1": separation("Spot1"),
				"Spot2": separation("Spot2"),
			},
		},
	},
}

// carrier writes Attributes into a dictionary of its own, so that the entries
// can be round-tripped without building a complete form XObject.
type carrier struct {
	attrs *Attributes
}

func (c *carrier) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	dict := pdf.Dict{}
	if err := c.attrs.FillDict(e, dict); err != nil {
		return nil, err
	}
	ref := e.Alloc()
	if err := e.Out().Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}

func TestRoundTrip(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_4, pdf.V1_7, pdf.V2_0} {
		for _, tc := range testCases {
			t.Run(tc.name+"-"+v.String(), func(t *testing.T) {
				roundTripTest(t, v, tc.attrs)
			})
		}
	}
}

func roundTripTest(t *testing.T, version pdf.Version, original *Attributes) {
	t.Helper()

	buf, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(buf)

	embedded, err := rm.Embed(&carrier{attrs: original})
	if pdf.IsWrongVersion(err) {
		t.Skip()
	} else if err != nil {
		t.Fatal(err)
	}

	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := buf.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(buf)
	c := pdf.CursorAt(x, nil)
	dict, err := c.Dict(embedded)
	if err != nil {
		t.Fatal(err)
	}
	readAttrs, err := ExtractAttributes(c, dict)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(original, readAttrs); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func TestExtractAbsent(t *testing.T) {
	x := pdf.NewExtractor(mock.Getter)
	c := pdf.CursorAt(x, nil)

	attrs, err := ExtractAttributes(c, pdf.Dict{"BBox": pdf.Array{}})
	if err != nil {
		t.Fatal(err)
	}
	if attrs != nil {
		t.Errorf("expected nil attributes, got %+v", attrs)
	}
}

// TestExtractDropsUnusableColorants checks that colorant entries which cannot
// be written back unchanged are dropped on read.
func TestExtractDropsUnusableColorants(t *testing.T) {
	buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(buf)

	// a Separation space filed under a key which does not match its colorant
	wrongKey, err := rm.Embed(separation("Actual"))
	if err != nil {
		t.Fatal(err)
	}
	// a colour space which is not a Separation space
	notSeparation, err := rm.Embed(color.SpaceDeviceRGB)
	if err != nil {
		t.Fatal(err)
	}
	good, err := rm.Embed(separation("Spot1"))
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := buf.Close(); err != nil {
		t.Fatal(err)
	}

	dict := pdf.Dict{
		"Colorants": pdf.Dict{
			"Expected":  wrongKey,
			"NotSep":    notSeparation,
			"Rubbish":   pdf.Integer(42),
			"Spot1":     good,
			"AlsoWrong": pdf.Name("DeviceGray"),
		},
	}

	x := pdf.NewExtractor(buf)
	attrs, err := ExtractAttributes(pdf.CursorAt(x, nil), dict)
	if err != nil {
		t.Fatal(err)
	}
	if attrs == nil {
		t.Fatal("expected attributes")
	}
	if len(attrs.Colorants) != 1 {
		t.Fatalf("expected 1 colorant, got %d: %v", len(attrs.Colorants), attrs.Colorants)
	}
	if _, ok := attrs.Colorants["Spot1"]; !ok {
		t.Errorf("expected the Spot1 colorant to survive")
	}

	// what survives must be writable
	roundTripTest(t, pdf.V2_0, attrs)
}

func TestFillDictRejectsMismatchedColorant(t *testing.T) {
	buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(buf)

	attrs := &Attributes{
		Colorants: map[pdf.Name]*color.SpaceSeparation{
			"Expected": separation("Actual"),
		},
	}
	if _, err := rm.Embed(&carrier{attrs: attrs}); err == nil {
		t.Error("expected an error for a mismatched colorant name")
	}
}

// TestFillDictRejectsNilColorant checks that a colorant without a colour space
// is refused.  Such an entry has no PDF representation, and silently skipping
// it could turn a form into one which carries no printer's mark entries at all.
func TestFillDictRejectsNilColorant(t *testing.T) {
	buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(buf)

	attrs := &Attributes{
		Colorants: map[pdf.Name]*color.SpaceSeparation{"Spot1": nil},
	}
	if _, err := rm.Embed(&carrier{attrs: attrs}); err == nil {
		t.Error("expected an error for a colorant without a colour space")
	}
}

func TestFillDictVersion(t *testing.T) {
	buf, _ := memfile.NewPDFWriter(pdf.V1_3, nil)
	rm := pdf.NewResourceManager(buf)

	_, err := rm.Embed(&carrier{attrs: &Attributes{MarkStyle: "x"}})
	if !pdf.IsWrongVersion(err) {
		t.Errorf("expected a version error, got %v", err)
	}
}

func FuzzRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	for _, tc := range testCases {
		w, buf := memfile.NewPDFWriter(pdf.V2_0, opt)

		if err := memfile.AddBlankPage(w); err != nil {
			continue
		}

		rm := pdf.NewResourceManager(w)

		embedded, err := rm.Embed(&carrier{attrs: tc.attrs})
		if err != nil {
			continue
		}
		if err := rm.Close(); err != nil {
			continue
		}

		w.GetMeta().Trailer["Quir:E"] = embedded

		if err := w.Close(); err != nil {
			continue
		}

		f.Add(buf.Data)
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}

		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("missing object")
		}

		x := pdf.NewExtractor(r)
		c := pdf.CursorAt(x, nil)
		dict, err := c.Dict(obj)
		if err != nil || dict == nil {
			t.Skip("missing dictionary")
		}

		attrs, err := ExtractAttributes(c, dict)
		if err != nil {
			t.Skip("malformed printer's mark entries")
		}
		if attrs == nil {
			t.Skip("no printer's mark entries")
		}

		roundTripTest(t, pdf.GetVersion(r), attrs)
	})
}

// TestExtractEmptyIsAbsent checks that a dictionary whose printer's mark
// entries are all empty or unusable reads as absent.  Both entries are
// optional, so such a dictionary writes no entries at all, and reporting
// attributes here would break the read-write-read cycle.
func TestExtractEmptyIsAbsent(t *testing.T) {
	for _, tc := range []struct {
		name string
		dict pdf.Dict
	}{
		{
			name: "empty Colorants",
			dict: pdf.Dict{"Colorants": pdf.Dict{}},
		},
		{
			name: "unusable Colorants",
			dict: pdf.Dict{"Colorants": pdf.Dict{"Spot1": pdf.Integer(3)}},
		},
		{
			name: "empty MarkStyle",
			dict: pdf.Dict{"MarkStyle": pdf.TextString("")},
		},
		{
			name: "Colorants of the wrong type",
			dict: pdf.Dict{"Colorants": pdf.Integer(7)},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x := pdf.NewExtractor(mock.Getter)
			attrs, err := ExtractAttributes(pdf.CursorAt(x, nil), tc.dict)
			if err != nil {
				t.Fatal(err)
			}
			if attrs != nil {
				t.Errorf("expected nil attributes, got %+v", attrs)
			}
		})
	}
}
