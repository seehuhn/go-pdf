// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package jpx

import (
	"math"

	"seehuhn.de/go/icc"
	"seehuhn.de/go/jpx"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/cie"
)

// extendedLabRanges gives the a* and b* ranges of [labExtended].  The
// ranges enclose the colours of the e-sRGB cube, for all bit depths, and
// of the e-sYCC and sYCC domains Y′ ∈ [0, 1], Cb′, Cr′ ∈ [−0.5, 0.5].
// The bounds were found by sampling the e-sRGB and e-sYCC domains on a
// dense grid and widened by about 1%.
var extendedLabRanges = []float64{-4423, 1191, -849, 5133}

// labExtended is the colour space for e-sRGB, e-sYCC and sYCC samples.  Lab
// holds the colours outside the sRGB gamut which these encodings can
// represent, and can be used in every PDF version supporting JPXDecode.
var labExtended = func() *color.SpaceLab {
	cs, err := color.Lab(color.WhitePointD50, nil, extendedLabRanges)
	if err != nil {
		panic(err)
	}
	return cs
}()

// illuminants maps the illuminant codes of the CIELab enumerated colour
// space to their CIE 1931 XYZ white points, normalised to Y = 1.
var illuminants = map[uint32][]float64{
	jpx.IlluminantD50: color.WhitePointD50,
	jpx.IlluminantD65: color.WhitePointD65,
	jpx.IlluminantD75: xyToXYZ(0.29902, 0.31485),
	jpx.IlluminantSA:  xyToXYZ(0.44757, 0.40745),
	jpx.IlluminantSC:  xyToXYZ(0.31006, 0.31616),
	jpx.IlluminantF2:  xyToXYZ(0.37208, 0.37529),
	jpx.IlluminantF7:  xyToXYZ(0.31292, 0.32933),
	jpx.IlluminantF11: xyToXYZ(0.38052, 0.37713),
}

// xyToXYZ returns the XYZ coordinates with Y = 1 of the chromaticity
// (x, y).
func xyToXYZ(x, y float64) []float64 {
	return []float64{x / y, 1, (1 - x - y) / y}
}

// maxCode returns the largest unsigned sample value of depth n.
func maxCode(n int) float64 {
	return math.Ldexp(1, n) - 1
}

// resolveLab describes the CIELab enumerated colour space for channels
// with the given depths.  The second return value is false if the
// parameters are malformed, the illuminant is unknown, or the ranges
// cannot be represented by a PDF Lab colour space.
func resolveLab(s jpx.ColorSpec, depths []int) (ColorSpace, bool) {
	p, ok := s.LabParams([3]int(depths))
	if !ok {
		return ColorSpace{}, false
	}

	var wp []float64
	if kelvin, isCT := p.ColorTemperature(); isCT {
		var err error
		wp, err = color.WhitePointDaylight(float64(kelvin))
		if err != nil {
			return ColorSpace{}, false
		}
	} else if wp, ok = illuminants[p.IL]; !ok {
		return ColorSpace{}, false
	}

	// v ↦ (v-o)·r/(2^n-1), at the end points v = 0 and v = 2^n-1
	decode := make([]float64, 6)
	param := [3][2]uint32{{p.RL, p.OL}, {p.RA, p.OA}, {p.RB, p.OB}}
	for i, n := range depths {
		m := maxCode(n)
		r, o := float64(param[i][0]), float64(param[i][1])
		decode[2*i] = -o * r / m
		decode[2*i+1] = (m - o) * r / m
	}

	cs, err := color.Lab(wp, nil, decode[2:])
	if err != nil {
		return ColorSpace{}, false
	}
	return ColorSpace{Space: cs, Decode: decode}, true
}

// resolveESRGB describes the e-sRGB enumerated colour space for channels
// with the given depths.  A sample v of depth n encodes the non-linear
// value (v - 2^(n-2) - 2^(n-3)) / (255·2^(n-9)).
func resolveESRGB(depths []int) ColorSpace {
	decode := make([]float64, 6)
	for i, n := range depths {
		offset := math.Ldexp(3, n-3)
		scale := math.Ldexp(255, n-9)
		decode[2*i] = -offset / scale
		decode[2*i+1] = (maxCode(n) - offset) / scale
	}
	return ColorSpace{
		Space:   labExtended,
		Decode:  decode,
		Convert: esrgbToLab,
	}
}

// resolveESYCC describes the e-sYCC enumerated colour space for channels
// with the given depths.  A sample v of depth n encodes Y′ = v / (2^n-1)
// for the luma channel and Cb′, Cr′ = (v - 2^(n-1)) / (2^n-1) for the
// chroma channels.
func resolveESYCC(depths []int) ColorSpace {
	return resolveYCC(depths, yccToESRGB)
}

// resolveSYCC describes the sYCC enumerated colour space for channels
// with the given depths.  The samples are coded as for e-sYCC, but are
// converted to sRGB values by the full-range BT.601 inverse.  Colours
// outside the sRGB gamut, which sYCC can encode, are kept.
func resolveSYCC(depths []int) ColorSpace {
	return resolveYCC(depths, yccToRGB)
}

// resolveYCC describes a luma/chroma encoding whose values are turned
// into non-linear RGB by toRGB, and then into [labExtended].
func resolveYCC(depths []int, toRGB func(v []float64)) ColorSpace {
	decode := []float64{0, 1, 0, 0, 0, 0}
	for i := 1; i < 3; i++ {
		m := maxCode(depths[i])
		half := math.Ldexp(1, depths[i]-1)
		decode[2*i] = -half / m
		decode[2*i+1] = (m - half) / m
	}
	return ColorSpace{
		Space:  labExtended,
		Decode: decode,
		Convert: func(v []float64) {
			toRGB(v)
			esrgbToLab(v)
		},
	}
}

// yccToRGB converts sYCC values Y′, Cb′, Cr′ in place to the non-linear
// RGB values R′, G′, B′, which may lie outside [0, 1].
func yccToRGB(v []float64) {
	y, cb, cr := v[0], v[1], v[2]
	v[0] = y + 1.402*cr
	v[1] = y - 0.344136*cb - 0.714136*cr
	v[2] = y + 1.772*cb
}

// yccToESRGB converts e-sYCC values Y′, Cb′, Cr′ in place to the
// non-linear e-sRGB values R′, G′, B′.
func yccToESRGB(v []float64) {
	y, cb, cr := v[0], v[1], v[2]
	v[0] = y + 2.804*cr
	v[1] = y - 0.6882*cb - 1.4282*cr
	v[2] = y + 3.5440*cb + 0.0003*cr
}

// esrgbToLab converts non-linear e-sRGB values R′, G′, B′ in place to the
// components of [labExtended].
func esrgbToLab(v []float64) {
	X, Y, Z := cie.ExtendedSRGBToXYZ(v[0], v[1], v[2])
	labExtended.FromXYZ(X, Y, Z, icc.RelativeColorimetric, v, &icc.Workspace{})
}
