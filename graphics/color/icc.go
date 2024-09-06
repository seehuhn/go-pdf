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

	"seehuhn.de/go/icc"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/metadata"
)

// SpaceICCBased represents an ICC-based color space.
type SpaceICCBased struct {
	N      int
	Ranges []float64

	metadata *metadata.Stream
	profile  []byte
	def      []float64
}

// ICCBased returns a new ICC-based color space.
//
// TODO(voss): extract n and ranges from the profile data.
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

// ColorSpaceFamily returns /ICCBased.
// This implements the [Space] interface.
func (s *SpaceICCBased) ColorSpaceFamily() pdf.Name {
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
func (s *SpaceICCBased) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	w := rm.Out
	if err := pdf.CheckVersion(w, "ICCBased color space", pdf.V1_3); err != nil {
		return nil, zero, err
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
		mRef, _, err := pdf.ResourceManagerEmbed(rm, s.metadata)
		if err != nil {
			return nil, zero, err
		}
		dict["Metadata"] = mRef
	}

	sRef := w.Alloc()
	body, err := w.OpenStream(sRef, dict, pdf.FilterFlate{})
	if err != nil {
		return nil, zero, err
	}
	_, err = body.Write(s.profile)
	if err != nil {
		return nil, zero, err
	}
	err = body.Close()
	if err != nil {
		return nil, zero, err
	}

	return pdf.Array{FamilyICCBased, sRef}, zero, nil
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
