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
	"testing"

	"seehuhn.de/go/postscript/type1"
)

func TestStandardFonts(t *testing.T) {
	loader := New()

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
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			tp, r, err := loader.Open(name)
			if err != nil {
				t.Fatalf("error loading font %q: %v", name, err)
			}
			if tp != FontTypeType1 {
				t.Errorf("expected font type %v, got %v", FontTypeType1, tp)
			}

			_, err = type1.Read(r)
			if err != nil {
				t.Errorf("error reading font %q: %v", name, err)
			}

			err = r.Close()
			if err != nil {
				t.Errorf("error closing font %q: %v", name, err)
			}
		})
	}
}
