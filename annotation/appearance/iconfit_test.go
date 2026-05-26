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
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

var iconFitCases = []*IconFit{
	{},
	{
		ScaleWhen:     IconScaleWhenBigger,
		Scaling:       IconScalingProportional,
		LeftoverSpace: &[2]float64{0.25, 0.75},
		FitToBounds:   true,
	},
	{ScaleWhen: IconScaleNever},
	{Scaling: IconScalingAnamorphic, SingleUse: true},
	{LeftoverSpace: &[2]float64{0, 1}},
	{FitToBounds: true, SingleUse: true},
}

func roundTripIconFit(t *testing.T, version pdf.Version, data *IconFit) {
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
	if err := rm.Close(); err != nil {
		t.Fatalf("rm.Close failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("w.Close failed: %v", err)
	}

	x := pdf.NewExtractor(w)
	decoded, err := pdf.ExtractorGet(x, nil, ref, ExtractIconFit)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	if diff := cmp.Diff(data, decoded); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func TestIconFitRoundTrip(t *testing.T) {
	for _, version := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for i, data := range iconFitCases {
			t.Run(fmt.Sprintf("%d/%s", i, version), func(t *testing.T) {
				roundTripIconFit(t, version, data)
			})
		}
	}
}

func FuzzIconFitRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{HumanReadable: true}
	for _, version := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for _, data := range iconFitCases {
			w, buf := memfile.NewPDFWriter(version, opt)
			if memfile.AddBlankPage(w) != nil {
				continue
			}
			rm := pdf.NewResourceManager(w)
			ref, err := rm.Embed(data)
			if err != nil {
				continue
			}
			if rm.Close() != nil {
				continue
			}
			w.GetMeta().Trailer["Quir:E"] = ref
			if w.Close() != nil {
				continue
			}
			f.Add(buf.Data)
		}
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}
		objPDF := r.GetMeta().Trailer["Quir:E"]
		if objPDF == nil {
			t.Skip("missing PDF object")
		}

		x := pdf.NewExtractor(r)
		objGo, _ := pdf.ExtractorGet(x, nil, objPDF, ExtractIconFit)
		if objGo == nil {
			t.Skip("no icon fit dictionary")
		}

		roundTripIconFit(t, pdf.GetVersion(r), objGo)
	})
}
