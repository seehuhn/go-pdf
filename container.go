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
)

// TODO(voss): find a better name for this
type Getter interface {
	GetMeta() *MetaInfo
	Get(Reference, bool) (Object, error)
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
func Resolve(r Getter, obj Object) (Object, error) {
	return resolve(r, obj, true)
}

func resolve(r Getter, obj Object, canObjStm bool) (Object, error) {
	origObj := obj

	count := 0
	for {
		ref, isReference := obj.(Reference)
		if !isReference {
			break
		}
		count++
		if count > 16 {
			return nil, &MalformedFileError{
				Err: errors.New("too many levels of indirection"),
				Loc: []string{"object " + origObj.(Reference).String()},
			}
		}

		var err error
		obj, err = r.Get(ref, canObjStm)
		if err != nil {
			return nil, err
		}
	}

	return obj, nil
}

func resolveAndCast[T Object](r Getter, obj Object) (x T, err error) {
	obj, err = Resolve(r, obj)
	if err != nil {
		return x, err
	}

	if obj == nil {
		return x, nil
	}

	var isCorrectType bool
	x, isCorrectType = obj.(T)
	if isCorrectType {
		return x, nil
	}

	return x, &MalformedFileError{
		Err: fmt.Errorf("expected %T but got %T", x, obj),
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
	GetInteger = resolveAndCast[Integer]
	GetName    = resolveAndCast[Name]
	GetReal    = resolveAndCast[Real]
	GetStream  = resolveAndCast[*Stream]
	GetString  = resolveAndCast[String]
)

// GetDictTyped resolves any indirect reference and checks that the resulting
// object is a dictionary.  The function also checks that the "Type" entry of
// the dictionary, if set, is equal to the given type.
func GetDictTyped(r Getter, obj Object, tp Name) (Dict, error) {
	dict, err := GetDict(r, obj)
	if dict == nil || err != nil {
		return nil, err
	}
	val, err := GetName(r, dict["Type"])
	if err != nil {
		return nil, err
	}
	if val != tp && val != "" {
		return nil, fmt.Errorf("expected dict type %q, got %q", tp, val)
	}
	return dict, nil
}

// DecodeStream returns a reader for the decoded stream data.
// If numFilters is non-zero, only the first numFilters filters are decoded.
func DecodeStream(r Getter, x *Stream, numFilters int) (io.Reader, error) {
	filters, err := getFilters(r, x)
	if err != nil {
		return nil, err
	}

	v := V1_2
	if r != nil {
		v = r.GetMeta().Version
	}

	out := x.R
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

// Filters extracts the information contained in the /Filter and /DecodeParms
// entries of the stream dictionary.
//
// TODO(voss): remove?
func getFilters(r Getter, x *Stream) ([]Filter, error) {
	decodeParams, err := resolve(r, x.Dict["DecodeParms"], false)
	if err != nil {
		return nil, err
	}
	filter, err := resolve(r, x.Dict["Filter"], false)
	if err != nil {
		return nil, err
	}

	var res []Filter
	switch f := filter.(type) {
	case nil:
		// pass
	case Name:
		pDict, err := toDict(decodeParams)
		if err != nil {
			return nil, err
		}
		res = append(res, makeFilter(f, pDict))
	case Array:
		pa, ok := decodeParams.(Array)
		if !ok {
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
				x, err := toDict(pai)
				if err != nil {
					return nil, err
				}
				pDict = x
			}
			res = append(res, makeFilter(name, pDict))
		}
	default:
		return nil, errors.New("invalid /Filter field")
	}
	return res, nil
}

// TODO(voss): find a better name for this
type Putter interface {
	Close() error
	GetMeta() *MetaInfo
	Alloc() Reference
	Put(ref Reference, obj Object) error
	OpenStream(ref Reference, dict Dict, filters ...Filter) (io.WriteCloser, error)

	// TODO(voss): allow to set the object ID for the containing stream?
	WriteCompressed(refs []Reference, objects ...Object) error

	AutoClose(obj io.Closer)
}

func IsTagged(pdf Putter) bool {
	// TODO(voss): what can we do if catalog.MarkInfo is an indirect object?
	catalog := pdf.GetMeta().Catalog
	markInfo, _ := catalog.MarkInfo.(Dict)
	if markInfo == nil {
		return false
	}
	marked, _ := markInfo["Marked"].(Boolean)
	return bool(marked)
}

func GetVersion(pdf Putter) Version {
	return pdf.GetMeta().Version
}
