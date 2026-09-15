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

package cmap

import (
	"testing"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
)

func TestUTF8Templates(t *testing.T) {
	for _, f := range []*File{UTF8H, UTF8V} {
		if !f.CodeSpaceRange.Equivalent(charcode.UTF8) {
			t.Errorf("%s: code space is not UTF-8", f.Name)
		}
		if len(f.CIDSingles)+len(f.CIDRanges) != 0 || f.Parent != nil {
			t.Errorf("%s: template has mappings", f.Name)
		}
		if f.IsPredefined() {
			t.Errorf("%s: template counts as predefined", f.Name)
		}
	}
	if UTF8H.WMode != font.Horizontal || UTF8V.WMode != font.Vertical {
		t.Error("template writing modes are wrong")
	}
}
