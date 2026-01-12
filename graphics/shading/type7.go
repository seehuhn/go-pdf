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
	"slices"

	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
)

// PDF 2.0 sections: 8.7.4.3 8.7.4.5.8

// Type7 represents a type 7 (tensor-product patch mesh) shading.
//
// This type implements the [seehuhn.de/go/pdf/graphics.Shading] interface.
type Type7 struct {
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

	// Patches contains the patch data for the tensor-product mesh.
	Patches []Type7Patch

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

var _ graphics.Shading = (*Type7)(nil)

// Type7Patch represents a single patch in a type 7 shading.
type Type7Patch struct {
	// ControlPoints contains the 16 control points defining the tensor-product patch.
	// These are arranged in a 4×4 grid but stored in stream order according to the PDF spec.
	ControlPoints [16]vec.Vec2

	// CornerColors contains the color values for the 4 corners (c₀₀, c₀₃, c₃₃, c₃₀).
	// If Function is present, each corner has a single parametric value.
	// Otherwise, each corner has n color components.
	CornerColors [][]float64

	// Flag determines how the patch connects to other patches.
	// 0 = new patch (no connection)
	// 1, 2, 3 = connect to specific edge of previous patch
	Flag uint8
}

// ShadingType implements the [Shading] interface.
func (s *Type7) ShadingType() int {
	return 7
}

// Equal implements the [Shading] interface.
func (s *Type7) Equal(other graphics.Shading) bool {
	if s == nil || other == nil {
		return s == nil && other == nil
	}
	o, ok := other.(*Type7)
	if !ok {
		return false
	}
	return color.SpacesEqual(s.ColorSpace, o.ColorSpace) &&
		s.BitsPerCoordinate == o.BitsPerCoordinate &&
		s.BitsPerComponent == o.BitsPerComponent &&
		s.BitsPerFlag == o.BitsPerFlag &&
		slices.Equal(s.Decode, o.Decode) &&
		type7PatchesEqual(s.Patches, o.Patches) &&
		function.Equal(s.F, o.F) &&
		slices.Equal(s.Background, o.Background) &&
		s.BBox.Equal(o.BBox) &&
		s.AntiAlias == o.AntiAlias
}

func type7PatchesEqual(a, b []Type7Patch) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ControlPoints != b[i].ControlPoints || a[i].Flag != b[i].Flag ||
			!cornerColorsEqual(a[i].CornerColors, b[i].CornerColors) {
			return false
		}
	}
	return true
}

// Embed implements the [Shading] interface.
func (s *Type7) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	// Version check
	if err := pdf.CheckVersion(rm.Out(), "Type 7 shadings", pdf.V1_3); err != nil {
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
		"ShadingType":       pdf.Integer(7),
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
			// New patch: 32 coordinates (16 points) + 4 corner colors
			totalBits += 32*s.BitsPerCoordinate + 4*numValues*s.BitsPerComponent
		} else {
			// Connected patch: 24 coordinates (12 explicit points) + 2 corner colors
			totalBits += 24*s.BitsPerCoordinate + 2*numValues*s.BitsPerComponent
		}
	}

	// Create one buffer for all patches
	buf := make([]byte, (totalBits+7)/8)
	var bufBytePos, bufBitsFree = 0, 8

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
			// New patch: write all 16 control points in stream order
			for i := 0; i < 16; i++ {
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
			// Connected patch: write explicit control points (stream indices 4-15)
			for i := 4; i < 16; i++ {
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

// Control point mapping tables for Type7 tensor-product patches.
// Type7 uses a 4×4 grid of control points but streams them in a specific spiral order.

// streamToGrid maps stream position (0-15) to grid coordinates (row, col).
// Based on PDF spec diagram: stream positions go in spiral pattern around boundary,
// then fill internal positions.
var streamToGrid = [16][2]int{
	{0, 0}, // stream index 0 (position 1) → p₀₀ (bottom-left)
	{1, 0}, // stream index 1 (position 2) → p₀₁
	{2, 0}, // stream index 2 (position 3) → p₀₂
	{3, 0}, // stream index 3 (position 4) → p₀₃ (top-left)
	{3, 1}, // stream index 4 (position 5) → p₁₃
	{3, 2}, // stream index 5 (position 6) → p₂₃
	{3, 3}, // stream index 6 (position 7) → p₃₃ (top-right)
	{2, 3}, // stream index 7 (position 8) → p₃₂
	{1, 3}, // stream index 8 (position 9) → p₃₁
	{0, 3}, // stream index 9 (position 10) → p₃₀ (bottom-right)
	{0, 2}, // stream index 10 (position 11) → p₂₀
	{0, 1}, // stream index 11 (position 12) → p₁₀
	{1, 1}, // stream index 12 (position 13) → p₁₁ (internal)
	{2, 1}, // stream index 13 (position 14) → p₁₂ (internal)
	{2, 2}, // stream index 14 (position 15) → p₂₂ (internal)
	{1, 2}, // stream index 15 (position 16) → p₂₁ (internal)
}

// gridToStreamOrder maps grid coordinates (row, col) to stream position (0-15).
// This is the inverse of streamToGrid for efficient lookup.
var gridToStreamOrder = [4][4]int{
	{0, 11, 10, 9}, // row 0: p₀₀, p₁₀, p₂₀, p₃₀
	{1, 12, 15, 8}, // row 1: p₀₁, p₁₁, p₂₁, p₃₁
	{2, 13, 14, 7}, // row 2: p₀₂, p₁₂, p₂₂, p₃₂
	{3, 4, 5, 6},   // row 3: p₀₃, p₁₃, p₂₃, p₃₃
}

// validateControlPointMappings verifies that streamToGrid and gridToStreamOrder
// are mathematical inverses, ensuring the control point mapping is correct.
func validateControlPointMappings() error {
	// Verify streamToGrid → gridToStreamOrder → streamToGrid round trip
	for streamIdx := 0; streamIdx < 16; streamIdx++ {
		row, col := streamToGrid[streamIdx][0], streamToGrid[streamIdx][1]
		if row < 0 || row >= 4 || col < 0 || col >= 4 {
			return fmt.Errorf("streamToGrid[%d] = (%d,%d) is out of bounds", streamIdx, row, col)
		}
		backToStream := gridToStreamOrder[row][col]
		if backToStream != streamIdx {
			return fmt.Errorf("mapping inconsistency: stream %d → grid (%d,%d) → stream %d",
				streamIdx, row, col, backToStream)
		}
	}

	// Verify gridToStreamOrder → streamToGrid → gridToStreamOrder round trip
	for row := 0; row < 4; row++ {
		for col := 0; col < 4; col++ {
			streamIdx := gridToStreamOrder[row][col]
			if streamIdx < 0 || streamIdx >= 16 {
				return fmt.Errorf("gridToStreamOrder[%d][%d] = %d is out of bounds", row, col, streamIdx)
			}
			backRow, backCol := streamToGrid[streamIdx][0], streamToGrid[streamIdx][1]
			if backRow != row || backCol != col {
				return fmt.Errorf("mapping inconsistency: grid (%d,%d) → stream %d → grid (%d,%d)",
					row, col, streamIdx, backRow, backCol)
			}
		}
	}

	return nil
}

// Type7EdgeConnection defines how a connected patch inherits data from the previous patch.
type Type7EdgeConnection struct {
	// ImplicitStreamIndices contains the stream indices in the previous patch
	// that define the first 4 control points of the current patch.
	// These correspond to the left edge (column 0) of the current patch grid.
	ImplicitStreamIndices [4]int

	// ImplicitColorIndices contains indices in the previous patch's CornerColors array
	// that define the first 2 corner colors of the current patch.
	ImplicitColorIndices [2]int
}

// type7EdgeConnections defines the implicit data inheritance for each edge flag value.
// Based on PDF specification Table 85.
var type7EdgeConnections = map[uint8]Type7EdgeConnection{
	// f = 1: Connect to previous patch's right edge (column 3)
	// Current (0,0),(0,1),(0,2),(0,3) get previous (0,3),(1,3),(2,3),(3,3)
	1: {
		ImplicitStreamIndices: [4]int{9, 8, 7, 6}, // stream indices for (0,3),(1,3),(2,3),(3,3)
		ImplicitColorIndices:  [2]int{1, 2},       // c₀₀ = c₀₃ previous, c₀₃ = c₃₃ previous
	},

	// f = 2: Connect to previous patch's top edge (row 3)
	// Current (0,0),(0,1),(0,2),(0,3) get previous (3,3),(3,2),(3,1),(3,0)
	2: {
		ImplicitStreamIndices: [4]int{6, 5, 4, 3}, // stream indices for (3,3),(3,2),(3,1),(3,0)
		ImplicitColorIndices:  [2]int{2, 3},       // c₀₀ = c₃₃ previous, c₀₃ = c₃₀ previous
	},

	// f = 3: Connect to previous patch's left edge (column 0)
	// Current (0,0),(0,1),(0,2),(0,3) get previous (3,0),(2,0),(1,0),(0,0)
	3: {
		ImplicitStreamIndices: [4]int{3, 2, 1, 0}, // stream indices for (3,0),(2,0),(1,0),(0,0)
		ImplicitColorIndices:  [2]int{3, 0},       // c₀₀ = c₃₀ previous, c₀₃ = c₀₀ previous
	},
}

// extractType7 reads a Type 7 (tensor-product patch mesh) shading from a PDF stream.
func extractType7(x *pdf.Extractor, stream *pdf.Stream) (*Type7, error) {
	d := stream.Dict
	s := &Type7{}

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
	// Type7 shading Decode array must have 4 + 2*n elements:
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
	patches, err := parseType7Patches(data, s)
	if err != nil {
		return nil, fmt.Errorf("failed to parse patches: %w", err)
	}
	s.Patches = patches

	return s, nil
}

// parseType7Patches parses patch data from binary stream data.
func parseType7Patches(data []byte, s *Type7) ([]Type7Patch, error) {
	numComponents := s.ColorSpace.Channels()
	numColorValues := numComponents
	if s.F != nil {
		numColorValues = 1 // Single parametric value if Function is present
	}

	patches := []Type7Patch{}
	bitOffset := 0

	// bit extraction helper (same as Type4/5/6)
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

	// coordinate decoding helper (same as Type4/5/6)
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
			// New patch: 32 coordinates (16 points) + 4 corner colors
			requiredBits = 32*s.BitsPerCoordinate + 4*numColorValues*s.BitsPerComponent
		} else {
			// Connected patch: 24 coordinates (12 explicit points) + 2 corner colors
			requiredBits = 24*s.BitsPerCoordinate + 2*numColorValues*s.BitsPerComponent
		}

		// Check if we have enough bits remaining for this patch
		if bitOffset+requiredBits > len(data)*8 {
			break
		}

		patch := Type7Patch{Flag: flag}

		if flag == 0 {
			// New patch: read 32 coordinates (16 points) + 4 corner colors
			// Extract all 16 control points in stream order
			for i := 0; i < 16; i++ {
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
			// Connected patch: read 24 coordinates (12 explicit points) + 2 corner colors
			if len(patches) == 0 {
				return nil, fmt.Errorf("connected patch (flag=%d) with no previous patch", flag)
			}

			conn, ok := type7EdgeConnections[flag]
			if !ok {
				return nil, fmt.Errorf("invalid edge flag: %d", flag)
			}

			prevPatch := patches[len(patches)-1]

			// Copy implicit control points from previous patch to left edge (column 0)
			// Current patch grid positions (0,0), (0,1), (0,2), (0,3)
			// correspond to stream indices 0, 1, 2, 3
			for i := 0; i < 4; i++ {
				patch.ControlPoints[i] = prevPatch.ControlPoints[conn.ImplicitStreamIndices[i]]
			}

			// Copy implicit corner colors from previous patch
			patch.CornerColors = make([][]float64, 4)
			for i := 0; i < 2; i++ {
				patch.CornerColors[i] = make([]float64, numColorValues)
				copy(patch.CornerColors[i], prevPatch.CornerColors[conn.ImplicitColorIndices[i]])
			}

			// Extract explicit control points (stream indices 4-15, which are 12 points)
			for i := 4; i < 16; i++ {
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

// init validates the control point mappings at package initialization.
func init() {
	if err := validateControlPointMappings(); err != nil {
		panic(fmt.Sprintf("Type7 control point mapping validation failed: %v", err))
	}
}
