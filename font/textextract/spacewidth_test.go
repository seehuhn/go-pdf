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

package textextract

import (
	"iter"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/dict"
)

// mockDict implements dict.Dict with a fixed set of characters.
type mockDict struct {
	chars []font.Code
}

func (d *mockDict) Embed(*pdf.EmbedHelper) (pdf.Native, error) { return nil, nil }
func (d *mockDict) MakeFont() font.Instance                    { return nil }
func (d *mockDict) FontInfo() any                              { return nil }
func (d *mockDict) Codec() *charcode.Codec                     { return nil }
func (d *mockDict) GlyphWidth(text string) (float64, bool) {
	for _, c := range d.chars {
		if c.Text == text {
			return c.Width, true
		}
	}
	return 0, false
}

// mockFontWithDict implements font.Instance and has a GetDict method.
type mockFontWithDict struct {
	mockFontInstance
	dict dict.Dict
}

func (f *mockFontWithDict) GetDict() dict.Dict { return f.dict }

// mockFontInstance is a minimal font.Instance implementation.
type mockFontInstance struct{}

func (f *mockFontInstance) PostScriptName() string               { return "Test" }
func (f *mockFontInstance) WritingMode() font.WritingMode        { return font.Horizontal }
func (f *mockFontInstance) Codec() *charcode.Codec               { return nil }
func (f *mockFontInstance) Codes(pdf.String) iter.Seq[font.Code] { return nil }
func (f *mockFontInstance) FontInfo() any                        { return nil }
func (f *mockFontInstance) ResourceName() pdf.Name               { return "" }
func (f *mockFontInstance) Embed(*pdf.EmbedHelper) (pdf.Native, error) {
	return nil, nil
}

func TestSpaceWidthDefault(t *testing.T) {
	// a font without GetDict returns the default
	f := &mockFontInstance{}
	got := SpaceWidth(f)
	if got < 200 || got > 1000 {
		t.Errorf("SpaceWidth out of range: %g", got)
	}
}

func TestSpaceWidthNilDict(t *testing.T) {
	// a font with GetDict returning nil returns the default
	f := &mockFontWithDict{dict: nil}
	got := SpaceWidth(f)
	if got < 200 || got > 1000 {
		t.Errorf("SpaceWidth out of range: %g", got)
	}
}

func TestSpaceWidthRange(t *testing.T) {
	// result is always in [200, 1000]
	tests := []struct {
		name  string
		chars []font.Code
	}{
		{
			name:  "no characters",
			chars: nil,
		},
		{
			name: "narrow space",
			chars: []font.Code{
				{Text: " ", Width: 0.100},
			},
		},
		{
			name: "wide space",
			chars: []font.Code{
				{Text: " ", Width: 0.900},
			},
		},
		{
			name: "typical Latin",
			chars: []font.Code{
				{Text: " ", Width: 0.250},
				{Text: "a", Width: 0.500},
				{Text: "e", Width: 0.450},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := &mockDict{chars: tc.chars}
			f := &mockFontWithDict{dict: d}
			got := SpaceWidth(f)
			if got < 200 || got > 1000 {
				t.Errorf("SpaceWidth out of range [200, 1000]: %g", got)
			}
		})
	}
}

func TestSpaceWidthUsesCharacterWidths(t *testing.T) {
	// a font with a wider space should produce a larger SpaceWidth
	narrowDict := &mockDict{chars: []font.Code{
		{Text: " ", Width: 0.200},
	}}
	wideDict := &mockDict{chars: []font.Code{
		{Text: " ", Width: 0.500},
	}}

	narrow := SpaceWidth(&mockFontWithDict{dict: narrowDict})
	wide := SpaceWidth(&mockFontWithDict{dict: wideDict})

	if narrow >= wide {
		t.Errorf("wider space should produce larger SpaceWidth: narrow=%g, wide=%g", narrow, wide)
	}
}
