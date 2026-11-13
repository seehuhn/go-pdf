// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package image

import (
	"bytes"
	"image"
	"image/jpeg"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func inlineOp(dict pdf.Dict, data []byte) content.Operator {
	return content.Operator{
		Name: content.OpInlineImage,
		Args: []pdf.Object{dict, pdf.String(data)},
	}
}

func TestExtractInlineSampled(t *testing.T) {
	// a 2×1 RGB image: one red and one blue pixel
	data := []byte{255, 0, 0, 0, 0, 255}
	want := []float32{1, 0, 0, 0, 0, 1}

	for _, tc := range []struct {
		name string
		dict pdf.Dict
	}{
		{"abbreviated", pdf.Dict{
			"W": pdf.Integer(2), "H": pdf.Integer(1), "BPC": pdf.Integer(8),
			"CS": pdf.Name("RGB"), "I": pdf.Boolean(true),
			"Intent": pdf.Name("Saturation"),
		}},
		{"full", pdf.Dict{
			"Width": pdf.Integer(2), "Height": pdf.Integer(1),
			"BitsPerComponent": pdf.Integer(8), "ColorSpace": pdf.Name("DeviceRGB"),
			"Interpolate": pdf.Boolean(true), "Intent": pdf.Name("Saturation"),
		}},
		{"abbreviation wins", pdf.Dict{
			"W": pdf.Integer(2), "Width": pdf.Integer(7),
			"H": pdf.Integer(1), "Height": pdf.Integer(7),
			"BPC": pdf.Integer(8), "BitsPerComponent": pdf.Integer(1),
			"CS": pdf.Name("RGB"), "ColorSpace": pdf.Name("G"),
			"I": pdf.Boolean(true), "Interpolate": pdf.Boolean(false),
			"Intent": pdf.Name("Saturation"),
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ExtractInline(inlineOp(tc.dict, data), nil)
			if err != nil {
				t.Fatal(err)
			}
			img, ok := got.(*Dict)
			if !ok {
				t.Fatalf("got %T, want *Dict", got)
			}
			if img.Width != 2 || img.Height != 1 || img.BitsPerComponent != 8 {
				t.Errorf("got %d×%d at %d bpc, want 2×1 at 8 bpc",
					img.Width, img.Height, img.BitsPerComponent)
			}
			if img.ColorSpace != color.SpaceDeviceRGB {
				t.Errorf("got colour space %v, want DeviceRGB", img.ColorSpace)
			}
			if !img.Interpolate {
				t.Error("Interpolate not set")
			}
			if img.Intent != graphics.Saturation {
				t.Errorf("got intent %q, want Saturation", img.Intent)
			}
			loaded, err := img.Load()
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(want, loaded.Pix); diff != "" {
				t.Errorf("pixels differ (-want +got):\n%s", diff)
			}
		})
	}
}

func TestExtractInlineMask(t *testing.T) {
	dict := pdf.Dict{
		"W": pdf.Integer(8), "H": pdf.Integer(1), "IM": pdf.Boolean(true),
		"D": pdf.Array{pdf.Integer(1), pdf.Integer(0)}, "I": pdf.Boolean(true),
	}
	got, err := ExtractInline(inlineOp(dict, []byte{0xF0}), nil)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := got.(*Mask)
	if !ok {
		t.Fatalf("got %T, want *Mask", got)
	}
	if m.Width != 8 || m.Height != 1 || !m.Inverted || !m.Interpolate {
		t.Errorf("got %d×%d, Inverted=%t, Interpolate=%t",
			m.Width, m.Height, m.Inverted, m.Interpolate)
	}
	if _, err := m.LoadAlpha(); err != nil {
		t.Fatal(err)
	}
}

// TestExtractInlineMaskIgnoresBPC checks that an invalid BitsPerComponent
// entry of an image mask is ignored, as for an image mask XObject.
func TestExtractInlineMaskIgnoresBPC(t *testing.T) {
	dict := pdf.Dict{
		"W": pdf.Integer(8), "H": pdf.Integer(1), "IM": pdf.Boolean(true),
		"BPC": pdf.Integer(8),
	}
	got, err := ExtractInline(inlineOp(dict, []byte{0xF0}), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.(*Mask); !ok {
		t.Fatalf("got %T, want *Mask", got)
	}
}

// TestExtractInlineFilterFixesBPC checks that the sample size delivered by
// DCTDecode wins over a missing or contradicting BitsPerComponent entry.
func TestExtractInlineFilterFixesBPC(t *testing.T) {
	src := image.NewGray(image.Rect(0, 0, 8, 8))
	buf := &bytes.Buffer{}
	if err := jpeg.Encode(buf, src, nil); err != nil {
		t.Fatal(err)
	}

	for _, bpc := range []pdf.Object{nil, pdf.Integer(16), pdf.Integer(3)} {
		dict := pdf.Dict{
			"W": pdf.Integer(8), "H": pdf.Integer(8),
			"CS": pdf.Name("G"), "F": pdf.Name("DCT"),
		}
		if bpc != nil {
			dict["BPC"] = bpc
		}
		got, err := ExtractInline(inlineOp(dict, buf.Bytes()), nil)
		if err != nil {
			t.Fatalf("BPC %v: %v", bpc, err)
		}
		if img := got.(*Dict); img.BitsPerComponent != 8 {
			t.Errorf("BPC %v: got %d bits, want 8", bpc, img.BitsPerComponent)
		}
	}
}

func TestExtractInlineMalformed(t *testing.T) {
	res := &content.Resources{
		ColorSpace: map[pdf.Name]color.Space{"P": color.SpacePatternColored},
	}
	for _, tc := range []struct {
		name string
		dict pdf.Dict
	}{
		{"missing ColorSpace", pdf.Dict{
			"W": pdf.Integer(1), "H": pdf.Integer(1), "BPC": pdf.Integer(8)}},
		{"unknown colour space resource", pdf.Dict{
			"W": pdf.Integer(1), "H": pdf.Integer(1), "BPC": pdf.Integer(8),
			"CS": pdf.Name("Missing")}},
		{"pattern colour space", pdf.Dict{
			"W": pdf.Integer(1), "H": pdf.Integer(1), "BPC": pdf.Integer(8),
			"CS": pdf.Name("P")}},
		{"missing BitsPerComponent", pdf.Dict{
			"W": pdf.Integer(1), "H": pdf.Integer(1), "CS": pdf.Name("G")}},
		{"zero width", pdf.Dict{
			"W": pdf.Integer(0), "H": pdf.Integer(1), "BPC": pdf.Integer(8),
			"CS": pdf.Name("G")}},
		{"missing height", pdf.Dict{
			"W": pdf.Integer(1), "IM": pdf.Boolean(true)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ExtractInline(inlineOp(tc.dict, []byte{0}), res); err == nil {
				t.Error("expected an error")
			}
		})
	}

	op := content.Operator{Name: content.OpXObject, Args: []pdf.Object{pdf.Name("X")}}
	if _, err := ExtractInline(op, nil); err == nil {
		t.Error("expected an error for a Do operator")
	}
}

// TestExtractInlineEmbed checks that an inline image, once read, can be
// written as an image XObject and read back with the same contents.
func TestExtractInlineEmbed(t *testing.T) {
	sampled := inlineOp(pdf.Dict{
		"W": pdf.Integer(2), "H": pdf.Integer(2), "BPC": pdf.Integer(4),
		"CS": pdf.Name("G"), "D": pdf.Array{pdf.Integer(1), pdf.Integer(0)},
	}, []byte{0x1F, 0xA5})
	mask := inlineOp(pdf.Dict{
		"W": pdf.Integer(3), "H": pdf.Integer(2), "IM": pdf.Boolean(true),
	}, []byte{0xA0, 0x40})

	for _, op := range []content.Operator{sampled, mask} {
		img, err := ExtractInline(op, nil)
		if err != nil {
			t.Fatal(err)
		}
		inlineRoundTrip(t, img)
	}
}

// inlineRoundTrip writes an image read by [ExtractInline] as an image
// XObject, reads it back and checks that both reads decode to the same
// samples.
func inlineRoundTrip(t *testing.T, first graphics.Image) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)
	ref, err := rm.Embed(first)
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	c := pdf.CursorAt(pdf.NewExtractor(w), nil)

	switch first := first.(type) {
	case *Dict:
		second, err := ExtractDict(c, ref, false)
		if err != nil {
			t.Fatal(err)
		}
		a, err := first.Load()
		if err != nil {
			t.Fatal(err)
		}
		b, err := second.Load()
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(a.Pix, b.Pix); diff != "" {
			t.Errorf("pixels differ (-first +second):\n%s", diff)
		}
		if diff := cmp.Diff(first.Decode, second.Decode); diff != "" {
			t.Errorf("Decode differs (-first +second):\n%s", diff)
		}
	case *Mask:
		second, err := ExtractMask(c, ref, false)
		if err != nil {
			t.Fatal(err)
		}
		a, err := first.LoadAlpha()
		if err != nil {
			t.Fatal(err)
		}
		b, err := second.LoadAlpha()
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(a.Pix, b.Pix); diff != "" {
			t.Errorf("mask differs (-first +second):\n%s", diff)
		}
	default:
		t.Fatalf("unexpected type %T", first)
	}
}

func FuzzExtractInline(f *testing.F) {
	for _, s := range []string{
		"BI /W 2 /H 1 /BPC 8 /CS /RGB ID \xff\x00\x00\x00\x00\xff EI",
		"BI /W 2 /H 2 /BPC 4 /CS /G /D [1 0] /I true ID \x1f\xa5 EI",
		"BI /W 8 /H 1 /IM true /D [1 0] ID \xf0 EI",
		"BI /W 4 /H 4 /BPC 1 /CS [/I /RGB 1 <000000ffffff>] ID \x50\xa0\x50\xa0 EI",
		"BI /W 2 /H 2 /BPC 8 /CS /G /F /AHx ID 00ff7f80> EI",
		"BI /Width 3 /W 1 /H 1 /BPC 16 /CS /CMYK /Intent /Perceptual ID 01234567 EI",
	} {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		stream := content.NewScanner(func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(data)), nil
		})
		for name, args := range stream.NewIter().All() {
			if name != content.OpInlineImage {
				continue
			}
			img, err := ExtractInline(content.Operator{Name: name, Args: args}, nil)
			if err != nil {
				continue
			}
			inlineRoundTrip(t, img)
		}
	})
}
