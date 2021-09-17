// seehuhn.de/go/pdf - support for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package opentype

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt"
)

// Embed embeds an OpenType font into a pdf file.
func Embed(w *pdf.Writer, name string, fname string, subset map[rune]bool) (*font.Font, error) {
	tt, err := sfnt.Open(fname, nil)
	if err != nil {
		return nil, err
	}
	defer tt.Close()

	if !tt.IsOpenType() {
		return nil, errors.New("not a OpenType font")
	}

	// step 1: write a copy of the font file into the font stream.
	dict := pdf.Dict{
		"Subtype": pdf.Name("OpenType"),
	}
	stm, FontFile, err := w.OpenStream(dict, nil,
		&pdf.FilterInfo{Name: "FlateDecode"})
	if err != nil {
		return nil, err
	}
	isCFF := tt.HasTables("CFF ")
	exOpt := &sfnt.ExportOptions{}
	// The list of tables to include is from PDF 32000-1:2008, table 126.
	// TODO(voss): include "gasp"?
	if isCFF {
		exOpt.Include = map[string]bool{
			"CFF ": true,
			"cmap": true,
		}
	} else {
		exOpt.Include = map[string]bool{
			"glyf": true,
			"head": true,
			"hhea": true,
			"hmtx": true,
			"loca": true,
			"maxp": true,
			"cvt ": true,
			"fpgm": true,
			"prep": true,
			"gasp": true,
		}
	}

	// case "glyf", "head", "hhea", "hmtx", "loca", "maxp", "cvt ", "fpgm", "prep":
	// 	// TODO(voss): include "gasp"?

	_, err = tt.Export(stm, exOpt)
	if err != nil {
		return nil, err
	}
	err = stm.Close()
	if err != nil {
		return nil, err
	}

	// factor for converting from TrueType FUnit to PDF glyph units
	q := 1000 / float64(tt.GlyphUnits) // TODO(voss): fix this

	_ = FontFile
	_ = q
	panic("not implemented")
}
