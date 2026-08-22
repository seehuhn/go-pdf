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

// Package moviepage draws the page shared by the four movie and media
// viewer tests.
//
// The four tests form a two-by-two survey: the deprecated Movie
// annotation against Screen annotations driven by rendition actions,
// each with the clip embedded in the PDF file and referenced as an
// external file.  All four pages use the same A5 landscape layout, so
// that a viewer's treatment of one can be compared against the others
// without allowing for a change of page geometry.
package moviepage

import (
	"io"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/viewer-tests/internal/testmovie"
)

// FileName is the name the clip is given inside the PDF file, and the
// name the external variants expect to find next to test.pdf.
const FileName = "movie.mp4"

// Description describes the clip, for the file specification.
const Description = "Tick movie (320x240, H.264/AAC, 30 s, ticks once per second)"

// Screen geometry: 4:3, matching the clip, centred on the page.  At A5
// the clip is shown at its native 320x240, which is as tall as the page
// allows once the caption and the button row have their space.
const (
	screenW = 320.0
	screenH = 240.0
	screenY = 90.0
)

// vertical positions of the page furniture
const (
	titleY  = 384.0
	whatY   = 362.0
	howY    = 348.0
	bandY   = 48.0 // button row and the note which replaces it
	bandH   = 28.0
	footerY = 24.0
)

// The button row is wider than the screen: five labels do not fit across
// 320pt, and the wider row lines up with the caption above instead.
const bandW = 400.0

// type sizes
const (
	titleSize  = 15.0
	bodySize   = 10.0
	noteSize   = 9.0
	buttonSize = 9.0
	footerSize = 8.0
)

// Screen is the rectangle which hosts playback.  It is the same on all
// four pages.
var Screen = pdf.Rectangle{
	LLx: pdf.Round((document.A5r.URx-screenW)/2, 2),
	LLy: screenY,
	URx: pdf.Round((document.A5r.URx+screenW)/2, 2),
	URy: screenY + screenH,
}

// footer is shown on all four pages, below the button row.
const footer = "The picture counts the playback time; the sound ticks once per second.  " +
	"Watch whether the two stay in step."

// Page is a test page under construction.
type Page struct {
	// Doc is the underlying page, for the resource manager and the page
	// dictionary.
	Doc *document.Page

	body font.Instance
}

// New creates filename as a single A5 landscape page and draws the
// furniture shared by all four tests: the title, the two lines
// describing what is being tested and how the clip is delivered, an
// outline marking the playback area, and the footer.
//
// The caller adds the annotation under test over Screen, fills the band
// below it with Note or Buttons, and closes the page.
func New(filename, title, what, how string) (*Page, error) {
	paper := document.A5r
	opt := &pdf.WriterOptions{HumanReadable: true}
	page, err := document.CreateSinglePage(filename, paper, pdf.V1_7, opt)
	if err != nil {
		return nil, err
	}

	titleFont := font.Must(standard.HelveticaBold.New())
	bodyFont := font.Must(standard.Helvetica.New())

	page.TextBegin()
	page.TextSetFont(titleFont, titleSize)
	page.TextSetMatrix(matrix.Translate(0, titleY))
	page.TextShowAligned(title, paper.URx, 0.5)
	page.TextSetFont(bodyFont, bodySize)
	page.TextSetMatrix(matrix.Translate(0, whatY))
	page.TextShowAligned(what, paper.URx, 0.5)
	page.TextSetMatrix(matrix.Translate(0, howY))
	page.TextShowAligned(how, paper.URx, 0.5)
	page.TextEnd()

	// mark the playback area, so that the page still shows where the
	// movie belongs in a viewer which draws nothing there
	page.SetStrokeColor(color.DeviceGray(0.6))
	page.SetLineWidth(0.5)
	page.Rectangle(Screen.LLx, Screen.LLy, Screen.Dx(), Screen.Dy())
	page.Stroke()

	page.TextBegin()
	page.TextSetFont(bodyFont, footerSize)
	page.TextSetMatrix(matrix.Translate(0, footerY))
	page.TextShowAligned(footer, paper.URx, 0.5)
	page.TextEnd()

	return &Page{Doc: page, body: bodyFont}, nil
}

// Note fills the band below the screen with a centred line of text.
// The movie tests use this where the media tests put their buttons.
func (p *Page) Note(text string) {
	page := p.Doc
	page.SetFillColor(color.DeviceGray(0.35))
	page.TextBegin()
	page.TextSetFont(p.body, noteSize)
	page.TextSetMatrix(matrix.Translate(0, bandY+bandH/2-noteSize/3))
	page.TextShowAligned(text, document.A5r.URx, 0.5)
	page.TextEnd()
	page.SetFillColor(color.Black)
}

// Buttons draws a row of labelled buttons in the band below the screen
// and returns their rectangles in the order the labels were given.  The caller turns each rectangle into a
// hotspot by adding an annotation over it.
func (p *Page) Buttons(labels []string) []pdf.Rectangle {
	const gap = 8.0

	n := float64(len(labels))
	w := pdf.Round((bandW-(n-1)*gap)/n, 2)
	x0 := pdf.Round((document.A5r.URx-bandW)/2, 2)

	page := p.Doc
	rects := make([]pdf.Rectangle, len(labels))
	for i, label := range labels {
		x := pdf.Round(x0+float64(i)*(w+gap), 2)
		r := pdf.Rectangle{LLx: x, LLy: bandY, URx: x + w, URy: bandY + bandH}
		rects[i] = r

		page.SetFillColor(color.DeviceGray(0.9))
		page.SetStrokeColor(color.Black)
		page.SetLineWidth(0.75)
		page.Rectangle(r.LLx, r.LLy, r.URx-r.LLx, r.URy-r.LLy)
		page.FillAndStroke()

		page.SetFillColor(color.Black)
		page.TextBegin()
		page.TextSetFont(p.body, buttonSize)
		page.TextSetMatrix(matrix.Translate(r.LLx, (r.LLy+r.URy)/2-buttonSize/3))
		page.TextShowAligned(label, r.URx-r.LLx, 0.5)
		page.TextEnd()
	}
	return rects
}

// Add places annotations on the page.
func (p *Page) Add(annots ...annotation.Annotation) {
	p.Doc.Page.Annots = append(p.Doc.Page.Annots, annots...)
}

// Close writes the page and closes the file.
func (p *Page) Close() error {
	return p.Doc.Close()
}

// EmbeddedSpec returns a file specification which carries the clip in an
// embedded file stream, so that every conformant viewer has the bytes.
func EmbeddedSpec() *file.Specification {
	data := testmovie.Data()
	stream := &file.Stream{
		MimeType: "video/mp4",
		Size:     int64(len(data)),
		WriteData: func(w io.Writer) error {
			_, err := w.Write(data)
			return err
		},
	}
	return &file.Specification{
		FileName:        FileName,
		FileNameUnicode: FileName,
		Description:     Description,
		EmbeddedFiles:   map[string]*file.Stream{"F": stream, "UF": stream},
	}
}

// ExternalSpec returns a file specification which names the clip without
// embedding it, leaving the viewer to resolve movie.mp4 relative to the
// PDF file.
func ExternalSpec() *file.Specification {
	return &file.Specification{
		FileName:        FileName,
		FileNameUnicode: FileName,
		Description:     Description,
	}
}
