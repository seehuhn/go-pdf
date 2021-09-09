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

package font

import (
	"os"
	"testing"
)

func TestFormatString(t *testing.T) {
	s, err := formatPDFString("abc", []byte{'d', 'e'}, 'f', 2)
	if err != nil {
		t.Error(err)
	} else if s != "(abcdef2)" {
		t.Errorf("wrong result %q", s)
	}

	s, err = formatPDFString("x", 1.2)
	if err == nil {
		t.Error("missing error")
	} else if s != "" {
		t.Error("wrong string with error")
	}
}

func TestFormatName(t *testing.T) {
	name, err := formatPDFName("abc")
	if err != nil {
		t.Error(err)
	} else if name != "/abc" {
		t.Errorf("wrong result %q", name)
	}
}

func TestToUnicodeSimple(t *testing.T) {
	info := toUnicodeSimple("AAAAAA")
	err := toUnicodeTmpl.Execute(os.Stdout, info)
	if err != nil {
		t.Error(err)
	}
	// t.Error("fish")
}
