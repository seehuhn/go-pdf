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

package pdf

import (
	"fmt"
	"math"

	"seehuhn.de/go/pdf/internal/limits"
)

// resolvePath follows indirect references until it reaches a non-reference
// object.  It returns the resolved object together with the path extended by
// every reference that was followed (callers that only need the value can
// discard the path).
//
// References are followed with precise cycle detection ([CycleCheck.Seen]) and
// a depth cap ([limits.MaxExtractDepth]); a loop yields [ErrCycle] and an
// over-deep acyclic chain yields [ErrDepth].
//
// The canObjStm argument is passed to [Getter.Get]: pass false to forbid
// reading from an object stream, which is required while resolving stream
// lengths during object-stream construction to avoid infinite recursion.
func resolvePath(g Getter, path *CycleCheck, obj Object, canObjStm bool) (Native, *CycleCheck, error) {
	if obj == nil {
		return nil, path, nil
	}

	ref, isReference := obj.(Reference)
	if !isReference {
		return obj.AsPDF(0), path, nil
	}

	for {
		var err error
		path, err = path.step(ref)
		if err != nil {
			return nil, nil, err
		}

		next, err := g.Get(ref, canObjStm)
		if err != nil {
			return nil, nil, err
		}

		if ref, isReference = next.(Reference); !isReference {
			return next, path, nil
		}
	}
}

// step extends the path by ref, after checking that ref is not already on the
// path (cycle) and that the resulting depth stays within the limit.  It is the
// single place where reference following enforces its cycle and depth bounds,
// shared by [resolvePath] and the cache-aware loop in [Decode].
func (path *CycleCheck) step(ref Reference) (*CycleCheck, error) {
	if path.Seen(ref) {
		return nil, &MalformedFileError{
			Err: ErrCycle,
			Loc: []string{"object " + ref.String()},
		}
	}
	next := &CycleCheck{Ref: ref, Parent: path}
	if next.depth() > limits.MaxExtractDepth {
		return nil, &MalformedFileError{
			Err: ErrDepth,
			Loc: []string{"object " + ref.String()},
		}
	}
	return next, nil
}

// as asserts that an already-resolved object has type T.  A nil object yields
// the zero value without error; a mismatch yields a [MalformedFileError].
func as[T Native](resolved Native) (T, error) {
	var zero T
	if resolved == nil {
		return zero, nil
	}
	if v, ok := resolved.(T); ok {
		return v, nil
	}
	return zero, &MalformedFileError{
		Err: fmt.Errorf("expected %T but got %T", zero, resolved),
	}
}

// asInteger coerces an already-resolved object to an Integer.  A nil object
// yields 0.  Real values are rounded to the nearest integer; other types yield
// a [MalformedFileError].
func asInteger(resolved Native) (Integer, error) {
	switch x := resolved.(type) {
	case nil:
		return 0, nil
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

// asNumber coerces an already-resolved object to a Number.  A nil object yields
// 0; non-numeric types yield a [MalformedFileError].
func asNumber(resolved Native) (Number, error) {
	switch x := resolved.(type) {
	case nil:
		return 0, nil
	case Integer:
		return Number(x), nil
	case Real:
		return Number(x), nil
	default:
		return 0, &MalformedFileError{
			Err: fmt.Errorf("expected Number but got %T", resolved),
		}
	}
}
