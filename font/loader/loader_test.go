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

package loader

import (
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/postscript/afm"
	"seehuhn.de/go/postscript/type1"
)

// TestBuiltin verifies that the builtin fonts are available.
// This test can catch typos in the builtin font map file.
func TestBuiltin(t *testing.T) {
	loader := NewFontLoader()

	for key, val := range loader.lookup {
		if !val.isBuiltin {
			t.Errorf("%s/%d: non-builtin font in builtin map", key.psname, key.fontType)
		}

		// Make sure the font can be loaded.
		r, err := loader.Open(key.psname, key.fontType)
		if err != nil {
			t.Errorf("%s/%d: error loading builtin font: %v", key.psname, key.fontType, err)
			continue
		}
		_, err = io.Copy(io.Discard, r)
		if err != nil {
			t.Errorf("%s/%d: error reading builtin font: %v", key.psname, key.fontType, err)
		}
		err = r.Close()
		if err != nil {
			t.Errorf("%s/%d: error closing builtin font: %v", key.psname, key.fontType, err)
		}
	}
}

// TestStandardFonts checks that the 14 standard fonts are available.
func TestStandardFonts(t *testing.T) {
	loader := NewFontLoader()

	names := []string{
		"Courier",
		"Courier-Bold",
		"Courier-BoldOblique",
		"Courier-Oblique",
		"Helvetica",
		"Helvetica-Bold",
		"Helvetica-BoldOblique",
		"Helvetica-Oblique",
		"Times-Roman",
		"Times-Bold",
		"Times-BoldItalic",
		"Times-Italic",
		"Symbol",
		"ZapfDingbats",
	}

	// Type 1 font files
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			// Make sure the type file is found ...
			r, err := loader.Open(name, FontTypeType1)
			if err != nil {
				t.Fatalf("error loading font %q: %v", name, err)
			}

			// ... and can be read.
			font, err := type1.Read(r)
			if err != nil {
				t.Errorf("error reading font %q: %v", name, err)
			}

			err = r.Close()
			if err != nil {
				t.Errorf("error closing font %q: %v", name, err)
			}

			// Make sure the ADM file is found ...
			r, err = loader.Open(name, FontTypeAFM)
			if err != nil {
				t.Fatalf("error loading afm file %q: %v", name, err)
			}

			// ... and can be read.
			afm, err := afm.Read(r)
			if err != nil {
				t.Errorf("error reading afm file %q: %v", name, err)
			}

			err = r.Close()
			if err != nil {
				t.Errorf("error closing afm file %q: %v", name, err)
			}

			// Check that the font and the afm file match.
			if d := cmp.Diff(font.Encoding, afm.Encoding); d != "" {
				t.Errorf("font %q: encoding mismatch:\n%s", name, d)
			}
		})
	}
}
