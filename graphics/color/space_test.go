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
	"testing"

	"seehuhn.de/go/icc"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/limits"
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

	// Indexed colour space whose base is a multi-component DeviceN with a
	// Type 4 tint transform.  This exercises the nested
	// Indexed→DeviceN→Type4 path through ToXYZ with and without a Workspace.
	indexedDeviceNType4(),
}

// indexedDeviceNType4 builds an Indexed colour space over a two-colorant
// DeviceN whose alternate is DeviceRGB and whose tint transform is a Type 4
// function mapping (a, b) to (a, b, 0.5).
func indexedDeviceNType4() Space {
	dn := must(DeviceN([]pdf.Name{"a", "b"}, SpaceDeviceRGB, &function.Type4{
		Domain:  []float64{0, 1, 0, 1},
		Range:   []float64{0, 1, 0, 1, 0, 1},
		Program: "0.5",
	}, nil)).(*SpaceDeviceN)
	return must(Indexed([]Color{
		dn.New([]float64{0.2, 0.8}),
		dn.New([]float64{0.6, 0.3}),
	}))
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
			space2, err := ExtractSpace(x, nil, obj, false)
			if err != nil {
				t.Fatal(err)
			}
			if !SpacesEqual(space, space2) {
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

// TestExtractSpaceMalformedSeparationDeviceN verifies that ExtractSpace
// rejects /Separation and /DeviceN color spaces whose tint transform's
// arity does not match the colorant count and the alternate space's
// channel count.  These shapes are rejected by the [Separation] and
// [DeviceN] factory functions; without routing the read path through
// them, a malicious PDF can produce a color space whose ToXYZ later
// panics.
func TestExtractSpaceMalformedSeparationDeviceN(t *testing.T) {
	type2 := func(c0, c1 []pdf.Real) pdf.Dict {
		c0a := make(pdf.Array, len(c0))
		c1a := make(pdf.Array, len(c1))
		for i, v := range c0 {
			c0a[i] = v
		}
		for i, v := range c1 {
			c1a[i] = v
		}
		return pdf.Dict{
			"FunctionType": pdf.Integer(2),
			"Domain":       pdf.Array{pdf.Real(0), pdf.Real(1)},
			"C0":           c0a,
			"C1":           c1a,
			"N":            pdf.Real(1),
		}
	}

	cases := []struct {
		name string
		obj  pdf.Object
	}{
		{
			// alternate DeviceRGB has 3 channels; tint transform emits 5
			name: "separation-wrong-output-arity",
			obj: pdf.Array{
				pdf.Name("Separation"),
				pdf.Name("MyColorant"),
				pdf.Name("DeviceRGB"),
				type2([]pdf.Real{0, 0, 0, 0, 0}, []pdf.Real{1, 1, 1, 1, 1}),
			},
		},
		{
			// two colorants but Type 2 has nIn=1
			name: "devicen-wrong-input-arity",
			obj: pdf.Array{
				pdf.Name("DeviceN"),
				pdf.Array{pdf.Name("c1"), pdf.Name("c2")},
				pdf.Name("DeviceRGB"),
				type2([]pdf.Real{0, 0, 0}, []pdf.Real{1, 1, 1}),
			},
		},
		{
			// alternate DeviceCMYK has 4 channels; tint transform emits 3
			name: "devicen-wrong-output-arity",
			obj: pdf.Array{
				pdf.Name("DeviceN"),
				pdf.Array{pdf.Name("c1")},
				pdf.Name("DeviceCMYK"),
				type2([]pdf.Real{0, 0, 0}, []pdf.Real{1, 1, 1}),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			x := pdf.NewExtractor(r)
			_, err := ExtractSpace(x, nil, tc.obj, false)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !pdf.IsMalformed(err) {
				t.Errorf("expected MalformedFileError, got %T: %v", err, err)
			}
		})
	}
}

// TestExtractDeviceNTooManyColorants verifies that ExtractSpace rejects
// a /DeviceN color space whose colorant list exceeds
// limits.MaxImageChannels.  Without this cap, the scanner-level
// maxArrayLen of 1<<20 entries would let a malicious PDF carry up to
// ~1M colorant names and amplify the later per-channel float64
// allocation in image decoding by ~64×.
func TestExtractDeviceNTooManyColorants(t *testing.T) {
	names := make(pdf.Array, limits.MaxImageChannels+1)
	for i := range names {
		names[i] = pdf.Name(fmt.Sprintf("c%d", i))
	}
	obj := pdf.Array{
		pdf.Name("DeviceN"),
		names,
		pdf.Name("DeviceRGB"),
		// tint transform shape is irrelevant — the colorant-count cap
		// must fire before the transform is parsed.
		pdf.Dict{
			"FunctionType": pdf.Integer(2),
			"Domain":       pdf.Array{pdf.Real(0), pdf.Real(1)},
			"C0":           pdf.Array{pdf.Real(0), pdf.Real(0), pdf.Real(0)},
			"C1":           pdf.Array{pdf.Real(1), pdf.Real(1), pdf.Real(1)},
			"N":            pdf.Real(1),
		},
	}

	r, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	x := pdf.NewExtractor(r)
	_, err := ExtractSpace(x, nil, obj, false)
	if err == nil {
		t.Fatal("expected error for oversize DeviceN colorant list, got nil")
	}
	if !pdf.IsMalformed(err) {
		t.Errorf("expected MalformedFileError, got %T: %v", err, err)
	}
}

// TestIndexedAcceptsAllNonSpecialBases verifies that the Indexed factory
// builds palettes over every PDF base color space the spec permits — in
// particular ICCBased, sRGB, Separation, and DeviceN, which an earlier
// type-switch silently fell through and then panicked on.
func TestIndexedAcceptsAllNonSpecialBases(t *testing.T) {
	tintTransform := func(out int) *function.Type2 {
		c0 := make([]float64, out)
		c1 := make([]float64, out)
		for i := range out {
			c1[i] = 1
		}
		return &function.Type2{XMin: 0, XMax: 1, C0: c0, C1: c1, N: 1}
	}

	sep := must(Separation("PANTONE", SpaceDeviceRGB, tintTransform(3))).(*SpaceSeparation)
	dn := must(DeviceN([]pdf.Name{"c1"}, SpaceDeviceCMYK, tintTransform(4), nil)).(*SpaceDeviceN)
	iccRGB := must(ICCBased(icc.SRGBv2Profile, nil)).(*SpaceICCBased)

	cases := []struct {
		name   string
		colors []Color
	}{
		{"sRGB", []Color{
			FromValues(SpaceSRGB, []float64{0, 0, 0}, nil),
			FromValues(SpaceSRGB, []float64{1, 1, 1}, nil),
		}},
		{"ICCBased-RGB", []Color{
			FromValues(iccRGB, []float64{0, 0, 0}, nil),
			FromValues(iccRGB, []float64{1, 1, 1}, nil),
		}},
		{"Separation", []Color{
			sep.New(0),
			sep.New(0.5),
			sep.New(1),
		}},
		{"DeviceN", []Color{
			dn.New([]float64{0}),
			dn.New([]float64{1}),
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cs, err := Indexed(tc.colors)
			if err != nil {
				t.Fatalf("Indexed: %v", err)
			}
			// exercise lookupValues for every palette entry — this would
			// previously have silently returned Base.Default() for these
			// bases.
			for i := range tc.colors {
				vals := cs.lookupValues(i, &icc.Workspace{})
				if len(vals) != cs.Base.Channels() {
					t.Errorf("entry %d: got %d components, want %d", i, len(vals), cs.Base.Channels())
				}
			}
		})
	}
}

// TestExtractIndexedRejectsSpecialBase verifies that the read path
// rejects /Indexed color spaces whose base is itself /Pattern or
// /Indexed.  PDF 2.0 §8.6.6.3 forbids these.
func TestExtractIndexedRejectsSpecialBase(t *testing.T) {
	cases := []struct {
		name string
		obj  pdf.Object
	}{
		{
			name: "indexed-of-pattern",
			obj: pdf.Array{
				pdf.Name("Indexed"),
				pdf.Name("Pattern"),
				pdf.Integer(0),
				pdf.String{0},
			},
		},
		{
			name: "indexed-of-indexed",
			obj: pdf.Array{
				pdf.Name("Indexed"),
				pdf.Array{
					pdf.Name("Indexed"),
					pdf.Name("DeviceGray"),
					pdf.Integer(0),
					pdf.String{0},
				},
				pdf.Integer(0),
				pdf.String{0},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			x := pdf.NewExtractor(r)
			_, err := ExtractSpace(x, nil, tc.obj, false)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !pdf.IsMalformed(err) {
				t.Errorf("expected MalformedFileError, got %T: %v", err, err)
			}
		})
	}
}

// TestExtractPatternUncoloredRejectsPatternBase verifies that the read
// path rejects uncolored Pattern color spaces whose underlying space is
// itself a Pattern color space (colored or uncolored).  PDF 2.0
// §8.7.3.3 forbids this.
func TestExtractPatternUncoloredRejectsPatternBase(t *testing.T) {
	cases := []struct {
		name string
		obj  pdf.Object
	}{
		{
			name: "uncolored-over-colored-pattern",
			obj: pdf.Array{
				pdf.Name("Pattern"),
				pdf.Name("Pattern"),
			},
		},
		{
			name: "uncolored-over-uncolored-pattern",
			obj: pdf.Array{
				pdf.Name("Pattern"),
				pdf.Array{pdf.Name("Pattern"), pdf.Name("DeviceRGB")},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			x := pdf.NewExtractor(r)
			_, err := ExtractSpace(x, nil, tc.obj, false)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !pdf.IsMalformed(err) {
				t.Errorf("expected MalformedFileError, got %T: %v", err, err)
			}
		})
	}
}

// testUncoloredPattern is a minimal Pattern with PaintType=2 used to
// exercise PatternUncolored's input validation.
type testUncoloredPattern struct{}

func (testUncoloredPattern) PatternType() int                              { return 1 }
func (testUncoloredPattern) PaintType() int                                { return 2 }
func (testUncoloredPattern) Equal(other Pattern) bool                      { return false }
func (testUncoloredPattern) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) { return nil, nil }

// TestPatternUncoloredPanicsOnPatternBase verifies that PatternUncolored
// rejects a base color whose color space is itself a Pattern color
// space.
func TestPatternUncoloredPanicsOnPatternBase(t *testing.T) {
	pat := testUncoloredPattern{}
	cases := []struct {
		name string
		col  Color
	}{
		{
			name: "base-is-colored-pattern",
			col:  spacePatternColored{}.Default(), // colorColoredPattern, ColorSpace() == spacePatternColored
		},
		{
			name: "base-is-uncolored-pattern",
			// Build a legal uncolored-pattern color (base = DeviceRGB),
			// then feed it as a base — its ColorSpace() is spacePatternUncolored.
			col: PatternUncolored(pat, DeviceRGB{0, 0, 0}),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("expected panic, got none")
				}
			}()
			_ = PatternUncolored(pat, tc.col)
		})
	}
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
	decoded, err := ExtractSpace(x, nil, obj, false)
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
		r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}

		obj := r.GetMeta().Trailer["Quir:CS"]
		if obj == nil {
			t.Skip("missing color space")
		}

		x := pdf.NewExtractor(r)
		space, err := ExtractSpace(x, nil, obj, false)
		if err != nil {
			t.Skip("malformed color space")
		}

		spaceRoundTrip(t, pdf.GetVersion(r), space)
	})
}

// TestExtractSpaceDeepChainBounded guards against a stack-overflow DoS: a
// chain of distinct Separation color spaces, each whose alternate is the
// next, is acyclic, so the cycle guard never trips, yet recursing one frame
// per level would exhaust the Go stack. The ExtractorGet depth cap must turn
// this into a malformed-file error rather than a crash.
func TestExtractSpaceDeepChainBounded(t *testing.T) {
	depth := limits.MaxExtractDepth + 10
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)

	tint := pdf.Dict{
		"FunctionType": pdf.Integer(2),
		"Domain":       pdf.Array{pdf.Real(0), pdf.Real(1)},
		"C0":           pdf.Array{pdf.Real(0)},
		"C1":           pdf.Array{pdf.Real(1)},
		"N":            pdf.Real(1),
	}

	refs := make([]pdf.Reference, depth)
	for i := range refs {
		refs[i] = w.Alloc()
	}
	for i, ref := range refs {
		var alt pdf.Object = pdf.Name("DeviceRGB")
		if i+1 < depth {
			alt = refs[i+1]
		}
		obj := pdf.Array{pdf.Name("Separation"), pdf.Name("c"), alt, tint}
		if err := w.Put(ref, obj); err != nil {
			t.Fatal(err)
		}
	}

	x := pdf.NewExtractor(w)
	if _, err := ExtractSpace(x, nil, refs[0], false); !pdf.IsMalformed(err) {
		t.Errorf("err = %v, want malformed", err)
	}
}
