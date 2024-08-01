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
	"seehuhn.de/go/pdf/metadata"
)

// SpaceICCBased represents an ICC-based color space.
type SpaceICCBased struct {
	n        int
	ranges   []float64
	metadata *metadata.Stream
	profile  []byte
	def      []float64
}

// ICCBased returns a new ICC-based color space.
//
// TODO(voss): extract n and ranges from the profile data.
func ICCBased(n int, profile []byte, ranges []float64, metadata *metadata.Stream) (*SpaceICCBased, error) {
	if n != 1 && n != 3 && n != 4 {
		return nil, fmt.Errorf("ICCBased: invalid number of components %d", n)
	}
	if len(profile) == 0 {
		return nil, errors.New("ICCBased: missing profile")
	}
	if ranges == nil {
		ranges = make([]float64, 0, 2*n)
		for range n {
			ranges = append(ranges, 0, 1)
		}
	} else {
		if len(ranges) != 2*n {
			return nil, fmt.Errorf("ICCBased: invalid ranges")
		}
		for i := range n {
			if ranges[2*i] > ranges[2*i+1] {
				return nil, fmt.Errorf("ICCBased: invalid ranges")
			}
		}
	}

	def := make([]float64, n)
	for i := range n {
		x := 0.0
		if x < ranges[2*i] {
			x = ranges[2*i]
		} else if x > ranges[2*i+1] {
			x = ranges[2*i+1]
		}
		def[i] = x
	}

	res := &SpaceICCBased{
		n:        n,
		ranges:   ranges,
		metadata: metadata,
		profile:  profile,
		def:      def,
	}
	return res, nil
}

// New returns a new color in the ICC-based color space.
func (s *SpaceICCBased) New(values []float64) (Color, error) {
	if len(values) != s.n {
		return nil, fmt.Errorf("ICCBased: invalid number of components %d", len(values))
	}
	for i := range s.n {
		if values[i] < s.ranges[2*i] || values[i] > s.ranges[2*i+1] {
			return nil, fmt.Errorf("ICCBased: value out of range")
		}
	}
	return colorICCBased{s, values}, nil
}

// ColorSpaceFamily implements the [Space] interface.
func (s *SpaceICCBased) ColorSpaceFamily() pdf.Name {
	return "ICCBased"
}

// Embed implements the [Space] interface.
func (s *SpaceICCBased) Embed(rm *pdf.ResourceManager) (pdf.Object, pdf.Unused, error) {
	var zero pdf.Unused

	w := rm.Out
	if err := pdf.CheckVersion(w, "ICCBased color space", pdf.V1_3); err != nil {
		return nil, zero, err
	}

	dict := pdf.Dict{
		"N": pdf.Integer(s.n),
	}

	needsRange := false
	for i := range s.n {
		if math.Abs(s.ranges[2*i]-0) >= ε || math.Abs(s.ranges[2*i+1]-1) >= ε {
			needsRange = true
			break
		}
	}
	if needsRange {
		dict["Range"] = toPDF(s.ranges)
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

func (s *SpaceICCBased) defaultValues() []float64 {
	return s.def
}

type colorICCBased struct {
	Space *SpaceICCBased
	val   []float64
}

func (c colorICCBased) ColorSpace() Space {
	return c.Space
}

func (c colorICCBased) values() []float64 {
	return c.val
}
