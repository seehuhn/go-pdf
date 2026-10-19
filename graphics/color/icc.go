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
	"seehuhn.de/go/pdf/internal/cie"
)

// PDF 2.0 sections: 8.6.5.5

// SpaceICCBased represents an ICC-based color space.
type SpaceICCBased struct {
	N        int
	Ranges   []float64
	Profile  *icc.Profile
	Metadata *pdf.MetadataStream

	def []float64

	// cached bidirectional transforms for ToXYZ/FromXYZ, by rendering intent
	xf [4]struct {
		once sync.Once
		t    *icc.Transform
	}
}

// ICCBased returns a new ICC-based color space.
func ICCBased(profile []byte, metadata *pdf.MetadataStream) (*SpaceICCBased, error) {
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
		Metadata: metadata,
		Profile:  p,
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

// ComponentRange returns the value range of component i, as stored in the
// Range entry of the ICC-based color space.
// This implements the [Space] interface.
func (s *SpaceICCBased) ComponentRange(i int) (lo, hi float64) {
	return s.Ranges[2*i], s.Ranges[2*i+1]
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

	err := pdf.CheckVersion(w, "ICC profile version "+s.Profile.Version.String(),
		minPDFVersion(s.Profile.Version))
	if err != nil {
		return nil, err
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

	if s.Metadata != nil {
		if err := pdf.CheckVersion(w, "ICCBased Metadata", pdf.V1_4); err != nil {
			return nil, err
		}
		mRef, err := rm.Embed(s.Metadata)
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
	profileBytes, err := s.Profile.Encode()
	if err != nil {
		return nil, err
	}
	_, err = body.Write(profileBytes)
	if err != nil {
		return nil, err
	}
	err = body.Close()
	if err != nil {
		return nil, err
	}

	return pdf.Array{FamilyICCBased, sRef}, nil
}

// SpaceROMMRGB is the ROMM RGB (ProPhoto RGB) colour space, an ICCBased
// colour space with an ICC version 4 profile.  Embedding it requires
// PDF 1.7 or later; for older files, construct the space from
// [icc.ROMMv2Profile] using [ICCBased].
var SpaceROMMRGB = mustICCBased(icc.ROMMv4Profile)

// mustICCBased returns the ICC-based colour space for a built-in profile.
func mustICCBased(profile []byte) *SpaceICCBased {
	s, err := ICCBased(profile, nil)
	if err != nil {
		panic(err)
	}
	return s
}

// minPDFVersion returns the first PDF version which allows ICC profiles
// of the given version to be embedded.  PDF 2.0 allows profiles of any
// ICC version; earlier PDF versions each support the ICC versions
// current at the time.
func minPDFVersion(v icc.Version) pdf.Version {
	switch {
	case v>>24 == 2:
		return pdf.V1_3
	case v>>24 == 4 && v < icc.Version4_1_0:
		return pdf.V1_5
	case v>>24 == 4 && v < icc.Version4_2_0:
		return pdf.V1_6
	case v>>24 == 4 && v < icc.Version4_3_0:
		return pdf.V1_7
	}
	return pdf.V2_0
}

// Transform returns the cached bidirectional transform for the profile and
// the rendering intent ri, or nil if the profile cannot be turned into a
// transform.  A rendering intent outside the four defined values is treated
// as [icc.RelativeColorimetric].
func (s *SpaceICCBased) Transform(ri icc.RenderingIntent) *icc.Transform {
	if ri > icc.AbsoluteColorimetric {
		ri = icc.RelativeColorimetric
	}
	e := &s.xf[ri]
	e.once.Do(func() {
		e.t, _ = icc.NewTransform(s.Profile, ri)
	})
	return e.t
}

// SourceBlackPoint returns the black point of the profile as the source of a
// conversion under ri, or zero if the profile is unusable.
// This implements the [Space] interface.
func (s *SpaceICCBased) SourceBlackPoint(ri icc.RenderingIntent) [3]float64 {
	if t := s.Transform(ri); t != nil {
		return t.SourceBlackPoint()
	}
	return [3]float64{}
}

// DestinationBlackPoint returns the black point of the profile as the
// destination of a conversion under ri, or zero if the profile is unusable.
// This implements the [BidirectionalSpace] interface.
func (s *SpaceICCBased) DestinationBlackPoint(ri icc.RenderingIntent) [3]float64 {
	if t := s.Transform(ri); t != nil {
		return t.DestinationBlackPoint()
	}
	return [3]float64{}
}

// FromXYZ converts PCS-adapted CIE XYZ to ICC-based component values.
//
// The result is written to dst, which must have space for N components.
//
// ws supplies reusable scratch buffers to avoid per-call allocation in a
// hot loop; it must be non-nil (the zero value &icc.Workspace{} is valid)
// and must not be used from more than one goroutine at a time.
func (s *SpaceICCBased) FromXYZ(X, Y, Z float64, ri icc.RenderingIntent, dst []float64, ws *icc.Workspace) {
	t := s.Transform(ri)
	if t != nil && t.CanFromXYZ() && t.Channels() == s.N {
		t.FromXYZ(X, Y, Z, dst[:s.N], ws)
	} else {
		s.fallbackFromXYZ(X, Y, Z, dst[:s.N])
	}

	for i := range s.N {
		lo, hi := s.Ranges[2*i], s.Ranges[2*i+1]
		dst[i] = ClipComponent(dst[i]*(hi-lo)+lo, lo, hi)
	}
}

// Convert converts a color to the ICC-based color space.
// This implements the [stdcolor.Model] interface.
func (s *SpaceICCBased) Convert(c stdcolor.Color) stdcolor.Color {
	// fast path: already this ICC space
	if ic, ok := c.(colorICCBased); ok && ic.Space == s {
		return ic
	}

	X, Y, Z := ColorToXYZ(c)

	result := colorICCBased{Space: s}
	s.FromXYZ(X, Y, Z, icc.RelativeColorimetric, result.Values[:s.N], &icc.Workspace{})
	return result
}

func (s *SpaceICCBased) fallbackFromXYZ(X, Y, Z float64, dst []float64) {
	// simple fallback: convert XYZ to sRGB-like values
	r, g, b := cie.XYZToSRGB(X, Y, Z)
	switch s.N {
	case 1:
		dst[0] = rgbToGray(r, g, b)
	case 3:
		dst[0], dst[1], dst[2] = r, g, b
	case 4:
		rgbToCMYK(r, g, b, dst)
	default:
		clear(dst)
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

// ToXYZ converts ICC-based color values to CIE XYZ tristimulus values
// adapted to the Profile Connection Space white point.
// Values outside Ranges are adjusted to the nearest valid value.
func (s *SpaceICCBased) ToXYZ(values []float64, ri icc.RenderingIntent, ws *icc.Workspace) (X, Y, Z float64) {
	// normalise values from Ranges to [0,1] using caller-provided scratch
	norm := ws.Scratch(slotNorm, s.N)
	for i := range s.N {
		norm[i] = 0
		lo, hi := s.Ranges[2*i], s.Ranges[2*i+1]
		if hi > lo {
			// clamping the normalised value adjusts a component outside
			// Ranges to the nearest value the colour space allows
			norm[i] = clip01((values[i] - lo) / (hi - lo))
		}
	}

	// try ICC profile transform
	if t := s.Transform(ri); t != nil && t.CanToXYZ() {
		return t.ToXYZ(norm, ws)
	}

	// fallback without profile: treat as sRGB-like
	switch s.N {
	case 1:
		return cie.SRGBToXYZ(norm[0], norm[0], norm[0])
	case 3:
		return cie.SRGBToXYZ(norm[0], norm[1], norm[2])
	case 4:
		cyan, magenta, yellow, black := norm[0], norm[1], norm[2], norm[3]
		rf := (1 - cyan) * (1 - black)
		gf := (1 - magenta) * (1 - black)
		bf := (1 - yellow) * (1 - black)
		return cie.SRGBToXYZ(rf, gf, bf)
	default:
		return cie.SRGBToXYZ(0.5, 0.5, 0.5)
	}
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
// adapted to the Profile Connection Space white point.
func (c colorICCBased) ToXYZ() (X, Y, Z float64) {
	return c.Space.ToXYZ(c.Values[:c.Space.N], icc.RelativeColorimetric, &icc.Workspace{})
}

// RGBA implements the color.Color interface.
func (c colorICCBased) RGBA() (r, g, b, a uint32) {
	X, Y, Z := c.ToXYZ()
	rf, gf, bf := cie.XYZToSRGB(X, Y, Z)
	return toUint32(rf), toUint32(gf), toUint32(bf), 0xffff
}
