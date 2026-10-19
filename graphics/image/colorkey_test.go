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
	"image"
	"io"
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
)

// rawKeyDict returns a w×h image in cs with bpc-bit samples given by data.
func rawKeyDict(w, h int, cs color.Space, bpc int, data []byte, key []uint16) *Dict {
	return &Dict{
		Width:            w,
		Height:           h,
		ColorSpace:       cs,
		BitsPerComponent: bpc,
		MaskColors:       key,
		Data: &FlateSource{
			WriteData: func(out io.Writer) error {
				_, err := out.Write(data)
				return err
			},
		},
	}
}

// keyPlane evaluates key on interleaved-by-plane samples, giving 0 where
// every channel lies in its range and 255 elsewhere.
func keyPlane(planes [][]int32, key []uint16) []uint8 {
	res := make([]uint8, len(planes[0]))
	for i := range res {
		for c, p := range planes {
			if p[i] < int32(key[2*c]) || p[i] > int32(key[2*c+1]) {
				res[i] = 255
				break
			}
		}
	}
	return res
}

func TestKeyMaskRaw(t *testing.T) {
	cases := []struct {
		name string
		img  *Dict
		want []uint8
	}{
		{
			// 3×2 at 1 bit: rows 101, 011, padded to a byte each
			name: "gray 1 bit",
			img:  rawKeyDict(3, 2, color.SpaceDeviceGray, 1, []byte{0xA0, 0x60}, []uint16{1, 1}),
			want: []uint8{0, 255, 0, 255, 0, 0},
		},
		{
			name: "gray 2 bit",
			img:  rawKeyDict(4, 1, color.SpaceDeviceGray, 2, []byte{0x1B}, []uint16{1, 2}),
			want: []uint8{255, 0, 0, 255},
		},
		{
			name: "gray 4 bit",
			img:  rawKeyDict(3, 1, color.SpaceDeviceGray, 4, []byte{0x5F, 0x50}, []uint16{5, 5}),
			want: []uint8{0, 255, 0},
		},
		{
			// the second pixel matches in two components only
			name: "rgb 8 bit",
			img: rawKeyDict(2, 1, color.SpaceDeviceRGB, 8, []byte{
				200, 10, 30,
				200, 10, 31,
			}, []uint16{200, 200, 0, 10, 30, 30}),
			want: []uint8{0, 255},
		},
		{
			name: "gray 16 bit",
			img:  rawKeyDict(2, 1, color.SpaceDeviceGray, 16, []byte{0x12, 0x34, 0x12, 0x35}, []uint16{0x1234, 0x1234}),
			want: []uint8{0, 255},
		},
		{
			// missing samples count as 0
			name: "short data",
			img:  rawKeyDict(3, 1, color.SpaceDeviceGray, 8, []byte{7}, []uint16{0, 0}),
			want: []uint8{255, 0, 0},
		},
		{
			// a truncated 16-bit sample is padded with zero bytes, as in Load
			name: "short data 16 bit",
			img:  rawKeyDict(2, 1, color.SpaceDeviceGray, 16, []byte{0, 1, 0xFF}, []uint16{0, 0}),
			want: []uint8{255, 255},
		},
		{
			// a missing row counts as 0
			name: "missing row",
			img:  rawKeyDict(2, 2, color.SpaceDeviceGray, 4, []byte{0x12}, []uint16{0, 0}),
			want: []uint8{255, 255, 0, 0},
		},
		{
			name: "bad length",
			img:  rawKeyDict(1, 1, color.SpaceDeviceRGB, 8, []byte{0, 0, 0}, []uint16{0, 255}),
		},
		{
			name: "no key",
			img:  rawKeyDict(1, 1, color.SpaceDeviceGray, 8, []byte{0}, nil),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// reduce is ignored for non-JPX data
			data, err := c.img.LoadReduced(2)
			if err != nil {
				t.Fatal(err)
			}
			got := data.KeyMask
			if c.want == nil {
				if got != nil {
					t.Errorf("got a mask, want none")
				}
				return
			}
			if got == nil {
				t.Fatal("no mask")
			}
			if got.Rect != image.Rect(0, 0, c.img.Width, c.img.Height) {
				t.Errorf("mask size %v, want %d×%d", got.Rect, c.img.Width, c.img.Height)
			}
			if diff := cmp.Diff(c.want, got.Pix); diff != "" {
				t.Errorf("mask differs (-want +got):\n%s", diff)
			}
		})
	}
}

func TestKeyMaskJPX(t *testing.T) {
	rgb := jpxTestImage(6, 5, []int{8, 8, 8}, 40)
	// make two pixels carry the keyed colour
	for _, i := range []int{3, 17} {
		rgb.Planes[0].Pix[i] = 10
		rgb.Planes[1].Pix[i] = 20
		rgb.Planes[2].Pix[i] = 30
	}
	rgbKey := []uint16{10, 10, 20, 20, 30, 30}
	rgbWant := make([]uint8, 6*5)
	for i := range rgbWant {
		if i != 3 && i != 17 {
			rgbWant[i] = 255
		}
	}
	// make sure the random samples do not match by chance
	if diff := cmp.Diff(rgbWant, keyPlane(planesOf(rgb), rgbKey)); diff != "" {
		t.Fatalf("test image matches the key unexpectedly:\n%s", diff)
	}

	gray16 := jpxTestImage(4, 3, []int{16}, 41)
	gray16Key := []uint16{0, 30000}

	gray4 := jpxTestImage(4, 3, []int{4}, 42)
	// the upper end lies beyond the 4-bit range
	gray4Key := []uint16{5, 300}

	pal := paletteImage(10)
	palKey := []uint16{2, 4}

	indexed := pdf.Array{pdf.Name("Indexed"), pdf.Name("DeviceRGB"), pdf.Integer(9), pdf.String(make([]byte, 30))}

	expKey := []uint16{0, 4, 251, 255, 0, 255}

	cases := []struct {
		name   string
		src    []byte
		w, h   int
		extras pdf.Dict
		key    []uint16
		want   []uint8
	}{
		{"rgb 8 bit", encodeJPX(t, rgb), 6, 5, nil, rgbKey, rgbWant},
		{"explicit rgb", encodeJPX(t, rgb), 6, 5, pdf.Dict{"ColorSpace": pdf.Name("DeviceRGB")},
			rgbKey, rgbWant},
		{"gray 16 bit", encodeJPX(t, gray16), 4, 3, nil, gray16Key,
			keyPlane(planesOf(gray16), gray16Key)},
		{"gray 4 bit", encodeJPX(t, gray4), 4, 3, nil, gray4Key,
			keyPlane(planesOf(gray4), gray4Key)},
		{"palette", encodeJPX(t, pal), 6, 5, nil, palKey,
			keyPlane(planesOf(pal), palKey)},
		{"explicit indexed", encodeJPX(t, pal), 6, 5, pdf.Dict{"ColorSpace": indexed}, palKey,
			keyPlane(planesOf(pal), palKey)},
		{"palette expanded", encodeJPX(t, pal), 6, 5, pdf.Dict{"ColorSpace": pdf.Name("DeviceRGB")},
			expKey, keyPlane(expanded(pal), expKey)},
		{"large palette expanded", encodeJPX(t, paletteImage(300)), 6, 5, nil,
			expKey, keyPlane(expanded(paletteImage(300)), expKey)},
		{"bad length", encodeJPX(t, rgb), 6, 5, nil, []uint16{0, 255}, nil},
		{"smask in data", encodeJPX(t, opacityImage(false)), 5, 4, pdf.Dict{"SMaskInData": pdf.Integer(1)},
			[]uint16{0, 127, 0, 255, 0, 255},
			keyPlane(planesOf(opacityImage(false))[:3], []uint16{0, 127, 0, 255, 0, 255})},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := extractJPX(t, c.src, c.w, c.h, c.extras)
			if _, ok := c.extras["ColorSpace"]; ok && d.ColorSpace == nil {
				t.Fatal("ColorSpace not extracted")
			}
			d.MaskColors = c.key
			data, err := d.Load()
			if err != nil {
				t.Fatal(err)
			}
			got := data.KeyMask
			if c.want == nil {
				if got != nil {
					t.Errorf("got a mask, want none")
				}
				return
			}
			if got == nil {
				t.Fatal("no mask")
			}
			if got.Rect != image.Rect(0, 0, c.w, c.h) {
				t.Errorf("mask size %v, want %d×%d", got.Rect, c.w, c.h)
			}
			if diff := cmp.Diff(c.want, got.Pix); diff != "" {
				t.Errorf("mask differs (-want +got):\n%s", diff)
			}
		})
	}
}

func TestKeyMaskJPXReduced(t *testing.T) {
	d := extractJPX(t, gradientJPX(t, false), 64, 64, nil)
	key := []uint16{0, 100, 0, 255, 50, 150}
	d.MaskColors = key

	data := mustLoadReduced(t, d, 1)
	got := data.KeyMask
	if got == nil || got.Rect != data.Rect {
		t.Fatalf("mask %v, want size %v", got, data.Rect)
	}

	// the reduced 8-bit samples, recovered from the decoded data
	planes := make([][]int32, 3)
	for i := 0; i < len(data.Pix); i += 3 {
		for c := range 3 {
			planes[c] = append(planes[c], int32(math.Round(float64(data.Pix[i+c])*255)))
		}
	}
	if diff := cmp.Diff(keyPlane(planes, key), got.Pix); diff != "" {
		t.Errorf("mask differs (-want +got):\n%s", diff)
	}
}
