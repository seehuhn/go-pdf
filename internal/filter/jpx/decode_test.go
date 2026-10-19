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
	"errors"
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/icc"
	"seehuhn.de/go/jpx"
	"seehuhn.de/go/membudget"
	"seehuhn.de/go/pdf/graphics/color"
)

// comp describes one component of a test image.
type comp struct {
	depth  int
	signed bool
	dx, dy int
}

// newImage builds an image with random samples covering the full range
// of each component, and one colour channel per component.
func newImage(x0, w, h int, comps []comp, seed uint64) *jpx.Image {
	rng := rand.New(rand.NewPCG(seed, 1))
	img := &jpx.Image{XOffset: x0, Width: w, Height: h}
	for i, c := range comps {
		dx, dy := max(c.dx, 1), max(c.dy, 1)
		pw := ceilDiv(x0+w, dx) - ceilDiv(x0, dx)
		ph := ceilDiv(h, dy)
		img.Components = append(img.Components, jpx.Component{
			Depth: c.depth, Signed: c.signed, DX: dx, DY: dy, Width: pw, Height: ph,
		})
		lo, hi := int64(0), int64(1)<<c.depth-1
		if c.signed {
			lo, hi = -(int64(1) << (c.depth - 1)), int64(1)<<(c.depth-1)-1
		}
		pix := make([]int32, pw*ph)
		for k := range pix {
			pix[k] = int32(lo + rng.Int64N(hi-lo+1))
		}
		img.Planes = append(img.Planes, jpx.Plane{Width: pw, Height: ph, Pix: pix})
		img.Channels = append(img.Channels, jpx.Channel{
			Component: i, PaletteColumn: -1, Kind: jpx.ChannelColor, Assoc: i + 1,
		})
	}
	return img
}

func ceilDiv(a, b int) int { return (a + b - 1) / b }

// encode writes img losslessly as a JP2 file or bare codestream.
func encode(t *testing.T, img *jpx.Image, wrapper bool) []byte {
	t.Helper()
	var buf bytes.Buffer
	err := jpx.Encode(&buf, img, &jpx.EncodeParams{Wrapper: wrapper, Levels: 2})
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func decode(t *testing.T, data []byte) *Image {
	t.Helper()
	res, err := Decode(data, 0, true, membudget.New(64<<20))
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// toUnsigned converts one sample by the rule documented for
// [jpx.ToUnsigned].
func toUnsigned(v int32, d int, signed bool, depth int) int32 {
	u := int64(v)
	if signed {
		u += 1 << (d - 1)
	}
	mIn := int64(1)<<d - 1
	mOut := int64(1)<<depth - 1
	u = min(max(u, 0), mIn)
	return int32((2*u*mOut + mIn) / (2 * mIn))
}

// expectPlane returns the samples of component c of img at full
// resolution, converted to unsigned samples of the given depth.
func expectPlane(img *jpx.Image, c, depth int) []int32 {
	comp := img.Components[c]
	src := img.Planes[c]
	x0 := img.XOffset
	cx0 := ceilDiv(x0, comp.DX)
	res := make([]int32, img.Width*img.Height)
	for y := range img.Height {
		sy := min(y/comp.DY, src.Height-1)
		for x := range img.Width {
			sx := min(max((x0+x)/comp.DX, cx0), cx0+src.Width-1) - cx0
			v := src.Pix[sy*src.Width+sx]
			res[y*img.Width+x] = toUnsigned(v, comp.Depth, comp.Signed, depth)
		}
	}
	return res
}

// checkLinear checks that cs describes the colour space want with the
// default component ranges and no conversion.
func checkLinear(t *testing.T, cs ColorSpace, want color.Space) {
	t.Helper()
	if !color.SpacesEqual(cs.Space, want) {
		t.Errorf("colour space %v, want %v", cs.Space, want)
	}
	if cs.Convert != nil {
		t.Error("unexpected Convert")
	}
	var decode []float64
	if icc, ok := want.(*color.SpaceICCBased); ok {
		decode = icc.Ranges
	} else {
		for range want.Channels() {
			decode = append(decode, 0, 1)
		}
	}
	if diff := cmp.Diff(decode, cs.Decode); diff != "" {
		t.Errorf("Decode differs (-want +got):\n%s", diff)
	}
}

func TestDecodeSamples(t *testing.T) {
	type testCase struct {
		name    string
		x0      int
		comps   []comp
		wrapper bool
		depth   int
		cs      color.Space
	}
	cases := []testCase{
		{name: "gray", comps: []comp{{depth: 8}}, wrapper: true, depth: 8, cs: color.SpaceDeviceGray},
		{name: "gray-codestream", comps: []comp{{depth: 8}}, depth: 8, cs: color.SpaceDeviceGray},
		{name: "rgb", comps: []comp{{depth: 8}, {depth: 8}, {depth: 8}}, wrapper: true, depth: 8, cs: color.SpaceDeviceRGB},
		{name: "cmyk", comps: []comp{{depth: 8}, {depth: 8}, {depth: 8}, {depth: 8}}, wrapper: true, depth: 8, cs: color.SpaceDeviceCMYK},
		{name: "1-bit", comps: []comp{{depth: 1}}, wrapper: true, depth: 1, cs: color.SpaceDeviceGray},
		{name: "16-bit", comps: []comp{{depth: 16}}, wrapper: true, depth: 16, cs: color.SpaceDeviceGray},
		{name: "20-bit", comps: []comp{{depth: 20}}, depth: 16, cs: color.SpaceDeviceGray},
		{name: "signed", comps: []comp{{depth: 8, signed: true}}, depth: 8, cs: color.SpaceDeviceGray},
		{name: "mixed-depths", comps: []comp{{depth: 4}, {depth: 12}, {depth: 12}}, wrapper: true, depth: 12, cs: color.SpaceDeviceRGB},
		{name: "subsampled", comps: []comp{{depth: 8}, {depth: 8, dx: 2, dy: 2}, {depth: 8, dx: 2}}, depth: 8, cs: color.SpaceDeviceRGB},
		{name: "offset", x0: 1, comps: []comp{{depth: 8}, {depth: 8, dx: 2}, {depth: 8, dx: 2}}, depth: 8, cs: color.SpaceDeviceRGB},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			img := newImage(tc.x0, 13, 7, tc.comps, uint64(i))
			res := decode(t, encode(t, img, tc.wrapper))

			if res.Width != 13 || res.Height != 7 {
				t.Errorf("size %dx%d, want 13x7", res.Width, res.Height)
			}
			if res.BitsPerComponent != tc.depth {
				t.Errorf("BitsPerComponent = %d, want %d", res.BitsPerComponent, tc.depth)
			}
			var want [][]int32
			for c := range tc.comps {
				want = append(want, expectPlane(img, c, tc.depth))
			}
			if diff := cmp.Diff(want, res.Color); diff != "" {
				t.Errorf("colour samples differ (-want +got):\n%s", diff)
			}
			checkLinear(t, res.ColorSpace, tc.cs)
			if res.Opacity != nil || res.Palette != nil || res.Premultiplied {
				t.Error("unexpected opacity or palette")
			}
		})
	}
}

func TestDecodeSYCCSamples(t *testing.T) {
	img := newImage(0, 9, 5, []comp{{depth: 8}, {depth: 8}, {depth: 8}}, 1)
	img.ColorSpecs = []jpx.ColorSpec{{Method: 1, Enumerated: 18}}
	res := decode(t, encode(t, img, true))

	// the samples are returned as stored, the conversion is in ColorSpace
	var want [][]int32
	for _, p := range img.Planes {
		want = append(want, p.Pix)
	}
	if diff := cmp.Diff(want, res.Color); diff != "" {
		t.Errorf("colour samples differ (-want +got):\n%s", diff)
	}
	if res.ColorSpace.Space != color.Space(labExtended) || res.ColorSpace.Convert == nil {
		t.Error("sYCC not described by labExtended with a conversion")
	}
}

// TestDecodeIgnoreColorSpec checks that without useColorSpec, sYCC
// samples and palette entries are returned as stored.
func TestDecodeIgnoreColorSpec(t *testing.T) {
	img := newImage(0, 9, 5, []comp{{depth: 8}, {depth: 8}, {depth: 8}}, 1)
	img.ColorSpecs = []jpx.ColorSpec{{Method: 1, Enumerated: 18}}
	res, err := Decode(encode(t, img, true), 0, false, membudget.New(64<<20))
	if err != nil {
		t.Fatal(err)
	}
	var want [][]int32
	for _, p := range img.Planes {
		want = append(want, p.Pix)
	}
	if diff := cmp.Diff(want, res.Color); diff != "" {
		t.Errorf("colour samples differ (-want +got):\n%s", diff)
	}
	// the device space by channel count, not sYCC
	checkLinear(t, res.ColorSpace, color.SpaceDeviceRGB)

	img = newImage(0, 4, 4, []comp{{depth: 2}}, 9)
	entries := [][]int32{{0, 128, 128}, {255, 128, 128}, {100, 50, 200}, {30, 220, 90}}
	img.Palette = &jpx.Palette{
		Columns: []jpx.PaletteColumn{{Depth: 8}, {Depth: 8}, {Depth: 8}},
		Entries: entries,
	}
	img.Channels = []jpx.Channel{
		{Component: 0, PaletteColumn: 0, Kind: jpx.ChannelColor, Assoc: 1},
		{Component: 0, PaletteColumn: 1, Kind: jpx.ChannelColor, Assoc: 2},
		{Component: 0, PaletteColumn: 2, Kind: jpx.ChannelColor, Assoc: 3},
	}
	img.ColorSpecs = []jpx.ColorSpec{{Method: 1, Enumerated: 18}}
	res, err = Decode(encode(t, img, true), 0, false, membudget.New(64<<20))
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(&Palette{Bits: 8, Entries: entries}, res.Palette); diff != "" {
		t.Errorf("palette differs (-want +got):\n%s", diff)
	}
}

func TestDecodeOpacity(t *testing.T) {
	for _, kind := range []jpx.ChannelKind{jpx.ChannelOpacity, jpx.ChannelPremultipliedOpacity} {
		img := newImage(0, 8, 6, []comp{{depth: 4}, {depth: 8}, {depth: 8}, {depth: 8}}, 2)
		// opacity first, colour channels in reverse order
		img.Channels = []jpx.Channel{
			{Component: 0, PaletteColumn: -1, Kind: kind, Type: int(kind), Assoc: 0},
			{Component: 1, PaletteColumn: -1, Kind: jpx.ChannelColor, Assoc: 3},
			{Component: 2, PaletteColumn: -1, Kind: jpx.ChannelColor, Assoc: 2},
			{Component: 3, PaletteColumn: -1, Kind: jpx.ChannelColor, Assoc: 1},
		}
		res := decode(t, encode(t, img, true))

		want := [][]int32{expectPlane(img, 3, 8), expectPlane(img, 2, 8), expectPlane(img, 1, 8)}
		if diff := cmp.Diff(want, res.Color); diff != "" {
			t.Errorf("colour samples differ (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff(expectPlane(img, 0, 4), res.Opacity); diff != "" {
			t.Errorf("opacity differs (-want +got):\n%s", diff)
		}
		if res.BitsPerComponent != 8 || res.OpacityBits != 4 {
			t.Errorf("depths %d/%d, want 8/4", res.BitsPerComponent, res.OpacityBits)
		}
		if res.Premultiplied != (kind == jpx.ChannelPremultipliedOpacity) {
			t.Errorf("Premultiplied = %t", res.Premultiplied)
		}
	}
}

func TestDecodeOpacityChoice(t *testing.T) {
	img := newImage(0, 4, 4, []comp{{depth: 8}, {depth: 8}, {depth: 8}}, 3)
	img.Channels = []jpx.Channel{
		{Component: 0, PaletteColumn: -1, Kind: jpx.ChannelColor, Assoc: 1},
		{Component: 1, PaletteColumn: -1, Kind: jpx.ChannelOpacity, Assoc: 1},
		{Component: 2, PaletteColumn: -1, Kind: jpx.ChannelUnknown, Assoc: 65535},
	}
	res := decode(t, encode(t, img, true))
	if len(res.Color) != 1 {
		t.Fatalf("%d colour channels, want 1", len(res.Color))
	}
	// a sole opacity channel is used even with a specific association
	if diff := cmp.Diff(expectPlane(img, 1, 8), res.Opacity); diff != "" {
		t.Errorf("opacity differs (-want +got):\n%s", diff)
	}

	img.Channels[2] = jpx.Channel{Component: 2, PaletteColumn: -1, Kind: jpx.ChannelOpacity, Assoc: 0}
	res = decode(t, encode(t, img, true))
	if diff := cmp.Diff(expectPlane(img, 2, 8), res.Opacity); diff != "" {
		t.Errorf("opacity differs (-want +got):\n%s", diff)
	}
}

func TestDecodePalette(t *testing.T) {
	img := newImage(0, 6, 5, []comp{{depth: 3}}, 4)
	img.Palette = &jpx.Palette{
		Columns: []jpx.PaletteColumn{{Depth: 8}, {Depth: 4}, {Depth: 8, Signed: true}},
		Entries: [][]int32{
			{0, 0, -128},
			{255, 15, 127},
			{10, 5, 0},
			{20, 1, -1},
			{30, 2, 5},
		},
	}
	img.Channels = []jpx.Channel{
		{Component: 0, PaletteColumn: 0, Kind: jpx.ChannelColor, Assoc: 1},
		{Component: 0, PaletteColumn: 1, Kind: jpx.ChannelColor, Assoc: 2},
		{Component: 0, PaletteColumn: 2, Kind: jpx.ChannelColor, Assoc: 3},
	}
	res := decode(t, encode(t, img, true))

	idx := make([]int32, len(img.Planes[0].Pix))
	for i, v := range img.Planes[0].Pix {
		idx[i] = min(v, 4)
	}
	if diff := cmp.Diff([][]int32{idx}, res.Color); diff != "" {
		t.Errorf("indices differ (-want +got):\n%s", diff)
	}
	if res.BitsPerComponent != 3 {
		t.Errorf("BitsPerComponent = %d, want 3", res.BitsPerComponent)
	}
	want := &Palette{Bits: 8}
	for _, e := range img.Palette.Entries {
		want.Entries = append(want.Entries, []int32{
			e[0], toUnsigned(e[1], 4, false, 8), toUnsigned(e[2], 8, true, 8),
		})
	}
	if diff := cmp.Diff(want, res.Palette); diff != "" {
		t.Errorf("palette differs (-want +got):\n%s", diff)
	}
	checkLinear(t, res.ColorSpace, color.SpaceDeviceRGB)
}

// TestDecodePaletteTruncated checks that the indices of a kept palette
// are read from the retained bits of a component deeper than
// [jpx.MaxPrecision], as [jpx.ApplyPalette] does.
func TestDecodePaletteTruncated(t *testing.T) {
	const depth = jpx.MaxPrecision + 2
	pix := []int32{0, 1, 2, 3, 1, 0}
	img := &jpx.Image{
		Width: 3, Height: 2,
		Components: []jpx.Component{{
			Depth: depth, Truncation: 2, DX: 1, DY: 1, Width: 3, Height: 2,
		}},
		Planes: []jpx.Plane{{Width: 3, Height: 2, Pix: slices.Clone(pix)}},
		Palette: &jpx.Palette{
			Columns: []jpx.PaletteColumn{{Depth: 8}},
			Entries: [][]int32{{0}, {10}, {20}, {30}, {40}, {50}},
		},
		Channels: []jpx.Channel{
			{Component: 0, PaletteColumn: 0, Kind: jpx.ChannelColor, Assoc: 1},
		},
	}
	res := decode(t, encode(t, img, true))

	want := make([]int32, len(pix))
	for i, v := range pix {
		want[i] = min(v<<2, 5)
	}
	if diff := cmp.Diff([][]int32{want}, res.Color); diff != "" {
		t.Errorf("indices differ (-want +got):\n%s", diff)
	}
}

// TestDecodePaletteApplied checks that a palette is applied when an
// opacity channel reads it too.
func TestDecodePaletteApplied(t *testing.T) {
	img := newImage(0, 6, 5, []comp{{depth: 1}}, 5)
	img.Palette = &jpx.Palette{
		Columns: []jpx.PaletteColumn{{Depth: 8}, {Depth: 8}},
		Entries: [][]int32{{10, 200}, {20, 100}},
	}
	img.Channels = []jpx.Channel{
		{Component: 0, PaletteColumn: 0, Kind: jpx.ChannelColor, Assoc: 1},
		{Component: 0, PaletteColumn: 1, Kind: jpx.ChannelOpacity, Assoc: 0},
	}
	res := decode(t, encode(t, img, true))

	var grey, alpha []int32
	for _, v := range img.Planes[0].Pix {
		grey = append(grey, img.Palette.Entries[v][0])
		alpha = append(alpha, img.Palette.Entries[v][1])
	}
	if res.Palette != nil {
		t.Error("palette not applied")
	}
	if diff := cmp.Diff([][]int32{grey}, res.Color); diff != "" {
		t.Errorf("colour samples differ (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(alpha, res.Opacity); diff != "" {
		t.Errorf("opacity differs (-want +got):\n%s", diff)
	}
	if res.BitsPerComponent != 8 || res.OpacityBits != 8 {
		t.Errorf("depths %d/%d, want 8/8", res.BitsPerComponent, res.OpacityBits)
	}
}

func TestDecodeColorSpec(t *testing.T) {
	iccA := icc.SRGBv2Profile
	iccB := icc.SRGBv4Profile
	spaceA, err := color.ICCBased(iccA, nil)
	if err != nil {
		t.Fatal(err)
	}
	type testCase struct {
		name  string
		specs []jpx.ColorSpec
		want  color.Space
	}
	cases := []testCase{
		{
			name: "precedence",
			specs: []jpx.ColorSpec{
				{Method: 1, Enumerated: 16},
				{Method: 3, Precedence: 2, ICC: iccA},
				{Method: 2, Precedence: 1, ICC: iccB},
			},
			want: spaceA,
		},
		{
			name: "approximation",
			specs: []jpx.ColorSpec{
				{Method: 3, Precedence: 1, Approx: 0, ICC: iccA},
				{Method: 3, Precedence: 1, Approx: 3, ICC: iccB},
				{Method: 1, Precedence: 1, Approx: 2, Enumerated: 16},
				{Method: 1, Precedence: 0, Approx: 1, Enumerated: 16},
			},
			want: color.SpaceDeviceRGB,
		},
		{
			name: "unsupported skipped",
			specs: []jpx.ColorSpec{
				{Method: 4, Precedence: 5, Vendor: []byte("0123456789abcdef")},
				{Method: 1, Precedence: 4, Enumerated: 19},
				{Method: 1, Precedence: 3, Enumerated: 17}, // wrong channel count
				{Method: 3, Precedence: 2},                 // empty profile
				{Method: 1, Precedence: 1, Enumerated: 16},
			},
			want: color.SpaceDeviceRGB,
		},
		{
			name: "unusable profiles skipped",
			specs: []jpx.ColorSpec{
				{Method: 3, Precedence: 3, ICC: icc.CMYKProfile}, // wrong channel count
				{Method: 3, Precedence: 2, ICC: []byte("not an ICC profile")},
				{Method: 2, Precedence: 1, ICC: iccA},
				{Method: 1, Precedence: 0, Enumerated: 16},
			},
			want: spaceA,
		},
		{
			name: "romm",
			specs: []jpx.ColorSpec{
				{Method: 1, Enumerated: 21},
			},
			want: color.SpaceROMMRGB,
		},
		{
			name: "none usable",
			specs: []jpx.ColorSpec{
				{Method: 1, Enumerated: 12},
			},
			want: color.SpaceDeviceRGB,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			img := newImage(0, 4, 3, []comp{{depth: 8}, {depth: 8}, {depth: 8}}, 6)
			img.ColorSpecs = tc.specs
			res := decode(t, encode(t, img, true))
			checkLinear(t, res.ColorSpace, tc.want)
		})
	}
}

func TestDecodeBudget(t *testing.T) {
	img := newImage(0, 64, 64, []comp{{depth: 8}, {depth: 8}, {depth: 8}}, 7)
	data := encode(t, img, true)
	_, err := Decode(data, 0, true, membudget.New(10_000))
	if !errors.Is(err, membudget.ErrExceeded) {
		t.Errorf("got error %v, want budget exceeded", err)
	}
}

func TestDecodeMalformed(t *testing.T) {
	img := newImage(0, 16, 16, []comp{{depth: 8}}, 8)
	data := encode(t, img, true)
	inputs := [][]byte{
		nil,
		[]byte("not a JPEG 2000 file"),
		data[:20],
		{0xFF, 0x4F, 0xFF, 0x51},
	}
	for i, in := range inputs {
		_, err := Decode(in, 0, true, membudget.New(64<<20))
		if err == nil {
			t.Errorf("input %d: missing error", i)
		}
	}

	// a truncated codestream still yields an image
	res, err := Decode(data[:len(data)*3/4], 0, true, membudget.New(64<<20))
	if err != nil {
		t.Fatal(err)
	}
	if res.Width != 16 || res.Height != 16 || len(res.Color) != 1 || len(res.Color[0]) != 256 {
		t.Error("truncated image has the wrong shape")
	}
}

func TestDecodePaletteSYCC(t *testing.T) {
	img := newImage(0, 4, 4, []comp{{depth: 2}}, 9)
	entries := [][]int32{{0, 128, 128}, {255, 128, 128}, {100, 50, 200}, {30, 220, 90}}
	img.Palette = &jpx.Palette{
		Columns: []jpx.PaletteColumn{{Depth: 8}, {Depth: 8}, {Depth: 8}},
		Entries: entries,
	}
	img.Channels = []jpx.Channel{
		{Component: 0, PaletteColumn: 0, Kind: jpx.ChannelColor, Assoc: 1},
		{Component: 0, PaletteColumn: 1, Kind: jpx.ChannelColor, Assoc: 2},
		{Component: 0, PaletteColumn: 2, Kind: jpx.ChannelColor, Assoc: 3},
	}
	img.ColorSpecs = []jpx.ColorSpec{{Method: 1, Enumerated: 18}}
	res := decode(t, encode(t, img, true))

	// the entries are returned as stored
	if diff := cmp.Diff(&Palette{Bits: 8, Entries: entries}, res.Palette); diff != "" {
		t.Errorf("palette differs (-want +got):\n%s", diff)
	}
	if res.ColorSpace.Space != color.Space(labExtended) || res.ColorSpace.Convert == nil {
		t.Error("sYCC not described by labExtended with a conversion")
	}
}

func TestDecodeNoColour(t *testing.T) {
	img := newImage(0, 4, 4, []comp{{depth: 8}}, 10)
	img.Channels[0].Kind = jpx.ChannelUnknown
	img.Channels[0].Assoc = 65535
	_, err := Decode(encode(t, img, true), 0, true, membudget.New(64<<20))
	if err == nil {
		t.Error("missing error")
	}
}

func TestDecodeReduce(t *testing.T) {
	img := newImage(0, 20, 12, []comp{{depth: 8}, {depth: 8}, {depth: 8}}, 9)
	data := encode(t, img, true) // 2 levels, so 3 resolutions

	// reductions beyond the available levels are clamped
	cases := []struct{ reduce, w, h int }{
		{-1, 20, 12},
		{0, 20, 12},
		{1, 10, 6},
		{2, 5, 3},
		{3, 5, 3},
		{100, 5, 3},
	}
	for _, c := range cases {
		res, err := Decode(data, c.reduce, true, membudget.New(64<<20))
		if err != nil {
			t.Fatalf("reduce %d: %v", c.reduce, err)
		}
		if res.Width != c.w || res.Height != c.h {
			t.Errorf("reduce %d: got %dx%d, want %dx%d",
				c.reduce, res.Width, res.Height, c.w, c.h)
		}
		for i, p := range res.Color {
			if len(p) != c.w*c.h {
				t.Errorf("reduce %d: channel %d has %d samples", c.reduce, i, len(p))
			}
		}
	}
}

func TestDecodeReduceMalformed(t *testing.T) {
	_, err := Decode([]byte("not a JPEG 2000 file"), 1, true, membudget.New(64<<20))
	if err == nil {
		t.Error("missing error")
	}
}

// TestNewBudgetSuffices checks that legitimate but expensive codestreams
// decode within the budget from NewBudget: noise in deep components,
// small tiles, precincts and code-blocks, and a region of interest.
func TestNewBudgetSuffices(t *testing.T) {
	const w, h = 256, 256
	gray := func(depth int) []comp { return []comp{{depth: depth}} }
	roi := &jpx.Rectangle{X1: w, Y1: h}
	cases := []struct {
		name   string
		comps  []comp
		params jpx.EncodeParams
	}{
		{"16-bit CMYK and alpha", []comp{{depth: 16}, {depth: 16}, {depth: 16}, {depth: 16}, {depth: 16}}, jpx.EncodeParams{}},
		{"4x4 tiles", []comp{{depth: 8}, {depth: 8}, {depth: 8}}, jpx.EncodeParams{TileWidth: 4, TileHeight: 4}},
		{"2x2 tiles", gray(1), jpx.EncodeParams{TileWidth: 2, TileHeight: 2}},
		{"small precincts", gray(8), jpx.EncodeParams{Precincts: []jpx.PrecinctSize{{PPx: 1, PPy: 1}}}},
		{"small code-blocks with ROI", gray(12), jpx.EncodeParams{CodeBlockWidth: 4, CodeBlockHeight: 4, ROI: roi}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			img := newImage(0, w, h, tc.comps, 11)
			p := tc.params
			p.Wrapper = true
			var buf bytes.Buffer
			if err := jpx.Encode(&buf, img, &p); err != nil {
				t.Fatal(err)
			}
			budget, err := NewBudget(buf.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Decode(buf.Bytes(), 0, true, budget); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestNewBudgetGarbage(t *testing.T) {
	if _, err := NewBudget([]byte("not a JPEG 2000 file")); err == nil {
		t.Error("missing error")
	}
}

// TestUsable checks that the writer's view of a colour specification,
// through [ColorChannels] and [Usable], agrees with the choice of
// [Decode].
func TestUsable(t *testing.T) {
	specs := []jpx.ColorSpec{
		{Method: 1, Enumerated: 12},
		{Method: 1, Enumerated: 14},
		{Method: 1, Enumerated: 14, EnumParams: []byte{1, 2, 3}},
		{Method: 1, Enumerated: 16},
		{Method: 1, Enumerated: 17},
		{Method: 1, Enumerated: 18},
		{Method: 1, Enumerated: 19},
		{Method: 1, Enumerated: 20},
		{Method: 1, Enumerated: 21},
		{Method: 1, Enumerated: 24},
		{Method: 2, ICC: icc.SRGBv4Profile},
		{Method: 3, ICC: icc.CMYKProfile},
		{Method: 3, ICC: []byte("not an ICC profile")},
		{Method: 4, Vendor: make([]byte, 16)},
	}

	palette := newImage(0, 5, 4, []comp{{depth: 3}}, 14)
	palette.Palette = &jpx.Palette{
		Columns: []jpx.PaletteColumn{{Depth: 12}, {Depth: 10}, {Depth: 8}},
		Entries: [][]int32{{0, 0, 0}, {4095, 1023, 255}, {100, 200, 50}},
	}
	palette.Channels = nil
	for c := range 3 {
		palette.Channels = append(palette.Channels,
			jpx.Channel{Component: 0, PaletteColumn: c, Kind: jpx.ChannelColor, Assoc: c + 1})
	}
	images := map[string]*jpx.Image{
		"gray":    newImage(0, 5, 4, []comp{{depth: 8}}, 15),
		"rgb":     newImage(0, 5, 4, []comp{{depth: 12}, {depth: 8}, {depth: 4}}, 16),
		"cmyk":    newImage(0, 5, 4, []comp{{depth: 8}, {depth: 8}, {depth: 8}, {depth: 8}}, 17),
		"palette": palette,
	}
	for name, img := range images {
		for _, s := range specs {
			img.ColorSpecs = []jpx.ColorSpec{s}
			data := encode(t, img, true)
			h, err := jpx.ReadHeader(bytes.NewReader(data), nil)
			if err != nil {
				t.Fatal(err)
			}
			n, depths := ColorChannels(h)
			if n != len(depths) {
				t.Fatalf("%s: %d channels, %d depths", name, n, len(depths))
			}
			ok := Usable(s, depths)
			want, resolved := resolve(s, depths)
			if ok != resolved {
				t.Errorf("%s, spec %d/%d: Usable %t, resolve %t", name, s.Method, s.Enumerated, ok, resolved)
			}
			if !ok {
				want = deviceSpace(n)
			}

			res := decode(t, data)
			if !color.SpacesEqual(res.ColorSpace.Space, want.Space) {
				t.Errorf("%s, spec %d/%d: Decode uses %v, want %v",
					name, s.Method, s.Enumerated, res.ColorSpace.Space, want.Space)
			}
			if diff := cmp.Diff(want.Decode, res.ColorSpace.Decode); diff != "" {
				t.Errorf("%s, spec %d/%d: Decode differs (-want +got):\n%s",
					name, s.Method, s.Enumerated, diff)
			}
		}
	}

	// colour channels of a palette image have the depths of the columns
	_, depths := ColorChannels(&palette.Header)
	if diff := cmp.Diff([]int{12, 10, 8}, depths); diff != "" {
		t.Errorf("palette depths differ (-want +got):\n%s", diff)
	}
}

func FuzzDecode(f *testing.F) {
	specs := [][]jpx.ColorSpec{
		nil,
		{{Method: 1, Enumerated: 14}},
		{{Method: 1, Enumerated: 14, EnumParams: make([]byte, 28)}},
		{{Method: 1, Enumerated: 20}},
		{{Method: 1, Enumerated: 21}},
		{{Method: 1, Enumerated: 24}},
		{{Method: 1, Enumerated: 18}},
		{{Method: 2, ICC: icc.SRGBv4Profile}},
	}
	for i, s := range specs {
		img := newImage(0, 7, 5, []comp{{depth: 8}, {depth: 12}, {depth: 8, signed: true}}, uint64(i))
		img.ColorSpecs = s
		var buf bytes.Buffer
		if err := jpx.Encode(&buf, img, &jpx.EncodeParams{Wrapper: true, Levels: 1}); err != nil {
			f.Fatal(err)
		}
		f.Add(buf.Bytes())
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		budget := membudget.New(min(1<<20+1024*int64(len(data)), 4<<20))
		res, err := Decode(data, 0, true, budget)
		if err != nil {
			return
		}
		cs := res.ColorSpace
		n := len(res.Color)
		if res.Palette != nil {
			n = len(res.Palette.Entries[0])
		}
		if cs.Space == nil {
			return
		}
		if cs.Space.Channels() != n || len(cs.Decode) != 2*n {
			t.Fatalf("%d colour channels, colour space %v with Decode %v", n, cs.Space, cs.Decode)
		}
		v := make([]float64, n)
		for _, x := range []float64{0, 0.5, 1} {
			for c := range v {
				v[c] = cs.Decode[2*c] + x*(cs.Decode[2*c+1]-cs.Decode[2*c])
			}
			if cs.Convert != nil {
				cs.Convert(v)
			}
			for _, vc := range v {
				if math.IsNaN(vc) || math.IsInf(vc, 0) {
					t.Fatalf("colour value %v is not finite", v)
				}
			}
		}
	})
}
