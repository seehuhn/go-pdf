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

package jpx

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"seehuhn.de/go/icc"
	"seehuhn.de/go/jpx"
	"seehuhn.de/go/membudget"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/cie"
)

// labEP returns the EP field of a CIELab colour specification.
func labEP(rl, ol, ra, oa, rb, ob, il uint32) []byte {
	buf := make([]byte, 0, 28)
	for _, v := range []uint32{rl, ol, ra, oa, rb, ob, il} {
		buf = binary.BigEndian.AppendUint32(buf, v)
	}
	return buf
}

// decodeSpecs decodes a 3-channel image with the given depths and colour
// specifications.
func decodeSpecs(t *testing.T, comps []comp, specs ...jpx.ColorSpec) *Image {
	t.Helper()
	img := newImage(0, 5, 4, comps, 12)
	img.ColorSpecs = specs
	return decode(t, encode(t, img, true))
}

var rgb8 = []comp{{depth: 8}, {depth: 8}, {depth: 8}}

var approx = cmpopts.EquateApprox(0, 1e-9)

func TestDecodeLab(t *testing.T) {
	ct := func(kelvin uint32) uint32 { return 'C'<<24 | 'T'<<16 | kelvin }
	daylight, err := color.WhitePointDaylight(6000)
	if err != nil {
		t.Fatal(err)
	}
	type testCase struct {
		name   string
		comps  []comp
		ep     []byte
		wp     []float64
		decode []float64
	}
	cases := []testCase{
		{
			name:   "default 8 bit",
			comps:  rgb8,
			wp:     color.WhitePointD50,
			decode: []float64{0, 100, -128 * 170 / 255., 127 * 170 / 255., -96 * 200 / 255., 159 * 200 / 255.},
		},
		{
			name:   "default 12 bit",
			comps:  []comp{{depth: 12}, {depth: 12}, {depth: 12}},
			wp:     color.WhitePointD50,
			decode: []float64{0, 100, -2048 * 170 / 4095., 2047 * 170 / 4095., -1536 * 200 / 4095., 2559 * 200 / 4095.},
		},
		{
			name:   "explicit",
			comps:  rgb8,
			ep:     labEP(100, 10, 255, 128, 100, 50, jpx.IlluminantD65),
			wp:     color.WhitePointD65,
			decode: []float64{-1000 / 255., 245 * 100 / 255., -128, 127, -5000 / 255., 205 * 100 / 255.},
		},
		{
			name:   "colour temperature",
			comps:  rgb8,
			ep:     labEP(100, 0, 255, 128, 255, 128, ct(6000)),
			wp:     daylight,
			decode: []float64{0, 100, -128, 127, -128, 127},
		},
		{
			name:   "signed",
			comps:  []comp{{depth: 8, signed: true}, {depth: 8, signed: true}, {depth: 8, signed: true}},
			ep:     labEP(100, 0, 255, 128, 255, 128, jpx.IlluminantD50),
			wp:     color.WhitePointD50,
			decode: []float64{0, 100, -128, 127, -128, 127},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := decodeSpecs(t, tc.comps, jpx.ColorSpec{Method: 1, Enumerated: 14, EnumParams: tc.ep})
			cs, ok := res.ColorSpace.Space.(*color.SpaceLab)
			if !ok {
				t.Fatalf("colour space %v, want Lab", res.ColorSpace.Space)
			}
			if diff := cmp.Diff(tc.wp, cs.WhitePoint[:], approx); diff != "" {
				t.Errorf("white point differs (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.decode, res.ColorSpace.Decode, approx); diff != "" {
				t.Errorf("Decode differs (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.decode[2:], cs.Ranges[:], approx); diff != "" {
				t.Errorf("ranges differ (-want +got):\n%s", diff)
			}
			if res.ColorSpace.Convert != nil {
				t.Error("unexpected Convert")
			}
		})
	}
}

// TestDecodeLabUnusable checks that a CIELab specification which cannot
// be represented gives way to the next specification.
func TestDecodeLabUnusable(t *testing.T) {
	cases := map[string][]byte{
		"short EP":         {0, 0, 0, 100},
		"zero range":       labEP(100, 0, 0, 128, 255, 128, jpx.IlluminantD50),
		"unknown":          labEP(100, 0, 255, 128, 255, 128, 0x12345678),
		"too cold":         labEP(100, 0, 255, 128, 255, 128, 'C'<<24|'T'<<16|3000),
		"too hot":          labEP(100, 0, 255, 128, 255, 128, 'C'<<24|'T'<<16|30000),
		"huge a range":     labEP(100, 0, 0xFFFFFFFF, 0x7FFFFFFF, 255, 128, jpx.IlluminantD50),
		"huge b range":     labEP(100, 0, 255, 128, 0xFFFFFFFF, 0, jpx.IlluminantD50),
		"huge b offset":    labEP(100, 0, 255, 128, 255, 0xFFFFFFFF, jpx.IlluminantD50),
		"huge a, offset 0": labEP(100, 0, 0x10000000, 0, 255, 128, jpx.IlluminantD50),
	}
	for name, ep := range cases {
		t.Run(name, func(t *testing.T) {
			res := decodeSpecs(t, rgb8,
				jpx.ColorSpec{Method: 1, Precedence: 1, Enumerated: 14, EnumParams: ep},
				jpx.ColorSpec{Method: 1, Enumerated: 21})
			if res.ColorSpace.Space != color.SpaceROMMRGB {
				t.Errorf("colour space %v, want ROMM RGB", res.ColorSpace.Space)
			}
		})
	}
}

func TestIlluminants(t *testing.T) {
	// CIE 1931 2° white points, as tabulated in ASTM E308
	want := map[uint32][]float64{
		jpx.IlluminantD50: {0.9642, 1, 0.8251},
		jpx.IlluminantD65: {0.9505, 1, 1.0888},
		jpx.IlluminantD75: {0.9497, 1, 1.2264},
		jpx.IlluminantSA:  {1.0985, 1, 0.3558},
		jpx.IlluminantSC:  {0.9807, 1, 1.1823},
		jpx.IlluminantF2:  {0.9915, 1, 0.6732},
		jpx.IlluminantF7:  {0.9502, 1, 1.0863},
		jpx.IlluminantF11: {1.0090, 1, 0.6433},
	}
	tol := cmpopts.EquateApprox(0, 2e-3)
	for il, wp := range want {
		cs, ok := resolve(jpx.ColorSpec{
			Method: 1, Enumerated: 14,
			EnumParams: labEP(100, 0, 255, 128, 255, 128, il),
		}, []int{8, 8, 8})
		if !ok {
			t.Errorf("illuminant %08x: not usable", il)
			continue
		}
		got := cs.Space.(*color.SpaceLab).WhitePoint
		if diff := cmp.Diff(wp, got[:], tol); diff != "" {
			t.Errorf("illuminant %08x: white point differs (-want +got):\n%s", il, diff)
		}
	}
}

func TestDecodeROMM(t *testing.T) {
	res := decodeSpecs(t, rgb8, jpx.ColorSpec{Method: 1, Enumerated: 21})
	checkLinear(t, res.ColorSpace, color.SpaceROMMRGB)

	primaries := [][2][]float64{
		{{1, 0, 0}, {0.7977, 0.2881, 0}},
		{{0, 1, 0}, {0.1352, 0.7118, 0}},
		{{0, 0, 1}, {0.0314, 0.0001, 0.8249}},
	}
	tol := cmpopts.EquateApprox(0, 2e-3)
	for _, p := range primaries {
		X, Y, Z := res.ColorSpace.Space.ToXYZ(p[0], icc.RelativeColorimetric, &icc.Workspace{})
		if diff := cmp.Diff(p[1], []float64{X, Y, Z}, tol); diff != "" {
			t.Errorf("primary %v differs (-want +got):\n%s", p[0], diff)
		}
	}
}

// convertSample maps a sample, normalised to [0, 1], through Decode and
// Convert.
func convertSample(cs ColorSpace, x []float64) []float64 {
	v := make([]float64, len(x))
	for c, xc := range x {
		lo, hi := cs.Decode[2*c], cs.Decode[2*c+1]
		v[c] = lo + xc*(hi-lo)
	}
	if cs.Convert != nil {
		cs.Convert(v)
	}
	return v
}

// checkInRange checks that v is a finite colour of labExtended.
func checkInRange(t *testing.T, v []float64) {
	t.Helper()
	r := extendedLabRanges
	for _, x := range v {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			t.Fatalf("%v is not finite", v)
		}
	}
	if v[0] < 0 || v[0] > 100 || v[1] < r[0] || v[1] > r[1] || v[2] < r[2] || v[2] > r[3] {
		t.Errorf("%v outside the colour space", v)
	}
}

func TestDecodeESRGB(t *testing.T) {
	for _, n := range []int{8, 10, 12, 16} {
		comps := []comp{{depth: n}, {depth: n}, {depth: n}}
		res := decodeSpecs(t, comps, jpx.ColorSpec{Method: 1, Enumerated: 20})
		cs := res.ColorSpace
		if cs.Space != color.Space(labExtended) {
			t.Fatalf("%d bit: colour space %v, want labExtended", n, cs.Space)
		}

		m := math.Ldexp(1, n) - 1
		offset := math.Ldexp(1, n-2) + math.Ldexp(1, n-3)
		scale := 255 * math.Ldexp(1, n-9)
		want := []float64{-offset / scale, (m - offset) / scale}
		want = append(append(want, want...), want...)
		if diff := cmp.Diff(want, cs.Decode, approx); diff != "" {
			t.Errorf("%d bit: Decode differs (-want +got):\n%s", n, diff)
		}

		// the code offset+scale encodes white
		w := (offset + scale) / m
		got := convertSample(cs, []float64{w, w, w})
		if diff := cmp.Diff([]float64{100, 0, 0}, got, cmpopts.EquateApprox(0, 1e-6)); diff != "" {
			t.Errorf("%d bit: white differs (-want +got):\n%s", n, diff)
		}
		// the code offset encodes black
		b := offset / m
		got = convertSample(cs, []float64{b, b, b})
		if diff := cmp.Diff([]float64{0, 0, 0}, got, cmpopts.EquateApprox(0, 1e-6)); diff != "" {
			t.Errorf("%d bit: black differs (-want +got):\n%s", n, diff)
		}
		for _, x := range [][]float64{{0, 0, 0}, {1, 1, 1}, {1, 0, 0}, {0, 1, 1}, {0, 0, 1}} {
			checkInRange(t, convertSample(cs, x))
		}
	}
}

func TestDecodeESYCC(t *testing.T) {
	res := decodeSpecs(t, rgb8, jpx.ColorSpec{Method: 1, Enumerated: 24})
	cs := res.ColorSpace
	if cs.Space != color.Space(labExtended) {
		t.Fatalf("colour space %v, want labExtended", cs.Space)
	}
	want := []float64{0, 1, -128 / 255., 127 / 255., -128 / 255., 127 / 255.}
	if diff := cmp.Diff(want, cs.Decode, approx); diff != "" {
		t.Errorf("Decode differs (-want +got):\n%s", diff)
	}

	// grey axis: the code 128 is chroma zero
	for _, y := range []float64{0, 0.25, 0.5, 1} {
		got := convertSample(cs, []float64{y, 128 / 255., 128 / 255.})
		if math.Abs(got[1]) > 1e-6 || math.Abs(got[2]) > 1e-6 {
			t.Errorf("Y′ = %g: a*, b* = %g, %g, want 0", y, got[1], got[2])
		}
		v := []float64{y, y, y}
		esrgbToLab(v)
		if math.Abs(got[0]-v[0]) > 1e-6 {
			t.Errorf("Y′ = %g: L* = %g, want %g", y, got[0], v[0])
		}
	}

	// half chroma in Cr
	v := []float64{0.5, 0, 0.25}
	yccToESRGB(v)
	if diff := cmp.Diff([]float64{1.201, 0.14295, 0.500075}, v, approx); diff != "" {
		t.Errorf("e-sYCC conversion differs (-want +got):\n%s", diff)
	}
	got := convertSample(cs, []float64{0.5, 128 / 255., (128 + 0.25*255) / 255.})
	esrgbToLab(v)
	if diff := cmp.Diff(v, got, cmpopts.EquateApprox(0, 1e-9)); diff != "" {
		t.Errorf("half chroma differs (-want +got):\n%s", diff)
	}
	if got[1] <= 0 {
		t.Errorf("a* = %g for a red-shifted colour, want positive", got[1])
	}

	for _, x := range [][]float64{{0, 0, 0}, {1, 1, 1}, {0, 1, 0}, {1, 0, 1}, {0.5, 0, 1}} {
		checkInRange(t, convertSample(cs, x))
	}
}

func TestDecodeSYCC(t *testing.T) {
	res := decodeSpecs(t, rgb8, jpx.ColorSpec{Method: 1, Enumerated: 18})
	cs := res.ColorSpace
	if cs.Space != color.Space(labExtended) {
		t.Fatalf("colour space %v, want labExtended", cs.Space)
	}
	want := []float64{0, 1, -128 / 255., 127 / 255., -128 / 255., 127 / 255.}
	if diff := cmp.Diff(want, cs.Decode, approx); diff != "" {
		t.Errorf("Decode differs (-want +got):\n%s", diff)
	}

	// grey axis: the code 128 is chroma zero
	for _, y := range []float64{0, 0.25, 0.5, 1} {
		got := convertSample(cs, []float64{y, 128 / 255., 128 / 255.})
		if math.Abs(got[1]) > 1e-6 || math.Abs(got[2]) > 1e-6 {
			t.Errorf("Y′ = %g: a*, b* = %g, %g, want 0", y, got[1], got[2])
		}
		v := []float64{y, y, y}
		esrgbToLab(v)
		if math.Abs(got[0]-v[0]) > 1e-6 {
			t.Errorf("Y′ = %g: L* = %g, want %g", y, got[0], v[0])
		}
	}

	// half chroma in Cr
	v := []float64{0.5, 0, 0.25}
	yccToRGB(v)
	if diff := cmp.Diff([]float64{0.8505, 0.321466, 0.5}, v, approx); diff != "" {
		t.Errorf("sYCC conversion differs (-want +got):\n%s", diff)
	}
	got := convertSample(cs, []float64{0.5, 128 / 255., (128 + 0.25*255) / 255.})
	esrgbToLab(v)
	if diff := cmp.Diff(v, got, cmpopts.EquateApprox(0, 1e-9)); diff != "" {
		t.Errorf("half chroma differs (-want +got):\n%s", diff)
	}
	if got[1] <= 0 {
		t.Errorf("a* = %g for a red-shifted colour, want positive", got[1])
	}

	// Y′ = 0.9, Cr′ = 0.5 gives R′ = 1.601, outside the sRGB gamut
	out := convertSample(cs, []float64{0.9, 128 / 255., (128 + 0.5*255) / 255.})
	rgb := []float64{0.9, 0, 0.5}
	yccToRGB(rgb)
	if rgb[0] <= 1 {
		t.Fatalf("R′ = %g, want > 1", rgb[0])
	}
	clipped := []float64{1, rgb[1], rgb[2]}
	esrgbToLab(clipped)
	esrgbToLab(rgb)
	if diff := cmp.Diff(rgb, out, cmpopts.EquateApprox(0, 1e-9)); diff != "" {
		t.Errorf("out-of-gamut colour differs (-want +got):\n%s", diff)
	}
	if out[1] <= clipped[1] {
		t.Errorf("a* = %g, want more than the clipped value %g", out[1], clipped[1])
	}
	checkInRange(t, out)

	for _, x := range [][]float64{{0, 0, 0}, {1, 1, 1}, {0, 1, 0}, {1, 0, 1}, {0.5, 0, 1}} {
		checkInRange(t, convertSample(cs, x))
	}
}

// TestExtendedLabRanges checks that the ranges of labExtended enclose all
// e-sRGB, e-sYCC and sYCC colours.
func TestExtendedLabRanges(t *testing.T) {
	wide, err := color.Lab(color.WhitePointD50, nil, []float64{-1e4, 1e4, -1e4, 1e4})
	if err != nil {
		t.Fatal(err)
	}
	ws := &icc.Workspace{}
	r := extendedLabRanges
	v := make([]float64, 3)
	check := func(domain string, rgb []float64) {
		X, Y, Z := cie.ExtendedSRGBToXYZ(rgb[0], rgb[1], rgb[2])
		wide.FromXYZ(X, Y, Z, icc.RelativeColorimetric, v, ws)
		if v[1] < r[0] || v[1] > r[1] || v[2] < r[2] || v[2] > r[3] {
			t.Fatalf("%s: a*, b* = %g, %g outside the ranges %v", domain, v[1], v[2], r)
		}
	}

	const N = 64
	grid := func(lo, hi float64, i int) float64 { return lo + (hi-lo)*float64(i)/N }
	// e-sRGB cube for depth n → ∞; smaller depths give a subset
	lo, hi := -0.375*512/255, 0.625*512/255
	for i := range N + 1 {
		for j := range N + 1 {
			for k := range N + 1 {
				check("e-sRGB", []float64{grid(lo, hi, i), grid(lo, hi, j), grid(lo, hi, k)})
				ycc := []float64{grid(0, 1, i), grid(-0.5, 0.5, j), grid(-0.5, 0.5, k)}
				yccToESRGB(ycc)
				check("e-sYCC", ycc)
				ycc = []float64{grid(0, 1, i), grid(-0.5, 0.5, j), grid(-0.5, 0.5, k)}
				yccToRGB(ycc)
				check("sYCC", ycc)
			}
		}
	}
}

// BenchmarkDecodeESRGB measures decoding a 12-bit e-sRGB image, and
// separately the conversion of its samples to Lab.
func BenchmarkDecodeESRGB(b *testing.B) {
	const size = 1024
	comps := []comp{{depth: 12}, {depth: 12}, {depth: 12}}
	img := newImage(0, size, size, comps, 13)
	img.ColorSpecs = []jpx.ColorSpec{{Method: 1, Enumerated: 20}}
	var buf bytes.Buffer
	if err := jpx.Encode(&buf, img, &jpx.EncodeParams{Wrapper: true, Levels: 2}); err != nil {
		b.Fatal(err)
	}
	data := buf.Bytes()

	b.Run("decode", func(b *testing.B) {
		for b.Loop() {
			if _, err := Decode(data, 0, true, membudget.New(1<<30)); err != nil {
				b.Fatal(err)
			}
		}
	})

	res, err := Decode(data, 0, true, membudget.New(1<<30))
	if err != nil {
		b.Fatal(err)
	}
	b.Run("convert", func(b *testing.B) {
		cs := res.ColorSpace
		maxV := float64(int(1)<<res.BitsPerComponent - 1)
		v := make([]float64, 3)
		for b.Loop() {
			for i := range size * size {
				for c := range 3 {
					lo, hi := cs.Decode[2*c], cs.Decode[2*c+1]
					v[c] = lo + float64(res.Color[c][i])/maxV*(hi-lo)
				}
				cs.Convert(v)
			}
		}
	})
}
