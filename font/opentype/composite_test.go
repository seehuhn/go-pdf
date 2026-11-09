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

package opentype_test

import (
	"testing"

	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/pdf/internal/debug/makefont"
)

func TestCompositeRequiresCMap(t *testing.T) {
	for name, info := range map[string]func() *sfnt.Font{
		"cff":  makefont.OpenType,
		"glyf": makefont.TrueType,
	} {
		if _, err := opentype.NewComposite(info(), nil); err == nil {
			t.Errorf("%s: nil options accepted", name)
		}
		if _, err := opentype.NewComposite(info(), &opentype.OptionsComposite{}); err == nil {
			t.Errorf("%s: nil CMap accepted", name)
		}
	}
}
