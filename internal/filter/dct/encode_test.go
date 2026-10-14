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

package dct

import (
	"bytes"
	"io"
	"testing"

	"seehuhn.de/go/membudget"
)

func TestEncodeDecode(t *testing.T) {
	const w, h = 24, 20
	stripes := [4][4]byte{
		{10, 200, 30, 0}, {180, 20, 60, 40}, {90, 90, 200, 120}, {0, 0, 0, 230},
	}
	src := make([]byte, 0, w*h*4)
	for range h {
		for x := range w {
			src = append(src, stripes[4*x/w][:]...)
		}
	}

	var buf bytes.Buffer
	p := EncodeParams{Width: w, Height: h, Components: 4, Transform: true, Quality: 100}
	if err := Encode(&buf, bytes.NewReader(src), p); err != nil {
		t.Fatal(err)
	}

	zero := 0
	for _, c := range []struct {
		name string
		ct   *int
	}{{"nil", nil}, {"0", &zero}} {
		rc, err := Decode(bytes.NewReader(buf.Bytes()), c.ct, membudget.New(1<<30))
		if err != nil {
			t.Fatal(err)
		}
		got, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(src) {
			t.Fatalf("colorTransform %s: decoded %d bytes, want %d", c.name, len(got), len(src))
		}
		for i := range src {
			if d := int(got[i]) - int(src[i]); d < -3 || d > 3 {
				t.Fatalf("colorTransform %s: byte %d is %d, want %d", c.name, i, got[i], src[i])
			}
		}
	}
}
