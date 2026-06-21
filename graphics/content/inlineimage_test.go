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

package content

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
)

func TestDecodeInlineImageNoFilter(t *testing.T) {
	raw := []byte("hello world")
	op := Operator{
		Name: OpInlineImage,
		Args: []pdf.Object{pdf.Dict{}, pdf.String(raw)},
	}
	got, err := DecodeInlineImage(op, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, raw) {
		t.Errorf("expected %q, got %q", raw, got)
	}
}

func TestDecodeInlineImageFlateDecode(t *testing.T) {
	original := []byte("the quick brown fox jumps over the lazy dog")
	compressed := deflate(t, original)

	op := Operator{
		Name: OpInlineImage,
		Args: []pdf.Object{
			pdf.Dict{"Filter": pdf.Name("FlateDecode")},
			pdf.String(compressed),
		},
	}
	got, err := DecodeInlineImage(op, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Errorf("expected %q, got %q", original, got)
	}
}

func TestDecodeInlineImageAbbreviatedFilter(t *testing.T) {
	original := []byte("abbreviated filter test data here")
	compressed := deflate(t, original)

	op := Operator{
		Name: OpInlineImage,
		Args: []pdf.Object{
			pdf.Dict{"F": pdf.Name("Fl")},
			pdf.String(compressed),
		},
	}
	got, err := DecodeInlineImage(op, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Errorf("expected %q, got %q", original, got)
	}
}

func TestDecodeInlineImageUnknownFilter(t *testing.T) {
	op := Operator{
		Name: OpInlineImage,
		Args: []pdf.Object{
			pdf.Dict{"Filter": pdf.Name("NoSuchFilter")},
			pdf.String("data"),
		},
	}
	_, err := DecodeInlineImage(op, nil)
	if err == nil {
		t.Fatal("expected error for unknown filter")
	}
}

func TestDecodeInlineImageFilterArray(t *testing.T) {
	original := []byte("chained filter test")
	compressed := deflate(t, original)

	op := Operator{
		Name: OpInlineImage,
		Args: []pdf.Object{
			pdf.Dict{
				"Filter": pdf.Array{pdf.Name("Fl")},
			},
			pdf.String(compressed),
		},
	}
	got, err := DecodeInlineImage(op, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Errorf("expected %q, got %q", original, got)
	}
}

func TestDecodeInlineImageForbiddenFilter(t *testing.T) {
	for _, name := range []pdf.Name{"JBIG2Decode", "JPXDecode", "Crypt"} {
		t.Run(string(name), func(t *testing.T) {
			op := Operator{
				Name: OpInlineImage,
				Args: []pdf.Object{
					pdf.Dict{"Filter": name},
					pdf.String("data"),
				},
			}
			if _, err := DecodeInlineImage(op, nil); err == nil {
				t.Errorf("expected error for forbidden filter %s", name)
			}
		})
	}
}

func TestDecodeInlineImageForbiddenFilterInArray(t *testing.T) {
	op := Operator{
		Name: OpInlineImage,
		Args: []pdf.Object{
			pdf.Dict{"Filter": pdf.Array{pdf.Name("Fl"), pdf.Name("JBIG2Decode")}},
			pdf.String("data"),
		},
	}
	if _, err := DecodeInlineImage(op, nil); err == nil {
		t.Fatal("expected error for forbidden filter inside array")
	}
}

func TestDecodeInlineImageRejectsHugeDecodedBuffer(t *testing.T) {
	// DeviceCMYK at bpc=1, 10000×10000: the encoded form is ~48 MiB (well
	// under the 256 MiB encoded cap) but the decoded per-channel float64
	// buffer would be ~3.2 GiB, over the 2 GiB cap that image XObjects
	// reject.  The inline path must reject it before any allocation.
	dict := pdf.Dict{
		"W":   pdf.Integer(10000),
		"H":   pdf.Integer(10000),
		"CS":  pdf.Name("CMYK"),
		"BPC": pdf.Integer(1),
	}
	op := Operator{
		Name: OpInlineImage,
		Args: []pdf.Object{dict, pdf.String("x")},
	}
	if _, err := DecodeInlineImage(op, nil); err == nil {
		t.Fatal("expected error for oversized decoded float64 buffer")
	}
}

func TestDecodeInlineImageRejectsInvalidBPC(t *testing.T) {
	// A negative or non-whitelisted BPC must be rejected so it never reaches
	// the sample decoder.  A negative BPC over an Indexed colour space would
	// otherwise drive a negative shift count in image.DefaultDecode and panic.
	for _, bpc := range []pdf.Integer{-1, 0, 3, 5, 7, 32} {
		t.Run(fmt.Sprintf("bpc%d", bpc), func(t *testing.T) {
			dict := pdf.Dict{
				"W":   pdf.Integer(1),
				"H":   pdf.Integer(1),
				"BPC": bpc,
				"CS":  pdf.Array{pdf.Name("I"), pdf.Name("RGB"), pdf.Integer(1), pdf.String([]byte{0, 0, 0, 0xff, 0xff, 0xff})},
			}
			op := Operator{
				Name: OpInlineImage,
				Args: []pdf.Object{dict, pdf.String([]byte{0})},
			}
			if _, err := DecodeInlineImage(op, nil); err == nil {
				t.Errorf("expected error for BPC %d", bpc)
			}
		})
	}
}

func TestDecodeInlineImageAcceptsValidBPC(t *testing.T) {
	// valid bit depths, and a missing BPC (defaulted downstream), must decode
	for _, dict := range []pdf.Dict{
		{"W": pdf.Integer(1), "H": pdf.Integer(1), "BPC": pdf.Integer(1), "CS": pdf.Name("G")},
		{"W": pdf.Integer(1), "H": pdf.Integer(1), "BPC": pdf.Integer(8), "CS": pdf.Name("RGB")},
		{"W": pdf.Integer(1), "H": pdf.Integer(1), "BPC": pdf.Integer(16), "CS": pdf.Name("RGB")},
		{"W": pdf.Integer(1), "H": pdf.Integer(1), "CS": pdf.Name("RGB")}, // missing BPC
	} {
		op := Operator{
			Name: OpInlineImage,
			Args: []pdf.Object{dict, pdf.String([]byte{0, 0, 0, 0, 0, 0})},
		}
		if _, err := DecodeInlineImage(op, nil); err != nil {
			t.Errorf("unexpected error for %v: %v", dict, err)
		}
	}
}

func TestDecodeInlineImageMaskIgnoresBPC(t *testing.T) {
	// image masks are implicitly 1 bpc and must decode regardless of BPC
	dict := pdf.Dict{
		"W":  pdf.Integer(1),
		"H":  pdf.Integer(1),
		"IM": pdf.Boolean(true),
	}
	op := Operator{
		Name: OpInlineImage,
		Args: []pdf.Object{dict, pdf.String([]byte{0})},
	}
	if _, err := DecodeInlineImage(op, nil); err != nil {
		t.Fatal(err)
	}
}

func TestInlineImageColorSpaceDeviceNames(t *testing.T) {
	cases := []struct {
		name pdf.Name
		want color.Space
	}{
		{"G", color.SpaceDeviceGray},
		{"DeviceGray", color.SpaceDeviceGray},
		{"RGB", color.SpaceDeviceRGB},
		{"DeviceRGB", color.SpaceDeviceRGB},
		{"CMYK", color.SpaceDeviceCMYK},
		{"DeviceCMYK", color.SpaceDeviceCMYK},
	}
	for _, c := range cases {
		t.Run(string(c.name), func(t *testing.T) {
			// abbreviated key
			got := InlineImageColorSpace(pdf.Dict{"CS": c.name}, nil)
			if got != c.want {
				t.Errorf("CS=%s: got %v, want %v", c.name, got, c.want)
			}
			// full key
			got = InlineImageColorSpace(pdf.Dict{"ColorSpace": c.name}, nil)
			if got != c.want {
				t.Errorf("ColorSpace=%s: got %v, want %v", c.name, got, c.want)
			}
		})
	}
}

func TestInlineImageColorSpaceAbbreviationTakesPrecedence(t *testing.T) {
	// per §8.9.7: when both abbreviated and full key are present, the
	// abbreviated key takes precedence.
	dict := pdf.Dict{
		"CS":         pdf.Name("RGB"),
		"ColorSpace": pdf.Name("DeviceGray"),
	}
	got := InlineImageColorSpace(dict, nil)
	if got != color.SpaceDeviceRGB {
		t.Errorf("got %v, want SpaceDeviceRGB", got)
	}
}

func TestInlineImageColorSpaceIndexedArray(t *testing.T) {
	// [/I /RGB 1 <0102030405060708>] — 2-entry Indexed over DeviceRGB,
	// hival=1, 6 bytes of lookup data (2 entries × 3 channels).
	dict := pdf.Dict{
		"CS": pdf.Array{
			pdf.Name("I"),
			pdf.Name("RGB"),
			pdf.Integer(1),
			pdf.String{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
		},
	}
	got := InlineImageColorSpace(dict, nil)
	idx, ok := got.(*color.SpaceIndexed)
	if !ok {
		t.Fatalf("got %T, want *color.SpaceIndexed", got)
	}
	if idx.Channels() != 1 {
		t.Errorf("Channels() = %d, want 1", idx.Channels())
	}
	if idx.NumCol != 2 {
		t.Errorf("NumCol = %d, want 2", idx.NumCol)
	}
	if idx.Base != color.SpaceDeviceRGB {
		t.Errorf("Base = %v, want SpaceDeviceRGB", idx.Base)
	}
}

func TestInlineImageColorSpaceIndexedFullName(t *testing.T) {
	dict := pdf.Dict{
		"ColorSpace": pdf.Array{
			pdf.Name("Indexed"),
			pdf.Name("DeviceCMYK"),
			pdf.Integer(3),
			pdf.String(bytes.Repeat([]byte{0}, 16)), // 4 entries × 4 channels
		},
	}
	got := InlineImageColorSpace(dict, nil)
	if _, ok := got.(*color.SpaceIndexed); !ok {
		t.Fatalf("got %T, want *color.SpaceIndexed", got)
	}
}

func TestInlineImageColorSpaceIndexedBadBase(t *testing.T) {
	// CIE-based base is not permitted in inline image Indexed CS.
	dict := pdf.Dict{
		"CS": pdf.Array{
			pdf.Name("Indexed"),
			pdf.Name("CalGray"),
			pdf.Integer(1),
			pdf.String{0, 0},
		},
	}
	if got := InlineImageColorSpace(dict, nil); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestInlineImageColorSpaceResourceRef(t *testing.T) {
	custom := color.SpaceDeviceRGB
	res := &Resources{
		ColorSpace: map[pdf.Name]color.Space{"CS1": custom},
	}
	got := InlineImageColorSpace(pdf.Dict{"CS": pdf.Name("CS1")}, res)
	if got != custom {
		t.Errorf("got %v, want resource entry", got)
	}
}

func TestInlineImageColorSpaceResourceRefMissing(t *testing.T) {
	res := &Resources{ColorSpace: map[pdf.Name]color.Space{}}
	got := InlineImageColorSpace(pdf.Dict{"CS": pdf.Name("CS1")}, res)
	if got != nil {
		t.Errorf("got %v, want nil for missing resource", got)
	}
}

func TestInlineImageColorSpaceResourceRefNoRes(t *testing.T) {
	// non-device Name with no resources at all → nil.
	got := InlineImageColorSpace(pdf.Dict{"CS": pdf.Name("CS1")}, nil)
	if got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestInlineImageColorSpaceMissing(t *testing.T) {
	// no CS / ColorSpace entry.  Returns nil regardless of ImageMask;
	// callers decide whether nil is acceptable based on IM.
	if got := InlineImageColorSpace(pdf.Dict{}, nil); got != nil {
		t.Errorf("got %v, want nil", got)
	}
	if got := InlineImageColorSpace(pdf.Dict{"IM": pdf.Boolean(true)}, nil); got != nil {
		t.Errorf("got %v, want nil for image mask", got)
	}
}

func deflate(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := zlib.NewWriterLevel(&buf, zlib.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
