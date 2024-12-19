// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package font_test

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/font/type3"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/fonttypes"
)

// TODO(voss): remove
type Dict interface {
	Embed(w *pdf.Writer, fontDictRef pdf.Reference) error
}

var (
	_ Dict = (*type1.FontDict)(nil)
	_ Dict = (*cff.FontDictSimple)(nil)
	_ Dict = (*cff.FontDictComposite)(nil)
	_ Dict = (*truetype.FontDictSimple)(nil)
	_ Dict = (*truetype.FontDictComposite)(nil)
	_ Dict = (*opentype.FontDictCFFSimple)(nil)
	_ Dict = (*opentype.FontDictCFFComposite)(nil)
	_ Dict = (*opentype.FontDictGlyfSimple)(nil)
	_ Dict = (*opentype.FontDictGlyfComposite)(nil)
	_ Dict = (*type3.EmbedInfo)(nil)
)

// TestSpaceIsBlank tests that space characters of common fonts are blank.
func TestSpaceIsBlank(t *testing.T) {
	for _, sample := range fonttypes.All {
		t.Run(sample.Label, func(t *testing.T) {
			data, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
			rm := pdf.NewResourceManager(data)

			F := sample.MakeFont(rm)
			gg := F.Layout(nil, 10, " ")
			if len(gg.Seq) != 1 {
				t.Fatalf("expected 1 glyph, got %d", len(gg.Seq))
			}
			geom := F.GetGeometry()
			if !geom.GlyphExtents[gg.Seq[0].GID].IsZero() {
				t.Errorf("expected blank glyph, got %v",
					geom.GlyphExtents[gg.Seq[0].GID])
			}
		})
	}
}
