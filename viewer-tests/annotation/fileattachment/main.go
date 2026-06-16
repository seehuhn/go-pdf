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

package main

import (
	"fmt"
	"io"
	"math"
	"os"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/fallback"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/shading"
)

const (
	// horizontal spacing
	leftColStart  = 60.0
	leftColEnd    = 160.0
	rightColStart = 220.0
	rightColEnd   = 320.0
	commentStart  = 380.0

	// vertical spacing
	startY   = 780.0
	iconSize = 24.0
)

func main() {
	err := createDocument("test.pdf")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func createDocument(filename string) error {
	paper := document.A4
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	page, err := document.CreateSinglePage(filename, paper, pdf.V1_7, opt)
	if err != nil {
		return err
	}

	page.DrawShading(pageBackground(paper))

	B := font.Must(standard.TimesBold.New())
	H := font.Must(standard.Helvetica.New())

	w := &writer{
		page:  page,
		style: fallback.NewStyle(pdf.V1_7),
		yPos:  startY,
		font:  H,
	}

	// column headers
	page.TextBegin()
	page.TextSetMatrix(matrix.Translate(leftColStart-5, w.yPos))
	page.TextSetFont(B, 12)
	page.TextShow("Your PDF viewer")
	page.TextSetMatrix(matrix.Translate(rightColStart-5, w.yPos))
	page.TextShow("Quire appearance stream")
	page.TextEnd()
	w.yPos -= 24.0

	// all four standard icons with default colors
	allIcons := []annotation.FileAttachmentIcon{
		annotation.FileAttachmentIconPushPin,
		annotation.FileAttachmentIconPaperclip,
		annotation.FileAttachmentIconGraph,
		annotation.FileAttachmentIconTag,
	}
	for _, icon := range allIcons {
		fa := &annotation.FileAttachment{
			Common: annotation.Common{
				Contents: fmt.Sprintf("Icon: %s", icon),
				Border:   annotation.PDFDefaultBorder,
				Flags:    annotation.FlagPrint,
			},
			Markup: annotation.Markup{
				User: "Test User",
			},
			Icon: icon,
			FS:   sampleFile(fmt.Sprintf("%s-sample.txt", icon)),
		}
		err = w.addPair(fa, string(icon))
		if err != nil {
			return err
		}
	}

	// red-tinted push pin
	red := color.DeviceRGB{0.8, 0.15, 0.15}
	fa := &annotation.FileAttachment{
		Common: annotation.Common{
			Contents: "Red push pin",
			Color:    red,
			Border:   annotation.PDFDefaultBorder,
			Flags:    annotation.FlagPrint,
		},
		Markup: annotation.Markup{
			User: "Test User",
		},
		Icon: annotation.FileAttachmentIconPushPin,
		FS:   sampleFile("red-pin.txt"),
	}
	if err = w.addPair(fa, "Common.Color = red"); err != nil {
		return err
	}

	// blue-tinted paperclip
	blue := color.DeviceRGB{0.1, 0.3, 0.75}
	fa = &annotation.FileAttachment{
		Common: annotation.Common{
			Contents: "Blue paperclip",
			Color:    blue,
			Border:   annotation.PDFDefaultBorder,
			Flags:    annotation.FlagPrint,
		},
		Markup: annotation.Markup{
			User: "Test User",
		},
		Icon: annotation.FileAttachmentIconPaperclip,
		FS:   sampleFile("blue-clip.txt"),
	}
	if err = w.addPair(fa, "Common.Color = blue"); err != nil {
		return err
	}

	// tag with a thicker border
	fa = &annotation.FileAttachment{
		Common: annotation.Common{
			Contents: "Tag with border width 2",
			Border:   &annotation.Border{Width: 2, SingleUse: true},
			Flags:    annotation.FlagPrint,
		},
		Markup: annotation.Markup{
			User: "Test User",
		},
		Icon: annotation.FileAttachmentIconTag,
		FS:   sampleFile("tagged.txt"),
	}
	if err = w.addPair(fa, "Common.Border.Width = 2"); err != nil {
		return err
	}

	// no-icon default (empty Icon name falls back to PushPin)
	fa = &annotation.FileAttachment{
		Common: annotation.Common{
			Contents: "Unset Icon (defaults to PushPin)",
			Border:   annotation.PDFDefaultBorder,
			Flags:    annotation.FlagPrint,
		},
		Markup: annotation.Markup{
			User: "Test User",
		},
		FS: sampleFile("default.txt"),
	}
	if err = w.addPair(fa, "Icon: (unset)"); err != nil {
		return err
	}

	return page.Close()
}

// sampleFile returns a file specification with a small embedded text file.
// This gives each annotation real attached content so that activating it
// in a viewer offers a file to extract.
func sampleFile(name string) *file.Specification {
	content := fmt.Sprintf("Sample content for %s\n", name)
	stream := &file.Stream{
		MimeType: "text/plain",
		Size:     int64(len(content)),
		WriteData: func(w io.Writer) error {
			_, err := w.Write([]byte(content))
			return err
		},
	}
	return &file.Specification{
		FileName:        name,
		FileNameUnicode: name,
		Description:     "Sample attachment created by the Quire test generator.",
		EmbeddedFiles:   map[string]*file.Stream{"F": stream, "UF": stream},
	}
}

type writer struct {
	page  *document.Page
	style *fallback.Style
	yPos  float64
	font  font.Layouter
}

func (w *writer) addAnnotation(a annotation.Annotation) {
	w.page.Page.Annots = append(w.page.Page.Annots, a)
}

func (w *writer) addPair(left *annotation.FileAttachment, label string) error {
	leftCenter := (leftColStart + leftColEnd) / 2
	rightCenter := (rightColStart + rightColEnd) / 2

	w.page.TextBegin()
	w.page.TextSetFont(w.font, 10)
	w.page.TextSetMatrix(matrix.Translate(commentStart, w.yPos-iconSize/2-3))
	w.page.TextShow(label)
	w.page.TextEnd()

	right := clone(left)

	left.Rect = pdf.Rectangle{
		LLx: leftCenter - iconSize/2,
		LLy: w.yPos - iconSize,
		URx: leftCenter + iconSize/2,
		URy: w.yPos,
	}
	left.Contents += " (viewer)"

	right.Rect = pdf.Rectangle{
		LLx: rightCenter - iconSize/2,
		LLy: w.yPos - iconSize,
		URx: rightCenter + iconSize/2,
		URy: w.yPos,
	}
	right.Contents += " (quire)"

	if err := w.style.AddAppearance(right); err != nil {
		return err
	}

	w.addAnnotation(left)
	w.addAnnotation(right)

	w.yPos -= iconSize + 12.0
	return nil
}

func clone[T any](v *T) *T {
	if v == nil {
		return nil
	}
	c := *v
	return &c
}

func pageBackground(paper *pdf.Rectangle) graphics.Shading {
	alpha := 30.0 / 360 * 2 * math.Pi

	nx := math.Cos(alpha)
	ny := math.Sin(alpha)

	t0 := pdf.Round(paper.LLx*nx+paper.LLy*ny, 1)
	t1 := pdf.Round(paper.URx*nx+paper.URy*ny, 1)

	F := &function.Type4{
		Domain:  []float64{t0, t1},
		Range:   []float64{0, 1, 0, 1, 0, 1},
		Program: "dup 16 div floor 16 mul sub 8 ge {0.99 0.98 0.95}{0.96 0.94 0.89}ifelse",
	}

	return &shading.Type2{
		ColorSpace: color.SpaceDeviceRGB,
		P0:         vec.Vec2{X: pdf.Round(t0*nx, 1), Y: pdf.Round(t0*ny, 1)},
		P1:         vec.Vec2{X: pdf.Round(t1*nx, 1), Y: pdf.Round(t1*ny, 1)},
		F:          F,
		TMin:       t0,
		TMax:       t1,
	}
}
