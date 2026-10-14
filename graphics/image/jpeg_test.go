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
	"errors"
	"fmt"
	"io"
	"testing"

	"seehuhn.de/go/icc"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// dctTestSamples returns smooth test samples, w*h*n bytes.
func dctTestSamples(w, h, n int) []byte {
	buf := make([]byte, 0, w*h*n)
	for y := range h {
		for x := range w {
			for c := range n {
				buf = append(buf, byte(40+x+y+c*20))
			}
		}
	}
	return buf
}

func writerOf(data []byte) func(io.Writer) error {
	return func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	}
}

func TestNewDCTSourceDefaults(t *testing.T) {
	calRGB, err := color.CalRGB(color.WhitePointD65, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	lab, err := color.Lab(color.WhitePointD65, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	indexed, err := color.Indexed([]color.Color{
		color.DeviceRGB{0, 0, 0}, color.DeviceRGB{1, 1, 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	iccRGB, err := color.ICCBased(icc.SRGBv2Profile, nil)
	if err != nil {
		t.Fatal(err)
	}
	tint := &function.Type2{XMin: 0, XMax: 1, C0: []float64{1, 1, 1}, C1: []float64{0, 0, 0}, N: 1}
	sep, err := color.Separation("spot", color.SpaceDeviceRGB, tint)
	if err != nil {
		t.Fatal(err)
	}
	tint4 := &function.Type4{
		Domain:  []float64{0, 1, 0, 1, 0, 1, 0, 1},
		Range:   []float64{0, 1, 0, 1, 0, 1},
		Program: "pop pop pop pop 0.5 0.5 0.5",
	}
	deviceN, err := color.DeviceN([]pdf.Name{"a", "b", "c", "d"}, color.SpaceDeviceRGB, tint4, nil)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name      string
		cs        color.Space
		colors    int
		transform bool
	}{
		{"DeviceGray", color.SpaceDeviceGray, 1, false},
		{"DeviceRGB", color.SpaceDeviceRGB, 3, true},
		{"CalRGB", calRGB, 3, true},
		{"DeviceCMYK", color.SpaceDeviceCMYK, 4, true},
		{"ICCBased3", iccRGB, 3, true},
		{"Lab", lab, 3, false},
		{"Indexed", indexed, 1, false},
		{"Separation", sep, 1, false},
		{"DeviceN", deviceN, 4, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := NewDCTSource(8, 4, c.cs, nil)
			if s.Colors != c.colors || s.ColorTransform != c.transform {
				t.Errorf("got (%d, %v), want (%d, %v)",
					s.Colors, s.ColorTransform, c.colors, c.transform)
			}
			if s.Width != 8 || s.Height != 4 {
				t.Errorf("wrong size %dx%d", s.Width, s.Height)
			}
		})
	}
}

func TestDCTSourceRoundTrip(t *testing.T) {
	const width, height = 24, 16
	cases := []struct {
		cs        color.Space
		transform bool
	}{
		{color.SpaceDeviceGray, false},
		{color.SpaceDeviceRGB, true},
		{color.SpaceDeviceRGB, false},
		{color.SpaceDeviceCMYK, true},
		{color.SpaceDeviceCMYK, false},
	}
	for _, c := range cases {
		name := fmt.Sprintf("%s/%v", c.cs.Family(), c.transform)
		t.Run(name, func(t *testing.T) {
			n := c.cs.Channels()
			samples := dctTestSamples(width, height, n)
			src := NewDCTSource(width, height, c.cs, writerOf(samples))
			src.ColorTransform = c.transform
			src.Quality = 100

			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			rm := pdf.NewResourceManager(w)
			ref, err := rm.Embed(&Dict{
				Width:            width,
				Height:           height,
				ColorSpace:       c.cs,
				BitsPerComponent: 8,
				Data:             src,
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := rm.Close(); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}

			// check the stream dictionary
			obj, err := pdf.Resolve(w, ref)
			if err != nil {
				t.Fatal(err)
			}
			stm, ok := obj.(*pdf.Stream)
			if !ok {
				t.Fatalf("got %T, want stream", obj)
			}
			if stm.Dict["Filter"] != pdf.Name("DCTDecode") {
				t.Errorf("wrong Filter %v", stm.Dict["Filter"])
			}
			parms := stm.Dict["DecodeParms"]
			if n == 1 {
				if parms != nil {
					t.Errorf("unexpected DecodeParms %v", parms)
				}
			} else {
				want := pdf.Integer(0)
				if c.transform {
					want = 1
				}
				pd, _ := parms.(pdf.Dict)
				if pd["ColorTransform"] != want {
					t.Errorf("wrong DecodeParms %v, want ColorTransform %d", parms, want)
				}
			}

			// check the pixels
			x := pdf.NewExtractor(w)
			img, err := ExtractDict(pdf.CursorAt(x, nil), ref, false)
			if err != nil {
				t.Fatal(err)
			}
			got, err := img.Data.Pixels()
			if err != nil {
				t.Fatal(err)
			}
			checkClose(t, got, samples)
		})
	}
}

func checkClose(t *testing.T, got, want []byte) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d bytes, want %d", len(got), len(want))
	}
	for i := range got {
		if d := int(got[i]) - int(want[i]); d < -3 || d > 3 {
			t.Fatalf("sample %d: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestDCTSourcePixels(t *testing.T) {
	const width, height = 19, 13
	for _, n := range []int{1, 3, 4} {
		for _, transform := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/%v", n, transform), func(t *testing.T) {
				samples := dctTestSamples(width, height, n)
				src := &DCTSource{
					WriteData:      writerOf(samples),
					Width:          width,
					Height:         height,
					Colors:         n,
					ColorTransform: transform,
					Quality:        100,
				}
				got, err := src.Pixels()
				if err != nil {
					t.Fatal(err)
				}
				checkClose(t, got, samples)
			})
		}
	}
}

type embedFunc func(*pdf.EmbedHelper) (pdf.Native, error)

func (f embedFunc) Embed(e *pdf.EmbedHelper) (pdf.Native, error) { return f(e) }

func TestDCTSourceErrors(t *testing.T) {
	good := dctTestSamples(8, 8, 3)
	failure := errors.New("write failure")
	cases := []struct {
		name string
		src  *DCTSource
	}{
		{"colors", &DCTSource{WriteData: writerOf(good), Width: 8, Height: 8, Colors: 2}},
		{"width", &DCTSource{WriteData: writerOf(good), Width: 0, Height: 8, Colors: 3}},
		{"height", &DCTSource{WriteData: writerOf(good), Width: 8, Height: -1, Colors: 3}},
		{"nil WriteData", &DCTSource{Width: 8, Height: 8, Colors: 3}},
		{"too few bytes", &DCTSource{WriteData: writerOf(good[:100]), Width: 8, Height: 8, Colors: 3}},
		{"too many bytes", &DCTSource{WriteData: writerOf(append(bytes.Clone(good), 1)), Width: 8, Height: 8, Colors: 3}},
		{"WriteData fails", &DCTSource{
			WriteData: func(w io.Writer) error { return failure },
			Width:     8, Height: 8, Colors: 3,
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			rm := pdf.NewResourceManager(w)
			_, err := rm.Embed(embedFunc(func(e *pdf.EmbedHelper) (pdf.Native, error) {
				ref := e.Alloc()
				return ref, c.src.WriteStream(e, ref, pdf.Dict{})
			}))
			if err == nil {
				t.Error("WriteStream: expected error")
			}
			if _, err := c.src.Pixels(); err == nil {
				t.Error("Pixels: expected error")
			}
		})
	}
}

func TestDCTSourceCheckGeometry(t *testing.T) {
	s := &DCTSource{Width: 8, Height: 4, Colors: 3}
	cases := []struct {
		name                          string
		width, height, channels, bits int
		ok                            bool
	}{
		{"match", 8, 4, 3, 8, true},
		{"bits", 8, 4, 3, 16, false},
		{"channels", 8, 4, 1, 8, false},
		{"width", 9, 4, 3, 8, false},
		{"height", 8, 5, 3, 8, false},
	}
	for _, c := range cases {
		err := s.CheckGeometry(c.width, c.height, c.channels, c.bits)
		if (err == nil) != c.ok {
			t.Errorf("%s: got error %v, want ok=%t", c.name, err, c.ok)
		}
	}
}

func TestSoftMaskDCTSource(t *testing.T) {
	cases := []struct {
		name   string
		colors int
		bits   int
		ok     bool
	}{
		{"gray 8", 1, 8, true},
		{"rgb", 3, 8, false},
		{"16 bit", 1, 16, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			rm := pdf.NewResourceManager(w)
			sm := &SoftMask{
				Width:            8,
				Height:           8,
				BitsPerComponent: c.bits,
				Source: &DCTSource{
					WriteData: writerOf(dctTestSamples(8, 8, c.colors)),
					Width:     8, Height: 8, Colors: c.colors,
				},
			}
			_, err := rm.Embed(sm)
			if (err == nil) != c.ok {
				t.Errorf("got error %v, want ok=%t", err, c.ok)
			}
		})
	}
}

func TestMaskDCTSource(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	m := &Mask{
		Width:  8,
		Height: 8,
		Source: &DCTSource{WriteData: writerOf(dctTestSamples(8, 8, 1)), Width: 8, Height: 8, Colors: 1},
	}
	if _, err := rm.Embed(m); err == nil {
		t.Error("expected error for DCTSource image mask")
	}
}
