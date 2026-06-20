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
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

var characteristicsCases = []*Characteristics{
	{},
	{
		BorderColor:     color.DeviceRGB{0, 0, 1},
		BackgroundColor: color.DeviceGray(0.5),
		Rotation:        90,
	},
	{
		Caption:         "OK",
		RolloverCaption: "Hover",
		DownCaption:     "Down",
		TextPosition:    TextPositionCaptionBelowIcon,
		SingleUse:       true,
	},
	{
		BorderColor:  color.DeviceCMYK{0, 0, 0, 1},
		Rotation:     270,
		Icon:         appA,
		RolloverIcon: appB,
		DownIcon:     appC,
		IconFit: &IconFit{
			ScaleWhen:     IconScaleWhenBigger,
			Scaling:       IconScalingProportional,
			LeftoverSpace: &[2]float64{0.25, 0.75},
			FitToBounds:   true,
		},
	},
	{
		Icon:         appA,
		TextPosition: TextPositionIconOnly,
	},
}

func roundTripCharacteristics(t *testing.T, version pdf.Version, data *Characteristics) {
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
	decoded, err := pdf.Decode(pdf.CursorAt(x, nil), ref, ExtractCharacteristics)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	opts := []cmp.Option{
		cmp.Comparer(func(a, b *form.Form) bool {
			if a == nil || b == nil {
				return a == b
			}
			return a.Equal(b)
		}),
	}
	if diff := cmp.Diff(data, decoded, opts...); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func TestCharacteristicsRoundTrip(t *testing.T) {
	for _, version := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for i, data := range characteristicsCases {
			t.Run(fmt.Sprintf("%d/%s", i, version), func(t *testing.T) {
				roundTripCharacteristics(t, version, data)
			})
		}
	}
}

func FuzzCharacteristicsRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{HumanReadable: true}
	for _, version := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for _, data := range characteristicsCases {
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
		objGo, _ := pdf.Decode(pdf.CursorAt(x, nil), objPDF, ExtractCharacteristics)
		if objGo == nil {
			t.Skip("no characteristics dictionary")
		}

		roundTripCharacteristics(t, pdf.GetVersion(r), objGo)
	})
}
