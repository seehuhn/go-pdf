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

const (
	// column positions
	leftColStart  = 100.0
	leftColEnd    = 160.0
	rightColStart = 220.0
	rightColEnd   = 280.0

	// annotation size
	squareSize = 24.0

	// vertical spacing
	startY   = 750.0
	rowStep  = 60.0
	groupGap = 80.0

	// default properties
	defaultLineWidth = 1.5
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

	background, err := pageBackground(paper)
	if err != nil {
		return err
	}
	page.DrawShading(background)

	w := &writer{
		annots:   pdf.Array{},
		page:     page,
		style:    fallback.NewStyle(),
		currentY: startY,
	}

	// Group 1: Basic fill colors
	colors := []struct {
		name  string
		color color.Color
	}{
		{"No fill", nil},
		{"Red fill", color.Red},
		{"Gray fill", color.DeviceGray(0.7)},
		{"CMYK fill", color.DeviceCMYK(0.3, 0.7, 0.0, 0.1)},
	}

	for _, c := range colors {
		square := &annotation.Square{
			Common: annotation.Common{
				Rect: pdf.Rectangle{
					LLx: leftColStart,
					LLy: w.currentY - squareSize,
					URx: leftColEnd,
					URy: w.currentY,
				},
				Contents: c.name,
				Color:    color.Black,
				Flags:    annotation.FlagPrint,
			},
			FillColor: c.color,
			BorderStyle: &annotation.BorderStyle{
				Width:     defaultLineWidth,
				SingleUse: true,
			},
		}
		err = w.addAnnotationPair(square)
		if err != nil {
			return err
		}

		w.currentY -= rowStep
	}

	// Group 2: Border style comparison
	w.currentY -= groupGap

	// Common.Border with dash
	borderSquare1 := &annotation.Square{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: leftColStart,
				LLy: w.currentY - squareSize,
				URx: leftColEnd,
				URy: w.currentY,
			},
			Contents: "Common.Border with dash",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
			Border:   &annotation.Border{Width: 2, DashArray: []float64{8, 3}, SingleUse: true},
		},
		FillColor: color.DeviceRGB(0.9, 0.9, 0.9),
	}
	err = w.addAnnotationPair(borderSquare1)
	if err != nil {
		return err
	}

	w.currentY -= rowStep

	// BorderStyle with dash
	borderSquare2 := &annotation.Square{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: leftColStart,
				LLy: w.currentY - squareSize,
				URx: leftColEnd,
				URy: w.currentY,
			},
			Contents: "BorderStyle with dash",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
		},
		FillColor: color.DeviceRGB(0.9, 0.9, 0.9),
		BorderStyle: &annotation.BorderStyle{
			Width:     2,
			Style:     "D",
			DashArray: []float64{8, 3},
			SingleUse: true,
		},
	}
	err = w.addAnnotationPair(borderSquare2)
	if err != nil {
		return err
	}

	w.currentY -= rowStep

	// No border style specified
	borderSquare3 := &annotation.Square{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: leftColStart,
				LLy: w.currentY - squareSize,
				URx: leftColEnd,
				URy: w.currentY,
			},
			Contents: "No border style",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
		},
		FillColor: color.DeviceRGB(0.9, 0.9, 0.9),
	}
	err = w.addAnnotationPair(borderSquare3)
	if err != nil {
		return err
	}

	// Group 3: Border effects (if supported)
	w.currentY -= groupGap

	// Cloud border effect with different intensities
	intensities := []float64{0.0, 0.5, 1.0, 1.5, 2.0}
	for _, intensity := range intensities {
		cloudSquare := &annotation.Square{
			Common: annotation.Common{
				Rect: pdf.Rectangle{
					LLx: leftColStart,
					LLy: w.currentY - squareSize,
					URx: leftColEnd,
					URy: w.currentY,
				},
				Contents: fmt.Sprintf("Cloud effect I=%.1f", intensity),
				Color:    color.Black,
				Flags:    annotation.FlagPrint,
			},
			FillColor: color.DeviceRGB(0.95, 0.95, 1.0),
			BorderStyle: &annotation.BorderStyle{
				Width:     1.5,
				SingleUse: true,
			},
			BorderEffect: &annotation.BorderEffect{
				Style:     "C",
				Intensity: intensity,
				SingleUse: true,
			},
		}
		err = w.addAnnotationPair(cloudSquare)
		if err != nil {
			return err
		}

		w.currentY -= rowStep
	}

	// Group 4: Margin adjustments (RD array)
	w.currentY -= groupGap

	margins := []struct {
		name   string
		margin []float64
	}{
		// {"No margins", nil},
		// {"Small margins", []float64{3, 3, 3, 3}},
		// {"Large margins", []float64{8, 8, 8, 8}},
		// {"Asymmetric", []float64{2, 5, 10, 3}},
	}

	for _, m := range margins {
		marginSquare := &annotation.Square{
			Common: annotation.Common{
				Rect: pdf.Rectangle{
					LLx: leftColStart,
					LLy: w.currentY - squareSize,
					URx: leftColEnd,
					URy: w.currentY,
				},
				Contents: m.name,
				Color:    color.Black,
				Flags:    annotation.FlagPrint,
			},
			FillColor: color.DeviceRGB(1.0, 0.95, 0.8),
			BorderStyle: &annotation.BorderStyle{
				Width:     2,
				SingleUse: true,
			},
			Margin: m.margin,
		}
		err = w.addAnnotationPair(marginSquare)
		if err != nil {
			return err
		}

		w.currentY -= rowStep
	}

	page.PageDict["Annots"] = w.annots

	return page.Close()
}

type writer struct {
	annots   pdf.Array
	page     *document.Page
	style    *fallback.Style
	currentY float64
}

func (w *writer) embed(a annotation.Annotation) error {
	obj, err := a.Encode(w.page.RM)
	if err != nil {
		return err
	}
	ref := w.page.RM.Out.Alloc()
	err = w.page.RM.Out.Put(ref, obj)
	if err != nil {
		return err
	}
	w.annots = append(w.annots, ref)
	return nil
}

func (w *writer) addAnnotationPair(square *annotation.Square) error {
	// embed left annotation as-is
	err := w.embed(square)
	if err != nil {
		return err
	}

	// create shallow copy for right column
	rightSquare := clone(square)

	// adjust coordinates for right column
	deltaX := rightColStart - leftColStart
	rightSquare.Rect.LLx += deltaX
	rightSquare.Rect.URx += deltaX

	w.style.AddAppearance(rightSquare)

	// embed right annotation
	return w.embed(rightSquare)
}

func clone[T any](v *T) *T {
	if v == nil {
		return nil
	}
	clone := *v
	return &clone
}

func pageBackground(paper *pdf.Rectangle) (graphics.Shading, error) {
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
		ColorSpace: color.DeviceRGBSpace,
		P0:         vec.Vec2{X: pdf.Round(t0*nx, 1), Y: pdf.Round(t0*ny, 1)},
		P1:         vec.Vec2{X: pdf.Round(t1*nx, 1), Y: pdf.Round(t1*ny, 1)},
		F:          F,
		TMin:       t0,
		TMax:       t1,
	}
	return background, nil
}
