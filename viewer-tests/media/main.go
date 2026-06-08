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

//go:generate go run gen_movie.go

// Command media writes test.pdf, a single landscape (16:9 presentation)
// page driving an embedded H.264/AAC movie through a Screen annotation and
// a Rendition action.  The movie carries a video timestamp counter and an
// audio track that ticks once per second.
//
// The large screen rectangle plays the movie in place (rendition OP 0).  A
// row of buttons below it exercises the rendition operation codes the viewer
// supports, all targeting the screen annotation via the action's AN entry:
// Play (0), Pause (2), Resume (3), Stop (1), and Play/Resume (4).
//
// The movie itself is generated separately by gen_movie.go (invoked via
// `go generate`) and embedded here via go:embed.
package main

import (
	_ "embed"
	"fmt"
	"io"
	"os"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/media"
	"seehuhn.de/go/pdf/optional"
)

//go:embed movie.mp4
var movieData []byte

func main() {
	if err := createDocument("test.pdf"); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func createDocument(filename string) error {
	// landscape 16:9 presentation page (PowerPoint widescreen, 13.333"x7.5")
	paper := &pdf.Rectangle{URx: 960, URy: 540}
	opt := &pdf.WriterOptions{HumanReadable: true}
	page, err := document.CreateSinglePage(filename, paper, pdf.V1_7, opt)
	if err != nil {
		return err
	}

	titleFont := font.Must(standard.HelveticaBold.New())
	bodyFont := font.Must(standard.Helvetica.New())

	page.TextBegin()
	page.TextSetFont(titleFont, 20)
	page.TextSetMatrix(matrix.Translate(80, paper.URy-50))
	page.TextShow("Rendition action test")
	page.TextSetFont(bodyFont, 12)
	page.TextSetMatrix(matrix.Translate(80, paper.URy-72))
	page.TextShow("Click the screen to play. The movie ticks once per second; use the buttons to control playback.")
	page.TextEnd()

	// the screen annotation that hosts playback (16:9, below the caption and
	// above the button row); reserve its reference up front so the rendition
	// actions can target it via AN
	screenRect := pdf.Rectangle{LLx: 180, LLy: 107.5, URx: 780, URy: 445}

	// one shared rendition: embedded once, referenced by every play action
	rend := newRendition()

	screen := &annotation.Screen{
		Common: annotation.Common{
			Rect:     screenRect,
			Contents: "Tick movie (1280x720, H.264/AAC, 30 s).",
			Border:   annotation.PDFDefaultBorder,
			Flags:    annotation.FlagPrint,
		},
		Title: "Tick movie",
	}
	screenRef := page.RM.GetReference(screen)
	screen.Action = renditionAction(rend, screenRef, 0) // play
	page.Page.Annots = append(page.Page.Annots, screen)

	// control buttons; OP 0 and 4 need a rendition, the rest act on the
	// active player
	buttons := []struct {
		label string
		op    uint
	}{
		{"Play", 0},
		{"Pause", 2},
		{"Resume", 3},
		{"Stop", 1},
		{"Play / Resume", 4},
	}

	const (
		btnW = 140.0
		btnH = 36.0
		gap  = 24.0
		btnY = 50.0
	)
	totalW := float64(len(buttons))*btnW + float64(len(buttons)-1)*gap
	x := (paper.URx - totalW) / 2

	for _, b := range buttons {
		r := pdf.Rectangle{LLx: x, LLy: btnY, URx: x + btnW, URy: btnY + btnH}
		drawButton(page, bodyFont, r, b.label)

		var actionRend media.Rendition
		if b.op == 0 || b.op == 4 {
			actionRend = rend
		}
		// the button face is drawn into the page content by drawButton; the
		// annotation is just an invisible hotspot that fires the rendition
		// action, so a borderless Link is the right type (a Screen would
		// reserve a playback region it does not have).
		btn := &annotation.Link{
			Common: annotation.Common{
				Rect:     r,
				Contents: b.label,
				Flags:    annotation.FlagPrint,
			},
			Action: renditionAction(actionRend, screenRef, b.op),
		}
		page.Page.Annots = append(page.Page.Annots, btn)

		x += btnW + gap
	}

	return page.Close()
}

// renditionAction builds a rendition action with the given operation code,
// targeting the screen annotation referenced by an.  r may be nil for the
// stop/pause/resume operations, which act on the active player.
func renditionAction(r media.Rendition, an pdf.Reference, op uint) *action.Rendition {
	return &action.Rendition{
		R:  r,
		AN: an,
		OP: optional.NewUInt(op),
	}
}

// newRendition builds the media rendition that plays the embedded movie with
// the player's controller UI shown.
func newRendition() *media.MediaRendition {
	stream := &file.Stream{
		MimeType: "video/mp4",
		Size:     int64(len(movieData)),
		WriteData: func(w io.Writer) error {
			_, err := w.Write(movieData)
			return err
		},
	}
	spec := &file.Specification{
		FileName:        "movie.mp4",
		FileNameUnicode: "movie.mp4",
		Description:     "Tick movie (1280x720, H.264/AAC, ticks once per second)",
		EmbeddedFiles:   map[string]*file.Stream{"F": stream, "UF": stream},
	}
	return &media.MediaRendition{
		RenditionCommon: media.RenditionCommon{Name: "Tick movie"},
		Clip: &media.MediaClipData{
			Name:        "Tick movie",
			DataFile:    spec,
			ContentType: "video/mp4",
		},
		Play: &media.MediaPlayParameters{
			MustHonour: &media.MediaPlayEntries{
				Controller: optional.NewBool(true),
				Fit:        media.FitMeet,
			},
		},
	}
}

// drawButton draws a light-grey labelled button rectangle.
func drawButton(page *document.Page, f font.Instance, r pdf.Rectangle, label string) {
	page.SetFillColor(color.DeviceGray(0.9))
	page.SetStrokeColor(color.Black)
	page.SetLineWidth(0.75)
	page.Rectangle(r.LLx, r.LLy, r.URx-r.LLx, r.URy-r.LLy)
	page.FillAndStroke()

	page.SetFillColor(color.Black)
	page.TextBegin()
	page.TextSetFont(f, 13)
	page.TextSetMatrix(matrix.Translate(r.LLx, (r.LLy+r.URy)/2-4))
	page.TextShowAligned(label, r.URx-r.LLx, 0.5)
	page.TextEnd()
}
