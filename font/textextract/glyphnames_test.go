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
	"testing"

	"seehuhn.de/go/pdf/font/dict"
)

func TestGlyphNameMappingUnsupportedFont(t *testing.T) {
	// fonts that don't return FontInfoGlyfEmbedded should give nil
	f := &mockFontInstance{}
	got := GlyphNameMapping(f)
	if got != nil {
		t.Errorf("expected nil for unsupported font, got %v", got)
	}
}

func TestGlyphNameMappingNilFontFile(t *testing.T) {
	// FontInfoGlyfEmbedded with nil FontFile should give nil
	f := &mockFontInfoGlyfEmbedded{
		fontInfo: &dict.FontInfoGlyfEmbedded{
			FontFile: nil,
		},
	}
	got := GlyphNameMapping(f)
	if got != nil {
		t.Errorf("expected nil for nil FontFile, got %v", got)
	}
}

// mockFontInfoGlyfEmbedded wraps mockFontInstance but returns a
// FontInfoGlyfEmbedded from FontInfo.
type mockFontInfoGlyfEmbedded struct {
	mockFontInstance
	fontInfo *dict.FontInfoGlyfEmbedded
}

func (f *mockFontInfoGlyfEmbedded) FontInfo() any {
	return f.fontInfo
}
