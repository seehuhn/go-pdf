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

package pdf

import (
	"bytes"
	"strings"
	"testing"
)

func TestFindXref(t *testing.T) {
	in := "%PDF-1.7\nhello\nstartxref\n9\n%%EOF"
	r := &Reader{
		size: int64(len(in)),
		r:    strings.NewReader(in),
	}
	start, err := r.findXRef()
	if err != nil {
		t.Error(err)
	}
	if start != 9 {
		t.Errorf("wrong xref start, expected 9 but got %d", start)
	}
}

func TestLastOccurence(t *testing.T) {
	buf := make([]byte, 2048)
	pat := "ABC"
	copy(buf[1023:], pat)

	r := &Reader{
		size: int64(len(buf)),
		r:    bytes.NewReader(buf),
	}
	pos, err := r.lastOccurence(pat)
	if err != nil {
		t.Fatal(err)
	}
	if pos != 1023 {
		t.Errorf("found wrong position: expected 1023, got %d", pos)
	}
}
