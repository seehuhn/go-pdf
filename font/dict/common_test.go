// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package dict

import (
	"testing"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/internal/stdmtx"
)

func TestWidthsAreCompatibleBuiltin(t *testing.T) {
	builtin := func(byte) string { return encoding.UseBuiltin }
	for _, name := range []string{"Courier", "Helvetica", "Symbol", "ZapfDingbats"} {
		info := stdmtx.Metrics[name]

		// widths as a reader fills them in for a font without /Encoding
		var ww [256]float64
		for code := range 256 {
			ww[code] = info.Width[info.Encoding[code]]
		}
		if !widthsAreCompatible(ww[:], builtin, info) {
			t.Errorf("%s: built-in widths not compatible", name)
		}

		ww['A'] += 10
		if widthsAreCompatible(ww[:], builtin, info) {
			t.Errorf("%s: modified widths compatible", name)
		}
	}
}

func TestFontDescriptorIsCompatible(t *testing.T) {
	// descriptor returns a descriptor matching the metrics of a standard font
	descriptor := func(name string) *font.Descriptor {
		m := stdmtx.Metrics[name]
		return &font.Descriptor{
			FontName:     name,
			FontFamily:   m.FontFamily,
			FontWeight:   m.FontWeight,
			IsFixedPitch: m.IsFixedPitch,
			IsSerif:      m.IsSerif,
			IsSymbolic:   m.IsSymbolic,
			IsItalic:     m.ItalicAngle != 0,
			ItalicAngle:  m.ItalicAngle,
			Ascent:       m.Ascent,
			Descent:      m.Descent,
			CapHeight:    m.CapHeight,
			XHeight:      m.XHeight,
			StemV:        m.StemV,
			StemH:        m.StemH,
		}
	}

	cases := []struct {
		name   string
		font   string
		modify func(fd *font.Descriptor)
		want   bool
	}{
		{"oblique", "Helvetica-Oblique", func(fd *font.Descriptor) {}, true},
		{"upright", "Helvetica", func(fd *font.Descriptor) {}, true},
		{"oblique without italic flag", "Helvetica-Oblique", func(fd *font.Descriptor) {
			fd.IsItalic = false
		}, false},
		{"upright with italic flag", "Helvetica", func(fd *font.Descriptor) {
			fd.IsItalic = true
		}, false},
		{"wrong italic angle", "Helvetica-Oblique", func(fd *font.Descriptor) {
			fd.ItalicAngle = -20
		}, false},
		{"upright with italic angle", "Helvetica", func(fd *font.Descriptor) {
			fd.ItalicAngle = -12
		}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stdInfo := stdmtx.Metrics[c.font]
			if c.font == "Helvetica-Oblique" && stdInfo.ItalicAngle != -12 {
				t.Fatalf("unexpected italic angle %g", stdInfo.ItalicAngle)
			}
			fd := descriptor(c.font)
			c.modify(fd)
			if got := fontDescriptorIsCompatible(fd, stdInfo); got != c.want {
				t.Errorf("got %t, want %t", got, c.want)
			}
		})
	}
}
