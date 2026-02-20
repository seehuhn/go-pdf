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
	"math"
	"os"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/fallback"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/shading"
)

const (
	leftColStart  = 36.0
	rightColStart = 220.0
	commentStart  = 400.0

	startY  = 780.0
	stampW  = 160.0
	stampH  = 50.0
	spacing = 16.0
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

	B := standard.TimesBold.New()
	H := standard.Helvetica.New()

	w := &writer{
		page:  page,
		style: fallback.NewStyle(),
		yPos:  startY,
		font:  H,
	}

	page.TextBegin()
	page.TextSetMatrix(matrix.Translate(leftColStart, w.yPos))
	page.TextSetFont(B, 12)
	page.TextShow("Your PDF viewer")
	page.TextSetMatrix(matrix.Translate(rightColStart, w.yPos))
	page.TextShow("Quire appearance stream")
	page.TextEnd()
	w.yPos -= 24.0

	// 1. default (Draft), no color
	err = w.addStampPair(&annotation.Stamp{
		Common: annotation.Common{
			Contents: "Draft (default, no color)",
			Flags:    annotation.FlagPrint,
		},
	})
	if err != nil {
		return err
	}

	// 2. Approved
	err = w.addStampPair(&annotation.Stamp{
		Common: annotation.Common{
			Contents: "Approved",
			Flags:    annotation.FlagPrint,
		},
		Icon: annotation.StampIconApproved,
	})
	if err != nil {
		return err
	}

	// 3. Confidential
	err = w.addStampPair(&annotation.Stamp{
		Common: annotation.Common{
			Contents: "Confidential",
			Flags:    annotation.FlagPrint,
		},
		Icon: annotation.StampIconConfidential,
	})
	if err != nil {
		return err
	}

	// 4. NotForPublicRelease (longest name)
	err = w.addStampPair(&annotation.Stamp{
		Common: annotation.Common{
			Contents: "NotForPublicRelease",
			Flags:    annotation.FlagPrint,
		},
		Icon: annotation.StampIconNotForPublicRelease,
	})
	if err != nil {
		return err
	}

	// 5. TopSecret with blue color
	err = w.addStampPair(&annotation.Stamp{
		Common: annotation.Common{
			Contents: "TopSecret, blue",
			Flags:    annotation.FlagPrint,
			Color:    color.Blue,
		},
		Icon: annotation.StampIconTopSecret,
	})
	if err != nil {
		return err
	}

	// 6. Final with green color
	err = w.addStampPair(&annotation.Stamp{
		Common: annotation.Common{
			Contents: "Final, green",
			Flags:    annotation.FlagPrint,
			Color:    color.DeviceRGB{0, 0.5, 0},
		},
		Icon: annotation.StampIconFinal,
	})
	if err != nil {
		return err
	}

	// 7. Draft with border
	err = w.addStampPair(&annotation.Stamp{
		Common: annotation.Common{
			Contents: "Draft, Border.Width=3",
			Flags:    annotation.FlagPrint,
			Border:   &annotation.Border{Width: 3, SingleUse: true},
		},
		Icon: annotation.StampIconDraft,
	})
	if err != nil {
		return err
	}

	// 8. Expired
	err = w.addStampPair(&annotation.Stamp{
		Common: annotation.Common{
			Contents: "Expired",
			Flags:    annotation.FlagPrint,
		},
		Icon: annotation.StampIconExpired,
	})
	if err != nil {
		return err
	}

	return page.Close()
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

func (w *writer) addStampPair(left *annotation.Stamp) error {
	w.page.TextBegin()
	w.page.TextSetFont(w.font, 9)
	w.page.TextSetMatrix(matrix.Translate(commentStart, w.yPos-stampH/2-3))
	w.page.TextShow(left.Contents)
	w.page.TextEnd()

	right := clone(left)

	left.Rect = pdf.Rectangle{
		LLx: leftColStart,
		LLy: w.yPos - stampH,
		URx: leftColStart + stampW,
		URy: w.yPos,
	}
	left.Contents += " (viewer)"

	right.Rect = pdf.Rectangle{
		LLx: rightColStart,
		LLy: w.yPos - stampH,
		URx: rightColStart + stampW,
		URy: w.yPos,
	}
	right.Contents += " (quire)"

	err := w.style.AddAppearance(right)
	if err != nil {
		return err
	}

	w.addAnnotation(left)
	w.addAnnotation(right)

	w.yPos -= stampH + spacing
	return nil
}

func clone[T any](v *T) *T {
	if v == nil {
		return nil
	}
	clone := *v
	return &clone
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

	background := &shading.Type2{
		ColorSpace: color.SpaceDeviceRGB,
		P0:         vec.Vec2{X: pdf.Round(t0*nx, 1), Y: pdf.Round(t0*ny, 1)},
		P1:         vec.Vec2{X: pdf.Round(t1*nx, 1), Y: pdf.Round(t1*ny, 1)},
		F:          F,
		TMin:       t0,
		TMax:       t1,
	}
	return background
}
