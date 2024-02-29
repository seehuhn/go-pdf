// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package color

import (
	"errors"
	"fmt"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
)

// ShadingType4 represents a type 4 (free-form Gouraud-shaded triangle mesh) shading.
type ShadingType4 struct {
	ColorSpace        Space
	BitsPerCoordinate int
	BitsPerComponent  int
	BitsPerFlag       int
	Decode            []float64
	Vertices          []ShadingType4Vertex

	F          function.Func
	Background []float64
	BBox       *pdf.Rectangle
	AntiAlias  bool
}

// ShadingType4Vertex represents a single vertex in a type 4 shading.
type ShadingType4Vertex struct {
	X, Y  float64
	Flag  uint8
	Color []float64
}

// Embed implements the [Shading] interface.
func (s *ShadingType4) Embed(w pdf.Putter, _ bool, defName pdf.Name) (*EmbeddedShading, error) {
	if s.ColorSpace == nil {
		return nil, errors.New("missing ColorSpace")
	} else if isPattern(s.ColorSpace) {
		return nil, errors.New("invalid ColorSpace")
	}
	numComponents := len(s.ColorSpace.defaultColor().values())
	if have := len(s.Background); have > 0 {
		if have != numComponents {
			err := fmt.Errorf("wrong number of background values: expected %d, got %d",
				numComponents, have)
			return nil, err
		}
	}
	switch s.BitsPerCoordinate {
	case 1, 2, 4, 8, 12, 16, 24, 32:
		// pass
	default:
		return nil, fmt.Errorf("invalid BitsPerComponent: %d", s.BitsPerComponent)
	}
	switch s.BitsPerComponent {
	case 1, 2, 4, 8, 12, 16:
		// pass
	default:
		return nil, fmt.Errorf("invalid BitsPerComponent: %d", s.BitsPerComponent)
	}
	switch s.BitsPerFlag {
	case 2, 4, 8:
		// pass
	default:
		return nil, fmt.Errorf("invalid BitsPerFlag: %d", s.BitsPerFlag)
	}
	numValues := numComponents
	if s.F != nil {
		numValues = 1
	}
	decodeLen := 4 + 2*numValues
	if have := len(s.Decode); have != decodeLen {
		return nil, fmt.Errorf("wrong number of decode values: expected %d, got %d",
			decodeLen, have)
	}
	for i := 0; i < decodeLen; i += 2 {
		if s.Decode[i] > s.Decode[i+1] {
			return nil, fmt.Errorf("invalid decode values: %v", s.Decode)
		}
	}
	for i, v := range s.Vertices {
		if v.Flag > 2 {
			return nil, fmt.Errorf("vertex %d: invalid flag: %d", i, v.Flag)
		}
		if have := len(v.Color); have != numValues {
			return nil, fmt.Errorf("vertex %d: wrong number of color values: expected %d, got %d",
				i, numValues, have)
		}
	}
	if s.F != nil && isIndexed(s.ColorSpace) {
		return nil, errors.New("Function not allowed for indexed color space")
	}

	dict := pdf.Dict{
		"ShadingType":       pdf.Integer(4),
		"ColorSpace":        s.ColorSpace.PDFObject(),
		"BitsPerCoordinate": pdf.Integer(s.BitsPerCoordinate),
		"BitsPerComponent":  pdf.Integer(s.BitsPerComponent),
		"BitsPerFlag":       pdf.Integer(s.BitsPerFlag),
		"Decode":            toPDF(s.Decode),
	}
	if len(s.Background) > 0 {
		dict["Background"] = toPDF(s.Background)
	}
	if s.BBox != nil {
		dict["BBox"] = s.BBox
	}
	if s.AntiAlias {
		dict["AntiAlias"] = pdf.Boolean(true)
	}
	if s.F != nil {
		dict["Function"] = s.F
	}

	vertexBits := s.BitsPerFlag + 2*s.BitsPerCoordinate + numValues*s.BitsPerComponent
	vertexBytes := (vertexBits + 7) / 8

	ref := w.Alloc()
	stm, err := w.OpenStream(ref, dict)
	if err != nil {
		return nil, err
	}

	// Write packed bit data for each vertex:
	//   - s.BitsPerFlag bits for the flag
	//   - s.BitsPerCoordinate bits for the x coordinate
	//   - s.BitsPerCoordinate bits for the y coordinate
	//   - numValues * s.BitsPerComponent bits for the color
	// Most-significant bits use used first.
	buf := make([]byte, vertexBytes)
	var bufBytePos, bufBitsFree int
	addBits := func(bits uint32, n int) {
		for n > 0 {
			if n < bufBitsFree {
				buf[bufBytePos] |= byte(bits << (bufBitsFree - n))
				bufBitsFree -= n
				break
			}
			buf[bufBytePos] |= byte(bits >> (n - bufBitsFree))
			n -= bufBitsFree
			bufBitsFree = 8
			bufBytePos++
		}
	}
	coord := func(x, xMin, xMax float64, bits int) uint32 {
		limit := int64(1) << bits
		z := int64(math.Floor((x - xMin) / (xMax - xMin) * float64(limit)))
		if z < 0 {
			z = 0
		} else if z >= limit {
			z = limit - 1
		}
		return uint32(z)
	}

	for _, v := range s.Vertices {
		for i := range buf {
			buf[i] = 0
		}
		bufBytePos = 0
		bufBitsFree = 8
		addBits(uint32(v.Flag), s.BitsPerFlag)
		addBits(coord(v.X, s.Decode[0], s.Decode[1], s.BitsPerCoordinate), s.BitsPerCoordinate)
		addBits(coord(v.Y, s.Decode[2], s.Decode[3], s.BitsPerCoordinate), s.BitsPerCoordinate)
		for i, c := range v.Color {
			addBits(coord(c, s.Decode[4+2*i], s.Decode[4+2*i+1], s.BitsPerComponent), s.BitsPerComponent)
		}
		_, err := stm.Write(buf)
		if err != nil {
			return nil, err
		}
	}
	err = stm.Close()
	if err != nil {
		return nil, err
	}

	return &EmbeddedShading{pdf.Res{DefName: defName, Ref: ref}}, nil
}
