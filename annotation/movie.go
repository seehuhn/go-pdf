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

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/movie"
)

// PDF 2.0 sections: 12.5.2 12.5.6.17

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

	// Movie is the movie dictionary that describes the movie's static
	// characteristics.
	Movie *movie.Movie

	// Activation controls whether and how the movie shall be played when the
	// annotation is activated.
	//
	// The value nil means to not play the movie on activation.
	// [movie.DefaultActivation] means play with the default parameters. Any
	// other value supplies an explicit activation dictionary.
	Activation *movie.Activation
}

var _ Annotation = (*Movie)(nil)

// AnnotationType returns "Movie".
// This implements the [Annotation] interface.
func (m *Movie) AnnotationType() pdf.Name {
	return "Movie"
}

func (m *Movie) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if m.Movie == nil {
		return nil, errors.New("movie annotation must have a Movie")
	}
	if err := pdf.CheckVersion(rm.Out, "movie annotation", pdf.V1_2); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("Movie"),
	}

	// Add common annotation fields
	if err := m.Common.fillDict(rm, dict, isMarkup(m), false); err != nil {
		return nil, err
	}

	// T (optional)
	if m.Title != "" {
		dict["T"] = pdf.TextString(m.Title)
	}

	// Movie (required)
	movieObj, err := rm.Embed(m.Movie)
	if err != nil {
		return nil, err
	}
	dict["Movie"] = movieObj

	// A (optional) — sentinel-based dispatch
	switch m.Activation {
	case nil:
		dict["A"] = pdf.Boolean(false)
	case movie.DefaultActivation:
		// omit — PDF default is true (play with defaults)
	default:
		actObj, err := rm.Embed(m.Activation)
		if err != nil {
			return nil, err
		}
		dict["A"] = actObj
	}

	return dict, nil
}
