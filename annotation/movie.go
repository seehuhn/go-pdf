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

package annotation

import "seehuhn.de/go/pdf"

// Movie represents a movie annotation containing animated graphics and sound
// to be presented on the computer screen and through the speakers. When the
// annotation is activated, the movie is played.
//
// NOTE: Movie annotations are deprecated in PDF 2.0 and superseded by the
// general multimedia framework.
type Movie struct {
	Common

	// Title (optional) is the title of the movie annotation. Movie actions may
	// use this title to reference the movie annotation.
	Title string

	// Movie (required) is a movie dictionary that describes the movie's
	// static characteristics.
	Movie pdf.Reference

	// A (optional) is a flag or dictionary specifying whether and how to play
	// the movie when the annotation is activated. If this value is a dictionary,
	// it is a movie activation dictionary specifying how to play the movie.
	// If the value is the boolean true, the movie is played using default
	// activation parameters. If the value is false, the movie is not played.
	// Default value: true.
	A pdf.Object
}

var _ Annotation = (*Movie)(nil)

// AnnotationType returns "Movie".
// This implements the [Annotation] interface.
func (m *Movie) AnnotationType() pdf.Name {
	return "Movie"
}

func decodeMovie(x *pdf.Extractor, dict pdf.Dict) (*Movie, error) {
	r := x.R
	movie := &Movie{}

	// Extract common annotation fields
	if err := decodeCommon(x, &movie.Common, dict); err != nil {
		return nil, err
	}

	// Extract movie-specific fields
	// T (optional)
	if t, err := pdf.GetTextString(r, dict["T"]); err == nil && t != "" {
		movie.Title = string(t)
	}

	// Movie (required)
	if movieRef, ok := dict["Movie"].(pdf.Reference); ok {
		movie.Movie = movieRef
	}

	// A (optional) - can be boolean or dictionary, default is true
	if a, ok := dict["A"]; ok {
		movie.A = a
	} else {
		movie.A = pdf.Boolean(true) // PDF default value
	}

	return movie, nil
}

func (m *Movie) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "movie annotation", pdf.V1_2); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("Movie"),
	}

	// Add common annotation fields
	if err := m.Common.fillDict(rm, dict, isMarkup(m)); err != nil {
		return nil, err
	}

	// Add movie-specific fields
	// T (optional)
	if m.Title != "" {
		dict["T"] = pdf.TextString(m.Title)
	}

	// Movie (required)
	if m.Movie != 0 {
		dict["Movie"] = m.Movie
	}

	// A (optional) - only write if not the default value true
	if m.A != nil {
		if b, isBool := m.A.(pdf.Boolean); !isBool || !bool(b) {
			dict["A"] = m.A
		}
	}

	return dict, nil
}
