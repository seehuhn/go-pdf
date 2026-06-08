// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/fallback"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/shading"
)

func main() {
	err := withoutAP("A.pdf")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	err = withAP("B.pdf")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// withoutAP generates a PDF 1.7 file where ca=CA and appearance streams are
// left to the viewer.
func withoutAP(filename string) error {
	paper := &pdf.Rectangle{
		URx: 500,
		URy: 500,
	}
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	page, err := document.CreateSinglePage(filename, paper, pdf.V1_7, opt)
	if err != nil {
		return err
	}

	page.DrawShading(pageBackground(paper))

	for CA := 0.1; CA <= 1.0; CA += 0.2 {
		x := CA * 500
		y := CA * 500

		const d = 32.0

		text := fmt.Sprintf("CA=%.1f, ca=%.1f", CA, CA)
		rect := pdf.Rectangle{
			LLx: x - d,
			LLy: y - d,
			URx: x + d,
			URy: y + d,
		}

		a := &annotation.Square{
			Common: annotation.Common{
				Rect: rect.Round(1),
				Border: &annotation.Border{
					Width:     8,
					SingleUse: true,
				},
				Color:                   color.DeviceRGB{0.4975, 0.9333, 0.2483},
				NonStrokingTransparency: 1 - CA,
				StrokingTransparency:    1 - CA,
				Contents:                text,
				Flags:                   annotation.FlagPrint,
			},
			FillColor: color.DeviceRGB{0.5056, 0.9555, 0.9956},
		}

		page.Page.Annots = append(page.Page.Annots, a)
	}

	return page.Close()
}

// withAP generates a PDF 2.0 file where ca and CA vary independently.
// PDF 2.0 requires appearance streams for all annotations.
func withAP(filename string) error {
	paper := &pdf.Rectangle{
		URx: 500,
		URy: 500,
	}
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	page, err := document.CreateSinglePage(filename, paper, pdf.V2_0, opt)
	if err != nil {
		return err
	}

	page.DrawShading(pageBackground(paper))

	style := fallback.NewStyle()

	for ca := 0.1; ca <= 1.0; ca += 0.2 {
		for CA := 0.1; CA <= 1.0; CA += 0.2 {
			x := CA * 500
			y := ca * 500

			const d = 32.0

			text := fmt.Sprintf("CA=%.1f, ca=%.1f", CA, ca)
			rect := pdf.Rectangle{
				LLx: x - d,
				LLy: y - d,
				URx: x + d,
				URy: y + d,
			}

			a := &annotation.Square{
				Common: annotation.Common{
					Rect: rect.Round(1),
					Border: &annotation.Border{
						Width:     8,
						SingleUse: true,
					},
					Color:                   color.DeviceRGB{0.4975, 0.9333, 0.2483},
					NonStrokingTransparency: 1 - ca,
					StrokingTransparency:    1 - CA,
					Contents:                text,
					Flags:                   annotation.FlagPrint,
				},
				FillColor: color.DeviceRGB{0.5056, 0.9555, 0.9956},
			}
			err = style.AddAppearance(a)
			if err != nil {
				return err
			}

			page.Page.Annots = append(page.Page.Annots, a)
		}
	}

	return page.Close()
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
		Program: "dup 16 div floor 16 mul sub 8 ge {0.1915 0.1816 0.5348}{0.9047 0.1464 0.1457}ifelse",
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
