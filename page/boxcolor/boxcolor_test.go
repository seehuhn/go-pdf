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

package boxcolor

import (
	"bytes"
	"errors"
	"maps"
	"math"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/mock"
)

var testCases = []struct {
	name    string
	version pdf.Version
	data    *Info
}{
	{
		name:    "empty",
		version: pdf.V1_7,
		data:    &Info{},
	},
	{
		name:    "crop_box",
		version: pdf.V1_7,
		data: &Info{
			CropBox: &Style{
				Color:     color.DeviceRGB{1, 0, 0},
				LineWidth: 1,
				Style:     StyleSolid,
			},
		},
	},
	{
		name:    "bleed_box",
		version: pdf.V1_7,
		data: &Info{
			BleedBox: &Style{
				Color:     color.DeviceRGB{0, 1, 0},
				LineWidth: 2,
				Style:     StyleSolid,
			},
		},
	},
	{
		name:    "trim_box_dashed",
		version: pdf.V1_7,
		data: &Info{
			TrimBox: &Style{
				LineWidth:   1,
				Style:       StyleDashed,
				DashPattern: defaultDashPattern,
			},
		},
	},
	{
		name:    "art_box_custom_dash",
		version: pdf.V1_7,
		data: &Info{
			ArtBox: &Style{
				Color:       color.DeviceRGB{0, 0, 1},
				LineWidth:   1.5,
				Style:       StyleDashed,
				DashPattern: []float64{4, 2},
			},
		},
	},
	{
		name:    "custom_width",
		version: pdf.V2_0,
		data: &Info{
			CropBox: &Style{
				LineWidth: 2.5,
				Style:     StyleSolid,
			},
		},
	},
	{
		name:    "all_boxes",
		version: pdf.V1_7,
		data: &Info{
			CropBox: &Style{
				Color:     color.DeviceRGB{1, 0, 0},
				LineWidth: 1,
				Style:     StyleSolid,
			},
			BleedBox: &Style{
				Color:     color.DeviceRGB{0, 1, 0},
				LineWidth: 2,
				Style:     StyleSolid,
			},
			TrimBox: &Style{
				Color:       color.DeviceRGB{0, 0, 1},
				LineWidth:   1,
				Style:       StyleDashed,
				DashPattern: defaultDashPattern,
			},
			ArtBox: &Style{
				Color:     color.DeviceRGB{1, 1, 0},
				LineWidth: 0.5,
				Style:     StyleSolid,
			},
		},
	},
	{
		name:    "full_custom_style",
		version: pdf.V1_7,
		data: &Info{
			CropBox: &Style{
				Color:       color.DeviceRGB{0.5, 0.5, 0.5},
				LineWidth:   3,
				Style:       StyleDashed,
				DashPattern: []float64{10, 5},
			},
		},
	},
	{
		name:    "single_use",
		version: pdf.V1_7,
		data: &Info{
			CropBox: &Style{
				Color:       color.DeviceRGB{1, 0, 0},
				LineWidth:   1,
				Style:       StyleDashed,
				DashPattern: defaultDashPattern,
				SingleUse:   true,
			},
			BleedBox: &Style{
				Color:     color.DeviceRGB{0, 0.5, 0},
				LineWidth: 2,
				Style:     StyleSolid,
				SingleUse: true,
			},
			SingleUse: true,
		},
	},
}

var testVersions = []pdf.Version{pdf.V1_7, pdf.V2_0}

func roundTripTest(t *testing.T, version pdf.Version, data *Info) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(t, version, nil)
	rm := pdf.NewResourceManager(w)

	embedded, err := rm.Embed(data)
	if err != nil {
		if pdf.IsWrongVersion(err) {
			t.Skip("version not supported")
		}
		t.Fatalf("embed failed: %v", err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("rm.Close failed: %v", err)
	}

	err = w.Close()
	if err != nil {
		t.Fatalf("w.Close failed: %v", err)
	}

	x := pdf.NewExtractor(w)
	decoded, err := pdf.CursorAt(x, nil).Decode(embedded, ExtractInfo)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	if diff := cmp.Diff(data, decoded); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func TestInfoRoundTrip(t *testing.T) {
	for _, v := range testVersions {
		t.Run(v.String(), func(t *testing.T) {
			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					roundTripTest(t, v, tc.data)
				})
			}
		})
	}
}

func FuzzInfoRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}

	for _, tc := range testCases {
		w, buf := memfile.NewPDFWriter(f, tc.version, opt)

		rm := pdf.NewResourceManager(w)

		embedded, err := rm.Embed(tc.data)
		if err != nil {
			continue
		}

		err = rm.Close()
		if err != nil {
			continue
		}

		w.GetMeta().Trailer["Quir:E"] = embedded
		err = w.Close()
		if err != nil {
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
			t.Skip("missing test object")
		}

		x := pdf.NewExtractor(r)
		data, err := pdf.CursorAt(x, nil).Decode(obj, ExtractInfo)
		if err != nil {
			t.Skip("malformed object")
		}
		if data == nil {
			t.Skip("nil object")
		}

		// reading accepts versions the writer cannot reproduce
		version := pdf.GetVersion(r)
		if !version.IsSupported() {
			t.Skip("version cannot be written")
		}

		roundTripTest(t, version, data)
	})
}

func TestStyleValidation(t *testing.T) {
	buf, _ := memfile.NewPDFWriter(t, pdf.V1_4, nil)
	rm := pdf.NewResourceManager(buf)

	// negative line width should fail
	style := &Style{
		LineWidth: -1,
	}

	_, err := rm.Embed(style)
	if err == nil {
		t.Error("expected error for negative line width, but got none")
	}

	// solid style with non-nil dash pattern should fail
	style = &Style{
		Style:       StyleSolid,
		DashPattern: []float64{3},
	}
	_, err = rm.Embed(style)
	if err == nil {
		t.Error("expected error for solid style with dash pattern, but got none")
	}
}

// TestStyleDashRules checks the rules of PDF 32000-2 8.4.3.6 for the /D entry
// of a box style dictionary: the lengths must be non-negative and must not
// all be zero.  Values which a PDF file can contain but the rules do not
// allow are repaired on read, so that they can be written back out again.
func TestStyleDashRules(t *testing.T) {
	t.Run("read", func(t *testing.T) {
		for _, tc := range []struct {
			in   pdf.Array
			want []float64
		}{
			{pdf.Array{pdf.Integer(3), pdf.Integer(2)}, []float64{3, 2}},
			{pdf.Array{pdf.Integer(-1), pdf.Integer(2)}, []float64{0, 2}},
			{pdf.Array{pdf.Integer(-1)}, []float64{3}}, // default
			{pdf.Array{pdf.Integer(0), pdf.Integer(0)}, []float64{3}},
			{pdf.Array{}, []float64{3}},
		} {
			w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			dict := pdf.Dict{"S": pdf.Name("D"), "D": tc.in}

			style, err := ExtractStyle(pdf.NewCursor(w), dict, true)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.want, style.DashPattern); diff != "" {
				t.Errorf("%v: DashPattern (-want +got):\n%s", tc.in, diff)
			}

			// whatever we read must be writable again
			rm := pdf.NewResourceManager(w)
			if _, err := rm.Embed(style); err != nil {
				t.Errorf("%v: cannot write back: %v", tc.in, err)
			}
		}
	})

	t.Run("write", func(t *testing.T) {
		for _, tc := range []struct {
			dash    []float64
			wantErr bool
		}{
			{[]float64{3, 2}, false},
			{[]float64{0, 2}, false}, // a zero length is allowed
			{[]float64{-1}, true},
			{[]float64{0, 0}, true},
			{[]float64{math.NaN()}, true},
			{[]float64{math.Inf(1)}, true},
		} {
			w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			rm := pdf.NewResourceManager(w)

			style := &Style{
				Style:       StyleDashed,
				LineWidth:   1,
				DashPattern: tc.dash,
				SingleUse:   true,
			}
			_, err := rm.Embed(style)
			if (err != nil) != tc.wantErr {
				t.Errorf("%v: got error %v, want error: %v", tc.dash, err, tc.wantErr)
			}
		}
	})
}

// TestStyleNonNumeric checks that a colour or dash pattern with an element
// which is not a number is read as absent, rather than with that element
// replaced by zero.
func TestStyleNonNumeric(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	c := pdf.NewCursor(w)
	decode := func(extra pdf.Dict) *Style {
		t.Helper()
		dict := pdf.Dict{"S": pdf.Name("D")}
		maps.Copy(dict, extra)
		style, err := ExtractStyle(c, dict, true)
		if err != nil {
			t.Fatal(err)
		}
		rm := pdf.NewResourceManager(w)
		if _, err := rm.Embed(style); err != nil {
			t.Errorf("cannot write back: %v", err)
		}
		return style
	}
	absent := decode(nil)

	for name, arr := range map[string]pdf.Array{
		"name":     {pdf.Integer(1), pdf.Name("x"), pdf.Integer(0)},
		"null":     {pdf.Integer(1), nil, pdf.Integer(0)},
		"dangling": {pdf.NewReference(99, 0), pdf.Integer(1), pdf.Integer(0)},
	} {
		if diff := cmp.Diff(absent, decode(pdf.Dict{"C": arr})); diff != "" {
			t.Errorf("C %s differs from absent entry (-absent +got):\n%s", name, diff)
		}
	}

	// elements beyond the first three are not used
	extra := pdf.Array{pdf.Integer(1), pdf.Integer(0), pdf.Integer(0), pdf.Name("x")}
	if got := decode(pdf.Dict{"C": extra}); got.Color != (color.DeviceRGB{1, 0, 0}) {
		t.Errorf("colour %v, want red", got.Color)
	}

	if got := decode(pdf.Dict{"C": pdf.Array{pdf.Integer(1), pdf.Integer(0), pdf.Integer(0)}}); got.Color != (color.DeviceRGB{1, 0, 0}) {
		t.Errorf("valid colour %v, want red", got.Color)
	}

	for name, arr := range map[string]pdf.Array{
		"name":     {pdf.Integer(5), pdf.Name("x")},
		"null":     {pdf.Integer(5), nil},
		"dangling": {pdf.NewReference(99, 0), pdf.Integer(5)},
	} {
		if diff := cmp.Diff(absent, decode(pdf.Dict{"D": arr})); diff != "" {
			t.Errorf("D %s differs from absent entry (-absent +got):\n%s", name, diff)
		}
	}
	if got := decode(pdf.Dict{"D": pdf.Array{pdf.Integer(5), pdf.Integer(2)}}); !slices.Equal(got.DashPattern, []float64{5, 2}) {
		t.Errorf("valid dash pattern %v, want [5 2]", got.DashPattern)
	}
}

// TestStyleReadErrors checks that read errors are returned from the colour
// and dash pattern entries.
func TestStyleReadErrors(t *testing.T) {
	errTest := errors.New("test read error")
	failRef := pdf.NewReference(7, 0)
	c := pdf.NewCursor(failGetter{Getter: mock.Getter, fail: failRef, err: errTest})
	for name, dict := range map[string]pdf.Dict{
		"C":         {"C": failRef},
		"C element": {"C": pdf.Array{pdf.Integer(1), failRef, pdf.Integer(0)}},
		"D":         {"S": pdf.Name("D"), "D": failRef},
		"D element": {"S": pdf.Name("D"), "D": pdf.Array{pdf.Integer(1), failRef}},
	} {
		if _, err := ExtractStyle(c, dict, true); !errors.Is(err, errTest) {
			t.Errorf("%s: got %v, want the read error", name, err)
		}
	}
}

// failGetter is a pdf.Getter which fails for one reference with an error
// that is not a malformed-file error.
type failGetter struct {
	pdf.Getter
	fail pdf.Reference
	err  error
}

func (g failGetter) Get(ref pdf.Reference, canObjStm bool) (pdf.Native, error) {
	if ref == g.fail {
		return nil, g.err
	}
	return g.Getter.Get(ref, canObjStm)
}
