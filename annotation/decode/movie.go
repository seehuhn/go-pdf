// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package decode

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/movie"
)

func decodeMovie(c pdf.Cursor, dict pdf.Dict) (*annotation.Movie, error) {
	annot := &annotation.Movie{}

	if err := decodeCommon(c, &annot.Common, dict); err != nil {
		return nil, err
	}

	// T (optional)
	if t, err := pdf.Optional(c.TextString(dict["T"])); err != nil {
		return nil, err
	} else if !t.IsZero() {
		annot.Title = t
	}

	// Movie (required): a movie annotation without a usable Movie cannot be
	// written back, so reject it here.  The page decoder drops annotations
	// that fail to decode, matching the permissive-reader policy.
	if m, err := c.DecodeOptional(dict["Movie"], movie.Extract); err != nil {
		return nil, err
	} else if m == nil {
		return nil, pdf.Error("movie annotation missing Movie")
	} else {
		annot.Movie = m
	}

	// A (optional) — bool, dict, or absent (PDF default true)
	if aObj, ok := dict["A"]; ok {
		resolved, err := pdf.Optional(c.Resolve(aObj))
		if err != nil {
			return nil, err
		}
		switch v := resolved.(type) {
		case pdf.Boolean:
			if bool(v) {
				annot.Activation = movie.DefaultActivation
			}
			// false → leave Activation nil (do not play)
		default:
			if act, err := c.DecodeOptional(aObj, movie.ExtractActivation); err != nil {
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
