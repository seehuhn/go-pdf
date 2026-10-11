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

// Package opi implements Open Prepress Interface (OPI) dictionaries.
//
// An OPI dictionary describes a low-resolution proxy for a high-resolution
// image, to be replaced by an OPI server before printing. It is referenced
// from the OPI entry of an image or form XObject dictionary.
//
// Two OPI specification versions exist, 1.3 and 2.0, modelled by [V13] and
// [V20]. These are versions of the Open Prepress Interface specification and
// are unrelated to PDF versions. The OPI feature as a whole is deprecated in
// PDF 2.0.
package opi

import (
	"math"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 14.11.7

// Dict is an OPI dictionary. It is implemented by [V13] (OPI version 1.3) and
// [V20] (OPI version 2.0).
type Dict interface {
	pdf.Embedder

	// Equal reports whether two OPI dictionaries are equal.
	Equal(Dict) bool

	isOPI()
}

// Extract reads an OPI version dictionary (the value of an XObject's OPI entry)
// and returns the OPI dictionary it contains.
func Extract(c pdf.Cursor, obj pdf.Object, _ bool) (Dict, error) {
	dict, err := c.Dict(obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing OPI version dictionary")
	}

	if _, ok := dict["1.3"]; ok {
		v, err := c.Decode(dict["1.3"], extractV13)
		if err != nil {
			return nil, err
		}
		return v, nil
	}
	if _, ok := dict["2.0"]; ok {
		v, err := c.Decode(dict["2.0"], extractV20)
		if err != nil {
			return nil, err
		}
		return v, nil
	}
	return nil, pdf.Error("OPI version dictionary has no 1.3 or 2.0 entry")
}

// embedVersion wraps an inner OPI dictionary in an OPI version dictionary
// keyed by the version name, embedding the inner dictionary as an indirect
// object unless singleUse is set.
func embedVersion(rm *pdf.EmbedHelper, version pdf.Name, inner pdf.Dict, singleUse bool) (pdf.Native, error) {
	if singleUse {
		return pdf.Dict{version: inner}, nil
	}
	ref := rm.Alloc()
	if err := rm.Out().Put(ref, inner); err != nil {
		return nil, err
	}
	return pdf.Dict{version: ref}, nil
}

// readNumbers resolves obj to an array and returns its elements as float64.
//
// The OPI arrays are positional, so a single malformed element invalidates
// the whole array: the result is then nil, as for an absent entry.
func readNumbers(c pdf.Cursor, obj pdf.Object) ([]float64, error) {
	arr, err := pdf.Optional(c.Array(obj))
	if err != nil || arr == nil {
		return nil, err
	}
	out := make([]float64, 0, len(arr))
	for _, el := range arr {
		v, err := c.Number(el)
		if pdf.IsMalformed(err) {
			return nil, nil
		} else if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// readInts resolves obj to an array and returns its elements as int.
// Like [readNumbers], it returns nil if any element is malformed.
func readInts(c pdf.Cursor, obj pdf.Object) ([]int, error) {
	arr, err := pdf.Optional(c.Array(obj))
	if err != nil || arr == nil {
		return nil, err
	}
	out := make([]int, 0, len(arr))
	for _, el := range arr {
		v, err := c.Integer(el)
		if pdf.IsMalformed(err) {
			return nil, nil
		} else if err != nil {
			return nil, err
		}
		out = append(out, int(v))
	}
	return out, nil
}

func numbersToArray(vals []float64) pdf.Array {
	arr := make(pdf.Array, len(vals))
	for i, v := range vals {
		arr[i] = pdf.Number(v)
	}
	return arr
}

func intsToArray(vals []int) pdf.Array {
	arr := make(pdf.Array, len(vals))
	for i, v := range vals {
		arr[i] = pdf.Integer(v)
	}
	return arr
}

// inUnitRange reports whether x lies in the range 0.0 to 1.0.
func inUnitRange(x float64) bool {
	return x >= 0 && x <= 1
}

// validGrayMap reports whether gm is a valid OPI 1.3 gray map: 2^n integers
// in the range 0 to 65535, where n is the number of bits per sample.  If
// imageType is known, n must match its bits per sample.
func validGrayMap(gm []int, imageType *[2]int) bool {
	n := len(gm)
	if n < 2 || n > 1<<16 || n&(n-1) != 0 {
		return false
	}
	if imageType != nil {
		if bits := imageType[1]; bits >= 1 && bits <= 16 && n != 1<<bits {
			return false
		}
	}
	for _, x := range gm {
		if x < 0 || x > 65535 {
			return false
		}
	}
	return true
}

// validPosition reports whether the corners [llx lly ulx uly urx ury lrx lry]
// of an OPI 1.3 Position entry form a parallelogram, up to rounding errors.
// Corners which are not finite, or whose differences overflow, are invalid.
func validPosition(p *[8]float64) bool {
	return approxEqual(p[2]-p[0], p[4]-p[6]) && approxEqual(p[3]-p[1], p[5]-p[7])
}

// approxEqual reports whether a and b are finite and equal up to rounding
// errors.
func approxEqual(a, b float64) bool {
	if math.IsInf(a, 0) || math.IsInf(b, 0) {
		return false
	}
	return math.Abs(a-b) <= 1e-6*max(1, math.Abs(a), math.Abs(b))
}

// validCrop reports whether a crop rectangle [left top right bottom] lies
// inside an image of the given size, as 0 ≤ left < right ≤ width and
// 0 ≤ top < bottom ≤ height.
func validCrop[T int | float64](size *[2]T, crop *[4]T) bool {
	return 0 <= crop[0] && crop[0] < crop[2] && crop[2] <= size[0] &&
		0 <= crop[1] && crop[1] < crop[3] && crop[3] <= size[1]
}

// validCrop13 reports whether an OPI 1.3 crop rectangle [left top right
// bottom] lies inside an image of the given size.  Unlike for OPI 2.0, the
// PDF specification does not fix the direction of the vertical axis, so top
// and bottom may come in either order but must differ.
func validCrop13[T int | float64](size *[2]T, crop *[4]T) bool {
	lo, hi := min(crop[1], crop[3]), max(crop[1], crop[3])
	return 0 <= crop[0] && crop[0] < crop[2] && crop[2] <= size[0] &&
		0 <= lo && lo < hi && hi <= size[1]
}
