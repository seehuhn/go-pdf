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

package cmap

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf/font/charcode"
)

func TestToUnicodeMapping(t *testing.T) {
	cmap := `/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CIDSystemInfo 3 dict dup begin
  /Registry (Test) def
  /Ordering (Test) def
  /Supplement 0 def
end def
/CMapName /Test def
/CMapType 2 def
1 begincodespacerange
<20> <ff>
endcodespacerange
1 beginbfrange
<20> <7e> <0020>
endbfrange
endcmap
CMapName currentdict /CMap defineresource pop
end
end`
	info, err := ReadToUnicode(strings.NewReader(cmap), nil)
	if err != nil {
		t.Fatal(err)
	}
	m := info.GetMapping()

	in := []byte{0x20}
	code, k := info.CS.Decode(in)
	if k != 1 {
		t.Fatalf("unexpected k=%d", k)
	}
	if !slices.Equal(m[code], []rune{0x20}) {
		t.Fatalf("unexpected mapping: %02x -> %q", in, m[code])
	}
}

func TestRoundtrip(t *testing.T) {
	info := &ToUnicodeOld{
		CS: charcode.Simple,
		Singles: []ToUnicodeSingleOld{
			{
				Code:  65,
				Value: []rune{'A'},
			},
			{
				Code:  100,
				Value: []rune("ffl"),
			},
		},
		Ranges: []ToUnicodeRangeOld{
			{
				First:  96,
				Last:   112,
				Values: [][]rune{[]rune("a")},
			},
			{
				First:  200,
				Last:   202,
				Values: [][]rune{[]rune("fl"), []rune("fi"), []rune("ff")},
			},
		},
	}

	buf := &bytes.Buffer{}
	err := info.Write(buf)
	if err != nil {
		t.Fatal(err)
	}

	// fmt.Println(buf.String())

	info2, err := ReadToUnicode(buf, nil)
	if err != nil {
		t.Fatal(err)
	}

	if d := cmp.Diff(info, info2); d != "" {
		t.Fatalf("unexpected diff (-want +got):\n%s", d)
	}
}

func FuzzToUnicode(f *testing.F) {
	f.Add(`/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CMapName /Test-Map def
/CMapType 2 def
1 begincodespacerange
<0000> <ffff>
endcodespacerange
1 beginbfchar
<1234> <5678>
endbfchar
endcmap
CMapName currentdict /CMap defineresource pop
end
end`)
	f.Add(`/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CMapName /Test-Map def
/CMapType 2 def
/CIDSystemInfo <<
/Registry (Adobe)
/Ordering (Identity)
/Supplement 0
>> def
1 begincodespacerange
<00> <ff>
endcodespacerange
2 beginbfchar
<41> <0041>
<64> <00660066006c>
endbfchar
2 beginbfrange
<60> <70> <0061>
<c8> <ca> [<0066006c> <00660069> <00660066>]
endbfrange
endcmap
CMapName currentdict /CMap defineresource pop
end
end`)
	f.Fuzz(func(t *testing.T, s string) {
		info1, err := ReadToUnicode(strings.NewReader(s), nil)
		if err != nil {
			return
		}

		buf := &bytes.Buffer{}
		err = info1.Write(buf)
		if err != nil {
			t.Fatal(err)
		}
		info2, err := ReadToUnicode(buf, nil)
		if err != nil {
			t.Fatal(err)
		}
		if d := cmp.Diff(info1, info2); d != "" {
			t.Fatalf("unexpected diff (-want +got):\n%s", d)
		}
	})
}
