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

// +build gofuzz

package pdf

import (
	"bytes"
	"errors"
	"fmt"
)

func fuzzGetInt(obj Object) (Integer, error) {
	switch x := obj.(type) {
	case Integer:
		return x, nil
	case *Reference:
		// Allow the fuzzer to generate different indirect integer values,
		// both positive and negative.
		return Integer(x.Number) - Integer(x.Generation), nil
	default:
		return 0, errors.New("not an integer")
	}
}

// Fuzz is the entrance point for github.com/dvyukov/go-fuzz
func Fuzz(data []byte) int {
	r := bytes.NewReader(data)
	s := newScanner(r, 0, fuzzGetInt, nil)
	obj, err := s.ReadObject()
	if err != nil {
		return 0
	}

	buf := &bytes.Buffer{}
	if obj == nil {
		buf.WriteString("null")
	} else {
		err = obj.PDF(buf)
	}
	if err != nil {
		fmt.Println(err)
		panic("buf1 write failed")
	}
	out1 := string(buf.Bytes())

	s = newScanner(buf, 0, fuzzGetInt, nil)
	obj, err = s.ReadObject()
	if err != nil {
		fmt.Printf("%q\n", out1)
		fmt.Println(err)
		panic("buf1 read failed")
	}

	buf.Reset()
	if obj == nil {
		buf.WriteString("null")
	} else {
		err = obj.PDF(buf)
	}
	if err != nil {
		fmt.Println(err)
		panic("buf2 write failed")
	}
	out2 := string(buf.Bytes())

	if out1 != out2 {
		fmt.Println(out1)
		fmt.Println(out2)
		panic("results differ")
	}

	return 1
}
