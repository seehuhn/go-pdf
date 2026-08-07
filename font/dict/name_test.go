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

package dict

import (
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestValidateFontName(t *testing.T) {
	for _, tc := range []struct {
		label    string
		psName   string
		tag      string
		fontName string
		ok       bool
	}{
		{"plain name", "Go-Regular", "", "Go-Regular", true},
		{"subset", "Go-Regular", "AAAAAA", "AAAAAA+Go-Regular", true},
		{"non-ASCII name", "宋体", "", "宋体", true},
		{"no name", "", "", "", false},
		{"invalid tag", "Go-Regular", "nope", "nope+Go-Regular", false},
		{"descriptor disagrees", "Go-Regular", "", "Go-Italic", false},

		// a name a font program cannot carry: the writer says so rather than
		// storing a name it would read back as something else
		{"not valid UTF-8", "Gr\xfc\xdfe", "", "Gr\xfc\xdfe", false},
		{"white space", "Go Regular", "", "Go Regular", false},
		{"delimiter", "Go(Regular)", "", "Go(Regular)", false},
		{
			"too long once tagged",
			strings.Repeat("x", 125), "AAAAAA", "AAAAAA+" + strings.Repeat("x", 125),
			false,
		},
	} {
		t.Run(tc.label, func(t *testing.T) {
			desc := &font.Descriptor{FontName: tc.fontName}

			err := validateFontName(desc, tc.psName, tc.tag)

			if tc.ok && err != nil {
				t.Errorf("the name %q was rejected: %v", tc.fontName, err)
			}
			if !tc.ok && err == nil {
				t.Errorf("the name %q was accepted", tc.fontName)
			}
		})
	}
}

// Every font dictionary describes the font it names, so a dictionary with no
// descriptor is reported rather than dereferenced.
func TestValidateNoDescriptor(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)

	for _, tc := range []struct {
		label    string
		validate func() error
	}{
		{"Type1", func() error { return (&Type1{}).validate(w) }},
		{"TrueType", func() error { return (&TrueType{}).validate(w) }},
		{"CIDFontType0", func() error { return (&CIDFontType0{}).validate() }},
		{"CIDFontType2", func() error { return (&CIDFontType2{}).validate() }},
	} {
		t.Run(tc.label, func(t *testing.T) {
			if err := tc.validate(); err == nil {
				t.Error("a dictionary with no font descriptor was accepted")
			}
		})
	}
}
