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
	Get(Reference) (Object, error)
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
func Resolve(r Getter, obj Object) (Object, error) {
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
		obj, err = r.Get(ref)
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
	GetArray  = resolveAndCast[Array]
	GetBool   = resolveAndCast[Bool]
	GetDict   = resolveAndCast[Dict]
	GetInt    = resolveAndCast[Integer]
	GetName   = resolveAndCast[Name]
	GetReal   = resolveAndCast[Real]
	GetStream = resolveAndCast[*Stream]
	GetString = resolveAndCast[String]
)

// TODO(voss): find a better name for this
type Putter interface {
	Close() error
	GetMeta() *MetaInfo
	Alloc() Reference
	Put(ref Reference, obj Object) error
	OpenStream(ref Reference, dict Dict, filters ...Filter) (io.WriteCloser, error)
	WriteCompressed(refs []Reference, objects ...Object) error
	AutoClose(res Closer)
}

func IsTagged(pdf Putter) bool {
	// TODO(voss): what can we do if catalog.MarkInfo is an indirect object?
	catalog := pdf.GetMeta().Catalog
	markInfo, _ := catalog.MarkInfo.(Dict)
	if markInfo == nil {
		return false
	}
	marked, _ := markInfo["Marked"].(Bool)
	return bool(marked)
}
