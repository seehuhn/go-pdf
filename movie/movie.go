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

package movie

import (
	"errors"
	"fmt"

	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/image"
)

// PDF 2.0 sections: 13.4

// Movie describes the static characteristics of a movie referenced from
// a movie annotation.
//
// Deprecated in PDF 2.0.
type Movie struct {
	// File identifies the self-describing movie file.  The file format
	// is unspecified and no portability guarantees are made.
	File *file.Specification

	// Aspect (optional) gives the width and height of the movie's
	// bounding box in pixels.  The zero value indicates that the entry
	// is omitted, which is appropriate for movies consisting entirely
	// of sound.
	Aspect Aspect

	// Rotate (optional) is the number of degrees by which the movie
	// shall be rotated clockwise relative to the page.  Must be a
	// multiple of 90.  Zero means no rotation.
	Rotate int

	// Poster (optional) controls how a poster image representing the
	// movie is displayed.  nil means no poster.  [PosterFromMovieFile]
	// requests that the poster be retrieved from the movie file.  Any
	// other value supplies an image XObject to be displayed as the
	// poster.
	Poster graphics.Image

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// Aspect is the bounding-box size of a movie in pixels.
type Aspect struct {
	Width, Height int
}

// PosterFromMovieFile is a sentinel value used as the value of
// [Movie.Poster] to request that the poster image be retrieved from
// the movie file (encoded as boolean true on the wire).
//
// Compare against this value using equality.  Its [Embed] method
// returns an error so accidentally passing it to
// [pdf.EmbedHelper.Embed] is caught.
var PosterFromMovieFile graphics.Image = posterFromMovieFile{}

type posterFromMovieFile struct{}

func (posterFromMovieFile) Subtype() pdf.Name      { return "Image" }
func (posterFromMovieFile) ResourceName() pdf.Name { return "" }
func (posterFromMovieFile) Bounds() rect.IntRect   { return rect.IntRect{} }
func (posterFromMovieFile) Embed(*pdf.EmbedHelper) (pdf.Native, error) {
	return nil, errors.New("movie: PosterFromMovieFile is a marker, not an embeddable image")
}

// Extract reads a movie dictionary from the PDF file.  The isDirect
// parameter indicates whether the object was stored directly (true)
// or reached via an indirect reference (false).
func Extract(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, isDirect bool) (*Movie, error) {
	dict, err := x.GetDict(path, obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing movie dictionary")
	}

	m := &Movie{SingleUse: isDirect}

	// F (required)
	if f, err := pdf.Optional(pdf.ExtractorGet(x, path, dict["F"], file.ExtractSpecification)); err != nil {
		return nil, err
	} else {
		m.File = f
	}
	if m.File == nil {
		return nil, pdf.Error("movie dictionary missing F entry")
	}

	// Aspect (optional)
	if arr, err := pdf.Optional(x.GetArray(path, dict["Aspect"])); err != nil {
		return nil, err
	} else if len(arr) >= 2 {
		w, errW := pdf.Optional(x.GetInteger(path, arr[0]))
		h, errH := pdf.Optional(x.GetInteger(path, arr[1]))
		if errW == nil && errH == nil && w > 0 && h > 0 {
			m.Aspect = Aspect{Width: int(w), Height: int(h)}
		}
	}

	// Rotate (optional, multiple of 90, default 0)
	if rot, err := pdf.Optional(x.GetInteger(path, dict["Rotate"])); err != nil {
		return nil, err
	} else if rot%90 == 0 {
		m.Rotate = int(rot)
	}

	// Poster (optional; boolean or image XObject stream; default false)
	if posterObj := dict["Poster"]; posterObj != nil {
		resolved, _ := pdf.Resolve(x.R, posterObj)
		switch v := resolved.(type) {
		case pdf.Boolean:
			if bool(v) {
				m.Poster = PosterFromMovieFile
			}
		case *pdf.Stream:
			if img, _ := pdf.Optional(pdf.ExtractorGet(x, path, posterObj, image.ExtractDict)); img != nil {
				m.Poster = img
			}
		}
	}

	return m, nil
}

// Embed converts the movie dictionary to its PDF representation.
func (m *Movie) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "movie dictionary", pdf.V1_2); err != nil {
		return nil, err
	}
	if m.File == nil {
		return nil, errors.New("movie: File is required")
	}
	if m.Rotate%90 != 0 {
		return nil, fmt.Errorf("movie: Rotate=%d must be a multiple of 90", m.Rotate)
	}
	if m.Aspect != (Aspect{}) {
		if m.Aspect.Width <= 0 || m.Aspect.Height <= 0 {
			return nil, fmt.Errorf("movie: Aspect %dx%d must have positive dimensions", m.Aspect.Width, m.Aspect.Height)
		}
	}

	dict := pdf.Dict{}

	// F (required)
	f, err := e.Embed(m.File)
	if err != nil {
		return nil, err
	}
	dict["F"] = f

	// Aspect (optional; zero = omitted)
	if m.Aspect != (Aspect{}) {
		dict["Aspect"] = pdf.Array{
			pdf.Integer(m.Aspect.Width),
			pdf.Integer(m.Aspect.Height),
		}
	}

	// Rotate (optional; default 0)
	if m.Rotate != 0 {
		dict["Rotate"] = pdf.Integer(m.Rotate)
	}

	// Poster (optional; nil = omitted, sentinel = true, else stream)
	switch m.Poster {
	case nil:
		// omit (default false)
	case PosterFromMovieFile:
		dict["Poster"] = pdf.Boolean(true)
	default:
		posterObj, err := e.Embed(m.Poster)
		if err != nil {
			return nil, err
		}
		dict["Poster"] = posterObj
	}

	if m.SingleUse {
		return dict, nil
	}
	ref := e.Alloc()
	if err := e.Out().Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}
