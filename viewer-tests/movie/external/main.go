// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Command external writes test.pdf, a single A5 landscape page with one
// Movie annotation (§13.4.5, removed in PDF 2.0) over the playback area.
// The clip is not embedded: the file specification names movie.mp4
// relative to the PDF file.
//
// Open test.pdf from within this directory, where the committed symlink
// movie.mp4 points at the shared clip.  The page layout is shared with
// the three sibling tests; see viewer-tests/internal/moviepage.
package main

import (
	"fmt"
	"os"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/movie"
	"seehuhn.de/go/pdf/viewer-tests/internal/moviepage"
)

const (
	title = "Movie annotation, external clip"
	what  = "Click the outlined area: the movie should play in place, with the default activation parameters."
	how   = "The clip is not embedded; the viewer has to resolve movie.mp4 next to test.pdf."
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
		Rect:       moviepage.Screen,
		Contents:   pdf.Text(moviepage.Description + ", external file movie.mp4."),
		Border:     &annotation.Border{Width: 1},
		Flags:      annotation.FlagPrint,
		Title:      pdf.Text("Tick movie"),
		Movie:      newMovie(),
		Activation: movie.DefaultActivation,
	}
	page.Add(annot)

	return page.Close()
}

// newMovie builds the movie whose file specification names the external
// file movie.mp4, without embedding data.
func newMovie() *movie.Movie {
	return &movie.Movie{
		File:   moviepage.ExternalSpec(),
		Aspect: movie.Aspect{Width: 320, Height: 240},
	}
}
