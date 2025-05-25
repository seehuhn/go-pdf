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

package ccittfax

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/image/ccitt"
)

func TestRoundTripTriangle(t *testing.T) {
	// Image: 8x8, black triangle, black on bottom-left, white on top-right.
	// Each byte represents one scanline of 8 pixels.
	// For Params.BlackIs1 == false we get:
	// Line 0 (top):   BWWWWWWW (0b01111111 = 0x7F)
	// Line 1:         BBWWWWWW (0b00111111 = 0x3F)
	// Line 2:         BBBWWWWW (0b00011111 = 0x1F)
	// Line 3:         BBBBWWWW (0b00001111 = 0x0F)
	// Line 4:         BBBBBWWW (0b00000111 = 0x07)
	// Line 5:         BBBBBBWW (0b00000011 = 0x03)
	// Line 6:         BBBBBBBW (0b00000001 = 0x01)
	// Line 7 (bottom):BBBBBBBB (0b00000000 = 0x00)
	originalData := []byte{
		0x7F, 0x3F, 0x1F, 0x0F, 0x07, 0x03, 0x01, 0x00,
	}

	for _, blackIs1 := range []bool{false, true} {
		t.Run(fmt.Sprintf("%t", blackIs1), func(t *testing.T) {
			param := &Params{
				BlackIs1: blackIs1,
				Columns:  8,
				MaxRows:  8,
				K:        0,
			}

			buf := &bytes.Buffer{}
			writer := NewWriter(buf, param)
			n, err := writer.Write(originalData)
			if err != nil {
				t.Fatal(err)
			}
			if n != len(originalData) {
				t.Fatalf("wrote %d bytes, expected %d", n, len(originalData))
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}

			reader := NewReader(bytes.NewReader(buf.Bytes()), param)
			decodedData, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(originalData, decodedData); diff != "" {
				t.Errorf("decoded data differs from original: %s", diff)
			}
		})
	}
}

func TestCompatibility(t *testing.T) {
	width := 62
	height := 62
	image := make([]byte, height*width/8)

	// draw a circle
	for y := range height {
		for x := range width {
			if (x-28)*(x-28)+(y-30)*(y-30) <= 29*29 {
				byteIndex := y*((width+7)/8) + (x / 8)
				bitPosition := 7 - (x % 8)
				image[byteIndex] |= 1 << bitPosition
			}
		}
	}

	param := &Params{
		Columns:   width,
		K:         0,
		EndOfLine: true,
		BlackIs1:  false,
	}

	// Encode with our encoder
	buf := &bytes.Buffer{}
	writer := NewWriter(buf, param)
	_, err := writer.Write(image)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	encoded := buf.Bytes()

	// Decode with golang.org/x/image/ccitt
	subformat := ccitt.Group3
	if param.K < 0 {
		subformat = ccitt.Group4
	}
	h := ccitt.AutoDetectHeight
	if param.MaxRows > 0 {
		h = param.MaxRows
	}
	r := ccitt.NewReader(bytes.NewReader(encoded), ccitt.MSB, subformat, param.Columns, h, &ccitt.Options{})

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(image, out); diff != "" {
		t.Errorf("standard library decoder produced different output: %s", diff)
	}
}
