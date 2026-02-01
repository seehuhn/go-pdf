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
	stdcolor "image/color"
	"math"
	"slices"

	"seehuhn.de/go/pdf"
)

// == Indexed ================================================================

// PDF 2.0 sections: 8.6.6.3

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

// Convert converts a color to the indexed color space by finding the
// nearest palette entry.
// This implements the [stdcolor.Model] interface.
func (s *SpaceIndexed) Convert(c stdcolor.Color) stdcolor.Color {
	// fast path: already this indexed space
	if ci, ok := c.(colorIndexed); ok && ci.Space == s {
		return ci
	}

	// find nearest palette entry by RGB distance
	r32, g32, b32, _ := c.RGBA()
	r := float64(r32) / 65535.0
	g := float64(g32) / 65535.0
	b := float64(b32) / 65535.0

	bestIdx := 0
	bestDist := math.MaxFloat64

	for idx := 0; idx < s.NumCol; idx++ {
		palColor := colorIndexed{Space: s, Index: idx}
		pr32, pg32, pb32, _ := palColor.RGBA()
		pr := float64(pr32) / 65535.0
		pg := float64(pg32) / 65535.0
		pb := float64(pb32) / 65535.0

		// squared RGB distance
		dist := (r-pr)*(r-pr) + (g-pg)*(g-pg) + (b-pb)*(b-pb)
		if dist < bestDist {
			bestDist = dist
			bestIdx = idx
		}
	}

	return colorIndexed{Space: s, Index: bestIdx}
}

type colorIndexed struct {
	Space *SpaceIndexed
	Index int
}

func (c colorIndexed) ColorSpace() Space {
	return c.Space
}

// RGBA implements the color.Color interface.
func (c colorIndexed) RGBA() (r, g, b, a uint32) {
	baseColor := c.getBaseColor()
	return baseColor.RGBA()
}

// getBaseColor returns the color from the palette at the given index.
func (c colorIndexed) getBaseColor() Color {
	base := c.Space.Base
	n := base.Channels()
	offset := c.Index * n

	if offset+n > len(c.Space.lookup) {
		return base.Default()
	}

	// decode the lookup table bytes to color values
	var min, max []float64
	switch space := base.(type) {
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
	default:
		return base.Default()
	}

	vals := make([]float64, n)
	for i := range n {
		b := float64(c.Space.lookup[offset+i])
		vals[i] = min[i] + (b/255.0)*(max[i]-min[i])
	}

	// construct a color in the base space
	switch space := base.(type) {
	case spaceDeviceGray:
		return DeviceGray(vals[0])
	case *SpaceCalGray:
		return space.New(vals[0])
	case spaceDeviceRGB:
		return DeviceRGB{vals[0], vals[1], vals[2]}
	case *SpaceCalRGB:
		return space.New(vals[0], vals[1], vals[2])
	case spaceDeviceCMYK:
		return DeviceCMYK{vals[0], vals[1], vals[2], vals[3]}
	case *SpaceLab:
		col, _ := space.New(vals[0], vals[1], vals[2])
		if col == nil {
			return base.Default()
		}
		return col
	default:
		return base.Default()
	}
}

// == Separation =============================================================

// PDF 2.0 sections: 8.6.6.4

// SpaceSeparation represents a Separation color space.
//
// A Separation color space provides a means for specifying the use of
// additional colorants or for isolating the control of individual color
// components of a device color space for a subtractive device.
//
// Use the [Separation] function to create a new Separation color space.
//
// See section 8.6.6.4 of ISO 32000-2:2020.
type SpaceSeparation struct {
	// Colorant specifies the name of the colorant that this Separation
	// color space represents. This can be any name, including the special
	// names All (all device colorants) and None (no visible output).
	// The names Cyan, Magenta, Yellow, and Black are reserved for the
	// components of a CMYK process color space.
	Colorant pdf.Name

	// Alternate is the alternate color space used when the device does
	// not have a colorant corresponding to Colorant. It may be any device
	// or CIE-based color space but not a special color space (Pattern,
	// Indexed, Separation, or DeviceN).
	Alternate Space

	// Transform is a function that maps tint values (0.0 to 1.0) to color
	// component values in the alternate color space. A tint of 0.0 produces
	// the lightest color (no colorant); 1.0 produces the darkest (full colorant).
	Transform pdf.Function
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
		Colorant:  colorant,
		Alternate: alternate,
		Transform: trfm,
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
// This implements the [pdf.Embedder] interface.
func (s *SpaceSeparation) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out(), "Separation color space", pdf.V1_2); err != nil {
		return nil, err
	}

	alt, err := rm.Embed(s.Alternate)
	if err != nil {
		return nil, err
	}
	trfm, err := rm.Embed(s.Transform)
	if err != nil {
		return nil, err
	}

	return pdf.Array{FamilySeparation, s.Colorant, alt, trfm}, nil
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

// Convert converts a color to the separation color space.
// Since separation spaces represent specific inks, this uses a luminance-based
// heuristic to estimate the tint value.
// This implements the [stdcolor.Model] interface.
func (s *SpaceSeparation) Convert(c stdcolor.Color) stdcolor.Color {
	// fast path: already this separation space
	if cs, ok := c.(colorSeparation); ok && cs.Space == s {
		return cs
	}

	// use luminance as inverse tint (lighter = less ink)
	r32, g32, b32, _ := c.RGBA()
	r := float64(r32) / 65535.0
	g := float64(g32) / 65535.0
	b := float64(b32) / 65535.0
	lum := 0.299*r + 0.587*g + 0.114*b

	// tint: 0 = no ink (light), 1 = full ink (dark)
	tint := clamp01(1 - lum)
	return colorSeparation{Space: s, Tint: tint}
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

// RGBA implements the color.Color interface.
func (c colorSeparation) RGBA() (r, g, b, a uint32) {
	// apply the transform function to get alternate color values
	altValues := c.Space.Transform.Apply(c.Tint)
	altColor := SCN(c.Space.Alternate.Default(), altValues, nil)
	return altColor.RGBA()
}

// == DeviceN ================================================================

// PDF 2.0 sections: 8.6.6.5

// SpaceDeviceN represents a DeviceN color space.
//
// DeviceN color spaces may contain an arbitrary number of color components,
// providing greater flexibility than standard device color spaces or individual
// Separation color spaces. They are used for high-fidelity color (e.g., PANTONE
// Hexachrome with six colorants) and multitone color systems (e.g., duotone).
//
// Use the [DeviceN] function to create a new DeviceN color space.
//
// See section 8.6.6.5 of ISO 32000-2:2020.
type SpaceDeviceN struct {
	// Colorants specifies the names of the individual color components.
	// Names must be unique except for "None" which may repeat.
	// The special name "All" is not allowed. The names "Cyan", "Magenta",
	// "Yellow", and "Black" are reserved for CMYK process colorants.
	Colorants []pdf.Name

	// Alternate is the alternate color space used when any colorant is not
	// available on the device. It may be any device or CIE-based color space
	// but not a special color space (Pattern, Indexed, Separation, or DeviceN).
	Alternate Space

	// Transform is a function that maps n tint values (one per colorant) to
	// m color component values in the alternate color space. Tint values range
	// from 0.0 (minimum/no colorant) to 1.0 (maximum/full colorant).
	Transform pdf.Function

	// Attributes is an optional dictionary containing additional information
	// about the color space components (Subtype, Colorants, Process, MixingHints).
	// If Subtype is "NChannel", additional entries are required.
	Attributes pdf.Dict
}

// DeviceN returns a new DeviceN color space.

// DeviceN returns a new DeviceN color space with the given component names,
// alternate color space, tint transformation function, and attributes
// dictionary (optional).
func DeviceN(names []pdf.Name, alternate Space, trfm pdf.Function, attr pdf.Dict) (*SpaceDeviceN, error) {
	seen := make(map[pdf.Name]bool)
	for _, name := range names {
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
		Colorants:  slices.Clone(names),
		Alternate:  alternate,
		Transform:  trfm,
		Attributes: attr,
	}, nil
}

// Family returns /DeviceN.
// This implements the [Space] interface.
func (s *SpaceDeviceN) Family() pdf.Name {
	return FamilyDeviceN
}

// Channels returns the dimensionality of the color space.
func (s *SpaceDeviceN) Channels() int {
	return len(s.Colorants)
}

// Embed adds the color space to a PDF file.
// This implements the [pdf.Embedder] interface.
func (s *SpaceDeviceN) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out(), "DeviceN color space", pdf.V1_3); err != nil {
		return nil, err
	}

	alt, err := rm.Embed(s.Alternate)
	if err != nil {
		return nil, err
	}

	trfm, err := rm.Embed(s.Transform)
	if err != nil {
		return nil, err
	}

	names := make(pdf.Array, len(s.Colorants))
	for i, name := range s.Colorants {
		names[i] = name
	}

	var res pdf.Array
	if s.Attributes == nil {
		res = pdf.Array{
			FamilyDeviceN,
			names,
			alt,
			trfm,
		}
	} else {
		res = pdf.Array{
			FamilyDeviceN,
			names,
			alt,
			trfm,
			s.Attributes,
		}
	}
	return res, nil
}

// Default returns the default color of the color space.
func (s *SpaceDeviceN) Default() Color {
	return s.New(make([]float64, s.Channels()))
}

// Convert converts a color to the DeviceN color space.
// This uses a luminance-based heuristic since DeviceN spaces can represent
// arbitrary colorants.
// This implements the [stdcolor.Model] interface.
func (s *SpaceDeviceN) Convert(c stdcolor.Color) stdcolor.Color {
	// fast path: already this DeviceN space
	if cd, ok := c.(colorDeviceN); ok && cd.Space == s {
		return cd
	}

	// use luminance as inverse tint for all channels
	r32, g32, b32, _ := c.RGBA()
	r := float64(r32) / 65535.0
	g := float64(g32) / 65535.0
	b := float64(b32) / 65535.0
	lum := 0.299*r + 0.587*g + 0.114*b
	tint := clamp01(1 - lum)

	values := make([]float64, s.Channels())
	for i := range values {
		values[i] = tint
	}
	return s.New(values)
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
	for i := range n {
		bits := binary.LittleEndian.Uint64([]byte(c.data[i*8 : (i+1)*8]))
		x[i] = math.Float64frombits(bits)
	}
	return x
}

// RGBA implements the color.Color interface.
func (c colorDeviceN) RGBA() (r, g, b, a uint32) {
	// apply the transform function to get alternate color values
	tints := c.get()
	altValues := c.Space.Transform.Apply(tints...)
	altColor := SCN(c.Space.Alternate.Default(), altValues, nil)
	return altColor.RGBA()
}
