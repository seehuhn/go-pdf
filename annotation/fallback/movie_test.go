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

package fallback

import (
	"testing"

	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/movie"
)

// stubImage is a minimal graphics.Image for exercising the poster branch
// without embedding a real image.
type stubImage struct{}

func (stubImage) Subtype() pdf.Name                          { return "Image" }
func (stubImage) ResourceName() pdf.Name                     { return "" }
func (stubImage) Bounds() rect.IntRect                       { return rect.IntRect{} }
func (stubImage) Embed(*pdf.EmbedHelper) (pdf.Native, error) { return pdf.Integer(0), nil }

var mediaRect = pdf.Rectangle{LLx: 10, LLy: 20, URx: 210, URy: 120}

func TestMoviePoster(t *testing.T) {
	s := NewStyle()
	poster := stubImage{}
	a := &annotation.Movie{
		Common: annotation.Common{Rect: mediaRect},
		Movie:  &movie.Movie{Poster: poster},
	}

	f := s.addMovieAppearance(a)

	if f.BBox != mediaRect {
		t.Errorf("BBox = %v, want %v", f.BBox, mediaRect)
	}
	if got := len(f.Res.XObject); got != 1 {
		t.Fatalf("XObject count = %d, want 1 (the poster)", got)
	}
	for _, x := range f.Res.XObject {
		if x != poster {
			t.Error("poster was copied, not reused (breaks resource dedup)")
		}
	}
}

func TestMoviePlaceholder(t *testing.T) {
	s := NewStyle()
	cases := map[string]*movie.Movie{
		"nil movie":        nil,
		"nil poster":       {},
		"poster from file": {Poster: movie.PosterFromMovieFile},
	}
	for name, m := range cases {
		t.Run(name, func(t *testing.T) {
			a := &annotation.Movie{
				Common: annotation.Common{Rect: mediaRect},
				Movie:  m,
			}
			f := s.addMovieAppearance(a)

			if f.BBox != mediaRect {
				t.Errorf("BBox = %v, want %v", f.BBox, mediaRect)
			}
			if len(f.Res.XObject) != 0 {
				t.Errorf("placeholder must not reference an XObject, got %d", len(f.Res.XObject))
			}
			if f.Content == nil {
				t.Error("placeholder content is empty")
			}
		})
	}
}

func TestMovieZeroRect(t *testing.T) {
	s := NewStyle()
	zero := pdf.Rectangle{LLx: 5, LLy: 5, URx: 5, URy: 5}
	a := &annotation.Movie{Common: annotation.Common{Rect: zero}}

	f := s.addMovieAppearance(a)

	if f.Content != nil {
		t.Error("zero-area Rect should produce empty content")
	}
	if f.BBox != zero {
		t.Errorf("BBox = %v, want %v", f.BBox, zero)
	}
}
