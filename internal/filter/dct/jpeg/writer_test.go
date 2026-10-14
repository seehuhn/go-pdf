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

package jpeg

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image/color"
	stdjpeg "image/jpeg"
	"testing"

	"seehuhn.de/go/membudget"
)

// stripeColors are the colours of the four vertical stripes used by
// stripeImage, indexed by component count.
var stripeColors = map[int][4][]byte{
	1: {{20}, {90}, {160}, {230}},
	3: {{200, 40, 40}, {40, 180, 60}, {50, 70, 210}, {230, 220, 90}},
	4: {{10, 200, 30, 0}, {180, 20, 60, 40}, {90, 90, 200, 120}, {0, 0, 0, 230}},
}

// stripeImage returns interleaved samples for a w×h image made of four
// solid vertical stripes.  For w=24 the stripe boundaries fall on even
// columns, so 4:2:0 chroma subsampling does not mix stripe colours.
func stripeImage(w, h, nComp int) []byte {
	cols := stripeColors[nComp]
	data := make([]byte, 0, w*h*nComp)
	for range h {
		for x := range w {
			data = append(data, cols[min(4*x/w, 3)]...)
		}
	}
	return data
}

// cellImage returns interleaved RGB samples which are constant on
// 2×2 cells, so 4:2:0 chroma subsampling is lossless apart from rounding.
func cellImage(w, h int) []byte {
	data := make([]byte, 0, w*h*3)
	for y := range h {
		for x := range w {
			data = append(data, byte(40+15*(x/2)), byte(30+20*(y/2)), 128)
		}
	}
	return data
}

func encodeBytes(t *testing.T, data []byte, p EncodeParams) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := EncodeStream(&buf, bytes.NewReader(data), p); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func decodeBytes(t *testing.T, jpg []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	err := DecodeStream(bytes.NewReader(jpg), nil, &out, membudget.New(1<<30))
	if err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func checkClose(t *testing.T, got, want []byte, w, nComp, tol int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("decoded %d bytes, want %d", len(got), len(want))
	}
	for i := range want {
		d := int(got[i]) - int(want[i])
		if d < -tol || d > tol {
			pix := i / nComp
			t.Fatalf("pixel (%d,%d) component %d: got %d, want %d",
				pix%w, pix/w, i%nComp, got[i], want[i])
		}
	}
}

func TestEncodeRoundTrip(t *testing.T) {
	const w, h = 24, 20
	cases := []struct {
		name      string
		nComp     int
		transform bool
	}{
		{"gray", 1, false},
		{"rgb-transform", 3, true},
		{"rgb-raw", 3, false},
		{"cmyk-raw", 4, false},
		{"cmyk-ycbcrk", 4, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := stripeImage(w, h, c.nComp)
			jpg := encodeBytes(t, src, EncodeParams{
				Width: w, Height: h, Components: c.nComp,
				Transform: c.transform, Quality: 100,
			})
			got := decodeBytes(t, jpg)
			checkClose(t, got, src, w, c.nComp, 3)
		})
	}
}

type segment struct {
	marker  byte
	payload []byte
}

// segments returns the marker segments of a JPEG file from SOI up to
// and including SOS.  SOI itself is reported with an empty payload.
func segments(t *testing.T, jpg []byte) []segment {
	t.Helper()
	if len(jpg) < 2 || jpg[0] != 0xff || jpg[1] != soiMarker {
		t.Fatal("missing SOI marker")
	}
	res := []segment{{marker: soiMarker}}
	pos := 2
	for {
		if pos+4 > len(jpg) || jpg[pos] != 0xff {
			t.Fatalf("malformed segment at offset %d", pos)
		}
		marker := jpg[pos+1]
		n := int(binary.BigEndian.Uint16(jpg[pos+2:]))
		if n < 2 || pos+2+n > len(jpg) {
			t.Fatalf("bad segment length %d at offset %d", n, pos)
		}
		res = append(res, segment{marker, jpg[pos+4 : pos+2+n]})
		pos += 2 + n
		if marker == sosMarker {
			return res
		}
	}
}

func TestEncodeMarkers(t *testing.T) {
	for _, nComp := range []int{1, 3, 4} {
		for _, transform := range []bool{false, true} {
			jpg := encodeBytes(t, stripeImage(24, 20, nComp), EncodeParams{
				Width: 24, Height: 20, Components: nComp, Transform: transform,
			})
			segs := segments(t, jpg)
			chroma := transform && nComp >= 3

			var app14, sof, sos []byte
			for i, s := range segs {
				switch s.marker {
				case app0Marker:
					t.Errorf("%d/%v: unexpected APP0 segment", nComp, transform)
				case app14Marker:
					if i != 1 {
						t.Errorf("%d/%v: APP14 at position %d, want 1", nComp, transform, i)
					}
					app14 = s.payload
				case sof0Marker:
					sof = s.payload
				case sosMarker:
					sos = s.payload
				}
			}

			if nComp < 3 {
				if app14 != nil {
					t.Errorf("%d/%v: unexpected APP14 segment", nComp, transform)
				}
			} else {
				var flag byte
				if transform {
					flag = byte(nComp - 2)
				}
				want := append([]byte("Adobe\x00\x64\x00\x00\x00\x00"), flag)
				if !bytes.Equal(app14, want) {
					t.Errorf("%d/%v: APP14 payload % x, want % x", nComp, transform, app14, want)
				}
			}

			if len(sof) != 6+3*nComp || int(sof[5]) != nComp {
				t.Fatalf("%d/%v: bad SOF0 segment % x", nComp, transform, sof)
			}
			if len(sos) != 4+2*nComp || int(sos[0]) != nComp {
				t.Fatalf("%d/%v: bad SOS segment % x", nComp, transform, sos)
			}
			for i := range nComp {
				hv, tq := sof[6+3*i+1], sof[6+3*i+2]
				var wantHV, wantTable byte = 0x11, 0
				if chroma && (i == 0 || i == 3) {
					wantHV = 0x22
				}
				if chroma && (i == 1 || i == 2) {
					wantTable = 1
				}
				if sof[6+3*i] != byte(i+1) || sos[1+2*i] != byte(i+1) {
					t.Errorf("%d/%v: component %d has wrong ID", nComp, transform, i)
				}
				if hv != wantHV {
					t.Errorf("%d/%v: component %d sampling %#02x, want %#02x", nComp, transform, i, hv, wantHV)
				}
				if tq != wantTable || sos[2+2*i] != wantTable<<4|wantTable {
					t.Errorf("%d/%v: component %d uses wrong tables", nComp, transform, i)
				}
			}
		}
	}
}

func TestEncodeOddSize(t *testing.T) {
	const w, h = 13, 9
	src := cellImage(w, h)
	jpg := encodeBytes(t, src, EncodeParams{
		Width: w, Height: h, Components: 3, Transform: true, Quality: 100,
	})
	got := decodeBytes(t, jpg)
	checkClose(t, got, src, w, 3, 3)
}

func TestEncodeShortInput(t *testing.T) {
	p := EncodeParams{Width: 24, Height: 20, Components: 3, Transform: true}
	for _, n := range []int{0, 1, 24 * 3 * 17} {
		var buf bytes.Buffer
		err := EncodeStream(&buf, bytes.NewReader(make([]byte, n)), p)
		if err == nil {
			t.Errorf("%d input bytes: expected an error", n)
		}
	}
}

func TestEncodeBadParams(t *testing.T) {
	cases := []EncodeParams{
		{Width: 8, Height: 8, Components: 2},
		{Width: 0, Height: 8, Components: 1},
		{Width: 8, Height: 0, Components: 1},
		{Width: 1 << 16, Height: 8, Components: 1},
		{Width: 8, Height: 8, Components: 1, Quality: 101},
		{Width: 8, Height: 8, Components: 1, Quality: -1},
	}
	for _, p := range cases {
		data := make([]byte, max(p.Width*p.Height*p.Components, 0))
		var buf bytes.Buffer
		if err := EncodeStream(&buf, bytes.NewReader(data), p); err == nil {
			t.Errorf("%+v: expected an error", p)
		}
	}
}

func TestEncodeStdlibDecodes(t *testing.T) {
	const w, h = 24, 20

	// full decodes
	for _, nComp := range []int{1, 3} {
		src := stripeImage(w, h, nComp)
		jpg := encodeBytes(t, src, EncodeParams{
			Width: w, Height: h, Components: nComp, Transform: true, Quality: 100,
		})
		img, err := stdjpeg.Decode(bytes.NewReader(jpg))
		if err != nil {
			t.Fatalf("%d components: %v", nComp, err)
		}
		if b := img.Bounds(); b.Dx() != w || b.Dy() != h {
			t.Fatalf("%d components: decoded size %v", nComp, b)
		}
		got := make([]byte, 0, len(src))
		for y := range h {
			for x := range w {
				c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
				if nComp == 1 {
					got = append(got, c.R)
				} else {
					got = append(got, c.R, c.G, c.B)
				}
			}
		}
		checkClose(t, got, src, w, nComp, 3)
	}

	// header-only checks
	cases := []struct {
		nComp     int
		transform bool
		model     color.Model
	}{
		{3, false, color.RGBAModel},
		{4, false, color.CMYKModel},
		{4, true, color.CMYKModel},
	}
	for _, c := range cases {
		jpg := encodeBytes(t, stripeImage(w, h, c.nComp), EncodeParams{
			Width: w, Height: h, Components: c.nComp, Transform: c.transform,
		})
		cfg, err := stdjpeg.DecodeConfig(bytes.NewReader(jpg))
		if err != nil {
			t.Fatalf("%d/%v: %v", c.nComp, c.transform, err)
		}
		if cfg.Width != w || cfg.Height != h || cfg.ColorModel != c.model {
			t.Errorf("%d/%v: unexpected config %+v", c.nComp, c.transform, cfg)
		}
	}
}

func TestEncodeQuality(t *testing.T) {
	const w, h = 24, 20
	src := stripeImage(w, h, 3)
	for _, q := range []int{0, 1, 30, 100} {
		jpg := encodeBytes(t, src, EncodeParams{
			Width: w, Height: h, Components: 3, Transform: true, Quality: q,
		})
		got := decodeBytes(t, jpg)
		if len(got) != len(src) {
			t.Errorf("quality %d: decoded %d bytes, want %d", q, len(got), len(src))
		}
	}
}

// failWriter implements the encoder's buffered writer interface and
// fails every write.
type failWriter struct{}

var errWrite = errors.New("write failed")

func (failWriter) Write([]byte) (int, error) { return 0, errWrite }
func (failWriter) WriteByte(byte) error      { return errWrite }
func (failWriter) Flush() error              { return errWrite }

func TestEncodeWriteError(t *testing.T) {
	const w, h = 24, 20
	p := EncodeParams{Width: w, Height: h, Components: 1}
	err := EncodeStream(failWriter{}, bytes.NewReader(stripeImage(w, h, 1)), p)
	if err == nil {
		t.Error("expected an error")
	}
}
