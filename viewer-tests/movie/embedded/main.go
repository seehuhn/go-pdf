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

// Command embedded writes test.pdf, a single A5 landscape page with one
// Movie annotation (§13.4.5, removed in PDF 2.0) over the playback area.
// The clip is embedded in the PDF file.
//
// Clicking the area should play the movie in place, with the
// specification's default activation parameters.  The page layout is
// shared with the three sibling tests; see viewer-tests/internal/moviepage.
package main

import (
	"fmt"
	"os"

	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/movie"
	"seehuhn.de/go/pdf/viewer-tests/internal/moviepage"
)

const (
	title = "Movie annotation, embedded clip"
	what  = "Click the outlined area: the movie should play in place, with the default activation parameters."
	how   = "The clip is embedded in the PDF file, so a viewer which supports the annotation has the data."
	note  = "The Movie annotation has no operations of its own; the media tests drive playback with rendition actions."
)

func main() {
	if err := createDocument("test.pdf"); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func createDocument(filename string) error {
	page, err := moviepage.New(filename, title, what, how)
	if err != nil {
		return err
	}
	page.Note(note)

	annot := &annotation.Movie{
		Common: annotation.Common{
			Rect:     moviepage.Screen,
			Contents: moviepage.Description + ", embedded in the PDF file.",
			Border:   annotation.PDFDefaultBorder,
			Flags:    annotation.FlagPrint,
		},
		Title:      "Tick movie",
		Movie:      newMovie(),
		Activation: movie.DefaultActivation,
	}
	page.Add(annot)

	return page.Close()
}

// newMovie builds the movie whose file specification carries the clip in
// an embedded file stream.
func newMovie() *movie.Movie {
	return &movie.Movie{
		File:   moviepage.EmbeddedSpec(),
		Aspect: movie.Aspect{Width: 320, Height: 240},
	}
}
