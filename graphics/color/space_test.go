// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package color

import (
	"bytes"
	"fmt"
	stdcolor "image/color"
	"math"
	"reflect"
	"testing"

	"seehuhn.de/go/icc"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// color.Space implements pdf.Embedder
var (
	_ pdf.Embedder = Space(nil)
)

// color.Space implements stdcolor.Model
var (
	_ stdcolor.Model = Space(nil)
)

// The following types implement the ColorSpace interface:
var (
	_ Space = spaceDeviceGray{}
	_ Space = spaceDeviceRGB{}
	_ Space = spaceDeviceCMYK{}
	_ Space = (*SpaceCalGray)(nil)
	_ Space = (*SpaceCalRGB)(nil)
	_ Space = (*SpaceLab)(nil)
	_ Space = (*SpaceICCBased)(nil)
	_ Space = spaceSRGB{} // a special case of ICCBased (built-in profiles)
	_ Space = spacePatternColored{}
	_ Space = spacePatternUncolored{}
	_ Space = (*SpaceIndexed)(nil)
	_ Space = (*SpaceSeparation)(nil)
	_ Space = (*SpaceDeviceN)(nil)
)

var testColorSpaces = []Space{
	spaceDeviceGray{},

	spaceDeviceRGB{},

	spaceDeviceCMYK{},

	must(CalGray(WhitePointD65, nil, 1)),
	must(CalGray(WhitePointD65, []float64{0.1, 0.1, 0.1}, 1.2)),

	must(CalRGB(WhitePointD50, nil, nil, nil)),
	must(CalRGB(WhitePointD50, []float64{0.1, 0.1, 0.1}, []float64{1.2, 1.1, 1.0},
		[]float64{0.9, 0.1, 0, 0, 1, 0, 0, 0, 1})),

	must(Lab(WhitePointD65, nil, nil)),
	must(Lab(WhitePointD65, []float64{0.1, 0, 0}, []float64{-90, 90, -110, 110})),

	must(ICCBased(icc.SRGBv2Profile, nil)),
	must(ICCBased(icc.SRGBv4Profile, nil)),

	spacePatternColored{},

	spacePatternUncolored{base: spaceDeviceGray{}},
	spacePatternUncolored{base: must(CalGray(WhitePointD65, nil, 1.2))},

	must(Indexed([]Color{DeviceRGB{0, 0, 0}, DeviceRGB{1, 1, 1}})),

	must(Separation("foo", SpaceDeviceRGB, &function.Type2{
		XMin: 0,
		XMax: 1,
		C0:   []float64{1, 0, 0},
		C1:   []float64{0, 1, 0},
		N:    1,
	})),

	must(DeviceN([]pdf.Name{"bar"}, SpaceDeviceRGB, &function.Type2{
		XMin: 0,
		XMax: 1,
		C0:   []float64{1, 0, 0},
		C1:   []float64{0, 1, 0},
		N:    1,
	}, nil)),
	// TODO(voss): DeviceN colour spaces
}

func TestDecodeSpace(t *testing.T) {
	for i, space := range testColorSpaces {
		t.Run(fmt.Sprintf("%02d-%s", i, space.Family()), func(t *testing.T) {
			r, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			rm := pdf.NewResourceManager(r)

			obj, err := rm.Embed(space)
			if err != nil {
				t.Fatal(err)
			}

			x := pdf.NewExtractor(r)
			space2, err := ExtractSpace(x, obj)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(space, space2) {
				t.Errorf("got %#v, want %#v", space2, space)
			}
		})
	}
}

func must(space Space, err error) Space {
	if err != nil {
		panic(err)
	}
	return space
}

func spaceRoundTrip(t *testing.T, version pdf.Version, space Space) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(w)

	obj, err := rm.Embed(space)
	if err != nil {
		if pdf.IsWrongVersion(err) {
			t.Skip("version not supported")
		}
		t.Fatalf("embed failed: %v", err)
	}
	err = rm.Close()
	if err != nil {
		t.Fatalf("close resource manager failed: %v", err)
	}

	x := pdf.NewExtractor(w)
	decoded, err := ExtractSpace(x, obj)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	if !SpacesEqual(space, decoded) {
		t.Errorf("round trip failed:\n  got:  %#v\n  want: %#v", decoded, space)
	}
}

func TestSpaceRoundTrip(t *testing.T) {
	for i, space := range testColorSpaces {
		for _, version := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
			name := fmt.Sprintf("%02d-%s-v%s", i, space.Family(), version)
			t.Run(name, func(t *testing.T) {
				spaceRoundTrip(t, version, space)
			})
		}
	}
}

// TestSpaceConvertIdentity verifies that converting the default colour
// of each space returns an equivalent colour.
func TestSpaceConvertIdentity(t *testing.T) {
	for i, space := range testColorSpaces {
		t.Run(fmt.Sprintf("%02d-%s", i, space.Family()), func(t *testing.T) {
			def := space.Default()
			converted := space.Convert(def)

			// the converted colour should have matching RGBA values
			r1, g1, b1, a1 := def.RGBA()
			r2, g2, b2, a2 := converted.RGBA()

			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				t.Errorf("Convert(Default()) != Default()\n  got:  RGBA(%d,%d,%d,%d)\n  want: RGBA(%d,%d,%d,%d)",
					r2, g2, b2, a2, r1, g1, b1, a1)
			}
		})
	}
}

// TestSpaceConvertKnownValues tests conversion with known input/output values.
func TestSpaceConvertKnownValues(t *testing.T) {
	// test DeviceGray conversion
	t.Run("DeviceGray-from-white", func(t *testing.T) {
		white := stdcolor.White
		result := SpaceDeviceGray.Convert(white)
		gray, ok := result.(DeviceGray)
		if !ok {
			t.Fatalf("expected DeviceGray, got %T", result)
		}
		if gray < 0.99 {
			t.Errorf("white -> DeviceGray = %f, want ~1.0", gray)
		}
	})

	t.Run("DeviceGray-from-black", func(t *testing.T) {
		black := stdcolor.Black
		result := SpaceDeviceGray.Convert(black)
		gray, ok := result.(DeviceGray)
		if !ok {
			t.Fatalf("expected DeviceGray, got %T", result)
		}
		if gray > 0.01 {
			t.Errorf("black -> DeviceGray = %f, want ~0.0", gray)
		}
	})

	// test DeviceRGB conversion
	t.Run("DeviceRGB-from-red", func(t *testing.T) {
		red := stdcolor.RGBA{R: 255, G: 0, B: 0, A: 255}
		result := SpaceDeviceRGB.Convert(red)
		rgb, ok := result.(DeviceRGB)
		if !ok {
			t.Fatalf("expected DeviceRGB, got %T", result)
		}
		if rgb[0] < 0.99 || rgb[1] > 0.01 || rgb[2] > 0.01 {
			t.Errorf("red -> DeviceRGB = %v, want [1,0,0]", rgb)
		}
	})

	// test DeviceCMYK conversion
	t.Run("DeviceCMYK-from-white", func(t *testing.T) {
		white := stdcolor.White
		result := SpaceDeviceCMYK.Convert(white)
		cmyk, ok := result.(DeviceCMYK)
		if !ok {
			t.Fatalf("expected DeviceCMYK, got %T", result)
		}
		// white = no ink
		if cmyk[0] > 0.01 || cmyk[1] > 0.01 || cmyk[2] > 0.01 || cmyk[3] > 0.01 {
			t.Errorf("white -> DeviceCMYK = %v, want [0,0,0,0]", cmyk)
		}
	})

	t.Run("DeviceCMYK-from-black", func(t *testing.T) {
		black := stdcolor.Black
		result := SpaceDeviceCMYK.Convert(black)
		cmyk, ok := result.(DeviceCMYK)
		if !ok {
			t.Fatalf("expected DeviceCMYK, got %T", result)
		}
		// black = full K
		if cmyk[3] < 0.99 {
			t.Errorf("black -> DeviceCMYK = %v, want [_,_,_,1]", cmyk)
		}
	})
}

// TestSpaceConvertRoundTrip tests that converting a colour back and forth
// produces stable results.
func TestSpaceConvertRoundTrip(t *testing.T) {
	colors := []stdcolor.Color{
		stdcolor.White,
		stdcolor.Black,
		stdcolor.RGBA{R: 255, G: 0, B: 0, A: 255},
		stdcolor.RGBA{R: 0, G: 255, B: 0, A: 255},
		stdcolor.RGBA{R: 0, G: 0, B: 255, A: 255},
		stdcolor.RGBA{R: 128, G: 128, B: 128, A: 255},
	}

	for i, space := range testColorSpaces {
		for j, c := range colors {
			name := fmt.Sprintf("%02d-%s-color%d", i, space.Family(), j)
			t.Run(name, func(t *testing.T) {
				// convert to space
				c1 := space.Convert(c)
				// convert again (should be stable)
				c2 := space.Convert(c1)

				r1, g1, b1, a1 := c1.RGBA()
				r2, g2, b2, a2 := c2.RGBA()

				if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
					t.Errorf("Convert not idempotent:\n  first:  RGBA(%d,%d,%d,%d)\n  second: RGBA(%d,%d,%d,%d)",
						r1, g1, b1, a1, r2, g2, b2, a2)
				}
			})
		}
	}
}

// TestConvertPreservesApproximateColor tests that conversion approximately
// preserves colour appearance (RGBA values are similar).
func TestConvertPreservesApproximateColor(t *testing.T) {
	// tolerance for RGBA comparison (allowing for gamut mapping)
	const tolerance = 0.15 * 65535.0

	colors := []stdcolor.Color{
		stdcolor.RGBA{R: 128, G: 128, B: 128, A: 255}, // neutral gray
		stdcolor.White,
		stdcolor.Black,
	}

	// test only spaces that can reasonably represent arbitrary colours
	representableSpaces := []Space{
		SpaceDeviceGray,
		SpaceDeviceRGB,
		SpaceSRGB,
	}

	for _, space := range representableSpaces {
		for j, c := range colors {
			name := fmt.Sprintf("%s-color%d", space.Family(), j)
			t.Run(name, func(t *testing.T) {
				converted := space.Convert(c)

				r1, g1, b1, _ := c.RGBA()
				r2, g2, b2, _ := converted.RGBA()

				dr := math.Abs(float64(r1) - float64(r2))
				dg := math.Abs(float64(g1) - float64(g2))
				db := math.Abs(float64(b1) - float64(b2))

				if dr > tolerance || dg > tolerance || db > tolerance {
					t.Errorf("colour not preserved:\n  input:  RGBA(%d,%d,%d)\n  output: RGBA(%d,%d,%d)\n  delta: (%.0f,%.0f,%.0f)",
						r1, g1, b1, r2, g2, b2, dr, dg, db)
				}
			})
		}
	}
}

func FuzzSpaceRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	for _, version := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for _, space := range testColorSpaces {
			w, buf := memfile.NewPDFWriter(version, opt)

			err := memfile.AddBlankPage(w)
			if err != nil {
				continue
			}

			rm := pdf.NewResourceManager(w)
			obj, err := rm.Embed(space)
			if err != nil {
				continue
			}
			err = rm.Close()
			if err != nil {
				continue
			}

			w.GetMeta().Trailer["Quir:CS"] = obj
			err = w.Close()
			if err != nil {
				continue
			}

			f.Add(buf.Data)
		}
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}

		obj := r.GetMeta().Trailer["Quir:CS"]
		if obj == nil {
			t.Skip("missing color space")
		}

		x := pdf.NewExtractor(r)
		space, err := ExtractSpace(x, obj)
		if err != nil {
			t.Skip("malformed color space")
		}

		spaceRoundTrip(t, pdf.GetVersion(r), space)
	})
}
