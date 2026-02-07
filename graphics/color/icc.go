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
	stdcolor "image/color"
	"math"
	"sync"

	"seehuhn.de/go/icc"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/metadata"
)

// PDF 2.0 sections: 8.6.5.5

// SpaceICCBased represents an ICC-based color space.
type SpaceICCBased struct {
	N      int
	Ranges []float64

	metadata *metadata.Stream
	profile  []byte
	def      []float64

	// cached transform for Convert() (PCSToDevice)
	transformOnce sync.Once
	transform     *icc.Transform

	// cached transform for RGBA() (DeviceToPCS)
	fwdTransformOnce sync.Once
	fwdTransform     *icc.Transform
}

// ICCBased returns a new ICC-based color space.
func ICCBased(profile []byte, metadata *metadata.Stream) (*SpaceICCBased, error) {
	if len(profile) == 0 {
		return nil, errors.New("ICCBased: missing profile")
	}

	p, err := icc.Decode(profile)
	if err != nil {
		return nil, err
	}

	n := p.ColorSpace.NumComponents()
	if n != 1 && n != 3 && n != 4 {
		return nil, fmt.Errorf("ICCBased: invalid number of components %d", n)
	}

	var ranges []float64
	// TODO(voss): revisit this once
	// https://github.com/pdf-association/pdf-issues/issues/31 is resolved.
	switch p.ColorSpace {
	case icc.GraySpace:
		ranges = []float64{0, 1}
	case icc.RGBSpace:
		ranges = []float64{0, 1, 0, 1, 0, 1}
	case icc.CMYKSpace:
		ranges = []float64{0, 1, 0, 1, 0, 1, 0, 1}
	case icc.CIELabSpace:
		ranges = []float64{0, 100, -128, 127, -128, 127}
	default:
		return nil, fmt.Errorf("ICCBased: unsupported color space %v", p.ColorSpace)
	}

	def := make([]float64, n)

	res := &SpaceICCBased{
		N:        n,
		Ranges:   ranges,
		metadata: metadata,
		profile:  profile,
		def:      def,
	}
	return res, nil
}

// Family returns /ICCBased.
// This implements the [Space] interface.
func (s *SpaceICCBased) Family() pdf.Name {
	return FamilyICCBased
}

// Channels returns the number of color channels.
// This implements the [Space] interface.
func (s *SpaceICCBased) Channels() int {
	return s.N
}

// Default returns the default color in an ICC-based color space.
func (s *SpaceICCBased) Default() Color {
	c := colorICCBased{Space: s}
	copy(c.Values[:], s.def)
	return c
}

// Embed adds the color space to a PDF file.
// This implements the [Space] interface.
func (s *SpaceICCBased) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	w := rm.Out()
	if err := pdf.CheckVersion(w, "ICCBased color space", pdf.V1_3); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"N": pdf.Integer(s.N),
	}

	needsRange := false
	for i := range s.N {
		if math.Abs(s.Ranges[2*i]-0) >= ε || math.Abs(s.Ranges[2*i+1]-1) >= ε {
			needsRange = true
			break
		}
	}
	if needsRange {
		dict["Range"] = toPDF(s.Ranges)
	}

	if s.metadata != nil {
		if err := pdf.CheckVersion(w, "ICCBased Metadata", pdf.V1_4); err != nil {
			return nil, err
		}
		mRef, err := rm.Embed(s.metadata)
		if err != nil {
			return nil, err
		}
		dict["Metadata"] = mRef
	}

	sRef := w.Alloc()
	body, err := w.OpenStream(sRef, dict, pdf.FilterFlate{})
	if err != nil {
		return nil, err
	}
	_, err = body.Write(s.profile)
	if err != nil {
		return nil, err
	}
	err = body.Close()
	if err != nil {
		return nil, err
	}

	return pdf.Array{FamilyICCBased, sRef}, nil
}

// Convert converts a color to the ICC-based color space.
// This implements the [stdcolor.Model] interface.
func (s *SpaceICCBased) Convert(c stdcolor.Color) stdcolor.Color {
	// fast path: already this ICC space
	if ic, ok := c.(colorICCBased); ok && ic.Space == s {
		return ic
	}

	// get XYZ from input colour (assumed sRGB)
	X, Y, Z := colorToXYZ(c)

	// initialise transform on first use
	s.transformOnce.Do(func() {
		p, err := icc.Decode(s.profile)
		if err != nil {
			return
		}
		s.transform, _ = icc.NewTransform(p, icc.PCSToDevice, icc.RelativeColorimetric)
	})

	var values []float64
	if s.transform != nil {
		values = s.transform.FromXYZ(X, Y, Z)
	} else {
		// fallback if transform not available
		values = s.fallbackFromXYZ(X, Y, Z)
	}

	result := colorICCBased{Space: s}
	for i := 0; i < s.N && i < len(values); i++ {
		// scale to the space's ranges
		min, max := s.Ranges[2*i], s.Ranges[2*i+1]
		result.Values[i] = clamp(values[i]*(max-min)+min, min, max)
	}
	return result
}

func (s *SpaceICCBased) fallbackFromXYZ(X, Y, Z float64) []float64 {
	// simple fallback: convert XYZ to sRGB-like values
	r, g, b := xyzToSRGB(X, Y, Z)
	switch s.N {
	case 1:
		// grayscale: use luminance
		return []float64{0.299*r + 0.587*g + 0.114*b}
	case 3:
		return []float64{r, g, b}
	case 4:
		// CMYK approximation
		cyan := 1 - r
		magenta := 1 - g
		yellow := 1 - b
		k := min(cyan, min(magenta, yellow))
		if k >= 1 {
			return []float64{0, 0, 0, 1}
		}
		return []float64{
			(cyan - k) / (1 - k),
			(magenta - k) / (1 - k),
			(yellow - k) / (1 - k),
			k,
		}
	default:
		return make([]float64, s.N)
	}
}

// New returns a new color in the ICC-based color space.
func (s *SpaceICCBased) New(values []float64) (Color, error) {
	if len(values) != s.N {
		return nil, fmt.Errorf("ICCBased: invalid number of components %d", len(values))
	}
	for i := range s.N {
		if values[i] < s.Ranges[2*i] || values[i] > s.Ranges[2*i+1] {
			return nil, fmt.Errorf("ICCBased: value out of range")
		}
	}

	c := colorICCBased{Space: s}
	copy(c.Values[:], values)
	return c, nil
}

type colorICCBased struct {
	Space  *SpaceICCBased
	Values [4]float64
}

func (c colorICCBased) ColorSpace() Space {
	return c.Space
}

// Components returns the colour component values.
func (c colorICCBased) Components() []float64 {
	return c.Values[:c.Space.N]
}

// ToXYZ returns the colour as CIE XYZ tristimulus values
// adapted to the D50 illuminant.
// It uses the embedded ICC profile when available, otherwise falls back
// to a naive conversion based on the number of components.
func (c colorICCBased) ToXYZ() (X, Y, Z float64) {
	// normalise values from Ranges to [0,1]
	norm := make([]float64, c.Space.N)
	for i := range c.Space.N {
		lo, hi := c.Space.Ranges[2*i], c.Space.Ranges[2*i+1]
		if hi > lo {
			norm[i] = (c.Values[i] - lo) / (hi - lo)
		} else {
			norm[i] = 0
		}
	}

	// try ICC profile transform
	c.Space.fwdTransformOnce.Do(func() {
		p, err := icc.Decode(c.Space.profile)
		if err != nil {
			return
		}
		c.Space.fwdTransform, _ = icc.NewTransform(p, icc.DeviceToPCS, icc.Perceptual)
	})
	if c.Space.fwdTransform != nil {
		return c.Space.fwdTransform.ToXYZ(norm)
	}

	// fallback without profile: treat as sRGB-like
	switch c.Space.N {
	case 1:
		return srgbToXYZ(norm[0], norm[0], norm[0])
	case 3:
		return srgbToXYZ(norm[0], norm[1], norm[2])
	case 4:
		cyan, magenta, yellow, black := norm[0], norm[1], norm[2], norm[3]
		rf := (1 - cyan) * (1 - black)
		gf := (1 - magenta) * (1 - black)
		bf := (1 - yellow) * (1 - black)
		return srgbToXYZ(rf, gf, bf)
	default:
		return srgbToXYZ(0.5, 0.5, 0.5)
	}
}

// RGBA implements the color.Color interface.
func (c colorICCBased) RGBA() (r, g, b, a uint32) {
	X, Y, Z := c.ToXYZ()
	rf, gf, bf := xyzToSRGB(X, Y, Z)
	return toUint32(rf), toUint32(gf), toUint32(bf), 0xffff
}
