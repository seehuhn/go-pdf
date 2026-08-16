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
	"math"
	"slices"

	"seehuhn.de/go/pdf"
)

func toPDF(x []float64) pdf.Array {
	res := make(pdf.Array, len(x))
	for i, xi := range x {
		res[i] = pdf.Number(xi)
	}
	return res
}

func isConst(x []float64, value float64) bool {
	for _, xi := range x {
		if math.Abs(xi-value) >= ε {
			return false
		}
	}
	return true
}

func isZero(x []float64) bool {
	return isConst(x, 0)
}

func isValues(x []float64, y ...float64) bool {
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		if math.Abs(x[i]-y[i]) >= ε {
			return false
		}
	}
	return true
}

// The predicates below define which parameters a CIE-based colour space can
// have.  They are the single description of validity in this package: the
// factory functions reject what they refuse, and the repair functions further
// down produce only values they accept.  Keeping the two sides on one
// definition is what stops the library from writing a file which its own
// reader would then repair.

func isValidWhitePoint(x []float64) bool {
	return len(x) == 3 && allFinite(x) &&
		x[0] > 0 && x[0] <= maxWhitePoint &&
		math.Abs(x[1]-1) <= ε &&
		x[2] > 0 && x[2] <= maxWhitePoint
}

func isValidBlackPoint(x []float64) bool {
	return len(x) == 3 && allFinite(x) &&
		x[0] >= 0 && x[1] >= 0 && x[2] >= 0
}

func isValidGamma(gamma float64) bool {
	return finite(gamma) && gamma > 0
}

func isValidGammaArray(x []float64) bool {
	if len(x) != 3 {
		return false
	}
	for _, gamma := range x {
		if !isValidGamma(gamma) {
			return false
		}
	}
	return true
}

// isValidMatrix reports whether a CalRGB matrix describes a usable colour
// space: it must map the unit cube to finite tristimulus values, and it must
// be invertible.
//
// A matrix which cannot be inverted collapses the colour space onto a plane or
// a line.  Such a space is well defined in the ToXYZ direction, but what it
// describes is unusable: an all-zero matrix renders every colour, white
// included, as black, and one with repeated columns renders every colour as
// the same saturated hue.
func isValidMatrix(x []float64) bool {
	return len(x) == 9 && allFinite(x) &&
		rowSumsFinite(x) && finite(1/matrixDet(x))
}

// rowSumsFinite reports whether a 3x3 matrix in column-major order maps the
// unit cube to finite values.  Entries close to the largest representable
// float64 pass every other test, but overflow when three of them are added
// up, and an infinity turns into a NaN in the chromatic adaptation which
// follows.
func rowSumsFinite(m []float64) bool {
	for i := range 3 {
		if !finite(math.Abs(m[i]) + math.Abs(m[3+i]) + math.Abs(m[6+i])) {
			return false
		}
	}
	return true
}

// matrixDet returns the determinant of a 3x3 matrix in column-major order.
func matrixDet(m []float64) float64 {
	return m[0]*(m[4]*m[8]-m[5]*m[7]) -
		m[3]*(m[1]*m[8]-m[2]*m[7]) +
		m[6]*(m[1]*m[5]-m[2]*m[4])
}

func isValidLabRanges(x []float64) bool {
	return len(x) == 4 && allFinite(x) &&
		x[0] < x[1] && x[2] < x[3] &&
		x[0] >= -maxLabRange && x[1] <= maxLabRange &&
		x[2] >= -maxLabRange && x[3] <= maxLabRange
}

// finite reports whether v is a finite number.
func finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// allFinite reports whether every entry of x is a finite number.
func allFinite(x []float64) bool {
	for _, v := range x {
		if !finite(v) {
			return false
		}
	}
	return true
}

// The repair functions below turn the parameters of a CIE-based colour space
// read from a file into values the corresponding factory function accepts.
// The factory functions stay strict, so that a caller cannot write an invalid
// file; a file which is already invalid is read as closely as the data allows.
//
// Each one is written as "keep what is already valid, otherwise mend it, and
// fall back to a value known to be valid", against the predicates above.  A
// repaired parameter therefore always satisfies the check its factory function
// applies, whatever the file contained.

// Bounds on the magnitude of the CIE-based parameters which scale a colour,
// beyond which a conversion overflows to an infinity and the chromatic
// adaptation which follows turns that into a NaN.
//
// Both are far outside the range of any usable colour space: a white point is
// a tristimulus normalised to Y=1, so X and Z lie near 1 for every real
// illuminant, and a* and b* stay within about ±128.
const (
	maxWhitePoint = 100
	maxLabRange   = 1e4
)

// repairWhitePoint returns a white point with positive X and Z of usable
// magnitude, and Y equal to 1.
//
// A white point whose Y differs from 1 is scaled by 1/Y, which preserves the
// chromaticity the producer specified.  One which cannot be scaled that way is
// replaced by the white point of the Profile Connection Space.
func repairWhitePoint(x []float64) []float64 {
	if isValidWhitePoint(x) {
		return x
	}
	// rescue the chromaticity the producer specified, where the data allows
	if len(x) == 3 && allFinite(x) && x[1] > 0 {
		s := 1 / x[1]
		scaled := []float64{x[0] * s, 1, x[2] * s}
		if isValidWhitePoint(scaled) {
			return scaled
		}
	}
	return slices.Clone(pcsWhitePoint)
}

// repairBlackPoint returns a black point with non-negative, finite entries.
// A nil argument is returned unchanged, so that the factory function fills in
// the default.
func repairBlackPoint(x []float64) []float64 {
	if x == nil || isValidBlackPoint(x) {
		return x
	}
	out := make([]float64, 3)
	for i := range out {
		// the comparison rejects NaN, which fails every ordered test
		if i < len(x) && x[i] > 0 && finite(x[i]) {
			out[i] = x[i]
		}
	}
	return out
}

// repairLabRanges returns a* and b* ranges which are ordered, non-empty and of
// usable magnitude.  A nil argument is returned unchanged, so that [Lab] fills
// in the default.
func repairLabRanges(x []float64) []float64 {
	if x == nil || isValidLabRanges(x) {
		return x
	}
	if len(x) != 4 || !allFinite(x) {
		return []float64{-100, 100, -100, 100}
	}
	out := slices.Clone(x)
	for i, v := range out {
		out[i] = ClipComponent(v, -maxLabRange, maxLabRange)
	}
	// clamping can collapse a range, which the loop below then replaces
	for i := 0; i < 4; i += 2 {
		if out[i] > out[i+1] {
			out[i], out[i+1] = out[i+1], out[i]
		}
		if out[i] == out[i+1] {
			out[i], out[i+1] = -100, 100
		}
	}
	return out
}

// repairGamma returns a positive, finite gamma value.
func repairGamma(gamma float64) float64 {
	if isValidGamma(gamma) {
		return gamma
	}
	return 1
}

// repairGammaArray repairs the three gamma values of a CalRGB colour space in
// place.  A nil argument is returned unchanged, so that [CalRGB] fills in the
// default.
func repairGammaArray(x []float64) []float64 {
	if x == nil || isValidGammaArray(x) {
		return x
	}
	if len(x) != 3 {
		return []float64{1, 1, 1}
	}
	for i, gamma := range x {
		x[i] = repairGamma(gamma)
	}
	return x
}

// repairMatrix returns a CalRGB matrix which describes a usable colour space,
// see [isValidMatrix].  A nil argument is returned unchanged, so that [CalRGB]
// fills in the default.
func repairMatrix(x []float64) []float64 {
	if x == nil || isValidMatrix(x) {
		return x
	}
	return []float64{1, 0, 0, 0, 1, 0, 0, 0, 1}
}

// deviceNAttributeKeys lists the entries a DeviceN attributes dictionary can
// hold, per Table 70.
var deviceNAttributeKeys = []pdf.Name{
	"Subtype", "Colorants", "Process", "MixingHints", "Order",
}

// repairDeviceNAttributes returns the entries of a DeviceN attributes
// dictionary which [DeviceN] accepts.
//
// PDF dictionaries carry developer-defined extensions (see 7.12), so an
// unrecognised key is an expected feature of real files rather than damage,
// and must not make the whole colour space unreadable.  Such an entry is
// dropped rather than kept, which loses the extension but keeps [DeviceN]
// strict about what a caller may write.
//
// A nil dictionary, and one left empty, are returned as nil so that the
// attributes are omitted entirely.
func repairDeviceNAttributes(attr pdf.Dict) pdf.Dict {
	if attr == nil {
		return nil
	}

	out := pdf.Dict{}
	for _, key := range deviceNAttributeKeys {
		if val, ok := attr[key]; ok {
			out[key] = val
		}
	}
	// an unknown Subtype leaves the entry out, so that the default applies
	if val := out["Subtype"]; val != nil &&
		val != pdf.Name("DeviceN") && val != pdf.Name("NChannel") {
		delete(out, "Subtype")
	}

	if len(out) == 0 {
		return nil
	}
	return out
}

const ε = 1e-6
