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
	gocolor "image/color"
	"io"
	"testing"

	"seehuhn.de/go/pdf/graphics/color"
)

func TestNewData(t *testing.T) {
	for _, cs := range []struct {
		name  string
		space color.Space
	}{
		{"DeviceGray", color.SpaceDeviceGray},
		{"DeviceRGB", color.SpaceDeviceRGB},
		{"DeviceCMYK", color.SpaceDeviceCMYK},
	} {
		t.Run(cs.name, func(t *testing.T) {
			d := NewData(cs.space, 4, 3)
			if d.Bounds() != image.Rect(0, 0, 4, 3) {
				t.Errorf("bounds = %v, want (0,0)-(4,3)", d.Bounds())
			}
			if d.ColorModel() != cs.space {
				t.Errorf("color model mismatch")
			}
		})
	}
}

func TestDataSetAt(t *testing.T) {
	d := NewData(color.SpaceDeviceRGB, 3, 2)

	// set a pixel
	col := color.DeviceRGB{0.25, 0.5, 0.75}
	d.Set(1, 0, col)

	// read it back
	got := d.At(1, 0).(color.Color)
	wantVals, _ := color.Values(col)
	gotVals, _ := color.Values(got)

	if !floatsClose(gotVals, wantVals, 1e-9) {
		t.Errorf("At(1,0) = %v, want %v", gotVals, wantVals)
	}

	// verify other pixels remain at default
	other := d.At(0, 0).(color.Color)
	defVals, _ := color.Values(color.SpaceDeviceRGB.Default())
	otherVals, _ := color.Values(other)
	if !floatsClose(otherVals, defVals, 1e-9) {
		t.Errorf("At(0,0) = %v, want default %v", otherVals, defVals)
	}
}

func TestDataOutOfBounds(t *testing.T) {
	d := NewData(color.SpaceDeviceGray, 2, 2)

	// out-of-bounds At returns default
	got := d.At(-1, 0)
	def := color.SpaceDeviceGray.Default()
	gr, gg, gb, ga := got.RGBA()
	dr, dg, db, da := def.RGBA()
	if gr != dr || gg != dg || gb != db || ga != da {
		t.Errorf("out-of-bounds At returned %v, want default", got)
	}

	got = d.At(2, 0)
	gr, gg, gb, ga = got.RGBA()
	if gr != dr || gg != dg || gb != db || ga != da {
		t.Errorf("out-of-bounds At(2,0) returned %v, want default", got)
	}

	// out-of-bounds Set is a no-op (should not panic)
	d.Set(-1, 0, color.DeviceGray(0.5))
	d.Set(0, -1, color.DeviceGray(0.5))
	d.Set(2, 0, color.DeviceGray(0.5))
	d.Set(0, 2, color.DeviceGray(0.5))
}

func TestDataLoadGray8(t *testing.T) {
	src := NewData(color.SpaceDeviceGray, 3, 2)
	src.Set(0, 0, color.DeviceGray(0))
	src.Set(1, 0, color.DeviceGray(0.5))
	src.Set(2, 0, color.DeviceGray(1))
	src.Set(0, 1, color.DeviceGray(0.25))
	src.Set(1, 1, color.DeviceGray(0.75))
	src.Set(2, 1, color.DeviceGray(0))

	dict := FromImage(src, color.SpaceDeviceGray, 8)
	loaded, err := dict.Load()
	if err != nil {
		t.Fatal(err)
	}

	if loaded.Bounds() != src.Bounds() {
		t.Fatalf("bounds mismatch: %v vs %v", loaded.Bounds(), src.Bounds())
	}

	for y := range 2 {
		for x := range 3 {
			srcVals, _ := color.Values(src.At(x, y).(color.Color))
			loadVals, _ := color.Values(loaded.At(x, y).(color.Color))
			if !floatsClose(srcVals, loadVals, 1.0/255+1e-9) {
				t.Errorf("pixel (%d,%d): loaded=%v, want=%v", x, y, loadVals, srcVals)
			}
		}
	}
}

func TestDataLoadRGB8(t *testing.T) {
	dict := &Dict{
		Width:            2,
		Height:           2,
		ColorSpace:       color.SpaceDeviceRGB,
		BitsPerComponent: 8,
		WriteData: func(w io.Writer) error {
			// row 0: red, green
			// row 1: blue, white
			_, err := w.Write([]byte{
				255, 0, 0, 0, 255, 0,
				0, 0, 255, 255, 255, 255,
			})
			return err
		},
	}

	loaded, err := dict.Load()
	if err != nil {
		t.Fatal(err)
	}

	type pixel struct {
		x, y int
		want color.DeviceRGB
	}
	tests := []pixel{
		{0, 0, color.DeviceRGB{1, 0, 0}},
		{1, 0, color.DeviceRGB{0, 1, 0}},
		{0, 1, color.DeviceRGB{0, 0, 1}},
		{1, 1, color.DeviceRGB{1, 1, 1}},
	}

	for _, tc := range tests {
		got, _ := color.Values(loaded.At(tc.x, tc.y).(color.Color))
		want, _ := color.Values(tc.want)
		if !floatsClose(got, want, 1e-9) {
			t.Errorf("pixel (%d,%d): got %v, want %v", tc.x, tc.y, got, want)
		}
	}
}

func TestDataLoadCMYK8(t *testing.T) {
	dict := &Dict{
		Width:            2,
		Height:           1,
		ColorSpace:       color.SpaceDeviceCMYK,
		BitsPerComponent: 8,
		WriteData: func(w io.Writer) error {
			// cyan, black
			_, err := w.Write([]byte{
				255, 0, 0, 0, 0, 0, 0, 255,
			})
			return err
		},
	}

	loaded, err := dict.Load()
	if err != nil {
		t.Fatal(err)
	}

	gotVals, _ := color.Values(loaded.At(0, 0).(color.Color))
	wantVals := []float64{1, 0, 0, 0}
	if !floatsClose(gotVals, wantVals, 1e-9) {
		t.Errorf("pixel (0,0): got %v, want %v", gotVals, wantVals)
	}

	gotVals, _ = color.Values(loaded.At(1, 0).(color.Color))
	wantVals = []float64{0, 0, 0, 1}
	if !floatsClose(gotVals, wantVals, 1e-9) {
		t.Errorf("pixel (1,0): got %v, want %v", gotVals, wantVals)
	}
}

func TestDataLoad1Bit(t *testing.T) {
	dict := &Dict{
		Width:            8,
		Height:           1,
		ColorSpace:       color.SpaceDeviceGray,
		BitsPerComponent: 1,
		WriteData: func(w io.Writer) error {
			// 10101010 = 0xAA
			_, err := w.Write([]byte{0xAA})
			return err
		},
	}

	loaded, err := dict.Load()
	if err != nil {
		t.Fatal(err)
	}

	for x := range 8 {
		vals, _ := color.Values(loaded.At(x, 0).(color.Color))
		var want float64
		if x%2 == 0 {
			want = 1 // bit=1 -> value=1
		} else {
			want = 0 // bit=0 -> value=0
		}
		if !floatsClose(vals, []float64{want}, 1e-9) {
			t.Errorf("pixel (%d,0): got %v, want %v", x, vals[0], want)
		}
	}
}

func TestDataLoad16Bit(t *testing.T) {
	dict := &Dict{
		Width:            2,
		Height:           1,
		ColorSpace:       color.SpaceDeviceGray,
		BitsPerComponent: 16,
		WriteData: func(w io.Writer) error {
			// pixel 0: 0x0000 = 0, pixel 1: 0xFFFF = 1
			_, err := w.Write([]byte{0, 0, 0xFF, 0xFF})
			return err
		},
	}

	loaded, err := dict.Load()
	if err != nil {
		t.Fatal(err)
	}

	vals0, _ := color.Values(loaded.At(0, 0).(color.Color))
	if !floatsClose(vals0, []float64{0}, 1e-9) {
		t.Errorf("pixel (0,0): got %v, want 0", vals0[0])
	}

	vals1, _ := color.Values(loaded.At(1, 0).(color.Color))
	if !floatsClose(vals1, []float64{1}, 1e-9) {
		t.Errorf("pixel (1,0): got %v, want 1", vals1[0])
	}
}

func TestDataLoadDecode(t *testing.T) {
	dict := &Dict{
		Width:            2,
		Height:           1,
		ColorSpace:       color.SpaceDeviceGray,
		BitsPerComponent: 8,
		Decode:           []float64{1, 0}, // inverted
		WriteData: func(w io.Writer) error {
			// sample 0 -> decode[0]=1, sample 255 -> decode[1]=0
			_, err := w.Write([]byte{0, 255})
			return err
		},
	}

	loaded, err := dict.Load()
	if err != nil {
		t.Fatal(err)
	}

	// pixel 0: sample=0 -> value=1 (inverted)
	vals0, _ := color.Values(loaded.At(0, 0).(color.Color))
	if !floatsClose(vals0, []float64{1}, 1e-9) {
		t.Errorf("pixel (0,0): got %v, want 1", vals0[0])
	}

	// pixel 1: sample=255 -> value=0 (inverted)
	vals1, _ := color.Values(loaded.At(1, 0).(color.Color))
	if !floatsClose(vals1, []float64{0}, 1e-9) {
		t.Errorf("pixel (1,0): got %v, want 0", vals1[0])
	}
}

func TestDataLoadRoundTrip(t *testing.T) {
	// create an image, encode it via FromImage, then Load it back
	src := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := range 4 {
		for x := range 4 {
			src.SetNRGBA(x, y, gocolor.NRGBA{
				R: uint8(x * 80),
				G: uint8(y * 80),
				B: uint8((x + y) * 40),
				A: 255,
			})
		}
	}

	dict := FromImage(src, color.SpaceDeviceRGB, 8)
	loaded, err := dict.Load()
	if err != nil {
		t.Fatal(err)
	}

	for y := range 4 {
		for x := range 4 {
			srcR, srcG, srcB, _ := src.At(x, y).RGBA()
			gotR, gotG, gotB, _ := loaded.At(x, y).RGBA()
			// allow tolerance for 8-bit quantization
			if diff32(srcR, gotR) > 0x200 || diff32(srcG, gotG) > 0x200 || diff32(srcB, gotB) > 0x200 {
				t.Errorf("pixel (%d,%d): src=(%d,%d,%d) got=(%d,%d,%d)",
					x, y, srcR, srcG, srcB, gotR, gotG, gotB)
			}
		}
	}
}

func TestDataSetConverts(t *testing.T) {
	// setting a DeviceRGB color on a DeviceGray image should convert
	d := NewData(color.SpaceDeviceGray, 1, 1)
	d.Set(0, 0, color.DeviceRGB{1, 1, 1})

	vals, _ := color.Values(d.At(0, 0).(color.Color))
	if len(vals) != 1 {
		t.Fatalf("expected 1 component, got %d", len(vals))
	}
	// white RGB -> should be close to gray 1
	if vals[0] < 0.9 {
		t.Errorf("expected gray close to 1, got %v", vals[0])
	}
}

func TestDataLoadPatternError(t *testing.T) {
	dict := &Dict{
		Width:            1,
		Height:           1,
		ColorSpace:       color.SpacePatternColored,
		BitsPerComponent: 8,
		WriteData: func(w io.Writer) error {
			return nil
		},
	}
	_, err := dict.Load()
	if err == nil {
		t.Error("expected error for pattern color space")
	}
}

func TestSampleNearest(t *testing.T) {
	// 3x2 gray image: row 0 = [0, 0.5, 1], row 1 = [0.25, 0.75, 0]
	d := NewData(color.SpaceDeviceGray, 3, 2)
	d.Set(0, 0, color.DeviceGray(0))
	d.Set(1, 0, color.DeviceGray(0.5))
	d.Set(2, 0, color.DeviceGray(1))
	d.Set(0, 1, color.DeviceGray(0.25))
	d.Set(1, 1, color.DeviceGray(0.75))
	d.Set(2, 1, color.DeviceGray(0))

	dst := make([]float64, 1)

	// in-bounds sample
	d.SampleNearest(1, 0, dst)
	if !floatsClose(dst, []float64{0.5}, 1e-9) {
		t.Errorf("SampleNearest(1,0) = %v, want 0.5", dst[0])
	}

	d.SampleNearest(0, 1, dst)
	if !floatsClose(dst, []float64{0.25}, 1e-9) {
		t.Errorf("SampleNearest(0,1) = %v, want 0.25", dst[0])
	}

	// clamping: negative coords -> (0,0)
	d.SampleNearest(-1, -1, dst)
	if !floatsClose(dst, []float64{0}, 1e-9) {
		t.Errorf("SampleNearest(-1,-1) = %v, want 0", dst[0])
	}

	// clamping: beyond bounds -> edge pixel
	d.SampleNearest(10, 10, dst)
	if !floatsClose(dst, []float64{0}, 1e-9) {
		t.Errorf("SampleNearest(10,10) = %v, want 0 (pixel 2,1)", dst[0])
	}
}

func TestSampleBilinear(t *testing.T) {
	// 2x2 gray image: [0, 1; 0, 1]
	d := NewData(color.SpaceDeviceGray, 2, 2)
	d.Set(0, 0, color.DeviceGray(0))
	d.Set(1, 0, color.DeviceGray(1))
	d.Set(0, 1, color.DeviceGray(0))
	d.Set(1, 1, color.DeviceGray(1))

	dst := make([]float64, 1)

	// center of pixel (0,0): coords (0.5, 0.5) -> value 0
	d.SampleBilinear(0.5, 0.5, dst)
	if !floatsClose(dst, []float64{0}, 1e-9) {
		t.Errorf("SampleBilinear(0.5,0.5) = %v, want 0", dst[0])
	}

	// center of pixel (1,0): coords (1.5, 0.5) -> value 1
	d.SampleBilinear(1.5, 0.5, dst)
	if !floatsClose(dst, []float64{1}, 1e-9) {
		t.Errorf("SampleBilinear(1.5,0.5) = %v, want 1", dst[0])
	}

	// midpoint between pixels (0,0) and (1,0) -> 0.5
	d.SampleBilinear(1.0, 0.5, dst)
	if !floatsClose(dst, []float64{0.5}, 1e-6) {
		t.Errorf("SampleBilinear(1.0,0.5) = %v, want 0.5", dst[0])
	}

	// edge clamping: far out-of-bounds should return edge value
	d.SampleBilinear(-10, 0.5, dst)
	if !floatsClose(dst, []float64{0}, 1e-9) {
		t.Errorf("SampleBilinear(-10,0.5) = %v, want 0", dst[0])
	}
}

func TestSampleBilinearRGB(t *testing.T) {
	// 2x1 RGB image: pixel 0 = red, pixel 1 = blue
	d := NewData(color.SpaceDeviceRGB, 2, 1)
	d.Set(0, 0, color.DeviceRGB{1, 0, 0})
	d.Set(1, 0, color.DeviceRGB{0, 0, 1})

	dst := make([]float64, 3)

	// midpoint between red and blue
	d.SampleBilinear(1.0, 0.5, dst)
	want := []float64{0.5, 0, 0.5}
	if !floatsClose(dst, want, 1e-6) {
		t.Errorf("SampleBilinear(1.0,0.5) = %v, want %v", dst, want)
	}
}

func TestToRGBA(t *testing.T) {
	d := NewData(color.SpaceDeviceRGB, 2, 1)
	d.Set(0, 0, color.DeviceRGB{1, 0, 0})
	d.Set(1, 0, color.DeviceRGB{0, 1, 0})

	rgba := d.ToRGBA()
	if rgba.Bounds() != d.Bounds() {
		t.Fatalf("bounds mismatch: %v vs %v", rgba.Bounds(), d.Bounds())
	}

	// red pixel
	r, g, b, a := rgba.At(0, 0).RGBA()
	if r>>8 != 255 || g>>8 != 0 || b>>8 != 0 || a>>8 != 255 {
		t.Errorf("pixel (0,0): got (%d,%d,%d,%d), want (255,0,0,255)", r>>8, g>>8, b>>8, a>>8)
	}

	// green pixel
	r, g, b, a = rgba.At(1, 0).RGBA()
	if r>>8 != 0 || g>>8 < 127 || b>>8 != 0 || a>>8 != 255 {
		t.Errorf("pixel (1,0): got (%d,%d,%d,%d), want (0,~128+,0,255)", r>>8, g>>8, b>>8, a>>8)
	}
}

func floatsClose(a, b []float64, tol float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		d := a[i] - b[i]
		if d < -tol || d > tol {
			return false
		}
	}
	return true
}

func diff32(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}
