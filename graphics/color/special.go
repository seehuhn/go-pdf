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
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"seehuhn.de/go/pdf"
)

// == Indexed ================================================================

// SpaceIndexed represents an indexed color space.
type SpaceIndexed struct {
	NumCol int
	Base   Space

	// lookup contains the color palette data as encoded bytes.
	lookup pdf.String
}

// Indexed returns a new indexed color space.
//
// The colors must all use the same color space, and the number of colors must
// be in the range from 1 to 256 (both inclusive).
func Indexed(colors []Color) (*SpaceIndexed, error) {
	if len(colors) == 0 || len(colors) > 256 {
		return nil, fmt.Errorf("Indexed: invalid number of colors: %d", len(colors))
	}

	space := colors[0].ColorSpace()
	var min, max []float64
	switch space := space.(type) {
	case spaceDeviceGray, *SpaceCalGray:
		min = []float64{0}
		max = []float64{1}
	case spaceDeviceRGB, *SpaceCalRGB:
		min = []float64{0, 0, 0}
		max = []float64{1, 1, 1}
	case spaceDeviceCMYK:
		min = []float64{0, 0, 0, 0}
		max = []float64{1, 1, 1, 1}
	case *SpaceLab:
		min = []float64{0, space.ranges[0], space.ranges[2]}
		max = []float64{100, space.ranges[1], space.ranges[3]}
	case spacePatternColored, spacePatternUncolored, *SpaceIndexed:
		return nil, fmt.Errorf("Indexed: invalid base color space %s", space.Family())
	}

	lookup := make(pdf.String, 0, len(colors)*len(min))
	for _, color := range colors {
		if color.ColorSpace() != space {
			return nil, errors.New("Indexed: inconsistent color spaces")
		}
		v := values(color)
		for i, x := range v {
			b := int(math.Floor((x - min[i]) / (max[i] - min[i]) * 256))
			if b < 0 {
				b = 0
			} else if b > 255 {
				b = 255
			}
			lookup = append(lookup, byte(b))
		}
	}

	return &SpaceIndexed{
		Base:   space,
		NumCol: len(colors),
		lookup: lookup,
	}, nil
}

// Family returns /Indexed.
// This implements the [Space] interface.
func (s *SpaceIndexed) Family() pdf.Name {
	return FamilyIndexed
}

// Channels returns 1
// This implements the [Space] interface.
func (s *SpaceIndexed) Channels() int {
	return 1
}

// Embed adds the color space to a PDF file.
// This implements the [Space] interface.
func (s *SpaceIndexed) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {

	if err := pdf.CheckVersion(rm.Out(), "Indexed color space", pdf.V1_1); err != nil {
		return nil, err
	}

	base, err := rm.Embed(s.Base)
	if err != nil {
		return nil, err
	}

	data := pdf.Array{
		pdf.Name("Indexed"),
		base,
		pdf.Integer(s.NumCol - 1),
		s.lookup,
	}

	return data, nil
}

// Default returns color 0 in the indexed color space.
// This implements the [Space] interface.
func (s *SpaceIndexed) Default() Color {
	return colorIndexed{Space: s, Index: 0}
}

// New returns a new indexed color.
func (s *SpaceIndexed) New(idx int) Color {
	if idx < 0 || idx >= s.NumCol {
		return nil
	}
	return colorIndexed{Space: s, Index: idx}
}

type colorIndexed struct {
	Space *SpaceIndexed
	Index int
}

func (c colorIndexed) ColorSpace() Space {
	return c.Space
}

// == Separation =============================================================

// SpaceSeparation represents a separation color space.
type SpaceSeparation struct {
	colorant  pdf.Name
	alternate Space
	trfm      pdf.Function
}

// Separation returns a new separation color space.
func Separation(colorant pdf.Name, alternate Space, trfm pdf.Function) (*SpaceSeparation, error) {
	if IsSpecial(alternate) {
		return nil, errors.New("Separation: invalid alternate color space")
	}

	nIn, nOut := trfm.Shape()
	if nIn != 1 || nOut != alternate.Channels() {
		return nil, errors.New("Separation: invalid transformation function")
	}

	return &SpaceSeparation{
		colorant:  colorant,
		alternate: alternate,
		trfm:      trfm,
	}, nil
}

// Family returns /Separation.
// This implements the [Space] interface.
func (s *SpaceSeparation) Family() pdf.Name {
	return FamilySeparation
}

// Channels returns 1.
// This implements the [Space] interface.
func (s *SpaceSeparation) Channels() int {
	return 1
}

// Embed adds the color space to a PDF file.
// This implements the pdf.Embedder interface.
func (s *SpaceSeparation) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {

	if err := pdf.CheckVersion(rm.Out(), "Separation color space", pdf.V1_2); err != nil {
		return nil, err
	}

	alt, err := rm.Embed(s.alternate)
	if err != nil {
		return nil, err
	}
	trfm, err := rm.Embed(s.trfm)
	if err != nil {
		return nil, err
	}

	return pdf.Array{FamilySeparation, s.colorant, alt, trfm}, nil
}

// New returns a new color in the separation color space.
// Tint must be between 0 (no ink, lightest) and 1 (full ink, darkest).
func (s *SpaceSeparation) New(tint float64) Color {
	return colorSeparation{Space: s, Tint: tint}
}

// Default returns the default color of the color space.
func (s *SpaceSeparation) Default() Color {
	return s.New(1)
}

type colorSeparation struct {
	Space *SpaceSeparation
	Tint  float64
}

// ColorSpace returns the color space of the color.
// This implements the [Color] interface.
func (c colorSeparation) ColorSpace() Space {
	return c.Space
}

// == DeviceN ================================================================

// SpaceDeviceN represents a DeviceN color space.
//
// See section 8.6.6.5 (DeviceN Color Spaces) of PDF 32000-1:2008 for details:
// https://opensource.adobe.com/dc-acrobat-sdk-docs/pdfstandards/PDF32000_2008.pdf#page=167
type SpaceDeviceN struct {
	colorants pdf.Array
	alternate Space
	trfm      pdf.Function
	attr      pdf.Dict
}

// DeviceN returns a new DeviceN color space.

// DeviceN returns a new DeviceN color space with the given component names,
// alternate color space, tint transformation function, and attributes
// dictionary (optional).
func DeviceN(names []pdf.Name, alternate Space, trfm pdf.Function, attr pdf.Dict) (*SpaceDeviceN, error) {
	namesArray := make(pdf.Array, len(names))
	seen := make(map[pdf.Name]bool)
	for i, name := range names {
		if name == "None" {
			continue
		}
		if name == "All" {
			return nil, errors.New("DeviceN: invalid colorant name")
		}
		if seen[name] {
			return nil, errors.New("DeviceN: duplicate colorant name")
		}
		seen[name] = true
		namesArray[i] = name
	}

	if alternate == nil || IsSpecial(alternate) {
		return nil, errors.New("DeviceN: invalid alternate color space")
	}

	nIn, nOut := trfm.Shape()
	if nIn != len(names) || nOut != alternate.Channels() {
		return nil, errors.New("DeviceN: invalid transformation function")
	}

	if attr != nil {
		for key := range attr {
			switch key {
			case "Subtype", "Colorants", "Process", "MixingHints", "Order":
				// pass
			default:
				return nil, fmt.Errorf("DeviceN: invalid attribute key %s", key)
			}
		}
		subtype := attr["Subtype"]
		if subtype != nil && subtype != pdf.Name("NChannel") && subtype != pdf.Name("DeviceN") {
			return nil, fmt.Errorf("DeviceN: invalid subtype %q", subtype)
		}
	}

	return &SpaceDeviceN{
		colorants: namesArray,
		alternate: alternate,
		trfm:      trfm,
		attr:      attr,
	}, nil
}

// Family returns /DeviceN.
// This implements the [Space] interface.
func (s *SpaceDeviceN) Family() pdf.Name {
	return FamilyDeviceN
}

// Channels returns the dimensionality of the color space.
func (s *SpaceDeviceN) Channels() int {
	return len(s.colorants)
}

// Embed adds the color space to a PDF file.
// This implements the pdf.Embedder interface.
func (s *SpaceDeviceN) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {

	if err := pdf.CheckVersion(rm.Out(), "DeviceN color space", pdf.V1_3); err != nil {
		return nil, err
	}

	alt, err := rm.Embed(s.alternate)
	if err != nil {
		return nil, err
	}

	trfm, err := rm.Embed(s.trfm)
	if err != nil {
		return nil, err
	}

	var res pdf.Array
	if s.attr == nil {
		res = pdf.Array{
			FamilyDeviceN,
			s.colorants,
			alt,
			trfm,
		}
	} else {
		res = pdf.Array{
			FamilyDeviceN,
			s.colorants,
			alt,
			trfm,
			s.attr,
		}
	}
	return res, nil
}

// Default returns the default color of the color space.
func (s *SpaceDeviceN) Default() Color {
	return s.New(make([]float64, s.Channels()))
}

// New returns a new color in the color space.
func (s *SpaceDeviceN) New(x []float64) Color {
	n := s.Channels()

	if len(x) != s.Channels() {
		panic("invalid number of color components")
	}

	buf := make([]byte, 0, 8*n)
	for _, v := range x {
		bits := math.Float64bits(v)
		buf = binary.LittleEndian.AppendUint64(buf, bits)
	}

	return colorDeviceN{Space: s, data: string(buf)}
}

type colorDeviceN struct {
	Space *SpaceDeviceN

	// data encoded the color components as a string, so that comparisons
	// using the "==" operator are possible.
	data string
}

// ColorSpace returns the color space of the color.
// This implements the [Color] interface.
func (c colorDeviceN) ColorSpace() Space {
	return c.Space
}

func (c *colorDeviceN) set(x []float64) {
	buf := make([]byte, 0, 8*len(x))
	for _, v := range x {
		bits := math.Float64bits(v)
		buf = binary.LittleEndian.AppendUint64(buf, bits)
	}
	c.data = string(buf)
}

func (c colorDeviceN) get() []float64 {
	n := c.Space.Channels()
	x := make([]float64, n)
	for i := 0; i < n; i++ {
		bits := binary.LittleEndian.Uint64([]byte(c.data[i*8 : (i+1)*8]))
		x[i] = math.Float64frombits(bits)
	}
	return x
}
