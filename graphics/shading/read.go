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
	"fmt"
	"io"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
)

// Read extracts a shading from a PDF file and returns a graphics.Shading.
func Read(r pdf.Getter, obj pdf.Object) (graphics.Shading, error) {
	obj, err := pdf.Resolve(r, obj)
	if err != nil {
		return nil, err
	} else if obj == nil {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing shading object"),
		}
	}

	var dict pdf.Dict
	switch obj := obj.(type) {
	case pdf.Dict:
		dict = obj
	case *pdf.Stream:
		dict = obj.Dict
	default:
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("shading must be a dictionary or stream"),
		}
	}

	st, ok := dict["ShadingType"]
	if !ok {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing /ShadingType entry"),
		}
	}

	stNum, err := pdf.GetInteger(r, st)
	if err != nil {
		return nil, err
	}

	switch stNum {
	case 1:
		return readType1(r, dict)
	case 2:
		return readType2(r, dict)
	case 3:
		return readType3(r, dict)
	case 4:
		if stream, ok := obj.(*pdf.Stream); ok {
			return readType4(r, stream)
		}
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("type 4 shading must be a stream"),
		}
	default:
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("unsupported shading type %d", stNum),
		}
	}
}

// readType1 reads a Type 1 (function-based) shading from a PDF dictionary.
func readType1(r pdf.Getter, d pdf.Dict) (*Type1, error) {
	s := &Type1{}

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

	// Read required Function
	fnObj, ok := d["Function"]
	if !ok {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing /Function entry"),
		}
	}
	fn, err := function.Extract(r, fnObj)
	if err != nil {
		return nil, fmt.Errorf("failed to read Function: %w", err)
	}
	s.F = fn

	// Read optional Domain
	if domainObj, ok := d["Domain"]; ok {
		domain, err := floatsFromPDF(r, domainObj)
		if err != nil {
			return nil, fmt.Errorf("failed to read Domain: %w", err)
		}
		s.Domain = domain
	}

	// Read optional Matrix
	if matrixObj, ok := d["Matrix"]; ok {
		matrix, err := floatsFromPDF(r, matrixObj)
		if err != nil {
			return nil, fmt.Errorf("failed to read Matrix: %w", err)
		}
		s.Matrix = matrix
	}

	// Read optional Background
	if bgObj, ok := d["Background"]; ok {
		bg, err := floatsFromPDF(r, bgObj)
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

	return s, nil
}

// readType2 reads a Type 2 (axial) shading from a PDF dictionary.
func readType2(r pdf.Getter, d pdf.Dict) (*Type2, error) {
	s := &Type2{}

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

	// Read required Coords
	coordsObj, ok := d["Coords"]
	if !ok {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing /Coords entry"),
		}
	}
	coords, err := floatsFromPDF(r, coordsObj)
	if err != nil {
		return nil, fmt.Errorf("failed to read Coords: %w", err)
	}
	if len(coords) != 4 {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("Coords must have 4 elements, got %d", len(coords)),
		}
	}
	s.X0, s.Y0, s.X1, s.Y1 = coords[0], coords[1], coords[2], coords[3]

	// Read required Function
	fnObj, ok := d["Function"]
	if !ok {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing /Function entry"),
		}
	}
	fn, err := function.Extract(r, fnObj)
	if err != nil {
		return nil, fmt.Errorf("failed to read Function: %w", err)
	}
	s.F = fn

	// Read optional Domain (renamed to TMin/TMax for Type2)
	if domainObj, ok := d["Domain"]; ok {
		domain, err := floatsFromPDF(r, domainObj)
		if err != nil {
			return nil, fmt.Errorf("failed to read Domain: %w", err)
		}
		if len(domain) >= 2 {
			s.TMin, s.TMax = domain[0], domain[1]
		}
	} else {
		s.TMin, s.TMax = 0.0, 1.0
	}

	// Read optional Extend
	if extendObj, ok := d["Extend"]; ok {
		extendArray, err := pdf.GetArray(r, extendObj)
		if err != nil {
			return nil, fmt.Errorf("failed to read Extend: %w", err)
		}
		if len(extendArray) >= 1 {
			extendStart, err := pdf.GetBoolean(r, extendArray[0])
			if err != nil {
				return nil, fmt.Errorf("failed to read Extend[0]: %w", err)
			}
			s.ExtendStart = bool(extendStart)
		}
		if len(extendArray) >= 2 {
			extendEnd, err := pdf.GetBoolean(r, extendArray[1])
			if err != nil {
				return nil, fmt.Errorf("failed to read Extend[1]: %w", err)
			}
			s.ExtendEnd = bool(extendEnd)
		}
	}

	// Read optional Background
	if bgObj, ok := d["Background"]; ok {
		bg, err := floatsFromPDF(r, bgObj)
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

	return s, nil
}

// readType3 reads a Type 3 (radial) shading from a PDF dictionary.
func readType3(r pdf.Getter, d pdf.Dict) (*Type3, error) {
	s := &Type3{}

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

	// Read required Coords
	coordsObj, ok := d["Coords"]
	if !ok {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing /Coords entry"),
		}
	}
	coords, err := floatsFromPDF(r, coordsObj)
	if err != nil {
		return nil, fmt.Errorf("failed to read Coords: %w", err)
	}
	if len(coords) != 6 {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("Coords must have 6 elements, got %d", len(coords)),
		}
	}
	s.X1, s.Y1, s.R1, s.X2, s.Y2, s.R2 = coords[0], coords[1], coords[2], coords[3], coords[4], coords[5]

	// Read required Function
	fnObj, ok := d["Function"]
	if !ok {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing /Function entry"),
		}
	}
	fn, err := function.Extract(r, fnObj)
	if err != nil {
		return nil, fmt.Errorf("failed to read Function: %w", err)
	}
	s.F = fn

	// Read optional Domain (renamed to TMin/TMax for Type3)
	if domainObj, ok := d["Domain"]; ok {
		domain, err := floatsFromPDF(r, domainObj)
		if err != nil {
			return nil, fmt.Errorf("failed to read Domain: %w", err)
		}
		if len(domain) >= 2 {
			s.TMin, s.TMax = domain[0], domain[1]
		}
	} else {
		s.TMin, s.TMax = 0.0, 1.0
	}

	// Read optional Extend
	if extendObj, ok := d["Extend"]; ok {
		extendArray, err := pdf.GetArray(r, extendObj)
		if err != nil {
			return nil, fmt.Errorf("failed to read Extend: %w", err)
		}
		if len(extendArray) >= 1 {
			extendStart, err := pdf.GetBoolean(r, extendArray[0])
			if err != nil {
				return nil, fmt.Errorf("failed to read Extend[0]: %w", err)
			}
			s.ExtendStart = bool(extendStart)
		}
		if len(extendArray) >= 2 {
			extendEnd, err := pdf.GetBoolean(r, extendArray[1])
			if err != nil {
				return nil, fmt.Errorf("failed to read Extend[1]: %w", err)
			}
			s.ExtendEnd = bool(extendEnd)
		}
	}

	// Read optional Background
	if bgObj, ok := d["Background"]; ok {
		bg, err := floatsFromPDF(r, bgObj)
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

	return s, nil
}

// readType4 reads a Type 4 (free-form Gouraud-shaded triangle mesh) shading from a PDF stream.
func readType4(r pdf.Getter, stream *pdf.Stream) (*Type4, error) {
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
	decode, err := floatsFromPDF(r, decodeObj)
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
		bg, err := floatsFromPDF(r, bgObj)
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

// floatsFromPDF reads an array of floats from a PDF object.
func floatsFromPDF(r pdf.Getter, obj pdf.Object) ([]float64, error) {
	array, err := pdf.GetArray(r, obj)
	if err != nil {
		return nil, err
	}

	result := make([]float64, len(array))
	for i, item := range array {
		num, err := pdf.GetNumber(r, item)
		if err != nil {
			return nil, fmt.Errorf("array element %d: %w", i, err)
		}
		result[i] = float64(num)
	}

	return result, nil
}
