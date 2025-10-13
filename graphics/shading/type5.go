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

package shading

import (
	"errors"
	"fmt"
	"io"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
)

// PDF 2.0 sections: 8.7.4.3 8.7.4.5.6

// Type5 represents a type 5 (lattice-form Gouraud-shaded triangle mesh) shading.
//
// This type implements the [seehuhn.de/go/pdf/graphics.Shading] interface.
type Type5 struct {
	// ColorSpace defines the color space for shading color values.
	ColorSpace color.Space

	// BitsPerCoordinate specifies the number of bits used to represent each vertex coordinate.
	BitsPerCoordinate int

	// BitsPerComponent specifies the number of bits used to represent each color component.
	BitsPerComponent int

	// VerticesPerRow specifies the number of vertices in each row of the lattice.
	// The value must be greater than or equal to 2.
	VerticesPerRow int

	// Decode specifies how to map vertex coordinates and color components into
	// the appropriate ranges of values.
	Decode []float64

	// Vertices contains the vertex data for the triangle mesh, organized in rows.
	Vertices []Type5Vertex

	// F (optional) is a 1->n function for mapping parametric values to colors.
	F pdf.Function

	// Background (optional) specifies the color for areas outside the
	// shading's bounds, when used in a shading pattern.
	Background []float64

	// BBox (optional) defines the shading's bounding box as a clipping boundary.
	BBox *pdf.Rectangle

	// AntiAlias controls whether to filter the shading function to prevent aliasing.
	AntiAlias bool
}

var _ graphics.Shading = (*Type5)(nil)

// Type5Vertex represents a single vertex in a type 5 shading.
type Type5Vertex struct {
	// X, Y are the vertex coordinates.
	X, Y float64

	// Color contains the color components for this vertex.
	Color []float64
}

// ShadingType implements the [Shading] interface.
func (s *Type5) ShadingType() int {
	return 5
}

// extractType5 reads a Type 5 (lattice-form Gouraud-shaded triangle mesh) shading from a PDF stream.
func extractType5(x *pdf.Extractor, stream *pdf.Stream, wasReference bool) (*Type5, error) {
	d := stream.Dict
	s := &Type5{}

	// Read required ColorSpace
	csObj, ok := d["ColorSpace"]
	if !ok {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing /ColorSpace entry"),
		}
	}
	cs, err := color.ExtractSpace(x, csObj)
	if err != nil {
		return nil, err
	}
	s.ColorSpace = cs

	// Read required BitsPerCoordinate
	bpcObj, ok := d["BitsPerCoordinate"]
	if !ok {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing /BitsPerCoordinate entry"),
		}
	}
	bpc, err := x.GetInteger(bpcObj)
	if err != nil {
		return nil, err
	}
	s.BitsPerCoordinate = int(bpc)

	// Read required BitsPerComponent
	bpcompObj, ok := d["BitsPerComponent"]
	if !ok {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing /BitsPerComponent entry"),
		}
	}
	bpcomp, err := x.GetInteger(bpcompObj)
	if err != nil {
		return nil, err
	}
	s.BitsPerComponent = int(bpcomp)

	// Read required VerticesPerRow
	vprObj, ok := d["VerticesPerRow"]
	if !ok {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing /VerticesPerRow entry"),
		}
	}
	vpr, err := x.GetInteger(vprObj)
	if err != nil {
		return nil, err
	}
	s.VerticesPerRow = int(vpr)

	// Read required Decode
	decodeObj, ok := d["Decode"]
	if !ok {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing /Decode entry"),
		}
	}
	decode, err := pdf.GetFloatArray(x.R, decodeObj)
	if err != nil {
		return nil, err
	}
	s.Decode = decode

	// Read optional Function
	if fnObj, ok := d["Function"]; ok {
		if fn, err := pdf.Optional(pdf.ExtractorGet(x, fnObj, function.Extract)); err != nil {
			return nil, err
		} else if fn != nil {
			s.F = fn
		}
	}

	// Validate Decode array length
	// Type5 shading Decode array must have 4 + 2*n elements:
	// - 4 elements for X,Y coordinates (xmin, xmax, ymin, ymax)
	// - 2*n elements for color components (cmin1, cmax1, cmin2, cmax2, ...)
	// where n is the number of color components in the vertex data
	var numColorComponents int
	if s.F != nil {
		// If function is present, color components are function inputs
		m, _ := s.F.Shape()
		numColorComponents = m
	} else {
		// If no function, color components are direct color space values
		numColorComponents = s.ColorSpace.Channels()
	}
	expectedDecodeLength := 4 + 2*numColorComponents // 4 for X,Y + 2 per color component
	if len(s.Decode) != expectedDecodeLength {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("invalid Decode array length: expected %d, got %d", expectedDecodeLength, len(s.Decode)),
		}
	}

	// Read optional Background
	if bgObj, ok := d["Background"]; ok {
		if bg, err := pdf.Optional(pdf.GetFloatArray(x.R, bgObj)); err != nil {
			return nil, err
		} else {
			s.Background = bg
		}
	}

	// Read optional BBox
	if bboxObj, ok := d["BBox"]; ok {
		if bbox, err := pdf.Optional(pdf.GetRectangle(x.R, bboxObj)); err != nil {
			return nil, err
		} else {
			s.BBox = bbox
		}
	}

	// Read optional AntiAlias
	if aaObj, ok := d["AntiAlias"]; ok {
		if aa, err := pdf.Optional(x.GetBoolean(aaObj)); err != nil {
			return nil, err
		} else {
			s.AntiAlias = bool(aa)
		}
	}

	// Read stream data to extract vertices
	stmReader, err := pdf.DecodeStream(x.R, stream, 0)
	if err != nil {
		return nil, err
	}
	defer stmReader.Close()

	data, err := io.ReadAll(stmReader)
	if err != nil {
		return nil, err
	}

	// Parse vertices from binary data
	vertices, err := parseType5Vertices(data, s)
	if err != nil {
		return nil, err
	}
	s.Vertices = vertices

	return s, nil
}

// parseType5Vertices parses vertex data from binary stream data.
func parseType5Vertices(data []byte, s *Type5) ([]Type5Vertex, error) {
	numComponents := s.ColorSpace.Channels()
	numValues := numComponents
	if s.F != nil {
		numValues = 1
	}

	// Type5 has no edge flags, so vertex bits calculation is simpler
	vertexBits := 2*s.BitsPerCoordinate + numValues*s.BitsPerComponent
	if vertexBits <= 0 {
		return nil, pdf.Errorf("invalid vertex bit configuration: total bits per vertex is %d", vertexBits)
	}

	// Calculate how many complete vertices we can extract from the data
	totalBits := len(data) * 8
	numVertices := totalBits / vertexBits

	if numVertices == 0 {
		return nil, fmt.Errorf("insufficient data: need at least %d bits per vertex, got %d total bits", vertexBits, totalBits)
	}

	// Validate lattice completeness
	if numVertices%s.VerticesPerRow != 0 {
		return nil, fmt.Errorf("invalid lattice: %d vertices is not a multiple of %d vertices per row", numVertices, s.VerticesPerRow)
	}

	numRows := numVertices / s.VerticesPerRow
	if numRows < 2 {
		return nil, fmt.Errorf("invalid lattice: need at least 2 rows for triangulation, got %d", numRows)
	}

	vertices := make([]Type5Vertex, numVertices)

	// bit extraction helper (same as Type4/6/7)
	extractBits := func(data []byte, bitOffset, numBits int) uint32 {
		var result uint32
		for i := 0; i < numBits; i++ {
			byteIndex := (bitOffset + i) / 8
			bitIndex := 7 - ((bitOffset + i) % 8)
			if byteIndex < len(data) && (data[byteIndex]&(1<<bitIndex)) != 0 {
				result |= 1 << (numBits - 1 - i)
			}
		}
		return result
	}

	// coordinate decoding helper (same as Type4/6/7)
	decodeCoord := func(encoded uint32, bits int, decodeMin, decodeMax float64) float64 {
		maxVal := (1 << bits) - 1
		t := float64(encoded) / float64(maxVal)
		return decodeMin + t*(decodeMax-decodeMin)
	}

	// Process vertices using continuous bit stream (not byte-aligned chunks)
	bitOffset := 0
	for i := 0; i < numVertices; i++ {
		// No flag extraction for Type5 - vertices are positioned by lattice structure

		// Extract X coordinate
		xEncoded := extractBits(data, bitOffset, s.BitsPerCoordinate)
		vertices[i].X = decodeCoord(xEncoded, s.BitsPerCoordinate, s.Decode[0], s.Decode[1])
		bitOffset += s.BitsPerCoordinate

		// Extract Y coordinate
		yEncoded := extractBits(data, bitOffset, s.BitsPerCoordinate)
		vertices[i].Y = decodeCoord(yEncoded, s.BitsPerCoordinate, s.Decode[2], s.Decode[3])
		bitOffset += s.BitsPerCoordinate

		// Extract color components
		vertices[i].Color = make([]float64, numValues)
		for j := 0; j < numValues; j++ {
			colorEncoded := extractBits(data, bitOffset, s.BitsPerComponent)
			decodeMin := s.Decode[4+2*j]
			decodeMax := s.Decode[4+2*j+1]
			vertices[i].Color[j] = decodeCoord(colorEncoded, s.BitsPerComponent, decodeMin, decodeMax)
			bitOffset += s.BitsPerComponent
		}
	}

	return vertices, nil
}

// Embed implements the [Shading] interface.
func (s *Type5) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {

	// Version check
	if err := pdf.CheckVersion(rm.Out(), "Type 5 shadings", pdf.V1_3); err != nil {
		return nil, err
	}

	if s.ColorSpace == nil {
		return nil, errors.New("missing ColorSpace")
	} else if s.ColorSpace.Family() == color.FamilyPattern {
		return nil, errors.New("invalid ColorSpace")
	}
	numComponents := s.ColorSpace.Channels()
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
		return nil, fmt.Errorf("invalid BitsPerCoordinate: %d", s.BitsPerCoordinate)
	}
	switch s.BitsPerComponent {
	case 1, 2, 4, 8, 12, 16:
		// pass
	default:
		return nil, fmt.Errorf("invalid BitsPerComponent: %d", s.BitsPerComponent)
	}
	if s.VerticesPerRow < 2 {
		return nil, fmt.Errorf("invalid VerticesPerRow: %d (must be >= 2)", s.VerticesPerRow)
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

	// Validate lattice structure
	numVertices := len(s.Vertices)
	if numVertices%s.VerticesPerRow != 0 {
		return nil, fmt.Errorf("invalid lattice: %d vertices is not a multiple of %d vertices per row", numVertices, s.VerticesPerRow)
	}
	numRows := numVertices / s.VerticesPerRow
	if numRows < 2 {
		return nil, fmt.Errorf("invalid lattice: need at least 2 rows for triangulation, got %d", numRows)
	}

	for i, v := range s.Vertices {
		if have := len(v.Color); have != numValues {
			return nil, fmt.Errorf("vertex %d: wrong number of color values: expected %d, got %d",
				i, numValues, have)
		}
	}
	if s.F != nil && s.ColorSpace.Family() == color.FamilyIndexed {
		return nil, errors.New("Function not allowed for indexed color space")
	}

	csE, err := rm.Embed(s.ColorSpace)
	if err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"ShadingType":       pdf.Integer(5),
		"ColorSpace":        csE,
		"BitsPerCoordinate": pdf.Integer(s.BitsPerCoordinate),
		"BitsPerComponent":  pdf.Integer(s.BitsPerComponent),
		"VerticesPerRow":    pdf.Integer(s.VerticesPerRow),
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
		fn, err := rm.Embed(s.F)
		if err != nil {
			return nil, err
		}
		dict["Function"] = fn
	}

	// Calculate total bits needed for all vertices
	vertexBits := 2*s.BitsPerCoordinate + numValues*s.BitsPerComponent
	totalBits := len(s.Vertices) * vertexBits

	// Create one buffer for all vertices
	buf := make([]byte, (totalBits+7)/8)
	var bufBytePos, bufBitsFree int = 0, 8

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

	// Write all vertices to one continuous buffer
	for _, v := range s.Vertices {
		// No flag bits for Type5 - vertices are positioned by lattice structure
		addBits(coord(v.X, s.Decode[0], s.Decode[1], s.BitsPerCoordinate), s.BitsPerCoordinate)
		addBits(coord(v.Y, s.Decode[2], s.Decode[3], s.BitsPerCoordinate), s.BitsPerCoordinate)
		for i, c := range v.Color {
			addBits(coord(c, s.Decode[4+2*i], s.Decode[4+2*i+1], s.BitsPerComponent), s.BitsPerComponent)
		}
	}

	ref := rm.Alloc()
	stm, err := rm.Out().OpenStream(ref, dict)
	if err != nil {
		return nil, err
	}

	_, err = stm.Write(buf)
	if err != nil {
		return nil, err
	}

	err = stm.Close()
	if err != nil {
		return nil, err
	}

	return ref, nil
}
