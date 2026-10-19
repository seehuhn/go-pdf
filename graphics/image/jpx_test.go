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
	"encoding/binary"
	"fmt"
	"math"
	"runtime"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"seehuhn.de/go/icc"
	"seehuhn.de/go/jpx"
	"seehuhn.de/go/membudget"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	pdfjpx "seehuhn.de/go/pdf/internal/filter/jpx"
)

// expectPix interleaves the given planes, mapping sample v of channel c
// to decode[2c] + v/maxV·(decode[2c+1]-decode[2c]).
func expectPix(planes [][]int32, bits int, decode []float64) []float32 {
	maxV := float64(int(1)<<bits - 1)
	n := len(planes)
	pix := make([]float32, len(planes[0])*n)
	for c, p := range planes {
		lo, hi := decode[2*c], decode[2*c+1]
		for i, v := range p {
			pix[i*n+c] = float32(lo + float64(v)/maxV*(hi-lo))
		}
	}
	return pix
}

func planesOf(img *jpx.Image) [][]int32 {
	var res [][]int32
	for _, p := range img.Planes {
		res = append(res, p.Pix)
	}
	return res
}

func TestLoadJPXDerived(t *testing.T) {
	type testCase struct {
		name   string
		depths []int
		cs     color.Space
	}
	cases := []testCase{
		{"gray", []int{8}, color.SpaceDeviceGray},
		{"gray16", []int{16}, color.SpaceDeviceGray},
		{"rgb", []int{8, 8, 8}, color.SpaceDeviceRGB},
		{"cmyk", []int{8, 8, 8, 8}, color.SpaceDeviceCMYK},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := jpxTestImage(5, 4, tc.depths, uint64(i))
			d := extractJPX(t, encodeJPX(t, src), 5, 4, nil)
			data, err := d.Load()
			if err != nil {
				t.Fatal(err)
			}
			if data.CS != tc.cs {
				t.Errorf("CS = %v, want %v", data.CS, tc.cs)
			}
			want := expectPix(planesOf(src), tc.depths[0], DefaultDecode(tc.cs, tc.depths[0]))
			if diff := cmp.Diff(want, data.Pix); diff != "" {
				t.Errorf("pixels differ (-want +got):\n%s", diff)
			}
			n := len(tc.depths)
			if data.NComp != n || data.Stride != 5*n || data.Rect.Dx() != 5 || data.Rect.Dy() != 4 {
				t.Errorf("wrong geometry %d %d %v", data.NComp, data.Stride, data.Rect)
			}
			if data.Alpha != nil {
				t.Error("unexpected alpha")
			}
		})
	}
}

func TestLoadJPXExplicitDecode(t *testing.T) {
	src := jpxTestImage(4, 3, []int{8, 8, 8}, 1)
	d := extractJPX(t, encodeJPX(t, src), 4, 3, pdf.Dict{
		"ColorSpace": pdf.Name("DeviceRGB"),
		"Decode":     pdf.Array{pdf.Integer(1), pdf.Integer(0), pdf.Integer(0), pdf.Integer(1), pdf.Number(0.25), pdf.Number(0.75)},
	})
	data, err := d.Load()
	if err != nil {
		t.Fatal(err)
	}
	want := expectPix(planesOf(src), 8, []float64{1, 0, 0, 1, 0.25, 0.75})
	if diff := cmp.Diff(want, data.Pix); diff != "" {
		t.Errorf("pixels differ (-want +got):\n%s", diff)
	}
}

// TestLoadJPXIndexedDecode checks that the default Decode array of an
// explicit Indexed colour space uses the depth of the JPEG 2000 data.
func TestLoadJPXIndexedDecode(t *testing.T) {
	src := jpxTestImage(4, 3, []int{2}, 2)
	d := extractJPX(t, encodeJPX(t, src), 4, 3, pdf.Dict{
		"ColorSpace": pdf.Array{pdf.Name("Indexed"), pdf.Name("DeviceGray"), pdf.Integer(3), pdf.String("\x00\x55\xaa\xff")},
	})
	if d.Decode != nil {
		t.Errorf("Decode = %v, want nil", d.Decode)
	}
	data, err := d.Load()
	if err != nil {
		t.Fatal(err)
	}
	want := expectPix(planesOf(src), 2, []float64{0, 3})
	if diff := cmp.Diff(want, data.Pix); diff != "" {
		t.Errorf("pixels differ (-want +got):\n%s", diff)
	}
}

func TestLoadJPXSYCC(t *testing.T) {
	src := jpxTestImage(4, 3, []int{8, 8, 8}, 3)
	src.ColorSpecs = []jpx.ColorSpec{{Method: 1, Enumerated: 18}}
	d := extractJPX(t, encodeJPX(t, src), 4, 3, nil)
	data, err := d.Load()
	if err != nil {
		t.Fatal(err)
	}

	// the conversion is covered by TestLoadJPXDerivedSpaces
	if _, ok := data.CS.(*color.SpaceLab); !ok {
		t.Errorf("CS = %v, want a Lab space", data.CS)
	}

	// an explicit colour space takes the samples as stored
	d = extractJPX(t, encodeJPX(t, src), 4, 3, pdf.Dict{"ColorSpace": pdf.Name("DeviceRGB")})
	data, err = d.Load()
	if err != nil {
		t.Fatal(err)
	}
	want := expectPix(planesOf(src), 8, []float64{0, 1, 0, 1, 0, 1})
	if diff := cmp.Diff(want, data.Pix); diff != "" {
		t.Errorf("explicit ColorSpace: pixels differ (-want +got):\n%s", diff)
	}

	// so does the colour key
	p := planesOf(src)
	key := []uint16{
		uint16(p[0][0]), uint16(p[0][0]),
		uint16(p[1][0]), uint16(p[1][0]),
		uint16(p[2][0]), uint16(p[2][0]),
	}
	d.MaskColors = key
	data, err = d.Load()
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(keyPlane(p, key), data.KeyMask.Pix); diff != "" {
		t.Errorf("explicit ColorSpace: colour key differs (-want +got):\n%s", diff)
	}
}

// paletteImage returns an image with one 10-bit index component and a
// palette of n RGB entries.
func paletteImage(n int) *jpx.Image {
	src := jpxTestImage(6, 5, []int{10}, 4)
	for i := range src.Planes[0].Pix {
		src.Planes[0].Pix[i] %= int32(n)
	}
	src.Palette = &jpx.Palette{Columns: []jpx.PaletteColumn{{Depth: 8}, {Depth: 8}, {Depth: 8}}}
	for i := range n {
		src.Palette.Entries = append(src.Palette.Entries, []int32{int32(i % 256), int32(255 - i%256), int32(i / 4 % 256)})
	}
	src.Channels = nil
	for c := range 3 {
		src.Channels = append(src.Channels, jpx.Channel{Component: 0, PaletteColumn: c, Kind: jpx.ChannelColor, Assoc: c + 1})
	}
	return src
}

// expanded returns the palette colours of the pixels of a palette image.
func expanded(src *jpx.Image) [][]int32 {
	res := make([][]int32, 3)
	for _, v := range src.Planes[0].Pix {
		for c := range 3 {
			res[c] = append(res[c], src.Palette.Entries[v][c])
		}
	}
	return res
}

func TestLoadJPXPalette(t *testing.T) {
	src := paletteImage(200)
	d := extractJPX(t, encodeJPX(t, src), 6, 5, nil)
	data, err := d.Load()
	if err != nil {
		t.Fatal(err)
	}
	cs, ok := data.CS.(*color.SpaceIndexed)
	if !ok {
		t.Fatalf("CS = %v, want Indexed", data.CS)
	}
	if cs.NumCol != 200 || cs.Base != color.SpaceDeviceRGB {
		t.Errorf("wrong Indexed space: %d colours, base %v", cs.NumCol, cs.Base)
	}
	for i, e := range src.Palette.Entries {
		want := []float64{float64(e[0]) / 255, float64(e[1]) / 255, float64(e[2]) / 255}
		if diff := cmp.Diff(want, cs.Lookup(i)); diff != "" {
			t.Fatalf("palette entry %d differs (-want +got):\n%s", i, diff)
		}
	}
	var want []float32
	for _, v := range src.Planes[0].Pix {
		want = append(want, float32(v))
	}
	if diff := cmp.Diff(want, data.Pix); diff != "" {
		t.Errorf("indices differ (-want +got):\n%s", diff)
	}
}

func TestLoadJPXPaletteExpanded(t *testing.T) {
	// more than 256 entries cannot form an Indexed space
	src := paletteImage(300)
	d := extractJPX(t, encodeJPX(t, src), 6, 5, nil)
	data, err := d.Load()
	if err != nil {
		t.Fatal(err)
	}
	if data.CS != color.SpaceDeviceRGB {
		t.Errorf("CS = %v, want DeviceRGB", data.CS)
	}
	want := expectPix(expanded(src), 8, []float64{0, 1, 0, 1, 0, 1})
	if diff := cmp.Diff(want, data.Pix); diff != "" {
		t.Errorf("pixels differ (-want +got):\n%s", diff)
	}

	// an explicit colour space with the palette's channel count
	src = paletteImage(10)
	d = extractJPX(t, encodeJPX(t, src), 6, 5, pdf.Dict{"ColorSpace": pdf.Name("DeviceRGB")})
	data, err = d.Load()
	if err != nil {
		t.Fatal(err)
	}
	want = expectPix(expanded(src), 8, []float64{0, 1, 0, 1, 0, 1})
	if diff := cmp.Diff(want, data.Pix); diff != "" {
		t.Errorf("pixels differ (-want +got):\n%s", diff)
	}
}

// TestLoadJPXGrayPalette checks that an explicit single-channel colour
// space applies a one-column palette, while an Indexed space takes the
// palette indices.
func TestLoadJPXGrayPalette(t *testing.T) {
	src := jpxTestImage(6, 5, []int{2}, 7)
	src.Palette = &jpx.Palette{
		Columns: []jpx.PaletteColumn{{Depth: 8}},
		Entries: [][]int32{{200}, {10}, {150}, {60}},
	}
	src.Channels[0].PaletteColumn = 0
	data := encodeJPX(t, src)

	d := extractJPX(t, data, 6, 5, pdf.Dict{"ColorSpace": pdf.Name("DeviceGray")})
	got, err := d.Load()
	if err != nil {
		t.Fatal(err)
	}
	var grey []int32
	for _, v := range src.Planes[0].Pix {
		grey = append(grey, src.Palette.Entries[v][0])
	}
	want := expectPix([][]int32{grey}, 8, []float64{0, 1})
	if diff := cmp.Diff(want, got.Pix); diff != "" {
		t.Errorf("DeviceGray: pixels differ (-want +got):\n%s", diff)
	}

	d = extractJPX(t, data, 6, 5, pdf.Dict{
		"ColorSpace": pdf.Array{pdf.Name("Indexed"), pdf.Name("DeviceGray"), pdf.Integer(3), pdf.String("\x00\x55\xaa\xff")},
	})
	got, err = d.Load()
	if err != nil {
		t.Fatal(err)
	}
	want = expectPix(planesOf(src), 2, []float64{0, 3})
	if diff := cmp.Diff(want, got.Pix); diff != "" {
		t.Errorf("Indexed: pixels differ (-want +got):\n%s", diff)
	}
}

func TestLoadJPXChannelMismatch(t *testing.T) {
	src := jpxTestImage(4, 3, []int{8, 8, 8}, 5)
	d := extractJPX(t, encodeJPX(t, src), 4, 3, pdf.Dict{"ColorSpace": pdf.Name("DeviceCMYK")})
	if _, err := d.Load(); err == nil {
		t.Error("missing error")
	}

	src = jpxTestImage(4, 3, []int{8, 8}, 5)
	d = extractJPX(t, encodeJPX(t, src), 4, 3, nil)
	if _, err := d.Load(); err == nil {
		t.Error("missing error for two colour channels")
	}
}

// opacityImage returns an RGB image with an 8-bit opacity channel.  With
// premultiplied set, the colour samples are premultiplied.
func opacityImage(premultiplied bool) *jpx.Image {
	src := jpxTestImage(5, 4, []int{8, 8, 8, 8}, 6)
	kind := jpx.ChannelOpacity
	if premultiplied {
		kind = jpx.ChannelPremultipliedOpacity
		alpha := src.Planes[3].Pix
		for c := range 3 {
			for i, v := range src.Planes[c].Pix {
				src.Planes[c].Pix[i] = (v*alpha[i] + 127) / 255
			}
		}
	}
	src.Channels[3] = jpx.Channel{Component: 3, PaletteColumn: -1, Kind: kind, Assoc: 0}
	return src
}

func TestLoadJPXAlpha(t *testing.T) {
	src := opacityImage(false)
	data := src.Planes[3].Pix
	wantAlpha := make([]float32, len(data))
	for i, v := range data {
		wantAlpha[i] = float32(float64(v) / 255)
	}
	wantPix := expectPix(planesOf(src)[:3], 8, []float64{0, 1, 0, 1, 0, 1})

	for _, smid := range []int{0, 1} {
		d := extractJPX(t, encodeJPX(t, src), 5, 4, pdf.Dict{"SMaskInData": pdf.Integer(smid)})
		img, err := d.Load()
		if err != nil {
			t.Fatal(err)
		}
		if img.NComp != 3 {
			t.Errorf("NComp = %d, want 3", img.NComp)
		}
		if diff := cmp.Diff(wantPix, img.Pix); diff != "" {
			t.Errorf("SMaskInData %d: pixels differ (-want +got):\n%s", smid, diff)
		}
		if smid == 0 {
			if img.Alpha != nil {
				t.Error("Alpha set for SMaskInData 0")
			}
		} else if diff := cmp.Diff(wantAlpha, img.Alpha); diff != "" {
			t.Errorf("alpha differs (-want +got):\n%s", diff)
		}
	}
}

func TestLoadJPXPremultiplied(t *testing.T) {
	src := opacityImage(true)
	d := extractJPX(t, encodeJPX(t, src), 5, 4, pdf.Dict{"SMaskInData": pdf.Integer(2)})
	img, err := d.Load()
	if err != nil {
		t.Fatal(err)
	}

	alpha := src.Planes[3].Pix
	want := make([]float32, 3*len(alpha))
	for c := range 3 {
		for i, v := range src.Planes[c].Pix {
			x := float64(v) / 255
			if a := float64(alpha[i]) / 255; a > 0 {
				x = min(x/a, 1)
			}
			want[3*i+c] = float32(x)
		}
	}
	if diff := cmp.Diff(want, img.Pix); diff != "" {
		t.Errorf("pixels differ (-want +got):\n%s", diff)
	}
	if len(img.Alpha) != len(alpha) {
		t.Errorf("got %d alpha values, want %d", len(img.Alpha), len(alpha))
	}
}

// TestLoadJPXSize checks that the size of the JPEG 2000 data wins over
// the size given in the image dictionary.
func TestLoadJPXSize(t *testing.T) {
	src := jpxTestImage(13, 7, []int{8}, 7)
	d := extractJPX(t, encodeJPX(t, src), 5, 5, nil)
	data, err := d.Load()
	if err != nil {
		t.Fatal(err)
	}
	if data.Rect.Dx() != 13 || data.Rect.Dy() != 7 || len(data.Pix) != 13*7 {
		t.Errorf("got %v with %d samples, want 13x7", data.Rect, len(data.Pix))
	}
}

// TestLoadJPXHostile checks that a codestream declaring a huge image is
// rejected by the pixel limit, without a matching allocation.
func TestLoadJPXHostile(t *testing.T) {
	src := jpxTestImage(16, 16, []int{8}, 8)
	var buf bytes.Buffer
	if err := jpx.Encode(&buf, src, &jpx.EncodeParams{Levels: 2}); err != nil {
		t.Fatal(err)
	}
	cs := buf.Bytes()
	// SIZ: Xsiz, Ysiz at offset 8, XTsiz, YTsiz at offset 24
	for _, off := range []int{8, 12, 24, 28} {
		binary.BigEndian.PutUint32(cs[off:], 60000)
	}
	d := extractJPX(t, cs, 16, 16, nil)

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err := d.Load()
	runtime.ReadMemStats(&after)
	if err == nil {
		t.Error("missing error")
	}
	if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 64<<20 {
		t.Errorf("allocated %d bytes", alloc)
	}
}

func TestLoadJPXGarbage(t *testing.T) {
	d := extractJPX(t, []byte("this is not a JPEG 2000 file"), 4, 4, nil)
	if _, err := d.Load(); err == nil {
		t.Error("missing error")
	}
}

// TestDictJPXDecodeRoundTrip checks that an explicit Decode array of a
// JPXDecode image survives a write-read cycle, also when it equals the
// default for some bit depth.
func TestDictJPXDecodeRoundTrip(t *testing.T) {
	src := jpxTestImage(4, 3, []int{8}, 9)
	for _, decode := range []pdf.Array{nil, {pdf.Integer(0), pdf.Integer(1)}, {pdf.Integer(1), pdf.Integer(0)}} {
		extras := pdf.Dict{"ColorSpace": pdf.Name("DeviceGray")}
		if decode != nil {
			extras["Decode"] = decode
		}
		orig := extractJPX(t, encodeJPX(t, src), 4, 3, extras)

		w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
		rm := pdf.NewResourceManager(w)
		ref, err := rm.Embed(orig)
		if err != nil {
			t.Fatal(err)
		}
		if err := rm.Close(); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		got, err := ExtractDict(pdf.CursorAt(pdf.NewExtractor(w), nil), ref, false)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(orig.Decode, got.Decode); diff != "" {
			t.Errorf("round trip failed (-want +got):\n%s", diff)
		}
	}
}

// TestLoadJPXFilterChain checks that filters before JPXDecode are
// applied.
func TestLoadJPXFilterChain(t *testing.T) {
	src := jpxTestImage(4, 3, []int{8}, 10)
	hex := fmt.Appendf(nil, "%x>", encodeJPX(t, src))
	d := extractJPX(t, hex, 4, 3, pdf.Dict{
		"Filter":      pdf.Array{pdf.Name("ASCIIHexDecode"), pdf.Name("JPXDecode")},
		"DecodeParms": pdf.Array{nil, nil},
	})
	data, err := d.Load()
	if err != nil {
		t.Fatal(err)
	}
	want := expectPix(planesOf(src), 8, []float64{0, 1})
	if diff := cmp.Diff(want, data.Pix); diff != "" {
		t.Errorf("pixels differ (-want +got):\n%s", diff)
	}
}

func TestToRGBAAlpha(t *testing.T) {
	d := NewData(color.SpaceDeviceGray, 2, 1)
	d.Pix[0], d.Pix[1] = 1, 0.5
	d.Alpha = []float32{0.5, 0}
	got := d.ToRGBA()
	want := []uint8{128, 128, 128, 128, 0, 0, 0, 0}
	if diff := cmp.Diff(want, got.Pix); diff != "" {
		t.Errorf("pixels differ (-want +got):\n%s", diff)
	}
}

func TestLoadJPXICC(t *testing.T) {
	type testCase struct {
		name  string
		specs []jpx.ColorSpec
		isICC bool
	}
	cases := []testCase{
		{"valid", []jpx.ColorSpec{{Method: 2, ICC: icc.SRGBv2Profile}}, true},
		{"invalid", []jpx.ColorSpec{{Method: 2, ICC: []byte("not an ICC profile")}}, false},
		{"wrong channel count", []jpx.ColorSpec{{Method: 2, ICC: icc.CMYKProfile}}, false},
		{"fallback after unusable profile", []jpx.ColorSpec{
			{Method: 2, Precedence: 1, ICC: icc.CMYKProfile},
			{Method: 2, Precedence: 0, ICC: icc.SRGBv2Profile},
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := jpxTestImage(4, 3, []int{8, 8, 8}, 11)
			src.ColorSpecs = tc.specs
			d := extractJPX(t, encodeJPX(t, src), 4, 3, nil)
			data, err := d.Load()
			if err != nil {
				t.Fatal(err)
			}
			_, isICC := data.CS.(*color.SpaceICCBased)
			if isICC != tc.isICC {
				t.Errorf("CS = %v, ICC-based %t, want %t", data.CS, isICC, tc.isICC)
			}
			if !isICC && data.CS != color.SpaceDeviceRGB {
				t.Errorf("CS = %v, want DeviceRGB", data.CS)
			}
		})
	}
}

// gradientJPX encodes a smooth 64×64 RGB image with three wavelet levels,
// optionally with an opacity channel.
func gradientJPX(t *testing.T, withAlpha bool) []byte {
	t.Helper()
	const n = 64
	img := jpxTestImage(n, n, []int{8, 8, 8, 8}, 1)
	for y := range n {
		for x := range n {
			i := y*n + x
			img.Planes[0].Pix[i] = int32(4 * x)
			img.Planes[1].Pix[i] = int32(4 * y)
			img.Planes[2].Pix[i] = int32(2 * (x + y))
			img.Planes[3].Pix[i] = int32(255 - 2*(x+y))
		}
	}
	if withAlpha {
		img.Channels[3] = jpx.Channel{Component: 3, PaletteColumn: -1, Kind: jpx.ChannelOpacity}
	} else {
		img.Components = img.Components[:3]
		img.Planes = img.Planes[:3]
		img.Channels = img.Channels[:3]
	}
	var buf bytes.Buffer
	if err := jpx.Encode(&buf, img, &jpx.EncodeParams{Wrapper: true, Levels: 3}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// checkReduced verifies that every pixel of a reduced decode of the
// smooth test image is close to the full-resolution pixel at the top-left
// corner of the area it covers: the reduced JPEG 2000 grid samples the
// even positions of each level.  At the image edges, where the wavelet
// filter sees the mirrored image, the error grows with the scale.
func checkReduced(t *testing.T, full, img *Data, scale int) {
	t.Helper()
	tol := float64(2*scale) / 255
	for y := range img.Rect.Dy() {
		for x := range img.Rect.Dx() {
			for c := range img.NComp {
				got := img.Pix[y*img.Stride+x*img.NComp+c]
				want := full.Pix[y*scale*full.Stride+x*scale*full.NComp+c]
				if math.Abs(float64(got-want)) > tol {
					t.Fatalf("scale %d: pixel (%d,%d) channel %d is %.3f, want %.3f",
						scale, x, y, c, got, want)
				}
			}
			if img.Alpha != nil {
				got := img.Alpha[y*img.Rect.Dx()+x]
				want := full.Alpha[y*scale*full.Rect.Dx()+x*scale]
				if math.Abs(float64(got-want)) > tol {
					t.Fatalf("scale %d: alpha (%d,%d) is %.3f, want %.3f",
						scale, x, y, got, want)
				}
			}
		}
	}
}

func TestLoadReducedJPX(t *testing.T) {
	d := extractJPX(t, gradientJPX(t, false), 64, 64, nil)
	full, err := d.Load()
	if err != nil {
		t.Fatal(err)
	}

	// three wavelet levels allow at most three reductions
	cases := []struct{ reduce, size int }{
		{0, 64},
		{1, 32},
		{2, 16},
		{10, 8},
	}
	for _, c := range cases {
		img, err := d.LoadReduced(c.reduce)
		if err != nil {
			t.Fatalf("reduce %d: %v", c.reduce, err)
		}
		if img.Rect.Dx() != c.size || img.Rect.Dy() != c.size {
			t.Errorf("reduce %d: got size %v, want %d×%d", c.reduce, img.Rect, c.size, c.size)
		}
		if len(img.Pix) != c.size*c.size*3 || img.Stride != 3*c.size {
			t.Errorf("reduce %d: got %d samples, stride %d", c.reduce, len(img.Pix), img.Stride)
		}
		checkReduced(t, full, img, 64/c.size)
	}

	if diff := cmp.Diff(full, mustLoadReduced(t, d, 0)); diff != "" {
		t.Errorf("LoadReduced(0) differs from Load (-want +got):\n%s", diff)
	}
}

func mustLoadReduced(t *testing.T, d *Dict, reduce int) *Data {
	t.Helper()
	img, err := d.LoadReduced(reduce)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestLoadReducedJPXAlpha(t *testing.T) {
	d := extractJPX(t, gradientJPX(t, true), 64, 64, pdf.Dict{"SMaskInData": pdf.Integer(1)})
	full := mustLoadReduced(t, d, 0)
	img := mustLoadReduced(t, d, 2)
	if img.Rect.Dx() != 16 || img.Rect.Dy() != 16 {
		t.Fatalf("got size %v, want 16×16", img.Rect)
	}
	if len(img.Alpha) != 16*16 {
		t.Fatalf("got %d alpha values, want %d", len(img.Alpha), 16*16)
	}

	checkReduced(t, full, img, 4)
}

func TestLoadReducedNonJPX(t *testing.T) {
	src := NewData(color.SpaceDeviceRGB, 8, 8)
	for y := range 8 {
		for x := range 8 {
			src.Set(x, y, color.DeviceRGB{float64(x) / 7, float64(y) / 7, 0.5})
		}
	}
	dict := FromImage(src, color.SpaceDeviceRGB, 8)

	want, err := dict.Load()
	if err != nil {
		t.Fatal(err)
	}
	got := mustLoadReduced(t, dict, 2)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("LoadReduced(2) differs from Load (-want +got):\n%s", diff)
	}
}

// derivedSpecs are colour specifications whose samples map to a colour
// space other than a device space.
var derivedSpecs = map[string]jpx.ColorSpec{
	"CIELab":   {Method: 1, Enumerated: 14},
	"e-sRGB":   {Method: 1, Enumerated: 20},
	"ROMM-RGB": {Method: 1, Enumerated: 21},
	"sYCC":     {Method: 1, Enumerated: 18},
	"e-sYCC":   {Method: 1, Enumerated: 24},
	"ICC":      {Method: 2, ICC: icc.SRGBv2Profile},
}

// specColorSpace returns the description of a colour specification for
// three channels of the given depth.
func specColorSpace(t *testing.T, spec jpx.ColorSpec, depth int) pdfjpx.ColorSpace {
	t.Helper()
	img := jpxTestImage(1, 1, []int{depth, depth, depth}, 0)
	img.ColorSpecs = []jpx.ColorSpec{spec}
	res, err := pdfjpx.Decode(encodeJPX(t, img), 0, true, membudget.New(1<<20))
	if err != nil {
		t.Fatal(err)
	}
	return res.ColorSpace
}

// mapSample maps the samples of one pixel, normalised to [0, 1], to
// colour values.
func mapSample(cs pdfjpx.ColorSpace, x []float64) []float64 {
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

var pixApprox = cmpopts.EquateApprox(0, 1e-5)

func TestLoadJPXDerivedSpaces(t *testing.T) {
	for name, spec := range derivedSpecs {
		t.Run(name, func(t *testing.T) {
			src := jpxTestImage(6, 5, []int{8, 8, 8}, 40)
			src.ColorSpecs = []jpx.ColorSpec{spec}
			d := extractJPX(t, encodeJPX(t, src), 6, 5, nil)
			data, err := d.Load()
			if err != nil {
				t.Fatal(err)
			}
			cs := specColorSpace(t, spec, 8)
			if !color.SpacesEqual(data.CS, cs.Space) {
				t.Errorf("CS = %v, want %v", data.CS, cs.Space)
			}
			var want []float32
			for i := range 6 * 5 {
				x := make([]float64, 3)
				for c := range 3 {
					x[c] = float64(src.Planes[c].Pix[i]) / 255
				}
				for _, v := range mapSample(cs, x) {
					want = append(want, float32(v))
				}
			}
			if diff := cmp.Diff(want, data.Pix, pixApprox); diff != "" {
				t.Errorf("pixels differ (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLoadJPXDerivedPalette(t *testing.T) {
	for name, spec := range derivedSpecs {
		for _, n := range []int{200, 300} {
			t.Run(fmt.Sprintf("%s/%d", name, n), func(t *testing.T) {
				src := paletteImage(n)
				src.ColorSpecs = []jpx.ColorSpec{spec}
				d := extractJPX(t, encodeJPX(t, src), 6, 5, nil)
				data, err := d.Load()
				if err != nil {
					t.Fatal(err)
				}
				cs := specColorSpace(t, spec, 8)
				entry := func(i int) []float64 {
					e := src.Palette.Entries[i]
					return mapSample(cs, []float64{float64(e[0]) / 255, float64(e[1]) / 255, float64(e[2]) / 255})
				}

				if n > 256 {
					if !color.SpacesEqual(data.CS, cs.Space) {
						t.Errorf("CS = %v, want %v", data.CS, cs.Space)
					}
					var want []float32
					for _, v := range src.Planes[0].Pix {
						for _, x := range entry(int(v)) {
							want = append(want, float32(x))
						}
					}
					if diff := cmp.Diff(want, data.Pix, pixApprox); diff != "" {
						t.Errorf("pixels differ (-want +got):\n%s", diff)
					}
					return
				}

				idx, ok := data.CS.(*color.SpaceIndexed)
				if !ok {
					t.Fatalf("CS = %v, want Indexed", data.CS)
				}
				tol := 1e-9
				if cs.Convert != nil {
					// a Lab space fitted to the palette
					base, ok := idx.Base.(*color.SpaceLab)
					if !ok {
						t.Fatalf("base = %v, want Lab", idx.Base)
					}
					for _, r := range base.Ranges {
						if r != math.Round(r) {
							t.Errorf("ranges %v are not whole numbers", base.Ranges)
						}
					}
					// one step of the 8-bit lookup table
					tol = max(100, base.Ranges[1]-base.Ranges[0], base.Ranges[3]-base.Ranges[2]) / 255
				} else if !color.SpacesEqual(idx.Base, cs.Space) {
					t.Errorf("base = %v, want %v", idx.Base, cs.Space)
				}
				for i := range n {
					want, got := entry(i), idx.Lookup(i)
					for c := range want {
						if math.Abs(want[c]-got[c]) > tol {
							t.Fatalf("palette entry %d: got %v, want %v", i, got, want)
						}
					}
				}
			})
		}
	}
}

func TestLoadJPXDerivedPremultiplied(t *testing.T) {
	for name, spec := range derivedSpecs {
		for _, premultiplied := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%t", name, premultiplied), func(t *testing.T) {
				src := opacityImage(premultiplied)
				src.ColorSpecs = []jpx.ColorSpec{spec}
				d := extractJPX(t, encodeJPX(t, src), 5, 4, pdf.Dict{"SMaskInData": pdf.Integer(1)})
				img, err := d.Load()
				if err != nil {
					t.Fatal(err)
				}
				cs := specColorSpace(t, spec, 8)
				alpha := src.Planes[3].Pix
				var want []float32
				for i := range alpha {
					x := make([]float64, 3)
					for c := range 3 {
						x[c] = float64(src.Planes[c].Pix[i]) / 255
						if a := float64(alpha[i]) / 255; premultiplied && a > 0 {
							x[c] = min(x[c]/a, 1)
						}
					}
					for _, v := range mapSample(cs, x) {
						want = append(want, float32(v))
					}
				}
				if diff := cmp.Diff(want, img.Pix, pixApprox); diff != "" {
					t.Errorf("pixels differ (-want +got):\n%s", diff)
				}
			})
		}
	}
}

// TestLoadJPXExplicitUnappliedPalette checks that samples which the image
// dictionary declares as colour values are un-premultiplied, even where
// the JPEG 2000 data has a palette which is not applied.
func TestLoadJPXExplicitUnappliedPalette(t *testing.T) {
	src := paletteImage(10)
	opacity := jpxTestImage(6, 5, []int{8}, 41)
	src.Components = append(src.Components, opacity.Components[0])
	src.Planes = append(src.Planes, opacity.Planes[0])
	src.Channels = append(src.Channels, jpx.Channel{
		Component: 1, PaletteColumn: -1, Kind: jpx.ChannelPremultipliedOpacity, Assoc: 0,
	})
	d := extractJPX(t, encodeJPX(t, src), 6, 5, pdf.Dict{
		"ColorSpace":  pdf.Name("DeviceGray"),
		"SMaskInData": pdf.Integer(1),
	})
	img, err := d.Load()
	if err != nil {
		t.Fatal(err)
	}
	var want []float32
	for i, v := range src.Planes[0].Pix {
		x := float64(v) / 1023
		if a := float64(opacity.Planes[0].Pix[i]) / 255; a > 0 {
			x = min(x/a, 1)
		}
		want = append(want, float32(x))
	}
	if diff := cmp.Diff(want, img.Pix); diff != "" {
		t.Errorf("pixels differ (-want +got):\n%s", diff)
	}
}

// TestJPXSourceDerivedSpaces checks that images with each colour
// specification can be written and read back, and that the colour space
// derived on reading can be embedded in PDF 1.7 (the version the ROMM RGB
// profile requires).
func TestJPXSourceDerivedSpaces(t *testing.T) {
	for name, spec := range derivedSpecs {
		for _, v := range []pdf.Version{pdf.V1_5, pdf.V1_7} {
			t.Run(fmt.Sprintf("%s/%s", name, v), func(t *testing.T) {
				src := jpxTestImage(6, 5, []int{8, 8, 8}, 42)
				src.ColorSpecs = []jpx.ColorSpec{spec}
				d := &Dict{Width: 6, Height: 5, Data: NewJPXSource(encodeJPX(t, src))}
				got, err := roundTrip(t, v, d, ExtractDict)
				if err != nil {
					t.Fatal(err)
				}
				if got.ColorSpace != nil {
					t.Errorf("ColorSpace = %v, want none", got.ColorSpace)
				}
				data, err := got.Load()
				if err != nil {
					t.Fatal(err)
				}
				if !color.SpacesEqual(data.CS, specColorSpace(t, spec, 8).Space) {
					t.Errorf("CS = %v", data.CS)
				}

				if v < pdf.V1_7 {
					return
				}
				w, _ := memfile.NewPDFWriter(t, v, nil)
				rm := pdf.NewResourceManager(w)
				if _, err := rm.Embed(data.CS); err != nil {
					t.Errorf("embedding %v: %v", data.CS, err)
				}
				if err := rm.Close(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestFitLab(t *testing.T) {
	lab, err := color.Lab(color.WhitePointD65, nil, []float64{-1000, 1000, -1000, 1000})
	if err != nil {
		t.Fatal(err)
	}
	got, err := fitLab(lab, [][]float64{{50, 3, -7.5}, {60, 3, 20.2}})
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([4]float64{3, 4, -8, 21}, got.Ranges); diff != "" {
		t.Errorf("ranges differ (-want +got):\n%s", diff)
	}
	if got.WhitePoint != lab.WhitePoint {
		t.Errorf("white point %v, want %v", got.WhitePoint, lab.WhitePoint)
	}
}
