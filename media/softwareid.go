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

package media

import (
	"errors"

	"seehuhn.de/go/pdf"
)

// SoftwareIdentifier identifies a piece of software by name, range of
// versions, and operating systems.  It is used to determine whether a given
// media player may be used.
type SoftwareIdentifier struct {
	// URI identifies the software.  The only defined scheme is
	// "vnd.adobe.swname".
	URI string

	// Low is the lower bound of the version range, as a sequence of
	// subversion numbers (major version first).  A nil value stands for
	// the version [0].
	Low []int

	// High is the upper bound of the version range.  A nil value stands for
	// infinity, that is, an unbounded upper end.
	High []int

	// LowExclusive reports whether the lower bound is exclusive.
	// When false, the bound is inclusive.
	LowExclusive bool

	// HighExclusive reports whether the upper bound is exclusive.
	// When false, the bound is inclusive.
	HighExclusive bool

	// OS lists the operating system identifiers to which this object
	// applies.  A nil value stands for all operating systems.
	OS []string

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// ExtractSoftwareIdentifier reads a software identifier dictionary.
func ExtractSoftwareIdentifier(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, isDirect bool) (*SoftwareIdentifier, error) {
	dict, err := x.GetDictTyped(path, obj, "SoftwareIdentifier")
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing software identifier dictionary")
	}

	s := &SoftwareIdentifier{SingleUse: isDirect}

	u, err := pdf.Optional(x.GetString(path, dict["U"]))
	if err != nil {
		return nil, err
	} else if len(u) == 0 {
		return nil, pdf.Error("software identifier missing U entry")
	}
	s.URI = string(u)

	if low, err := pdf.Optional(extractVersionArray(x, path, dict["L"])); err != nil {
		return nil, err
	} else {
		s.Low = low
	}
	if high, err := pdf.Optional(extractVersionArray(x, path, dict["H"])); err != nil {
		return nil, err
	} else {
		s.High = high
	}

	if li, err := pdf.Optional(x.GetBoolean(path, dict["LI"])); err != nil {
		return nil, err
	} else if dict["LI"] != nil {
		s.LowExclusive = !bool(li)
	}
	if hi, err := pdf.Optional(x.GetBoolean(path, dict["HI"])); err != nil {
		return nil, err
	} else if dict["HI"] != nil {
		s.HighExclusive = !bool(hi)
	}

	if os, err := pdf.Optional(x.GetArray(path, dict["OS"])); err != nil {
		return nil, err
	} else {
		for _, elem := range os {
			name, err := pdf.Optional(x.GetString(path, elem))
			if err != nil {
				return nil, err
			}
			s.OS = append(s.OS, string(name))
		}
	}

	return s, nil
}

// extractVersionArray reads a version array, returning nil if the value is
// missing, empty, or contains a negative subversion number.
func extractVersionArray(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object) ([]int, error) {
	arr, err := x.GetArray(path, obj)
	if err != nil {
		return nil, err
	}
	if len(arr) == 0 {
		return nil, nil
	}
	out := make([]int, 0, len(arr))
	for _, elem := range arr {
		n, err := pdf.Optional(x.GetInteger(path, elem))
		if err != nil {
			return nil, err
		}
		if n < 0 {
			return nil, nil // malformed: treat the whole array as absent
		}
		out = append(out, int(n))
	}
	return out, nil
}

// Embed converts the software identifier to its PDF representation.
func (s *SoftwareIdentifier) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "software identifier", pdf.V1_5); err != nil {
		return nil, err
	}
	if s.URI == "" {
		return nil, errors.New("software identifier: URI is required")
	}
	for _, n := range s.Low {
		if n < 0 {
			return nil, errors.New("software identifier: negative version number")
		}
	}
	for _, n := range s.High {
		if n < 0 {
			return nil, errors.New("software identifier: negative version number")
		}
	}

	dict := pdf.Dict{
		"U": pdf.String(s.URI),
	}
	if e.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("SoftwareIdentifier")
	}
	if s.Low != nil {
		dict["L"] = versionArray(s.Low)
	}
	if s.High != nil {
		dict["H"] = versionArray(s.High)
	}
	if s.LowExclusive {
		dict["LI"] = pdf.Boolean(false)
	}
	if s.HighExclusive {
		dict["HI"] = pdf.Boolean(false)
	}
	if s.OS != nil {
		arr := make(pdf.Array, len(s.OS))
		for i, name := range s.OS {
			arr[i] = pdf.String(name)
		}
		dict["OS"] = arr
	}

	if s.SingleUse {
		return dict, nil
	}
	ref := e.Alloc()
	if err := e.Out().Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}

func versionArray(v []int) pdf.Array {
	arr := make(pdf.Array, len(v))
	for i, n := range v {
		arr[i] = pdf.Integer(n)
	}
	return arr
}
