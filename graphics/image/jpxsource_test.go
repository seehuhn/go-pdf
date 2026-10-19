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
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/jpx"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
)

func TestJPXSourceLoad(t *testing.T) {
	type testCase struct {
		name   string
		depths []int
		cs     color.Space
	}
	cases := []testCase{
		{"gray", []int{8}, nil},
		{"rgb", []int{8, 8, 8}, nil},
		{"rgb16", []int{16, 16, 16}, nil},
		{"cmyk", []int{8, 8, 8, 8}, nil},
		{"explicit rgb", []int{8, 8, 8}, color.SpaceDeviceRGB},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := jpxTestImage(6, 5, tc.depths, uint64(20+i))
			d := &Dict{
				Width:      6,
				Height:     5,
				ColorSpace: tc.cs,
				Data:       NewJPXSource(encodeJPX(t, src)),
			}
			got, err := roundTrip(t, pdf.V1_7, d, ExtractDict)
			if err != nil {
				t.Fatal(err)
			}
			if !got.Data.IsJPX() || got.BitsPerComponent != 0 {
				t.Errorf("got IsJPX %t, BitsPerComponent %d", got.Data.IsJPX(), got.BitsPerComponent)
			}
			data, err := got.Load()
			if err != nil {
				t.Fatal(err)
			}
			decode := make([]float64, 0, 2*len(tc.depths))
			for range tc.depths {
				decode = append(decode, 0, 1)
			}
			want := expectPix(planesOf(src), tc.depths[0], decode)
			if diff := cmp.Diff(want, data.Pix); diff != "" {
				t.Errorf("pixels differ (-want +got):\n%s", diff)
			}
		})
	}
}

func TestJPXSourceSMaskInData(t *testing.T) {
	src := opacityImage(false)
	d := &Dict{
		Width:       5,
		Height:      4,
		SMaskInData: 1,
		Data:        NewJPXSource(encodeJPX(t, src)),
	}
	got, err := roundTrip(t, pdf.V1_7, d, ExtractDict)
	if err != nil {
		t.Fatal(err)
	}
	if got.SMaskInData != 1 {
		t.Errorf("SMaskInData = %d, want 1", got.SMaskInData)
	}
	data, err := got.Load()
	if err != nil {
		t.Fatal(err)
	}

	wantPix := expectPix(planesOf(src)[:3], 8, []float64{0, 1, 0, 1, 0, 1})
	if diff := cmp.Diff(wantPix, data.Pix); diff != "" {
		t.Errorf("pixels differ (-want +got):\n%s", diff)
	}
	wantAlpha := make([]float32, 5*4)
	for i, v := range src.Planes[3].Pix {
		wantAlpha[i] = float32(float64(v) / 255)
	}
	if diff := cmp.Diff(wantAlpha, data.Alpha); diff != "" {
		t.Errorf("alpha differs (-want +got):\n%s", diff)
	}
}

func TestJPXSourceInvalid(t *testing.T) {
	rgb := encodeJPX(t, jpxTestImage(6, 5, []int{8, 8, 8}, 30))
	var bare bytes.Buffer
	if err := jpx.Encode(&bare, jpxTestImage(6, 5, []int{8}, 31), &jpx.EncodeParams{Levels: 2}); err != nil {
		t.Fatal(err)
	}
	vendor := jpxTestImage(6, 5, []int{8}, 32)
	vendor.ColorSpecs = []jpx.ColorSpec{{Method: 4, Vendor: make([]byte, 16)}}
	badICC := jpxTestImage(6, 5, []int{8}, 33)
	badICC.ColorSpecs = []jpx.ColorSpec{{Method: 2, ICC: []byte("not an ICC profile")}}
	grayLab := jpxTestImage(6, 5, []int{8}, 34)
	grayLab.ColorSpecs = []jpx.ColorSpec{{Method: 1, Enumerated: 14}}
	jab := jpxTestImage(6, 5, []int{8, 8, 8}, 35)
	jab.ColorSpecs = []jpx.ColorSpec{{Method: 1, Enumerated: 19}}

	type testCase struct {
		name    string
		version pdf.Version
		d       *Dict
	}
	cases := []testCase{
		{"width", pdf.V1_7, &Dict{Width: 7, Height: 5, Data: NewJPXSource(rgb)}},
		{"height", pdf.V1_7, &Dict{Width: 6, Height: 4, Data: NewJPXSource(rgb)}},
		{"channels", pdf.V1_7, &Dict{Width: 6, Height: 5, ColorSpace: color.SpaceDeviceGray, Data: NewJPXSource(rgb)}},
		{"bits", pdf.V1_7, &Dict{Width: 6, Height: 5, BitsPerComponent: 8, Data: NewJPXSource(rgb)}},
		{"no opacity", pdf.V1_7, &Dict{Width: 6, Height: 5, SMaskInData: 1, Data: NewJPXSource(rgb)}},
		{"invalid ICC profile", pdf.V1_7, &Dict{Width: 6, Height: 5, Data: NewJPXSource(encodeJPX(t, badICC))}},
		{"bare codestream", pdf.V1_7, &Dict{Width: 6, Height: 5, ColorSpace: color.SpaceDeviceGray, Data: NewJPXSource(bare.Bytes())}},
		{"CIELab with one channel", pdf.V1_7, &Dict{Width: 6, Height: 5, Data: NewJPXSource(encodeJPX(t, grayLab))}},
		{"CIEJab", pdf.V1_7, &Dict{Width: 6, Height: 5, Data: NewJPXSource(encodeJPX(t, jab))}},
		{"vendor colour space", pdf.V1_7, &Dict{Width: 6, Height: 5, Data: NewJPXSource(encodeJPX(t, vendor))}},
		{"garbage", pdf.V1_7, &Dict{Width: 6, Height: 5, Data: NewJPXSource([]byte("not a JPEG 2000 file"))}},
		{"version", pdf.V1_4, &Dict{Width: 6, Height: 5, Data: NewJPXSource(rgb)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := roundTrip(t, tc.version, tc.d, ExtractDict); err == nil {
				t.Error("missing error")
			}
		})
	}
}

// TestJPXSourceVendorWithColorSpace checks that a vendor colour space in
// the JPEG 2000 data is accepted when the image dictionary has a
// ColorSpace.
func TestJPXSourceVendorWithColorSpace(t *testing.T) {
	src := jpxTestImage(6, 5, []int{8}, 33)
	src.ColorSpecs = []jpx.ColorSpec{{Method: 4, Vendor: make([]byte, 16)}}
	d := &Dict{Width: 6, Height: 5, ColorSpace: color.SpaceDeviceGray, Data: NewJPXSource(encodeJPX(t, src))}
	if _, err := roundTrip(t, pdf.V1_7, d, ExtractDict); err != nil {
		t.Fatal(err)
	}
}

// TestJPXSourcePalette checks that a palette image can be written with an
// Indexed colour space, which takes the palette indices, and with a
// colour space matching the palette columns, which takes the entries.
func TestJPXSourcePalette(t *testing.T) {
	src := jpxTestImage(4, 4, []int{2}, 9)
	entries := [][]int32{{0, 128, 128}, {255, 128, 128}, {100, 50, 200}, {30, 220, 90}}
	src.Palette = &jpx.Palette{
		Columns: []jpx.PaletteColumn{{Depth: 8}, {Depth: 8}, {Depth: 8}},
		Entries: entries,
	}
	src.Channels = []jpx.Channel{
		{Component: 0, PaletteColumn: 0, Kind: jpx.ChannelColor, Assoc: 1},
		{Component: 0, PaletteColumn: 1, Kind: jpx.ChannelColor, Assoc: 2},
		{Component: 0, PaletteColumn: 2, Kind: jpx.ChannelColor, Assoc: 3},
	}
	data := encodeJPX(t, src)

	var colors []color.Color
	for _, e := range entries {
		colors = append(colors, color.DeviceRGB{float64(e[0]) / 255, float64(e[1]) / 255, float64(e[2]) / 255})
	}
	indexed, err := color.Indexed(colors)
	if err != nil {
		t.Fatal(err)
	}

	for _, cs := range []color.Space{indexed, color.SpaceDeviceRGB} {
		d := &Dict{Width: 4, Height: 4, ColorSpace: cs, Data: NewJPXSource(data)}
		got, err := roundTrip(t, pdf.V1_7, d, ExtractDict)
		if err != nil {
			t.Fatal(err)
		}
		img, err := got.Load()
		if err != nil {
			t.Fatal(err)
		}
		if img.NComp != cs.Channels() || len(img.Pix) != 16*cs.Channels() {
			t.Errorf("%v: got %d channels, %d samples", cs, img.NComp, len(img.Pix))
		}
	}

	d := &Dict{Width: 4, Height: 4, ColorSpace: color.SpaceDeviceCMYK, Data: NewJPXSource(data)}
	if _, err := roundTrip(t, pdf.V1_7, d, ExtractDict); err == nil {
		t.Error("missing error for colour space not matching the palette")
	}
}
