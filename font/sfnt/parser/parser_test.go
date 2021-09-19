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
	"errors"
	"io"
	"strings"
	"testing"
)

func TestParser(t *testing.T) {
	buf := strings.NewReader("1234AB\xFF\xFF")
	p := New(buf)

	err := p.SetRegion("test", 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	x, err := p.ReadUInt16()
	if err != nil {
		t.Fatal(err)
	}
	if x != '1'*256+'2' {
		t.Errorf("wrong value, expected %d but got %d", '1'*256+'2', x)
	}
	_, err = p.ReadUInt16()
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("EOF not detected, got err=%s", err)
	}

	err = p.SetRegion("xyz", 4, 4)
	if err != nil {
		t.Fatal(err)
	}
	err = p.seek(6)
	if err == nil {
		t.Error("seek error not detected")
	} else if !strings.Contains(err.Error(), "xyz") {
		t.Error("table name not mentioned in error message", err)
	}
	err = p.seek(2)
	if err != nil {
		t.Fatal(err)
	}
	y, err := p.ReadInt16()
	if err != nil {
		t.Fatal(err)
	}
	if y != -1 {
		t.Errorf("wrong value, expected -1 but got %d", y)
	}
}
