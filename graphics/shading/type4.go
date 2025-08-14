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

// Type4 represents a type 4 (free-form Gouraud-shaded triangle mesh) shading.
//
// https://opensource.adobe.com/dc-acrobat-sdk-docs/pdfstandards/PDF32000_2008.pdf#page=189
//
// This type implements the [seehuhn.de/go/pdf/graphics.Shading] interface.
type Type4 struct {
	// ColorSpace defines the color space for shading color values.
	ColorSpace color.Space

	// BitsPerCoordinate specifies the number of bits used to represent each vertex coordinate.
	BitsPerCoordinate int

	// BitsPerComponent specifies the number of bits used to represent each color component.
	BitsPerComponent int

	// BitsPerFlag specifies the number of bits used to represent the edge flag for each vertex.
	BitsPerFlag int

	// Decode specifies how to map vertex coordinates and color components into
	// the appropriate ranges of values.
	Decode []float64

	// Vertices contains the vertex data for the triangle mesh.
	Vertices []Type4Vertex

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

var _ graphics.Shading = (*Type4)(nil)

// Type4Vertex represents a single vertex in a type 4 shading.
type Type4Vertex struct {
	// X, Y are the vertex coordinates.
	X, Y float64

	// Flag determines how the vertex connects to other vertices (0, 1, or 2).
	Flag uint8

	// Color contains the color components for this vertex.
	Color []float64
}

// ShadingType implements the [Shading] interface.
func (s *Type4) ShadingType() int {
	return 4
}

// extractType4 reads a Type 4 (free-form Gouraud-shaded triangle mesh) shading from a PDF stream.
func extractType4(r pdf.Getter, stream *pdf.Stream, wasReference bool) (*Type4, error) {
	d := stream.Dict
	s := &Type4{}

	// Read required ColorSpace
	csObj, ok := d["ColorSpace"]
	if !ok {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing /ColorSpace entry"),
		}
	}
	cs, err := color.ExtractSpace(r, csObj)
	if err != nil {
		return nil, fmt.Errorf("failed to read ColorSpace: %w", err)
	}
	s.ColorSpace = cs

	// Read required BitsPerCoordinate
	bpcObj, ok := d["BitsPerCoordinate"]
	if !ok {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing /BitsPerCoordinate entry"),
		}
	}
	bpc, err := pdf.GetInteger(r, bpcObj)
	if err != nil {
		return nil, fmt.Errorf("failed to read BitsPerCoordinate: %w", err)
	}
	s.BitsPerCoordinate = int(bpc)

	// Read required BitsPerComponent
	bpcompObj, ok := d["BitsPerComponent"]
	if !ok {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing /BitsPerComponent entry"),
		}
	}
	bpcomp, err := pdf.GetInteger(r, bpcompObj)
	if err != nil {
		return nil, fmt.Errorf("failed to read BitsPerComponent: %w", err)
	}
	s.BitsPerComponent = int(bpcomp)

	// Read required BitsPerFlag
	bpfObj, ok := d["BitsPerFlag"]
	if !ok {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing /BitsPerFlag entry"),
		}
	}
	bpf, err := pdf.GetInteger(r, bpfObj)
	if err != nil {
		return nil, fmt.Errorf("failed to read BitsPerFlag: %w", err)
	}
	s.BitsPerFlag = int(bpf)

	// Read required Decode
	decodeObj, ok := d["Decode"]
	if !ok {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing /Decode entry"),
		}
	}
	decode, err := pdf.GetFloatArray(r, decodeObj)
	if err != nil {
		return nil, fmt.Errorf("failed to read Decode: %w", err)
	}
	s.Decode = decode

	// We'll validate the Decode array length after reading the optional Function
	// since the number of color components depends on whether a Function is present

	// Read optional Function
	if fnObj, ok := d["Function"]; ok {
		fn, err := function.Extract(r, fnObj)
		if err != nil {
			return nil, fmt.Errorf("failed to read Function: %w", err)
		}
		s.F = fn
	}

	// Validate Decode array length
	// Type4 shading Decode array must have 4 + 2*n elements:
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
		bg, err := pdf.GetFloatArray(r, bgObj)
		if err != nil {
			return nil, fmt.Errorf("failed to read Background: %w", err)
		}
		s.Background = bg
	}

	// Read optional BBox
	if bboxObj, ok := d["BBox"]; ok {
		bbox, err := pdf.GetRectangle(r, bboxObj)
		if err != nil {
			return nil, fmt.Errorf("failed to read BBox: %w", err)
		}
		s.BBox = bbox
	}

	// Read optional AntiAlias
	if aaObj, ok := d["AntiAlias"]; ok {
		aa, err := pdf.GetBoolean(r, aaObj)
		if err != nil {
			return nil, fmt.Errorf("failed to read AntiAlias: %w", err)
		}
		s.AntiAlias = bool(aa)
	}

	// Read stream data to extract vertices
	stmReader, err := pdf.DecodeStream(r, stream, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to decode stream: %w", err)
	}
	defer stmReader.Close()

	data, err := io.ReadAll(stmReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read stream data: %w", err)
	}

	// Parse vertices from binary data
	vertices, err := parseType4Vertices(data, s)
	if err != nil {
		return nil, fmt.Errorf("failed to parse vertices: %w", err)
	}
	s.Vertices = vertices

	return s, nil
}

// parseType4Vertices parses vertex data from binary stream data.
func parseType4Vertices(data []byte, s *Type4) ([]Type4Vertex, error) {
	numComponents := s.ColorSpace.Channels()
	numValues := numComponents
	if s.F != nil {
		numValues = 1
	}

	vertexBits := s.BitsPerFlag + 2*s.BitsPerCoordinate + numValues*s.BitsPerComponent
	if vertexBits <= 0 {
		return nil, pdf.Errorf("invalid vertex bit configuration: total bits per vertex is %d", vertexBits)
	}
	vertexBytes := (vertexBits + 7) / 8

	if len(data)%vertexBytes != 0 {
		return nil, fmt.Errorf("invalid stream data length: %d bytes is not a multiple of %d", len(data), vertexBytes)
	}

	numVertices := len(data) / vertexBytes
	vertices := make([]Type4Vertex, numVertices)

	// bit extraction helper
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

	// coordinate decoding helper
	decodeCoord := func(encoded uint32, bits int, decodeMin, decodeMax float64) float64 {
		maxVal := (1 << bits) - 1
		t := float64(encoded) / float64(maxVal)
		return decodeMin + t*(decodeMax-decodeMin)
	}

	for i := 0; i < numVertices; i++ {
		vertexData := data[i*vertexBytes : (i+1)*vertexBytes]
		bitOffset := 0

		// Extract flag
		flag := extractBits(vertexData, bitOffset, s.BitsPerFlag)
		vertices[i].Flag = uint8(flag)
		bitOffset += s.BitsPerFlag

		// Extract X coordinate
		xEncoded := extractBits(vertexData, bitOffset, s.BitsPerCoordinate)
		vertices[i].X = decodeCoord(xEncoded, s.BitsPerCoordinate, s.Decode[0], s.Decode[1])
		bitOffset += s.BitsPerCoordinate

		// Extract Y coordinate
		yEncoded := extractBits(vertexData, bitOffset, s.BitsPerCoordinate)
		vertices[i].Y = decodeCoord(yEncoded, s.BitsPerCoordinate, s.Decode[2], s.Decode[3])
		bitOffset += s.BitsPerCoordinate

		// Extract color components
		vertices[i].Color = make([]float64, numValues)
		for j := 0; j < numValues; j++ {
			colorEncoded := extractBits(vertexData, bitOffset, s.BitsPerComponent)
			decodeMin := s.Decode[4+2*j]
			decodeMax := s.Decode[4+2*j+1]
			vertices[i].Color[j] = decodeCoord(colorEncoded, s.BitsPerComponent, decodeMin, decodeMax)
			bitOffset += s.BitsPerComponent
		}
	}

	return vertices, nil
}

// Embed implements the [Shading] interface.
func (s *Type4) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused
	if s.ColorSpace == nil {
		return nil, zero, errors.New("missing ColorSpace")
	} else if s.ColorSpace.Family() == color.FamilyPattern {
		return nil, zero, errors.New("invalid ColorSpace")
	}
	numComponents := s.ColorSpace.Channels()
	if have := len(s.Background); have > 0 {
		if have != numComponents {
			err := fmt.Errorf("wrong number of background values: expected %d, got %d",
				numComponents, have)
			return nil, zero, err
		}
	}
	switch s.BitsPerCoordinate {
	case 1, 2, 4, 8, 12, 16, 24, 32:
		// pass
	default:
		return nil, zero, fmt.Errorf("invalid BitsPerCoordinate: %d", s.BitsPerCoordinate)
	}
	switch s.BitsPerComponent {
	case 1, 2, 4, 8, 12, 16:
		// pass
	default:
		return nil, zero, fmt.Errorf("invalid BitsPerComponent: %d", s.BitsPerComponent)
	}
	switch s.BitsPerFlag {
	case 2, 4, 8:
		// pass
	default:
		return nil, zero, fmt.Errorf("invalid BitsPerFlag: %d", s.BitsPerFlag)
	}
	numValues := numComponents
	if s.F != nil {
		numValues = 1
	}
	decodeLen := 4 + 2*numValues
	if have := len(s.Decode); have != decodeLen {
		return nil, zero, fmt.Errorf("wrong number of decode values: expected %d, got %d",
			decodeLen, have)
	}
	for i := 0; i < decodeLen; i += 2 {
		if s.Decode[i] > s.Decode[i+1] {
			return nil, zero, fmt.Errorf("invalid decode values: %v", s.Decode)
		}
	}
	for i, v := range s.Vertices {
		if v.Flag > 2 {
			return nil, zero, fmt.Errorf("vertex %d: invalid flag: %d", i, v.Flag)
		}
		if have := len(v.Color); have != numValues {
			return nil, zero, fmt.Errorf("vertex %d: wrong number of color values: expected %d, got %d",
				i, numValues, have)
		}
	}
	if s.F != nil && s.ColorSpace.Family() == color.FamilyIndexed {
		return nil, zero, errors.New("Function not allowed for indexed color space")
	}

	csE, _, err := pdf.ResourceManagerEmbed(rm, s.ColorSpace)
	if err != nil {
		return nil, zero, err
	}

	dict := pdf.Dict{
		"ShadingType":       pdf.Integer(4),
		"ColorSpace":        csE,
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
		fn, _, err := pdf.ResourceManagerEmbed(rm, s.F)
		if err != nil {
			return nil, zero, err
		}
		dict["Function"] = fn
	}

	vertexBits := s.BitsPerFlag + 2*s.BitsPerCoordinate + numValues*s.BitsPerComponent
	vertexBytes := (vertexBits + 7) / 8

	ref := rm.Out.Alloc()
	stm, err := rm.Out.OpenStream(ref, dict)
	if err != nil {
		return nil, zero, err
	}

	// write packed bit data for each vertex:
	//   - s.BitsPerFlag bits for the flag
	//   - s.BitsPerCoordinate bits for the x coordinate
	//   - s.BitsPerCoordinate bits for the y coordinate
	//   - numValues * s.BitsPerComponent bits for the color
	// most-significant bits are used first.
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
			return nil, zero, err
		}
	}
	err = stm.Close()
	if err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
}
