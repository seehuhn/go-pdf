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

package sfntglyphs

import (
	"testing"

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/funit"

	"seehuhn.de/go/sfnt"
	sfntcmap "seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/glyf"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/pdfenc"
)

func TestNewTrueTypeSelector(t *testing.T) {
	// WinAnsi maps code 130 to "quotesinglbase", which has:
	//   Mac Roman code = 226
	//   Unicode = U+201A = 8218
	// These values diverge, so each method uses a different lookup path.
	const code byte = 130
	const cidVal = cid.CID(code) + 1

	enc := encoding.Simple(func(c byte) string {
		return pdfenc.WinAnsi.Encoding[c]
	})

	tests := []struct {
		name     string
		symbolic bool
		enc      encoding.Simple
		font     *sfnt.Font
		cid      cid.CID
		wantGID  glyph.ID
		wantOK   bool
	}{
		// boundary cases
		{
			name:    "CID 0",
			font:    ttFont(10, nil, nil),
			cid:     0,
			wantGID: 0,
			wantOK:  true,
		},
		{
			name:    "CID out of range",
			font:    ttFont(10, nil, nil),
			cid:     257,
			wantGID: 0,
			wantOK:  false,
		},
		{
			name:     "no cmap tables",
			symbolic: true,
			font:     ttFont(10, nil, nil),
			cid:      cidVal,
			wantGID:  0,
			wantOK:   false,
		},

		// each method in isolation
		{
			name:     "method A: (1,0) direct",
			symbolic: true,
			font: ttFont(10, sfntcmap.Table{
				{PlatformID: 1}: encFormat0(code, 1),
			}, nil),
			cid:     cidVal,
			wantGID: 1,
			wantOK:  true,
		},
		{
			name:     "method B: (3,0) with base offsets",
			symbolic: true,
			font: ttFont(10, sfntcmap.Table{
				{PlatformID: 3}: encFormat4(uint16(code), 2),
			}, nil),
			cid:     cidVal,
			wantGID: 2,
			wantOK:  true,
		},
		{
			name: "method C: encoding, Mac Roman, (1,0)",
			enc:  enc,
			font: ttFont(10, sfntcmap.Table{
				{PlatformID: 1}: encFormat0(226, 3), // Mac Roman code for "quotesinglbase"
			}, nil),
			cid:     cidVal,
			wantGID: 3,
			wantOK:  true,
		},
		{
			name:     "method D: encoding, AGL, (3,1)",
			symbolic: true,
			enc:      enc,
			font: ttFont(10, sfntcmap.Table{
				{PlatformID: 3, EncodingID: 1}: encFormat4(8218, 4), // U+201A
			}, nil),
			cid:     cidVal,
			wantGID: 4,
			wantOK:  true,
		},
		{
			name:     "method E: encoding, post table",
			symbolic: true,
			enc:      enc,
			font:     ttFont(10, nil, makePostNames(5, "quotesinglbase")),
			cid:      cidVal,
			wantGID:  5,
			wantOK:   true,
		},

		// priority: higher-priority method wins
		{
			name:     "D wins over B (symbolic, encoding)",
			symbolic: true,
			enc:      enc,
			font: ttFont(10, sfntcmap.Table{
				{PlatformID: 3}:                encFormat4(uint16(code), 2),
				{PlatformID: 3, EncodingID: 1}: encFormat4(8218, 4),
			}, nil),
			cid:     cidVal,
			wantGID: 4,
			wantOK:  true,
		},
		{
			name: "C wins over B (non-symbolic, encoding)",
			enc:  enc,
			font: ttFont(10, sfntcmap.Table{
				{PlatformID: 1}: encFormat0(226, 3),
				{PlatformID: 3}: encFormat4(uint16(code), 2),
			}, nil),
			cid:     cidVal,
			wantGID: 3,
			wantOK:  true,
		},
		{
			name:     "B wins over A (no encoding)",
			symbolic: true,
			font: ttFont(10, sfntcmap.Table{
				{PlatformID: 1}: encFormat0(code, 1),
				{PlatformID: 3}: encFormat4(uint16(code), 2),
			}, nil),
			cid:     cidVal,
			wantGID: 2,
			wantOK:  true,
		},

		// GID bounds
		{
			name:     "GID out of bounds clamped to 0",
			symbolic: true,
			font: ttFont(5, sfntcmap.Table{
				{PlatformID: 1}: encFormat0(code, 9), // GID 9 >= numGlyphs 5
			}, nil),
			cid:     cidVal,
			wantGID: 0,
			wantOK:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sel := NewTrueTypeSelector(tc.font, tc.symbolic, tc.enc)
			gid, ok := sel(tc.cid)
			if gid != tc.wantGID || ok != tc.wantOK {
				t.Errorf("got (%d, %v), want (%d, %v)",
					gid, ok, tc.wantGID, tc.wantOK)
			}
		})
	}
}

// ttFont creates a minimal TrueType font for testing.
func ttFont(numGlyphs int, cmaps sfntcmap.Table, names []string) *sfnt.Font {
	return &sfnt.Font{
		FamilyName: "Test",
		UnitsPerEm: 1000,
		Outlines: &glyf.Outlines{
			Glyphs: make(glyf.Glyphs, numGlyphs),
			Widths: make([]funit.Int16, numGlyphs),
			Names:  names,
		},
		CMapTable: cmaps,
	}
}

func encFormat0(code byte, gid byte) []byte {
	f := &sfntcmap.Format0{}
	f.Data[code] = gid
	return f.Encode(0)
}

func encFormat4(code uint16, gid glyph.ID) []byte {
	return sfntcmap.Format4{code: gid}.Encode(0)
}

func makePostNames(gid int, name string) []string {
	names := make([]string, gid+1)
	names[0] = ".notdef"
	names[gid] = name
	return names
}
