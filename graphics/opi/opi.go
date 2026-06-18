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
func Extract(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, _ bool) (Dict, error) {
	dict, err := x.GetDict(path, obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing OPI version dictionary")
	}

	if _, ok := dict["1.3"]; ok {
		v, err := pdf.ExtractorGet(x, path, dict["1.3"], extractV13)
		if err != nil {
			return nil, err
		}
		return v, nil
	}
	if _, ok := dict["2.0"]; ok {
		v, err := pdf.ExtractorGet(x, path, dict["2.0"], extractV20)
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
func readNumbers(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object) ([]float64, error) {
	arr, err := pdf.Optional(x.GetArray(path, obj))
	if err != nil || arr == nil {
		return nil, err
	}
	out := make([]float64, 0, len(arr))
	for _, el := range arr {
		v, err := pdf.Optional(x.GetNumber(path, el))
		if err != nil {
			return nil, err
		}
		out = append(out, float64(v))
	}
	return out, nil
}

// readInts resolves obj to an array and returns its elements as int.
func readInts(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object) ([]int, error) {
	arr, err := pdf.Optional(x.GetArray(path, obj))
	if err != nil || arr == nil {
		return nil, err
	}
	out := make([]int, 0, len(arr))
	for _, el := range arr {
		v, err := pdf.Optional(x.GetInteger(path, el))
		if err != nil {
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
