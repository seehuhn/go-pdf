// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package tounicodeold

import (
	"bytes"
	"testing"
	"unicode/utf16"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/postscript/type1"
)

func TestWrite(t *testing.T) {
	info := &Info{
		CodeSpace: []CodeSpaceRange{
			{First: 0, Last: 0xff},
		},
		Singles: []Single{
			{Code: 32, UTF16: utf16.Encode([]rune("lot's of space"))},
			{Code: 33, UTF16: nil},
		},
		Ranges: []Range{
			{
				First: 65,
				Last:  90,
				UTF16: [][]uint16{utf16.Encode([]rune("A"))},
			},
			{
				First: 100,
				Last:  102,
				UTF16: [][]uint16{utf16.Encode([]rune("fi")), utf16.Encode([]rune("fl")), utf16.Encode([]rune("ffl"))},
			},
		},
		Name: "Jochen-Chaotic-UCS2",
		ROS: &type1.CIDSystemInfo{
			Registry:   "Jochen",
			Ordering:   "Chaotic",
			Supplement: 12,
		},
	}

	buf := &bytes.Buffer{}
	err := info.Write(buf)
	if err != nil {
		t.Fatal(err)
	}

	info2, err := Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if d := cmp.Diff(info, info2); d != "" {
		t.Fatal(d)
	}
}
