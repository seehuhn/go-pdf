// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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
	"fmt"
	"os"
	"testing"

	"seehuhn.de/go/pdf/font/charcode"
)

func TestNewFile(t *testing.T) {
	ros := charcode.CodeSpaceRange{
		{
			Low:  []byte{0x00},
			High: []byte{0xff},
		},
	}
	codec, err := charcode.NewCodec(ros)
	if err != nil {
		t.Fatal(err)
	}

	code := func(s string) charcode.Code {
		x := []byte(s)
		code, k, ok := codec.Decode(x)
		if !ok || k != len(x) {
			panic(fmt.Sprintf("invalid code: % x", x))
		}
		return code
	}

	data := map[charcode.Code]Code{
		code("A"): testCode{2},
		code("B"): testCode{3},
		code("C"): testCode{4},
		code("D"): testCode{5},
		code("E"): testCode{6},

		code("a"): testCode{2},
		code("b"): testCode{3},
		code("c"): testCode{4},
		code("d"): testCode{5},
		code("e"): testCode{6},

		code("y"): testCode{2},
	}

	res := NewFile(codec, data)
	res.WriteTo(os.Stdout, true)
}

type testCode struct {
	cid CID
}

func (c testCode) CID() CID       { return c.cid }
func (c testCode) NotdefCID() CID { return 0 }
func (c testCode) Width() float64 { return 1000 }
func (c testCode) Text() string   { return "" }
