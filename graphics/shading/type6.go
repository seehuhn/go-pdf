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

	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
)

// PDF 2.0 sections: 8.7.4.3 8.7.4.5.7

// Type6 represents a type 6 (Coons patch mesh) shading.
//
// This type implements the [seehuhn.de/go/pdf/graphics.Shading] interface.
type Type6 struct {
	// ColorSpace defines the color space for shading color values.
	ColorSpace color.Space

	// BitsPerCoordinate specifies the number of bits used to represent each coordinate.
	BitsPerCoordinate int

	// BitsPerComponent specifies the number of bits used to represent each color component.
	BitsPerComponent int

	// BitsPerFlag specifies the number of bits used to represent the edge flag for each patch.
	BitsPerFlag int

	// Decode specifies how to map coordinates and color components into
	// the appropriate ranges of values.
	Decode []float64

	// Patches contains the patch data for the mesh.
	Patches []Type6Patch

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

var _ graphics.Shading = (*Type6)(nil)

// Type6Patch represents a single patch in a type 6 shading.
type Type6Patch struct {
	// ControlPoints contains the 12 control points defining the 4 boundary Bézier curves.
	// Layout follows PDF spec Figure 45: x₁y₁, x₂y₂, ..., x₁₂y₁₂
	ControlPoints [12]vec.Vec2

	// CornerColors contains the color values for the 4 corners (c₁, c₂, c₃, c₄).
	// If Function is present, each corner has a single parametric value.
	// Otherwise, each corner has n color components.
	CornerColors [][]float64

	// Flag determines how the patch connects to other patches.
	// 0 = new patch (no connection)
	// 1, 2, 3 = connect to specific edge of previous patch
	Flag uint8
}

// ShadingType implements the [Shading] interface.
func (s *Type6) ShadingType() int {
	return 6
}

// EdgeConnection defines how a connected patch inherits data from the previous patch.
type EdgeConnection struct {
	// ImplicitPoints contains indices in the previous patch's ControlPoints array
	// that define the first 4 control points of the current patch.
	ImplicitPoints [4]int

	// ImplicitColors contains indices in the previous patch's CornerColors array
	// that define the first 2 corner colors of the current patch.
	ImplicitColors [2]int
}

// edgeConnections defines the implicit data inheritance for each edge flag value.
// Based on PDF specification Table 84.
var edgeConnections = map[uint8]EdgeConnection{
	1: {[4]int{3, 4, 5, 6}, [2]int{1, 2}},   // Connect to D₂ edge of previous patch
	2: {[4]int{6, 7, 8, 9}, [2]int{2, 3}},   // Connect to C₂ edge of previous patch
	3: {[4]int{9, 10, 11, 0}, [2]int{3, 0}}, // Connect to D₁ edge of previous patch
}

// parseType6Patches parses patch data from binary stream data.
func parseType6Patches(data []byte, s *Type6) ([]Type6Patch, error) {
	numComponents := s.ColorSpace.Channels()
	numColorValues := numComponents
	if s.F != nil {
		numColorValues = 1 // Single parametric value if Function is present
	}

	patches := []Type6Patch{}
	bitOffset := 0

	// bit extraction helper (same as Type4/5)
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

	// coordinate decoding helper (same as Type4/5)
	decodeCoord := func(encoded uint32, bits int, decodeMin, decodeMax float64) float64 {
		maxVal := (1 << bits) - 1
		t := float64(encoded) / float64(maxVal)
		return decodeMin + t*(decodeMax-decodeMin)
	}

	for bitOffset < len(data)*8 {
		// Check if we have enough bits remaining for at least the flag
		if bitOffset+s.BitsPerFlag > len(data)*8 {
			break
		}

		// Extract edge flag
		flag := uint8(extractBits(data, bitOffset, s.BitsPerFlag))
		bitOffset += s.BitsPerFlag

		// Calculate required bits for this patch type
		var requiredBits int
		if flag == 0 {
			// New patch: 24 coordinates + 4 corner colors
			requiredBits = 24*s.BitsPerCoordinate + 4*numColorValues*s.BitsPerComponent
		} else {
			// Connected patch: 16 coordinates + 2 corner colors
			requiredBits = 16*s.BitsPerCoordinate + 2*numColorValues*s.BitsPerComponent
		}

		// Check if we have enough bits remaining for this patch
		if bitOffset+requiredBits > len(data)*8 {
			break
		}

		patch := Type6Patch{Flag: flag}

		if flag == 0 {
			// New patch: read 24 coordinates (12 points) + 4 corner colors
			// Extract all 12 control points
			for i := 0; i < 12; i++ {
				xEncoded := extractBits(data, bitOffset, s.BitsPerCoordinate)
				patch.ControlPoints[i].X = decodeCoord(xEncoded, s.BitsPerCoordinate, s.Decode[0], s.Decode[1])
				bitOffset += s.BitsPerCoordinate

				yEncoded := extractBits(data, bitOffset, s.BitsPerCoordinate)
				patch.ControlPoints[i].Y = decodeCoord(yEncoded, s.BitsPerCoordinate, s.Decode[2], s.Decode[3])
				bitOffset += s.BitsPerCoordinate
			}

			// Extract all 4 corner colors
			patch.CornerColors = make([][]float64, 4)
			for i := 0; i < 4; i++ {
				patch.CornerColors[i] = make([]float64, numColorValues)
				for j := 0; j < numColorValues; j++ {
					colorEncoded := extractBits(data, bitOffset, s.BitsPerComponent)
					decodeMin := s.Decode[4+2*j]
					decodeMax := s.Decode[4+2*j+1]
					patch.CornerColors[i][j] = decodeCoord(colorEncoded, s.BitsPerComponent, decodeMin, decodeMax)
					bitOffset += s.BitsPerComponent
				}
			}
		} else {
			// Connected patch: read 16 coordinates (8 points) + 2 corner colors
			if len(patches) == 0 {
				return nil, fmt.Errorf("connected patch (flag=%d) with no previous patch", flag)
			}

			conn, ok := edgeConnections[flag]
			if !ok {
				return nil, fmt.Errorf("invalid edge flag: %d", flag)
			}

			prevPatch := patches[len(patches)-1]

			// Copy implicit control points from previous patch
			for i := 0; i < 4; i++ {
				patch.ControlPoints[i] = prevPatch.ControlPoints[conn.ImplicitPoints[i]]
			}

			// Copy implicit corner colors from previous patch
			patch.CornerColors = make([][]float64, 4)
			for i := 0; i < 2; i++ {
				patch.CornerColors[i] = make([]float64, numColorValues)
				copy(patch.CornerColors[i], prevPatch.CornerColors[conn.ImplicitColors[i]])
			}

			// Extract explicit control points (indices 4-11)
			for i := 4; i < 12; i++ {
				xEncoded := extractBits(data, bitOffset, s.BitsPerCoordinate)
				patch.ControlPoints[i].X = decodeCoord(xEncoded, s.BitsPerCoordinate, s.Decode[0], s.Decode[1])
				bitOffset += s.BitsPerCoordinate

				yEncoded := extractBits(data, bitOffset, s.BitsPerCoordinate)
				patch.ControlPoints[i].Y = decodeCoord(yEncoded, s.BitsPerCoordinate, s.Decode[2], s.Decode[3])
				bitOffset += s.BitsPerCoordinate
			}

			// Extract explicit corner colors (indices 2-3)
			for i := 2; i < 4; i++ {
				patch.CornerColors[i] = make([]float64, numColorValues)
				for j := 0; j < numColorValues; j++ {
					colorEncoded := extractBits(data, bitOffset, s.BitsPerComponent)
					decodeMin := s.Decode[4+2*j]
					decodeMax := s.Decode[4+2*j+1]
					patch.CornerColors[i][j] = decodeCoord(colorEncoded, s.BitsPerComponent, decodeMin, decodeMax)
					bitOffset += s.BitsPerComponent
				}
			}
		}

		patches = append(patches, patch)
	}

	return patches, nil
}

// extractType6 reads a Type 6 (Coons patch mesh) shading from a PDF stream.
func extractType6(x *pdf.Extractor, stream *pdf.Stream) (*Type6, error) {
	d := stream.Dict
	s := &Type6{}

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

	// validate BitsPerCoordinate
	switch s.BitsPerCoordinate {
	case 1, 2, 4, 8, 12, 16, 24, 32:
		// valid
	default:
		return nil, pdf.Errorf("invalid /BitsPerCoordinate: %d", s.BitsPerCoordinate)
	}

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

	// validate BitsPerComponent
	switch s.BitsPerComponent {
	case 1, 2, 4, 8, 12, 16:
		// valid
	default:
		return nil, pdf.Errorf("invalid /BitsPerComponent: %d", s.BitsPerComponent)
	}

	// Read required BitsPerFlag
	bpfObj, ok := d["BitsPerFlag"]
	if !ok {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing /BitsPerFlag entry"),
		}
	}
	bpf, err := x.GetInteger(bpfObj)
	if err != nil {
		return nil, err
	}
	s.BitsPerFlag = int(bpf)

	// validate BitsPerFlag
	switch s.BitsPerFlag {
	case 2, 4, 8:
		// valid
	default:
		return nil, pdf.Errorf("invalid /BitsPerFlag: %d", s.BitsPerFlag)
	}

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

	// We'll validate the Decode array length after reading the optional Function
	// since the number of color components depends on whether a Function is present

	// Read optional Function
	if fnObj, ok := d["Function"]; ok {
		if fn, err := pdf.Optional(pdf.ExtractorGet(x, fnObj, function.Extract)); err != nil {
			return nil, err
		} else if fn != nil {
			s.F = fn
		}
	}

	// Validate Decode array length
	// Type6 shading Decode array must have 4 + 2*n elements:
	// - 4 elements for X,Y coordinates (xmin, xmax, ymin, ymax)
	// - 2*n elements for color components (cmin1, cmax1, cmin2, cmax2, ...)
	// where n is the number of color components in the patch data
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
		} else if len(bg) > 0 {
			if len(bg) != cs.Channels() {
				return nil, pdf.Errorf("wrong number of background values: expected %d, got %d", cs.Channels(), len(bg))
			}
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

	// Read stream data to extract patches
	stmReader, err := pdf.DecodeStream(x.R, stream, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to decode stream: %w", err)
	}
	defer stmReader.Close()

	data, err := io.ReadAll(stmReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read stream data: %w", err)
	}

	// Parse patches from binary data
	patches, err := parseType6Patches(data, s)
	if err != nil {
		return nil, fmt.Errorf("failed to parse patches: %w", err)
	}
	s.Patches = patches

	return s, nil
}

// Embed implements the [Shading] interface.
func (s *Type6) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	// Version check
	if err := pdf.CheckVersion(rm.Out(), "Type 6 shadings", pdf.V1_3); err != nil {
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
	for i, patch := range s.Patches {
		if patch.Flag > 3 {
			return nil, fmt.Errorf("patch %d: invalid flag: %d", i, patch.Flag)
		}
		if have := len(patch.CornerColors); have != 4 {
			return nil, fmt.Errorf("patch %d: expected 4 corner colors, got %d", i, have)
		}
		for j, corner := range patch.CornerColors {
			if have := len(corner); have != numValues {
				return nil, fmt.Errorf("patch %d corner %d: wrong number of color values: expected %d, got %d",
					i, j, numValues, have)
			}
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
		"ShadingType":       pdf.Integer(6),
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
		fn, err := rm.Embed(s.F)
		if err != nil {
			return nil, err
		}
		dict["Function"] = fn
	}

	// Calculate total bits needed for all patches
	var totalBits int
	for _, patch := range s.Patches {
		totalBits += s.BitsPerFlag
		if patch.Flag == 0 {
			// New patch: 24 coordinates + 4 corner colors
			totalBits += 24*s.BitsPerCoordinate + 4*numValues*s.BitsPerComponent
		} else {
			// Connected patch: 16 coordinates + 2 corner colors
			totalBits += 16*s.BitsPerCoordinate + 2*numValues*s.BitsPerComponent
		}
	}

	// Create one buffer for all patches
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

	// Write all patches to one continuous buffer
	for _, patch := range s.Patches {
		// Write edge flag
		addBits(uint32(patch.Flag), s.BitsPerFlag)

		if patch.Flag == 0 {
			// New patch: write all 12 control points
			for i := 0; i < 12; i++ {
				addBits(coord(patch.ControlPoints[i].X, s.Decode[0], s.Decode[1], s.BitsPerCoordinate), s.BitsPerCoordinate)
				addBits(coord(patch.ControlPoints[i].Y, s.Decode[2], s.Decode[3], s.BitsPerCoordinate), s.BitsPerCoordinate)
			}
			// Write all 4 corner colors
			for i := 0; i < 4; i++ {
				for j := 0; j < numValues; j++ {
					addBits(coord(patch.CornerColors[i][j], s.Decode[4+2*j], s.Decode[4+2*j+1], s.BitsPerComponent), s.BitsPerComponent)
				}
			}
		} else {
			// Connected patch: write explicit control points (indices 4-11)
			for i := 4; i < 12; i++ {
				addBits(coord(patch.ControlPoints[i].X, s.Decode[0], s.Decode[1], s.BitsPerCoordinate), s.BitsPerCoordinate)
				addBits(coord(patch.ControlPoints[i].Y, s.Decode[2], s.Decode[3], s.BitsPerCoordinate), s.BitsPerCoordinate)
			}
			// Write explicit corner colors (indices 2-3)
			for i := 2; i < 4; i++ {
				for j := 0; j < numValues; j++ {
					addBits(coord(patch.CornerColors[i][j], s.Decode[4+2*j], s.Decode[4+2*j+1], s.BitsPerComponent), s.BitsPerComponent)
				}
			}
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
