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

// Command external writes test.pdf, a single A5 landscape page with a
// Screen annotation (§13.4.5) over the playback area, driven by rendition
// actions (§12.6.4.10).  The clip is not embedded: the media clip names
// movie.mp4 relative to the PDF file.
//
// Clicking the area plays the movie in place with the player's controller
// UI; the buttons below exercise the operation codes, all targeting the
// screen annotation through the action's AN entry.  Open test.pdf from
// within this directory, where the committed symlink movie.mp4 points at
// the shared clip.  The page layout is shared with the three sibling
// tests; see viewer-tests/internal/moviepage.
package main

import (
	"fmt"
	"os"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/media"
	"seehuhn.de/go/pdf/optional"
	"seehuhn.de/go/pdf/viewer-tests/internal/moviepage"
)

const (
	title = "Rendition action, external clip"
	what  = "Click the outlined area to play the movie in place; the buttons drive the rendition operations."
	how   = "The clip is not embedded; the viewer has to resolve movie.mp4 next to test.pdf."
)

// the operations under test; OP 0 and 4 need a rendition, the rest act on
// the player which is already running
var buttons = []struct {
	label string
	op    uint
}{
	{"Play", 0},
	{"Pause", 2},
	{"Resume", 3},
	{"Stop", 1},
	{"Play / Resume", 4},
}

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

	// one shared rendition, referenced by every play action
	rend := newRendition()

	screen := &annotation.Screen{
		Common: annotation.Common{
			Rect:     moviepage.Screen,
			Contents: moviepage.Description + ", external file movie.mp4.",
			Border:   annotation.PDFDefaultBorder,
			Flags:    annotation.FlagPrint,
		},
		Title: "Tick movie",
	}
	// reserve the reference up front, so the actions can target it via AN
	screenRef := page.Doc.RM.GetReference(screen)
	screen.Action = renditionAction(rend, screenRef, 0) // play
	page.Add(screen)

	labels := make([]string, len(buttons))
	for i, b := range buttons {
		labels[i] = b.label
	}
	rects := page.Buttons(labels)

	for i, b := range buttons {
		var actionRend media.Rendition
		if b.op == 0 || b.op == 4 {
			actionRend = rend
		}
		// the button face is drawn into the page content; the annotation is
		// only an invisible hotspot firing the action, so a borderless Link
		// is the right type (a Screen would reserve a playback region it
		// does not have)
		page.Add(&annotation.Link{
			Common: annotation.Common{
				Rect:     rects[i],
				Contents: b.label,
				Flags:    annotation.FlagPrint,
			},
			Action: renditionAction(actionRend, screenRef, b.op),
		})
	}

	return page.Close()
}

// renditionAction builds a rendition action with the given operation code,
// targeting the screen annotation referenced by an.  r may be nil for the
// operations which act on the running player.
func renditionAction(r media.Rendition, an pdf.Reference, op uint) *action.Rendition {
	return &action.Rendition{
		R:  r,
		AN: an,
		OP: optional.NewUInt(op),
	}
}

// newRendition builds the rendition which plays the external file
// movie.mp4, with the player's controller UI shown.
func newRendition() *media.MediaRendition {
	return &media.MediaRendition{
		RenditionCommon: media.RenditionCommon{Name: "Tick movie"},
		Clip: &media.MediaClipData{
			Name:        "Tick movie",
			DataFile:    moviepage.ExternalSpec(),
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
