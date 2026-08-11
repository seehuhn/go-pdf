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

package image

import (
	"bytes"
	"io"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestFlateSourceNoPredictor checks the two spellings of "no predictor".
// A zero Predictor is a shorthand for [pdf.FlatePredictorNone], so the two
// must behave alike.  Colors, BitsPerComponent and Columns may not appear
// alongside either, which is why a source carrying them has to drop them
// rather than pass them to the filter.
func TestFlateSourceNoPredictor(t *testing.T) {
	pix := make([]byte, 3*4*4)
	for i := range pix {
		pix[i] = byte(i * 7)
	}

	var streams [2][]byte
	for i, predictor := range []pdf.FlatePredictor{0, pdf.FlatePredictorNone} {
		w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
		rm := pdf.NewResourceManager(w)
		dict := &Dict{
			Width:            4,
			Height:           4,
			ColorSpace:       color.SpaceDeviceRGB,
			BitsPerComponent: 8,
			Data: &FlateSource{
				WriteData: func(w io.Writer) error {
					_, err := w.Write(pix)
					return err
				},
				Predictor:        predictor,
				Width:            4,
				Colors:           3,
				BitsPerComponent: 8,
			},
		}

		ref, err := rm.Embed(dict)
		if err != nil {
			t.Fatalf("predictor %d: %v", predictor, err)
		}
		if err := rm.Close(); err != nil {
			t.Fatalf("predictor %d: %v", predictor, err)
		}

		c := pdf.NewCursor(w)
		stm, err := c.Stream(ref.(pdf.Reference))
		if err != nil {
			t.Fatal(err)
		}
		if _, present := stm.Dict["DecodeParms"]; present {
			t.Errorf("predictor %d: unexpected DecodeParms", predictor)
		}
		body, err := pdf.DecodeStream(w, nil, stm)
		if err != nil {
			t.Fatal(err)
		}
		got, err := io.ReadAll(body)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, pix) {
			t.Errorf("predictor %d: pixel data not recovered", predictor)
		}
		streams[i] = got
	}

	if !bytes.Equal(streams[0], streams[1]) {
		t.Error("zero predictor and FlatePredictorNone disagree")
	}
}

func TestFlatePredictorFor(t *testing.T) {
	indexed, err := color.Indexed([]color.Color{
		color.DeviceGray(0), color.DeviceGray(1),
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		cs   color.Space
		bpc  int
		want pdf.FlatePredictor
	}{
		{"gray 8 bit", color.SpaceDeviceGray, 8, pdf.FlatePredictorPNGOptimum},
		{"rgb 8 bit", color.SpaceDeviceRGB, 8, pdf.FlatePredictorPNGOptimum},
		{"cmyk 16 bit", color.SpaceDeviceCMYK, 16, pdf.FlatePredictorPNGOptimum},
		{"gray 4 bit", color.SpaceDeviceGray, 4, pdf.FlatePredictorNone},
		{"gray 2 bit", color.SpaceDeviceGray, 2, pdf.FlatePredictorNone},
		{"rgb 1 bit", color.SpaceDeviceRGB, 1, pdf.FlatePredictorNone},
		{"indexed 8 bit", indexed, 8, pdf.FlatePredictorNone},
		{"indexed 4 bit", indexed, 4, pdf.FlatePredictorNone},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := flatePredictorFor(tc.cs, tc.bpc); got != tc.want {
				t.Errorf("got predictor %d, want %d", got, tc.want)
			}
		})
	}
}

// TestNewFlateSource checks that the derived fields agree with the colour
// space they came from.  A Colors value which disagrees with the data cannot
// corrupt the image — the reader takes its stride from the same DecodeParms
// the writer emitted — but it points the predictor at the wrong neighbour and
// so costs compression.
func TestNewFlateSource(t *testing.T) {
	write := func(io.Writer) error { return nil }

	for _, tc := range []struct {
		name string
		cs   color.Space
		bpc  int
	}{
		{"gray 8 bit", color.SpaceDeviceGray, 8},
		{"rgb 8 bit", color.SpaceDeviceRGB, 8},
		{"cmyk 16 bit", color.SpaceDeviceCMYK, 16},
		{"rgb 4 bit", color.SpaceDeviceRGB, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := NewFlateSource(64, tc.cs, tc.bpc, write)
			if s.Width != 64 {
				t.Errorf("got Width %d, want 64", s.Width)
			}
			if s.Colors != tc.cs.Channels() {
				t.Errorf("got Colors %d, want %d", s.Colors, tc.cs.Channels())
			}
			if s.BitsPerComponent != tc.bpc {
				t.Errorf("got BitsPerComponent %d, want %d", s.BitsPerComponent, tc.bpc)
			}
			if want := flatePredictorFor(tc.cs, tc.bpc); s.Predictor != want {
				t.Errorf("got Predictor %d, want %d", s.Predictor, want)
			}
			if s.WriteData == nil {
				t.Error("WriteData not set")
			}
		})
	}
}
