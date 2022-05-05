// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package builder

import (
	"testing"
)

func TestLexBackup(t *testing.T) {
	l := &lexer{
		input: "abc",
	}

	var out []rune
	for {
		r := l.next()
		l.backup()
		r2 := l.next()
		if r != r2 {
			t.Errorf("%q != %q", r, r2)
		}
		if r == 0 {
			break
		}
		out = append(out, r)
	}
	if string(out) != l.input {
		t.Errorf("%q != %q", out, l.input)
	}
}
