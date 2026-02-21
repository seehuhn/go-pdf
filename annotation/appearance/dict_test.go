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

package appearance

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func makeTestAppearance(gray float64) *form.Form {
	b := builder.New(content.Form, nil)
	b.SetFillColor(color.DeviceGray(gray))
	b.Rectangle(0, 0, 24, 24)
	b.Fill()
	return &form.Form{
		Content: b.Stream,
		Res:     b.Resources,
		BBox:    pdf.Rectangle{LLx: 0, LLy: 0, URx: 24, URy: 24},
		Matrix:  matrix.Identity,
	}
}

var (
	appA = makeTestAppearance(0.25)
	appB = makeTestAppearance(0.5)
	appC = makeTestAppearance(0.75)
)

type testCase struct {
	name    string
	version pdf.Version
	data    *Dict
}

var testCases = []testCase{
	{
		name:    "streams/V1.7",
		version: pdf.V1_7,
		data: &Dict{
			Normal:   appA,
			RollOver: appB,
			Down:     appC,
		},
	},
	{
		name:    "streams/V2.0",
		version: pdf.V2_0,
		data: &Dict{
			Normal:   appA,
			RollOver: appB,
			Down:     appC,
		},
	},
	{
		name:    "single/V1.7",
		version: pdf.V1_7,
		data: &Dict{
			Normal:    appA,
			RollOver:  appB,
			Down:      appC,
			SingleUse: true,
		},
	},
	{
		name:    "single/V2.0",
		version: pdf.V2_0,
		data: &Dict{
			Normal:    appA,
			RollOver:  appB,
			Down:      appC,
			SingleUse: true,
		},
	},
	{
		name:    "maps/V1.7",
		version: pdf.V1_7,
		data: &Dict{
			NormalMap: map[pdf.Name]*form.Form{
				"On":  appA,
				"Off": appB,
			},
			RollOverMap: map[pdf.Name]*form.Form{
				"On":  appB,
				"Off": appC,
			},
			DownMap: map[pdf.Name]*form.Form{
				"On":  appC,
				"Off": appA,
			},
		},
	},
	{
		name:    "maps/V2.0",
		version: pdf.V2_0,
		data: &Dict{
			NormalMap: map[pdf.Name]*form.Form{
				"On":  appA,
				"Off": appB,
			},
			RollOverMap: map[pdf.Name]*form.Form{
				"On":  appB,
				"Off": appC,
			},
			DownMap: map[pdf.Name]*form.Form{
				"On":  appC,
				"Off": appA,
			},
		},
	},
	{
		name:    "normalOnly/V2.0",
		version: pdf.V2_0,
		data: &Dict{
			Normal:   appA,
			RollOver: appA,
			Down:     appA,
		},
	},
}

func roundTripTest(t *testing.T, version pdf.Version, data *Dict) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(w)
	ref, err := rm.Embed(data)
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
	decoded, err := pdf.ExtractorGet(x, ref, Extract)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	if diff := cmp.Diff(data, decoded); diff != "" {
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
		w, buf := memfile.NewPDFWriter(tc.version, opt)

		err := memfile.AddBlankPage(w)
		if err != nil {
			continue
		}

		rm := pdf.NewResourceManager(w)
		ref, err := rm.Embed(tc.data)
		if err != nil {
			continue
		}
		err = rm.Close()
		if err != nil {
			continue
		}

		w.GetMeta().Trailer["Quir:E"] = ref
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
		objGo, err := pdf.ExtractorGet(x, objPDF, Extract)
		if err != nil {
			t.Skip("malformed PDF object")
		}

		roundTripTest(t, pdf.GetVersion(r), objGo)
	})
}
