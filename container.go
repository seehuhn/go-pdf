// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package pdf

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
)

// Getter represents a PDF file opened for reading.
//
// TODO(voss): find a better name for this
type Getter interface {
	GetMeta() *MetaInfo

	// Get reads an object from the file.
	//
	// The argument canObjStm specifies whether the object may be read from an
	// object stream.  Normally, this should be set to true.  If canObjStm is
	// false and the object is in an object stream, an error is returned.
	Get(ref Reference, canObjStm bool) (Native, error)
}

// Resolve resolves references to indirect objects.
//
// If obj is a [Reference], the function reads the corresponding object from
// the file and returns the result.  If obj is not a [Reference], it is
// returned unchanged.  The function recursively follows chains of references
// until it resolves to a non-reference object.
//
// If a reference loop is encountered, the function returns an error of type
// [MalformedFileError].
//
// TODO(voss): rename to "Get"?
func Resolve(r Getter, obj Object) (Native, error) {
	return resolve(r, obj, true)
}

const maxRefDepth = 16

func resolve(r Getter, obj Object, canObjStm bool) (Native, error) {
	if obj == nil {
		return nil, nil
	}

	ref, isReference := obj.(Reference)
	if !isReference {
		// TODO(voss): which options should be used here?
		return obj.AsPDF(0), nil
	}

	origRef := ref

	count := 0
	for {
		count++
		if count > maxRefDepth {
			return nil, &MalformedFileError{
				Err: errors.New("too many levels of indirection"),
				Loc: []string{"object " + origRef.String()},
			}
		}

		next, err := r.Get(ref, canObjStm)
		if err != nil {
			return nil, err
		}
		ref, isReference = next.(Reference)
		if !isReference {
			return next, nil
		}
	}
}

func resolveAndCast[T Native](r Getter, obj Object) (x T, err error) {
	resolved, err := Resolve(r, obj)
	if err != nil {
		return x, err
	}

	if resolved == nil {
		return x, nil
	}

	var isCorrectType bool
	x, isCorrectType = resolved.(T)
	if isCorrectType {
		return x, nil
	}

	return x, &MalformedFileError{
		Err: fmt.Errorf("expected %T but got %T", x, resolved),
	}
}

// Helper functions for getting objects of a specific type.  Each of these
// functions calls Resolve on the object before attempting to convert it to the
// desired type.  If the object is `null`, a zero object is returned witout
// error.  If the object is of the wrong type, an error is returned.
//
// The signature of these functions is
//
//	func GetT(r Getter, obj Object) (x T, err error)
//
// where T is the type of the object to be returned.
var (
	GetArray   = resolveAndCast[Array]
	GetBoolean = resolveAndCast[Boolean]
	GetDict    = resolveAndCast[Dict]
	GetName    = resolveAndCast[Name]
	GetReal    = resolveAndCast[Real]
	GetStream  = resolveAndCast[*Stream]
	GetString  = resolveAndCast[String]
)

// GetInteger resolves any indirect reference and returns the object as an
// Integer.  If the object is `null`, the function returns 0, nil.
// Integers are returned as is.
// Floating point values are silently rounded to the nearest integer.
// All other object types result in an error.
func GetInteger(r Getter, obj Object) (Integer, error) {
	resolved, err := Resolve(r, obj)
	if resolved == nil {
		return 0, err
	}

	switch x := resolved.(type) {
	case Integer:
		return x, nil
	case Real:
		return Integer(math.Round(float64(x))), nil
	default:
		return 0, &MalformedFileError{
			Err: fmt.Errorf("expected Integer but got %T", resolved),
		}
	}
}

func getIntegerNoObjStm(r Getter, obj Object) (Integer, error) {
	obj, err := resolve(r, obj, false)
	if err != nil {
		return 0, err
	}
	if x, isCorrectType := obj.(Integer); isCorrectType {
		return x, nil
	}
	return 0, &MalformedFileError{
		Err: fmt.Errorf("expected Integer but got %T", obj),
	}
}

// GetFloatArray resolves any indirect reference and returns the object as a
// slice of float64 values. Each array element is converted to float64 using
// GetNumber.
//
// If the object is `null`, the function returns `nil, nil`.
func GetFloatArray(r Getter, obj Object) ([]float64, error) {
	array, err := GetArray(r, obj)
	if err != nil {
		return nil, err
	}
	if array == nil {
		return nil, nil
	}

	result := make([]float64, len(array))
	for i, item := range array {
		num, err := GetNumber(r, item)
		if err != nil {
			return nil, fmt.Errorf("array element %d: %w", i, err)
		}
		result[i] = float64(num)
	}

	return result, nil
}

// GetDictTyped resolves any indirect reference and checks that the resulting
// object is a dictionary.  The function also checks that the "Type" entry of
// the dictionary, if set, is equal to the given type.
//
// If the object is `null`, the function returns `nil, nil`.
func GetDictTyped(r Getter, obj Object, tp Name) (Dict, error) {
	dict, err := GetDict(r, obj)
	if dict == nil || err != nil {
		return nil, err
	}
	err = CheckDictType(r, dict, tp)
	if err != nil {
		return nil, err
	}

	return dict, nil
}

// CheckDictType checks that the "Type" entry of the dictionary, if present, is
// equal to the given type.
func CheckDictType(r Getter, obj Dict, wantType Name) error {
	haveType, err := GetName(r, obj["Type"])
	if err != nil {
		return err
	}
	if haveType != wantType && haveType != "" {
		return &MalformedFileError{
			Err: fmt.Errorf("expected dict type %q, got %q", wantType, haveType),
		}
	}
	return nil
}

// GetStreamReader returns an io.Reader which returns the decoded
// contents of a PDF stream.
//
// If ref is nil, the function returns an error which wraps os.ErrNotExist.
//
// This is a convenience function, combining [GetStream] and [DecodeStream].
func GetStreamReader(r Getter, ref Object) (io.ReadCloser, error) {
	stm, err := GetStream(r, ref)
	if err != nil {
		return nil, err
	} else if stm == nil {
		return nil, fmt.Errorf("no stream found: %w", os.ErrNotExist)
	}
	return DecodeStream(r, stm, 0)
}

// DecodeStream returns a reader for the decoded stream data. If numFilters is
// non-zero, only the first numFilters filters are decoded.
//
// If the x.R implements io.ReadSeeker, the stream will be reset to the
// beginning before decoding.  In particular, this is the case for streams read
// from a file.  This allows streams to be decoded multiple times.
//
// For encrypted PDFs, decryption is applied on-the-fly before any other
// filters. This does not count towards numFilters.
func DecodeStream(r Getter, x *Stream, numFilters int) (io.ReadCloser, error) {
	if seeker, ok := x.R.(io.Seeker); ok {
		_, err := seeker.Seek(0, io.SeekStart)
		if err != nil {
			return nil, err
		}
	}
	filters, err := GetFilters(r, x.Dict)
	if err != nil {
		return nil, err
	}

	v := V1_2
	if r != nil { // TODO(voss): is r == nil still possible?
		v = GetVersion(r)
	}

	out := io.NopCloser(x.R)

	// apply decryption before other filters
	if x.crypt != nil {
		out, err = x.crypt.Decode(v, out)
		if err != nil {
			return nil, err
		}
	}

	for i, fi := range filters {
		if numFilters > 0 && i >= numFilters {
			break
		}
		out, err = fi.Decode(v, out)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// GetFilters extracts the information contained in the /Filter and
// /DecodeParms entries of a stream dictionary.
func GetFilters(r Getter, dict Dict) ([]Filter, error) {
	decodeParams, err := resolve(r, dict["DecodeParms"], false)
	if err != nil {
		return nil, err
	}
	filter, err := resolve(r, dict["Filter"], false)
	if err != nil {
		return nil, err
	}

	var res []Filter
	switch f := filter.(type) {
	case nil:
		// pass
	case Name:
		var pDict Dict
		if decodeParams != nil {
			var ok bool
			pDict, ok = decodeParams.(Dict)
			if !ok {
				return nil, fmt.Errorf("wrong type, expected Dict but got %T", decodeParams)
			}
		}
		res = append(res, makeFilter(f, pDict))
	case Array:
		pa, ok := decodeParams.(Array)
		if !ok && decodeParams != nil {
			return nil, errors.New("invalid /DecodeParms field")
		}
		for i, fi := range f {
			fi, err := resolve(r, fi, false)
			if err != nil {
				return nil, err
			}
			name, ok := fi.(Name)
			if !ok {
				return nil, fmt.Errorf("wrong type, expected Name but got %T", fi)
			}
			var pDict Dict
			if len(pa) > i {
				pai, err := resolve(r, pa[i], false)
				if err != nil {
					return nil, err
				}
				if pai != nil {
					var ok bool
					pDict, ok = pai.(Dict)
					if !ok {
						return nil, fmt.Errorf("wrong type, expected Dict but got %T", pai)
					}
				}
			}
			res = append(res, makeFilter(name, pDict))
		}
	default:
		return nil, Error("invalid /Filter field")
	}
	return res, nil
}

// IsTagged returns true, if the PDF file is "tagged".
func IsTagged(pdf *Writer) bool {
	// TODO(voss): what can we do if catalog.MarkInfo is an indirect object?
	catalog := pdf.GetMeta().Catalog
	markInfo, _ := catalog.MarkInfo.(Dict)
	if markInfo == nil {
		return false
	}
	marked, _ := markInfo["Marked"].(Boolean)
	return bool(marked)
}

// GetVersion returns the PDF version used in a PDF file.
func GetVersion(pdf interface{ GetMeta() *MetaInfo }) Version {
	return pdf.GetMeta().Version
}
