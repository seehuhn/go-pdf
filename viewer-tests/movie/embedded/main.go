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

// Command embedded writes test.pdf, a single-page A4 document containing
// one Movie annotation playing an H.264/MP4 timestamp-counter movie
// embedded into the PDF file.  Clicking the annotation should play the
// movie in place with the PDF specification's default activation
// parameters.
//
// The movie data is shared with the other media tests and comes from
// the viewer-tests/internal/testmovie package.
package main

import (
	"fmt"
	"io"
	"os"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/movie"
	"seehuhn.de/go/pdf/viewer-tests/internal/testmovie"
)

const (
	movieW = 320.0
	movieH = 240.0
)

func main() {
	if err := createDocument("test.pdf"); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func createDocument(filename string) error {
	paper := document.A4
	opt := &pdf.WriterOptions{HumanReadable: true}
	page, err := document.CreateSinglePage(filename, paper, pdf.V1_7, opt)
	if err != nil {
		return err
	}

	captionFont := font.Must(standard.Helvetica.New())
	captionX := pdf.Round((paper.URx-movieW)/2, 2) // left-align with movie rect
	captionY := pdf.Round(paper.URy-80, 2)

	page.TextBegin()
	page.TextSetFont(captionFont, 12)
	page.TextSetMatrix(matrix.Translate(captionX, captionY))
	page.TextShow("Click the rectangle to play the embedded test movie.")
	page.TextEnd()

	rect := pdf.Rectangle{
		LLx: captionX,
		URx: captionX + movieW,
		LLy: captionY - 16 - movieH,
		URy: captionY - 16,
	}

	annot := &annotation.Movie{
		Common: annotation.Common{
			Rect:     rect,
			Contents: "Timestamp counter (751 frames @ 25 fps), embedded in the PDF file.",
			Border:   annotation.PDFDefaultBorder,
			Flags:    annotation.FlagPrint,
		},
		Title:      "Test movie",
		Movie:      newTestMovie(),
		Activation: movie.DefaultActivation,
	}
	page.Page.Annots = append(page.Page.Annots, annot)

	return page.Close()
}

// newTestMovie builds a *movie.Movie whose file specification embeds
// the shared test movie data.
func newTestMovie() *movie.Movie {
	stream := &file.Stream{
		MimeType: "video/mp4",
		Size:     int64(len(testmovie.Data())),
		WriteData: func(w io.Writer) error {
			_, err := w.Write(testmovie.Data())
			return err
		},
	}
	spec := &file.Specification{
		FileName:        "movie.mp4",
		FileNameUnicode: "movie.mp4",
		Description:     "Timestamp counter movie (751 frames at 25 fps, 320x240, H.264/MP4)",
		EmbeddedFiles:   map[string]*file.Stream{"F": stream, "UF": stream},
	}
	return &movie.Movie{
		File:   spec,
		Aspect: movie.Aspect{Width: int(movieW), Height: int(movieH)},
	}
}
