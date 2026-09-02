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

package extgstate_test

import (
	"bytes"
	"fmt"
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/extgstate"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/graphics/halftone"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

var testCases = []struct {
	name    string
	version pdf.Version
	data    *extgstate.ExtGState
}{
	{
		name:    "minimal",
		version: pdf.V1_7,
		data: &extgstate.ExtGState{
			Set:       graphics.StateLineWidth,
			LineWidth: 2.0,
		},
	},
	{
		name:    "text_knockout",
		version: pdf.V1_7,
		data: &extgstate.ExtGState{
			Set:          graphics.StateTextKnockout,
			TextKnockout: true,
		},
	},
	{
		name:    "line_styles",
		version: pdf.V1_7,
		data: &extgstate.ExtGState{
			Set:        graphics.StateLineWidth | graphics.StateLineCap | graphics.StateLineJoin | graphics.StateMiterLimit,
			LineWidth:  1.5,
			LineCap:    graphics.LineCapRound,
			LineJoin:   graphics.LineJoinBevel,
			MiterLimit: 10.0,
		},
	},
	{
		name:    "dash_pattern",
		version: pdf.V1_7,
		data: &extgstate.ExtGState{
			Set:         graphics.StateLineDash,
			DashPattern: []float64{3, 2, 1, 2},
			DashPhase:   1.5,
		},
	},
	{
		name:    "alpha_and_blend",
		version: pdf.V1_7,
		data: &extgstate.ExtGState{
			Set:             graphics.StateStrokeAlpha | graphics.StateFillAlpha | graphics.StateAlphaSourceFlag | graphics.StateBlendMode,
			StrokeAlpha:     0.7,
			FillAlpha:       0.5,
			AlphaSourceFlag: true,
			BlendMode:       graphics.BlendMode{graphics.BlendModeMultiply},
		},
	},
	{
		name:    "overprint",
		version: pdf.V1_7,
		data: &extgstate.ExtGState{
			Set:             graphics.StateOverprint | graphics.StateOverprintMode,
			OverprintStroke: true,
			OverprintFill:   false,
			OverprintMode:   1,
		},
	},
	{
		name:    "color_functions_nil",
		version: pdf.V1_7,
		data: &extgstate.ExtGState{
			Set:               graphics.StateBlackGeneration | graphics.StateUndercolorRemoval,
			BlackGeneration:   nil,
			UndercolorRemoval: nil,
		},
	},
	{
		name:    "transfer_function_identity",
		version: pdf.V1_7,
		data: &extgstate.ExtGState{
			Set: graphics.StateTransferFunction,
			TransferFunctions: graphics.TransferFunctions{
				Red:   function.Identity,
				Green: function.Identity,
				Blue:  function.Identity,
				Gray:  function.Identity,
			},
		},
	},
	{
		name:    "halftone_type1",
		version: pdf.V1_7,
		data: &extgstate.ExtGState{
			Set: graphics.StateHalftone,
			Halftone: &halftone.Type1{
				Frequency:    60,
				Angle:        45,
				SpotFunction: halftone.Round,
			},
		},
	},
	{
		name:    "tolerances",
		version: pdf.V1_7,
		data: &extgstate.ExtGState{
			Set:                 graphics.StateFlatnessTolerance | graphics.StateSmoothnessTolerance,
			FlatnessTolerance:   1.0,
			SmoothnessTolerance: 0.5,
		},
	},
	{
		name:    "halftone_origin_pdf2",
		version: pdf.V2_0,
		data: &extgstate.ExtGState{
			Set:             graphics.StateHalftoneOrigin,
			HalftoneOriginX: 10.0,
			HalftoneOriginY: 20.0,
		},
	},
	{
		name:    "black_point_compensation_pdf2",
		version: pdf.V2_0,
		data: &extgstate.ExtGState{
			Set:                    graphics.StateBlackPointCompensation,
			BlackPointCompensation: pdf.Name("ON"),
		},
	},
	{
		name:    "singleuse_true",
		version: pdf.V1_7,
		data: &extgstate.ExtGState{
			Set:              graphics.StateStrokeAdjustment,
			StrokeAdjustment: true,
			SingleUse:        true,
		},
	},
	{
		name:    "singleuse_false",
		version: pdf.V1_7,
		data: &extgstate.ExtGState{
			Set:              graphics.StateStrokeAdjustment,
			StrokeAdjustment: true,
			SingleUse:        false,
		},
	},
	{
		name:    "complex_state",
		version: pdf.V1_7,
		data: &extgstate.ExtGState{
			Set: graphics.StateLineWidth | graphics.StateLineCap | graphics.StateRenderingIntent |
				graphics.StateStrokeAlpha | graphics.StateFillAlpha | graphics.StateOverprint,
			LineWidth:       3.0,
			LineCap:         graphics.LineCapSquare,
			RenderingIntent: graphics.RenderingIntent("Perceptual"),
			StrokeAlpha:     0.8,
			FillAlpha:       0.6,
			OverprintStroke: false,
			OverprintFill:   true,
		},
	},
	{
		name:    "softmask_none",
		version: pdf.V1_7,
		data: &extgstate.ExtGState{
			Set:      graphics.StateSoftMask,
			SoftMask: nil, // represents /None in PDF
		},
	},
}

func roundTripTest(t *testing.T, version pdf.Version, data *extgstate.ExtGState) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(t, version, nil)

	// Embed the ExtGState
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
		t.Fatalf("resource manager close failed: %v", err)
	}

	// Extract the ExtGState
	x := pdf.NewExtractor(w)
	extracted, err := pdf.Decode(pdf.CursorAt(x, nil), embedded, extract.ExtGState)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	// Compare with appropriate transformers
	cmpFont := cmp.Comparer(func(a, b font.Instance) bool {
		if a == nil || b == nil {
			return a == b
		}
		// For now, just compare that both are non-nil
		// TODO: improve font comparison once we have better font equality
		return a.PostScriptName() == b.PostScriptName()
	})
	if diff := cmp.Diff(data, extracted, cmpFont); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func TestRoundTrip(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			roundTripTest(t, tc.version, tc.data)
		})
	}
}

func FuzzRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	for _, tc := range testCases {
		w, buf := memfile.NewPDFWriter(f, tc.version, opt)

		err := memfile.AddBlankPage(w)
		if err != nil {
			continue
		}

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

		// The reader accepts version numbers the writer cannot produce, so
		// that files written against a future version of the standard can
		// still be read (see [pdf.ParseVersion]).
		version := pdf.GetVersion(r)
		if !version.IsSupported() {
			t.Skip("version not supported")
		}

		objPDF := r.GetMeta().Trailer["Quir:E"]
		if objPDF == nil {
			t.Skip("missing PDF object")
		}

		x := pdf.NewExtractor(r)
		objGo, err := pdf.Decode(pdf.CursorAt(x, nil), objPDF, extract.ExtGState)
		if err != nil {
			t.Skip("malformed PDF object")
		}

		roundTripTest(t, version, objGo)
	})
}

// rangedFields lists the numeric graphics state parameters which are defined
// only over a range of values, together with that range and accessors for the
// corresponding Go field.  A value of math.MaxFloat64 for hi means that the
// parameter has no upper limit.
var rangedFields = []struct {
	key    pdf.Name
	bit    graphics.Bits
	get    func(*extgstate.ExtGState) float64
	set    func(*extgstate.ExtGState, float64)
	lo, hi float64
}{
	{
		key: "LW",
		bit: graphics.StateLineWidth,
		get: func(e *extgstate.ExtGState) float64 { return e.LineWidth },
		set: func(e *extgstate.ExtGState, x float64) { e.LineWidth = x },
		lo:  0, hi: math.MaxFloat64,
	},
	{
		key: "ML",
		bit: graphics.StateMiterLimit,
		get: func(e *extgstate.ExtGState) float64 { return e.MiterLimit },
		set: func(e *extgstate.ExtGState, x float64) { e.MiterLimit = x },
		lo:  1, hi: math.MaxFloat64,
	},
	{
		key: "CA",
		bit: graphics.StateStrokeAlpha,
		get: func(e *extgstate.ExtGState) float64 { return e.StrokeAlpha },
		set: func(e *extgstate.ExtGState, x float64) { e.StrokeAlpha = x },
		lo:  0, hi: 1,
	},
	{
		key: "ca",
		bit: graphics.StateFillAlpha,
		get: func(e *extgstate.ExtGState) float64 { return e.FillAlpha },
		set: func(e *extgstate.ExtGState, x float64) { e.FillAlpha = x },
		lo:  0, hi: 1,
	},
	{
		key: "FL",
		bit: graphics.StateFlatnessTolerance,
		get: func(e *extgstate.ExtGState) float64 { return e.FlatnessTolerance },
		set: func(e *extgstate.ExtGState, x float64) { e.FlatnessTolerance = x },
		lo:  0, hi: 100,
	},
	{
		key: "SM",
		bit: graphics.StateSmoothnessTolerance,
		get: func(e *extgstate.ExtGState) float64 { return e.SmoothnessTolerance },
		set: func(e *extgstate.ExtGState, x float64) { e.SmoothnessTolerance = x },
		lo:  0, hi: 1,
	},
}

// TestRangeOnRead checks that values outside the range a graphics state
// parameter is defined over are snapped into it when read, rather than being
// carried into the graphics state as given.
func TestRangeOnRead(t *testing.T) {
	for _, f := range rangedFields {
		cases := []struct{ in, want float64 }{
			{f.lo - 1, f.lo},
			{f.lo, f.lo},
			{min(f.lo+1, f.hi), min(f.lo+1, f.hi)},
		}
		if f.hi != math.MaxFloat64 {
			cases = append(cases, struct{ in, want float64 }{f.hi + 1, f.hi})
		}

		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s/%v", f.key, tc.in), func(t *testing.T) {
				w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
				dict := pdf.Dict{
					"Type": pdf.Name("ExtGState"),
					f.key:  pdf.Number(tc.in),
				}

				x := pdf.NewExtractor(w)
				e, err := pdf.Decode(pdf.CursorAt(x, nil), dict, extract.ExtGState)
				if err != nil {
					t.Fatal(err)
				}

				if got := f.get(e); got != tc.want {
					t.Errorf("%s = %v, want %v", f.key, got, tc.want)
				}
			})
		}
	}
}

// TestRangeOnWrite checks that out-of-range values are refused when writing,
// instead of producing an ExtGState the specification does not allow.  Values
// with no PDF number representation are covered too: an infinity is out of
// range because the bounds are finite, and a NaN because the check is written
// as an acceptance of the valid values.
func TestRangeOnWrite(t *testing.T) {
	for _, f := range rangedFields {
		bad := []float64{f.lo - 1, math.NaN(), math.Inf(+1), math.Inf(-1)}
		if f.hi != math.MaxFloat64 {
			bad = append(bad, f.hi+1)
		}

		for _, v := range bad {
			t.Run(fmt.Sprintf("%s/%v", f.key, v), func(t *testing.T) {
				w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
				rm := pdf.NewResourceManager(w)

				e := &extgstate.ExtGState{Set: f.bit}
				f.set(e, v)
				if _, err := rm.Embed(e); err == nil {
					t.Error("expected an error, got none")
				}
			})
		}
	}
}

// TestRangeOnWriteValid checks that the range test does not reject values
// inside the range.
func TestRangeOnWriteValid(t *testing.T) {
	for _, f := range rangedFields {
		valid := []float64{f.lo, f.lo + 1}
		if f.hi != math.MaxFloat64 {
			valid = []float64{f.lo, (f.lo + f.hi) / 2, f.hi}
		}

		for _, v := range valid {
			t.Run(fmt.Sprintf("%s/%v", f.key, v), func(t *testing.T) {
				w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
				rm := pdf.NewResourceManager(w)

				e := &extgstate.ExtGState{Set: f.bit}
				f.set(e, v)
				if _, err := rm.Embed(e); err != nil {
					t.Error(err)
				}
			})
		}
	}
}

// TestDashPatternOnRead checks that dash patterns the specification does not
// allow are repaired when read, so that they can be written back out again.
func TestDashPatternOnRead(t *testing.T) {
	for _, tc := range []struct {
		name        string
		in          pdf.Object
		wantPattern []float64
		wantPhase   float64
	}{
		{
			name:        "valid",
			in:          pdf.Array{pdf.Array{pdf.Integer(3), pdf.Integer(2)}, pdf.Integer(1)},
			wantPattern: []float64{3, 2},
			wantPhase:   1,
		},
		{
			name:        "negative length",
			in:          pdf.Array{pdf.Array{pdf.Integer(-1), pdf.Integer(2)}, pdf.Integer(1)},
			wantPattern: []float64{0, 2},
			wantPhase:   1,
		},
		{
			name:        "all zero",
			in:          pdf.Array{pdf.Array{pdf.Integer(0), pdf.Integer(0)}, pdf.Integer(5)},
			wantPattern: []float64{},
			wantPhase:   0,
		},
		{
			name:        "solid line with phase",
			in:          pdf.Array{pdf.Array{}, pdf.Integer(7)},
			wantPattern: []float64{},
			wantPhase:   0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			dict := pdf.Dict{
				"Type": pdf.Name("ExtGState"),
				"D":    tc.in,
			}

			x := pdf.NewExtractor(w)
			e, err := pdf.Decode(pdf.CursorAt(x, nil), dict, extract.ExtGState)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tc.wantPattern, e.DashPattern); diff != "" {
				t.Errorf("DashPattern (-want +got):\n%s", diff)
			}
			if e.DashPhase != tc.wantPhase {
				t.Errorf("DashPhase = %v, want %v", e.DashPhase, tc.wantPhase)
			}

			// the value we read must be one we can write back out
			rm := pdf.NewResourceManager(w)
			if _, err := rm.Embed(e); err != nil {
				t.Errorf("cannot write back: %v", err)
			}
		})
	}
}

// TestDashPatternOnWrite checks that dash patterns the specification does not
// allow are refused when writing.
func TestDashPatternOnWrite(t *testing.T) {
	for _, tc := range []struct {
		name    string
		pattern []float64
		phase   float64
	}{
		{"negative length", []float64{-1}, 0},
		{"all zero", []float64{0, 0}, 0},
		{"solid line with phase", []float64{}, 1},
		{"NaN length", []float64{math.NaN()}, 0},
		{"infinite length", []float64{math.Inf(1)}, 0},
		{"NaN phase", []float64{3}, math.NaN()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			rm := pdf.NewResourceManager(w)

			e := &extgstate.ExtGState{
				Set:         graphics.StateLineDash,
				DashPattern: tc.pattern,
				DashPhase:   tc.phase,
			}
			if _, err := rm.Embed(e); err == nil {
				t.Error("expected an error, got none")
			}
		})
	}
}
