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

package graphics

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics/halftone"
	"seehuhn.de/go/pdf/graphics/transfer"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

var testCases = []struct {
	name    string
	version pdf.Version
	data    *ExtGState
}{
	{
		name:    "minimal",
		version: pdf.V1_7,
		data: &ExtGState{
			Set:       StateLineWidth,
			LineWidth: 2.0,
		},
	},
	{
		name:    "text_knockout",
		version: pdf.V1_7,
		data: &ExtGState{
			Set:          StateTextKnockout,
			TextKnockout: true,
		},
	},
	{
		name:    "line_styles",
		version: pdf.V1_7,
		data: &ExtGState{
			Set:        StateLineWidth | StateLineCap | StateLineJoin | StateMiterLimit,
			LineWidth:  1.5,
			LineCap:    LineCapRound,
			LineJoin:   LineJoinBevel,
			MiterLimit: 10.0,
		},
	},
	{
		name:    "dash_pattern",
		version: pdf.V1_7,
		data: &ExtGState{
			Set:         StateLineDash,
			DashPattern: []float64{3, 2, 1, 2},
			DashPhase:   1.5,
		},
	},
	{
		name:    "alpha_and_blend",
		version: pdf.V1_7,
		data: &ExtGState{
			Set:             StateStrokeAlpha | StateFillAlpha | StateAlphaSourceFlag | StateBlendMode,
			StrokeAlpha:     0.7,
			FillAlpha:       0.5,
			AlphaSourceFlag: true,
			BlendMode:       pdf.Name("Multiply"),
		},
	},
	{
		name:    "overprint",
		version: pdf.V1_7,
		data: &ExtGState{
			Set:             StateOverprint | StateOverprintMode,
			OverprintStroke: true,
			OverprintFill:   false,
			OverprintMode:   1,
		},
	},
	{
		name:    "color_functions_nil",
		version: pdf.V1_7,
		data: &ExtGState{
			Set:               StateBlackGeneration | StateUndercolorRemoval,
			BlackGeneration:   nil,
			UndercolorRemoval: nil,
		},
	},
	{
		name:    "transfer_function_identity",
		version: pdf.V1_7,
		data: &ExtGState{
			Set: StateTransferFunction,
			TransferFunction: transfer.Functions{
				Red:   transfer.Identity,
				Green: transfer.Identity,
				Blue:  transfer.Identity,
				Gray:  transfer.Identity,
			},
		},
	},
	{
		name:    "halftone_type1",
		version: pdf.V1_7,
		data: &ExtGState{
			Set: StateHalftone,
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
		data: &ExtGState{
			Set:                 StateFlatnessTolerance | StateSmoothnessTolerance,
			FlatnessTolerance:   1.0,
			SmoothnessTolerance: 0.5,
		},
	},
	{
		name:    "halftone_origin_pdf2",
		version: pdf.V2_0,
		data: &ExtGState{
			Set:             StateHalftoneOrigin,
			HalftoneOriginX: 10.0,
			HalftoneOriginY: 20.0,
		},
	},
	{
		name:    "black_point_compensation_pdf2",
		version: pdf.V2_0,
		data: &ExtGState{
			Set:                    StateBlackPointCompensation,
			BlackPointCompensation: pdf.Name("ON"),
		},
	},
	{
		name:    "singleuse_true",
		version: pdf.V1_7,
		data: &ExtGState{
			Set:              StateStrokeAdjustment,
			StrokeAdjustment: true,
			SingleUse:        true,
		},
	},
	{
		name:    "singleuse_false",
		version: pdf.V1_7,
		data: &ExtGState{
			Set:              StateStrokeAdjustment,
			StrokeAdjustment: true,
			SingleUse:        false,
		},
	},
	{
		name:    "complex_state",
		version: pdf.V1_7,
		data: &ExtGState{
			Set: StateLineWidth | StateLineCap | StateRenderingIntent |
				StateStrokeAlpha | StateFillAlpha | StateOverprint,
			LineWidth:       3.0,
			LineCap:         LineCapSquare,
			RenderingIntent: RenderingIntent("Perceptual"),
			StrokeAlpha:     0.8,
			FillAlpha:       0.6,
			OverprintStroke: false,
			OverprintFill:   true,
		},
	},
}

func roundTripTest(t *testing.T, version pdf.Version, data *ExtGState) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(version, nil)

	// Embed the ExtGState
	rm := pdf.NewResourceManager(w)
	embedded, err := rm.Embed(data)
	if err != nil {
		t.Fatalf("embed failed: %v", err)
	}
	err = rm.Close()
	if err != nil {
		t.Fatalf("resource manager close failed: %v", err)
	}

	// Extract the ExtGState
	x := pdf.NewExtractor(w)
	extracted, err := ExtractExtGState(x, embedded)
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

func TestExtGStateRoundTrip(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			roundTripTest(t, tc.version, tc.data)
		})
	}
}

func FuzzExtGStateRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	for _, tc := range testCases {
		w, buf := memfile.NewPDFWriter(tc.version, opt)

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
		r, err := pdf.NewReader(bytes.NewReader(fileData), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}
		objPDF := r.GetMeta().Trailer["Quir:E"]
		if objPDF == nil {
			t.Skip("missing PDF object")
		}

		x := pdf.NewExtractor(r)
		objGo, err := ExtractExtGState(x, objPDF)
		if err != nil {
			t.Skip("malformed PDF object")
		}

		roundTripTest(t, r.GetMeta().Version, objGo)
	})
}
