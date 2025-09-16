// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package opentype_test

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/pdf/internal/debug/makefont"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestEmbedSimple(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		t.Run(v.String(), func(t *testing.T) {
			// step 1: embed a font instance into a simple PDF file
			w, _ := memfile.NewPDFWriter(v, nil)
			rm := pdf.NewResourceManager(w)

			fontData := makefont.OpenType()
			fontInstance, err := opentype.New(fontData, nil)
			if err != nil {
				t.Fatal(err)
			}

			ref, e, err := pdf.ResourceManagerEmbed(rm, fontInstance)
			if err != nil {
				t.Fatal(err)
			}
			e2, ok := e.(font.EmbeddedLayouter)
			if !ok {
				t.Fatal("does not implement font.EmbeddedLayouter")
			}

			// make sure a few glyphs are included and encoded
			gg := fontInstance.Layout(nil, 12, "Hello")
			for _, g := range gg.Seq {
				e2.AppendEncoded(nil, g.GID, string(g.Text))
			}

			err = rm.Close()
			if err != nil {
				t.Fatal(err)
			}

			// step 2: read back the font and verify that everything is as expected
			x := pdf.NewExtractor(w)
			dict, err := dict.DecodeType1(x, ref)
			if err != nil {
				t.Fatal(err)
			}

			if dict.PostScriptName != fontData.PostScriptName() {
				t.Errorf("wrong PostScript name: expected %v, got %v",
					fontData.PostScriptName(), dict.PostScriptName)
			}
			if len(dict.SubsetTag) != 6 {
				t.Errorf("wrong subset tag: %q", dict.SubsetTag)
			}

			// TODO(voss): more tests
		})
	}
}
