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
	goimage "image"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/jpx"
	"seehuhn.de/go/pdf"
)

// packSamples returns the samples as image data rows of the given depth.
func packSamples(samples []int32, width, bpc int) func(io.Writer) error {
	return func(w io.Writer) error {
		row := NewPixelRow(width, bpc)
		for y := range len(samples) / width {
			row.Reset()
			for _, v := range samples[y*width : (y+1)*width] {
				row.AppendBits(uint16(v))
			}
			if _, err := w.Write(row.Bytes()); err != nil {
				return err
			}
		}
		return nil
	}
}

// flateMaskAlpha returns the alpha image which the given 1-bit samples
// give as a Flate-encoded stencil mask.
func flateMaskAlpha(t *testing.T, samples []int32, width, height int, inverted bool) *goimage.Alpha {
	t.Helper()
	m := &Mask{
		Width:    width,
		Height:   height,
		Inverted: inverted,
		Source:   &FlateSource{Width: width, Colors: 1, BitsPerComponent: 1, WriteData: packSamples(samples, width, 1)},
	}
	got, err := roundTrip(t, pdf.V2_0, m, ExtractMask)
	if err != nil {
		t.Fatal(err)
	}
	alpha, err := got.LoadAlpha()
	if err != nil {
		t.Fatal(err)
	}
	return alpha
}

// flateSoftMaskAlpha returns the alpha image which the given samples
// give as a Flate-encoded soft mask.
func flateSoftMaskAlpha(t *testing.T, samples []int32, width, height, bpc int, decode []float64) *goimage.Alpha {
	t.Helper()
	sm := &SoftMask{
		Width:            width,
		Height:           height,
		BitsPerComponent: bpc,
		Decode:           decode,
		Source:           &FlateSource{Width: width, Colors: 1, BitsPerComponent: bpc, WriteData: packSamples(samples, width, bpc)},
	}
	got, err := roundTrip(t, pdf.V2_0, sm, ExtractSoftMask)
	if err != nil {
		t.Fatal(err)
	}
	alpha, err := got.LoadAlpha()
	if err != nil {
		t.Fatal(err)
	}
	return alpha
}

func TestMaskJPX(t *testing.T) {
	src := jpxTestImage(11, 6, []int{1}, 40)
	for _, inverted := range []bool{false, true} {
		var extras pdf.Dict
		if inverted {
			extras = pdf.Dict{"Decode": pdf.Array{pdf.Integer(1), pdf.Integer(0)}}
		}
		m := extractJPXMask(t, encodeJPX(t, src), 11, 6, extras)
		if !m.Source.IsJPX() || m.Inverted != inverted {
			t.Errorf("got IsJPX %t, Inverted %t", m.Source.IsJPX(), m.Inverted)
		}
		got, err := m.LoadAlpha()
		if err != nil {
			t.Fatal(err)
		}
		want := flateMaskAlpha(t, src.Planes[0].Pix, 11, 6, inverted)
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("inverted=%t: alpha differs (-want +got):\n%s", inverted, diff)
		}
	}
}

// TestMaskJPXBitsPerComponent checks that the BitsPerComponent entry of
// a JPXDecode stencil mask is ignored.
func TestMaskJPXBitsPerComponent(t *testing.T) {
	src := jpxTestImage(4, 3, []int{1}, 41)
	m := extractJPXMask(t, encodeJPX(t, src), 4, 3, pdf.Dict{"BitsPerComponent": pdf.Integer(8)})
	if _, err := m.LoadAlpha(); err != nil {
		t.Error(err)
	}
}

func TestMaskJPXInvalid(t *testing.T) {
	pal := jpxTestImage(4, 3, []int{1}, 42)
	pal.Palette = &jpx.Palette{
		Columns: []jpx.PaletteColumn{{Depth: 1}},
		Entries: [][]int32{{1}, {0}},
	}
	pal.Channels[0].PaletteColumn = 0
	for name, src := range map[string]*jpx.Image{
		"two channels": jpxTestImage(4, 3, []int{1, 1}, 43),
		"8 bits":       jpxTestImage(4, 3, []int{8}, 44),
		"palette":      pal,
	} {
		m := extractJPXMask(t, encodeJPX(t, src), 4, 3, nil)
		if _, err := m.LoadAlpha(); err == nil {
			t.Errorf("%s: missing error", name)
		}
	}
}

// TestMaskJPXSize checks that the size of the JPEG 2000 data wins over
// the size given in the mask dictionary.
func TestMaskJPXSize(t *testing.T) {
	src := jpxTestImage(13, 7, []int{1}, 45)
	m := extractJPXMask(t, encodeJPX(t, src), 5, 5, nil)
	got, err := m.LoadAlpha()
	if err != nil {
		t.Fatal(err)
	}
	want := flateMaskAlpha(t, src.Planes[0].Pix, 13, 7, false)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("alpha differs (-want +got):\n%s", diff)
	}
}

func TestMaskJPXSourceRoundTrip(t *testing.T) {
	src := jpxTestImage(9, 4, []int{1}, 46)
	m := &Mask{
		Width:    9,
		Height:   4,
		Inverted: true,
		Source:   NewJPXSource(encodeJPX(t, src)),
	}
	got, err := roundTrip(t, pdf.V2_0, m, ExtractMask)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Source.IsJPX() || !got.Inverted {
		t.Errorf("got IsJPX %t, Inverted %t", got.Source.IsJPX(), got.Inverted)
	}
	alpha, err := got.LoadAlpha()
	if err != nil {
		t.Fatal(err)
	}
	want := flateMaskAlpha(t, src.Planes[0].Pix, 9, 4, true)
	if diff := cmp.Diff(want, alpha); diff != "" {
		t.Errorf("alpha differs (-want +got):\n%s", diff)
	}
}

func TestMaskJPXSourceInvalid(t *testing.T) {
	for name, src := range map[string]*jpx.Image{
		"two channels": jpxTestImage(4, 3, []int{1, 1}, 47),
		"8 bits":       jpxTestImage(4, 3, []int{8}, 48),
		"opacity":      jpxTestImage(4, 3, []int{1, 1}, 49),
	} {
		if name == "opacity" {
			src.Channels[1].Kind = jpx.ChannelOpacity
			src.Channels[1].Type = 1
			src.Channels[1].Assoc = 0
		}
		m := &Mask{Width: 4, Height: 3, Source: NewJPXSource(encodeJPX(t, src))}
		if _, err := roundTrip(t, pdf.V2_0, m, ExtractMask); err == nil {
			t.Errorf("%s: missing error", name)
		}
	}
}

func TestSoftMaskJPX(t *testing.T) {
	type testCase struct {
		name   string
		bits   int
		decode []float64
	}
	cases := []testCase{
		{"8 bits", 8, nil},
		{"16 bits", 16, nil},
		{"8 bits inverted", 8, []float64{1, 0}},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := jpxTestImage(7, 5, []int{tc.bits}, uint64(50+i))
			extras := pdf.Dict{"ColorSpace": pdf.Name("DeviceGray")}
			if tc.decode != nil {
				extras["Decode"] = pdf.Array{pdf.Number(tc.decode[0]), pdf.Number(tc.decode[1])}
			}
			sm := extractJPXSoftMask(t, encodeJPX(t, src), 7, 5, extras)
			if !sm.Source.IsJPX() || sm.BitsPerComponent != 0 {
				t.Errorf("got IsJPX %t, BitsPerComponent %d", sm.Source.IsJPX(), sm.BitsPerComponent)
			}
			got, err := sm.LoadAlpha()
			if err != nil {
				t.Fatal(err)
			}
			want := flateSoftMaskAlpha(t, src.Planes[0].Pix, 7, 5, tc.bits, tc.decode)
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("alpha differs (-want +got):\n%s", diff)
			}
		})
	}
}

// TestSoftMaskJPXDecodeIgnored checks that the Decode array of a
// JPXDecode soft mask without ColorSpace is ignored.
func TestSoftMaskJPXDecodeIgnored(t *testing.T) {
	src := jpxTestImage(4, 3, []int{8}, 55)
	sm := extractJPXSoftMask(t, encodeJPX(t, src), 4, 3, pdf.Dict{
		"Decode": pdf.Array{pdf.Integer(1), pdf.Integer(0)},
	})
	if sm.Decode != nil {
		t.Errorf("Decode = %v, want nil", sm.Decode)
	}
}

func TestSoftMaskJPXInvalid(t *testing.T) {
	pal := jpxTestImage(4, 3, []int{2}, 56)
	pal.Palette = &jpx.Palette{
		Columns: []jpx.PaletteColumn{{Depth: 8}},
		Entries: [][]int32{{0}, {80}, {160}, {255}},
	}
	pal.Channels[0].PaletteColumn = 0
	for name, src := range map[string]*jpx.Image{
		"three channels": jpxTestImage(4, 3, []int{8, 8, 8}, 57),
		"palette":        pal,
	} {
		sm := extractJPXSoftMask(t, encodeJPX(t, src), 4, 3, nil)
		if _, err := sm.LoadAlpha(); err == nil {
			t.Errorf("%s: missing error", name)
		}
	}
}

// TestSoftMaskJPXSize checks that the size of the JPEG 2000 data wins
// over the size given in the soft-mask dictionary.
func TestSoftMaskJPXSize(t *testing.T) {
	src := jpxTestImage(13, 7, []int{8}, 58)
	sm := extractJPXSoftMask(t, encodeJPX(t, src), 5, 5, nil)
	got, err := sm.LoadAlpha()
	if err != nil {
		t.Fatal(err)
	}
	want := flateSoftMaskAlpha(t, src.Planes[0].Pix, 13, 7, 8, nil)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("alpha differs (-want +got):\n%s", diff)
	}
}

func TestSoftMaskJPXSourceRoundTrip(t *testing.T) {
	src := jpxTestImage(6, 4, []int{8}, 59)
	sm := &SoftMask{
		Width:  6,
		Height: 4,
		Decode: []float64{1, 0},
		Source: NewJPXSource(encodeJPX(t, src)),
	}
	got, err := roundTrip(t, pdf.V2_0, sm, ExtractSoftMask)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Source.IsJPX() || got.BitsPerComponent != 0 {
		t.Errorf("got IsJPX %t, BitsPerComponent %d", got.Source.IsJPX(), got.BitsPerComponent)
	}
	if diff := cmp.Diff(sm.Decode, got.Decode); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
	alpha, err := got.LoadAlpha()
	if err != nil {
		t.Fatal(err)
	}
	want := flateSoftMaskAlpha(t, src.Planes[0].Pix, 6, 4, 8, []float64{1, 0})
	if diff := cmp.Diff(want, alpha); diff != "" {
		t.Errorf("alpha differs (-want +got):\n%s", diff)
	}
}

func TestSoftMaskJPXSourceInvalid(t *testing.T) {
	grey := encodeJPX(t, jpxTestImage(4, 3, []int{8}, 60))
	rgb := encodeJPX(t, jpxTestImage(4, 3, []int{8, 8, 8}, 61))
	cases := map[string]*SoftMask{
		"BitsPerComponent": {Width: 4, Height: 3, BitsPerComponent: 8, Source: NewJPXSource(grey)},
		"three channels":   {Width: 4, Height: 3, Source: NewJPXSource(rgb)},
		"size":             {Width: 5, Height: 3, Source: NewJPXSource(grey)},
	}
	for name, sm := range cases {
		if _, err := roundTrip(t, pdf.V2_0, sm, ExtractSoftMask); err == nil {
			t.Errorf("%s: missing error", name)
		}
	}
}

// TestDictJPXSMaskJPX checks a JPXDecode image whose soft mask is also
// JPXDecode-encoded.
func TestDictJPXSMaskJPX(t *testing.T) {
	img := jpxTestImage(5, 4, []int{8, 8, 8}, 62)
	mask := jpxTestImage(5, 4, []int{8}, 63)
	d := &Dict{
		Width:  5,
		Height: 4,
		Data:   NewJPXSource(encodeJPX(t, img)),
		SMask: &SoftMask{
			Width:  5,
			Height: 4,
			Source: NewJPXSource(encodeJPX(t, mask)),
		},
	}
	got, err := roundTrip(t, pdf.V2_0, d, ExtractDict)
	if err != nil {
		t.Fatal(err)
	}
	data, err := got.Load()
	if err != nil {
		t.Fatal(err)
	}
	wantPix := expectPix(planesOf(img), 8, []float64{0, 1, 0, 1, 0, 1})
	if diff := cmp.Diff(wantPix, data.Pix); diff != "" {
		t.Errorf("pixels differ (-want +got):\n%s", diff)
	}
	if got.SMask == nil {
		t.Fatal("missing SMask")
	}
	alpha, err := got.SMask.LoadAlpha()
	if err != nil {
		t.Fatal(err)
	}
	wantAlpha := flateSoftMaskAlpha(t, mask.Planes[0].Pix, 5, 4, 8, nil)
	if diff := cmp.Diff(wantAlpha, alpha); diff != "" {
		t.Errorf("alpha differs (-want +got):\n%s", diff)
	}
}

// TestSoftMaskAlphaClamped checks that a Decode array mapping samples
// outside [0, 1] gives clamped alpha values.
func TestSoftMaskAlphaClamped(t *testing.T) {
	samples := []int32{0, 50, 200, 255}
	got := flateSoftMaskAlpha(t, samples, 4, 1, 8, []float64{-1, 2})
	want := []uint8{0, 0, 255, 255}
	if !bytes.Equal(got.Pix, want) {
		t.Errorf("got %v, want %v", got.Pix, want)
	}
}
