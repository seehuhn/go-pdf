// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package decode

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/movie"
)

func decodeMovie(c pdf.Cursor, dict pdf.Dict) (*annotation.Movie, error) {
	annot := &annotation.Movie{}

	// Extract common annotation fields
	if err := decodeCommon(c, &annot.Common, dict); err != nil {
		return nil, err
	}

	// T (optional)
	if t, err := pdf.Optional(c.TextString(dict["T"])); err != nil {
		return nil, err
	} else if t != "" {
		annot.Title = string(t)
	}

	// Movie (required): a movie annotation without a usable Movie cannot be
	// written back, so reject it here.  The page decoder drops annotations
	// that fail to decode, matching the permissive-reader policy.
	if m, err := pdf.DecodeOptional(c, dict["Movie"], movie.Extract); err != nil {
		return nil, err
	} else if m == nil {
		return nil, pdf.Error("movie annotation missing Movie")
	} else {
		annot.Movie = m
	}

	// A (optional) — bool, dict, or absent (PDF default true)
	if aObj, ok := dict["A"]; ok {
		resolved, _ := c.Resolve(aObj)
		switch v := resolved.(type) {
		case pdf.Boolean:
			if bool(v) {
				annot.Activation = movie.DefaultActivation
			}
			// false → leave Activation nil (do not play)
		default:
			if act, err := pdf.DecodeOptional(c, aObj, movie.ExtractActivation); err != nil {
				return nil, err
			} else if act != nil {
				annot.Activation = act
			} else {
				annot.Activation = movie.DefaultActivation
			}
		}
	} else {
		annot.Activation = movie.DefaultActivation
	}

	return annot, nil
}
