// seehuhn.de/go/pdf - a library for reading and writing PDF files
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

package parser

import (
	"bytes"
	"testing"
)

func TestPos(t *testing.T) {
	buf := bytes.NewReader([]byte{'0', '1', '2', '3', '4', '5', '6', '7'})
	p := New("test", buf)

	pos := p.Pos()
	if pos != 0 {
		t.Errorf("wrong position, expected 0 but got %d", pos)
	}

	_, err := p.ReadUInt16()
	if err != nil {
		t.Fatal(err)
	}

	pos = p.Pos()
	if pos != 2 {
		t.Errorf("wrong position, expected 2 but got %d", pos)
	}

	err = p.SeekPos(5)
	if err != nil {
		t.Fatal(err)
	}
	if p.Pos() != 5 {
		t.Errorf("wrong position, expected 5 but got %d", p.Pos())
	}
}
