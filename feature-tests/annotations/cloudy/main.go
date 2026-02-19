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
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/fallback"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
)

var paper = document.A4

const margin = 50.0

var gray = color.DeviceGray(0.75)

func main() {
	fmt.Println("writing test.pdf ...")
	err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	doc, err := document.CreateMultiPage("test.pdf", paper, pdf.V1_7, opt)
	if err != nil {
		return err
	}

	w := &writer{
		doc:   doc,
		style: fallback.NewStyle(),
		font:  standard.Helvetica.New(),
	}

	w.pageVaryingSizes()
	w.pageBaseDetection()
	w.pageVaryingIntensity()
	w.pageVaryingLineWidth()
	w.pageShapeTypes()
	w.pageFillStroke()

	err = w.close()
	if err != nil {
		return err
	}
	return doc.Close()
}

type writer struct {
	doc   *document.MultiPage
	page  *document.Page
	style *fallback.Style
	font  font.Layouter
	yPos  float64
}

func (w *writer) newPage(title string) {
	if w.page != nil {
		w.page.Close()
	}
	w.page = w.doc.AddPage()
	w.yPos = paper.URy - margin

	// draw title
	w.yPos -= 20
	w.page.PushGraphicsState()
	w.page.TextBegin()
	w.page.TextSetFont(w.font, 14)
	w.page.TextSetMatrix(matrix.Translate(margin, w.yPos))
	w.page.TextShow(title)
	w.page.TextEnd()
	w.page.PopGraphicsState()
	w.yPos -= 20
}

func (w *writer) close() error {
	if w.page != nil {
		return w.page.Close()
	}
	return nil
}

func (w *writer) drawLabel(x, y float64, text string) {
	w.page.PushGraphicsState()
	w.page.TextBegin()
	w.page.TextSetFont(w.font, 9)
	w.page.TextSetMatrix(matrix.Translate(x, y))
	w.page.TextShow(text)
	w.page.TextEnd()
	w.page.PopGraphicsState()
}

func (w *writer) addCloudySquare(rect pdf.Rectangle, lw, intensity float64, strokeCol, fillCol color.Color) {
	a := &annotation.Square{
		Common: annotation.Common{
			Rect:  rect,
			Flags: annotation.FlagPrint,
			Color: strokeCol,
		},
		FillColor: fillCol,
		BorderStyle: &annotation.BorderStyle{
			Width:     lw,
			Style:     "S",
			SingleUse: true,
		},
		BorderEffect: &annotation.BorderEffect{
			Style:     "C",
			Intensity: intensity,
			SingleUse: true,
		},
	}
	w.style.AddAppearance(a)
	w.page.Page.Annots = append(w.page.Page.Annots, a)

	// overlay: thin gray outline of the original rectangle
	outline := &annotation.Square{
		Common: annotation.Common{
			Rect:  rect,
			Flags: annotation.FlagPrint,
			Color: gray,
		},
		BorderStyle: &annotation.BorderStyle{
			Width:     0.5,
			Style:     "S",
			SingleUse: true,
		},
	}
	w.style.AddAppearance(outline)
	w.page.Page.Annots = append(w.page.Page.Annots, outline)
}

func (w *writer) addCloudyCircle(rect pdf.Rectangle, lw, intensity float64, strokeCol, fillCol color.Color) {
	a := &annotation.Circle{
		Common: annotation.Common{
			Rect:  rect,
			Flags: annotation.FlagPrint,
			Color: strokeCol,
		},
		FillColor: fillCol,
		BorderStyle: &annotation.BorderStyle{
			Width:     lw,
			Style:     "S",
			SingleUse: true,
		},
		BorderEffect: &annotation.BorderEffect{
			Style:     "C",
			Intensity: intensity,
			SingleUse: true,
		},
	}
	w.style.AddAppearance(a)
	w.page.Page.Annots = append(w.page.Page.Annots, a)

	// overlay: thin gray outline of the original ellipse
	outline := &annotation.Circle{
		Common: annotation.Common{
			Rect:  rect,
			Flags: annotation.FlagPrint,
			Color: gray,
		},
		BorderStyle: &annotation.BorderStyle{
			Width:     0.5,
			Style:     "S",
			SingleUse: true,
		},
	}
	w.style.AddAppearance(outline)
	w.page.Page.Annots = append(w.page.Page.Annots, outline)
}

func (w *writer) addCloudyPolygon(rect pdf.Rectangle, verts []float64, lw, intensity float64, strokeCol, fillCol color.Color) {
	a := &annotation.Polygon{
		Common: annotation.Common{
			Rect:  rect,
			Flags: annotation.FlagPrint,
			Color: strokeCol,
		},
		Vertices:  verts,
		FillColor: fillCol,
		BorderStyle: &annotation.BorderStyle{
			Width:     lw,
			Style:     "S",
			SingleUse: true,
		},
		BorderEffect: &annotation.BorderEffect{
			Style:     "C",
			Intensity: intensity,
			SingleUse: true,
		},
	}
	w.style.AddAppearance(a)
	w.page.Page.Annots = append(w.page.Page.Annots, a)

	// overlay: thin gray outline of the original polygon
	outline := &annotation.Polygon{
		Common: annotation.Common{
			Rect:  rect,
			Flags: annotation.FlagPrint,
			Color: gray,
		},
		Vertices: verts,
		BorderStyle: &annotation.BorderStyle{
			Width:     0.5,
			Style:     "S",
			SingleUse: true,
		},
	}
	w.style.AddAppearance(outline)
	w.page.Page.Annots = append(w.page.Page.Annots, outline)
}

// pageVaryingSizes shows cloudy squares in different sizes.
func (w *writer) pageVaryingSizes() {
	w.newPage("Cloudy Borders \u2014 Varying Size")

	// sizes near the cutoff (19 = no clouds, 20 = smallest cloud),
	// then increasing sizes
	sizes := []float64{19, 20, 30, 60, 100, 150, 200}
	x0 := margin + 20.0

	for _, size := range sizes {
		w.yPos -= 15
		if w.yPos-size < margin {
			break
		}
		rect := pdf.Rectangle{
			LLx: x0,
			LLy: w.yPos - size,
			URx: x0 + size,
			URy: w.yPos,
		}
		w.addCloudySquare(rect, 1, 1, color.Black, color.White)
		label := fmt.Sprintf("%.0f\u00d7%.0f pt", size, size)
		if size < 20 {
			label += " (below cutoff)"
		} else if size == 20 {
			label += " (smallest cloud)"
		}
		w.drawLabel(x0+size+20, w.yPos-size/2-3, label)
		w.yPos -= size
	}
}

// pageBaseDetection shows how rotation affects flat base detection.
func (w *writer) pageBaseDetection() {
	w.newPage("Cloudy Borders \u2014 Flat Base Detection")

	size := 80.0
	half := size / 2
	x0 := margin + 60.0

	// the flat base cutoff is at 15°
	angles := []float64{0, 5, 10, 14, 16, 20, 30, 45}

	for _, deg := range angles {
		w.yPos -= 20
		if w.yPos-size-20 < margin {
			break
		}

		cx := x0 + half
		cy := w.yPos - half
		rad := deg * math.Pi / 180

		// rotated square vertices
		verts := make([]float64, 8)
		corners := [][2]float64{{-half, -half}, {half, -half}, {half, half}, {-half, half}}
		for i, c := range corners {
			rx := c[0]*math.Cos(rad) - c[1]*math.Sin(rad)
			ry := c[0]*math.Sin(rad) + c[1]*math.Cos(rad)
			verts[2*i] = cx + rx
			verts[2*i+1] = cy + ry
		}

		rect := polygonBounds(verts, 15)
		w.addCloudyPolygon(rect, verts, 1, 1, color.Black, color.White)

		label := fmt.Sprintf("%.0f\u00b0", deg)
		if deg <= 15 {
			label += " (base)"
		} else {
			label += " (no base)"
		}
		w.drawLabel(rect.URx+15, cy-3, label)
		w.yPos = rect.LLy
	}
}

// pageVaryingIntensity shows cloudy squares with different intensity values.
func (w *writer) pageVaryingIntensity() {
	w.newPage("Cloudy Borders \u2014 Varying Intensity")

	intensities := []float64{0.5, 1, 1.5, 2}
	x0 := margin + 20.0
	size := 100.0

	for _, intensity := range intensities {
		pad := 15 + 10*intensity
		w.yPos -= pad
		if w.yPos-size < margin {
			break
		}
		rect := pdf.Rectangle{
			LLx: x0,
			LLy: w.yPos - size,
			URx: x0 + size,
			URy: w.yPos,
		}
		w.addCloudySquare(rect, 1, intensity, color.Black, color.White)
		w.drawLabel(x0+size+30, w.yPos-size/2-3, fmt.Sprintf("intensity = %.2g", intensity))
		w.yPos -= size
	}
}

// pageVaryingLineWidth shows cloudy squares with different line widths.
func (w *writer) pageVaryingLineWidth() {
	w.newPage("Cloudy Borders \u2014 Varying Line Width")

	lineWidths := []float64{0.5, 1, 2, 3}
	x0 := margin + 20.0
	size := 100.0

	for _, lw := range lineWidths {
		w.yPos -= 20
		if w.yPos-size < margin {
			break
		}
		rect := pdf.Rectangle{
			LLx: x0,
			LLy: w.yPos - size,
			URx: x0 + size,
			URy: w.yPos,
		}
		w.addCloudySquare(rect, lw, 1, color.Black, color.White)
		w.drawLabel(x0+size+30, w.yPos-size/2-3, fmt.Sprintf("lw = %.1f", lw))
		w.yPos -= size
	}
}

// pageShapeTypes shows cloudy borders on different shape types in a grid.
func (w *writer) pageShapeTypes() {
	w.newPage("Cloudy Borders \u2014 Shape Types")

	size := 100.0
	radius := 42.0
	col1 := margin + 30.0
	col2 := margin + 270.0
	rowGap := 40.0

	type item struct {
		label string
		draw  func(x, y float64)
	}

	items := []item{
		{"Square", func(x, y float64) {
			rect := pdf.Rectangle{LLx: x, LLy: y - size, URx: x + size, URy: y}
			w.addCloudySquare(rect, 1, 1, color.Black, color.White)
		}},
		{"Circle", func(x, y float64) {
			rect := pdf.Rectangle{LLx: x, LLy: y - size, URx: x + size, URy: y}
			w.addCloudyCircle(rect, 1, 1, color.Black, color.White)
		}},
		{"Triangle", func(x, y float64) {
			cx, cy := x+size/2, y-size/2
			verts := regularPolygonVertices(cx, cy, radius, 3)
			w.addCloudyPolygon(polygonBounds(verts, 15), verts, 1, 1, color.Black, color.White)
		}},
		{"Pentagon", func(x, y float64) {
			cx, cy := x+size/2, y-size/2
			verts := regularPolygonVertices(cx, cy, radius, 5)
			w.addCloudyPolygon(polygonBounds(verts, 15), verts, 1, 1, color.Black, color.White)
		}},
		{"Hexagon", func(x, y float64) {
			cx, cy := x+size/2, y-size/2
			verts := regularPolygonVertices(cx, cy, radius, 6)
			w.addCloudyPolygon(polygonBounds(verts, 15), verts, 1, 1, color.Black, color.White)
		}},
		{"L-shape", func(x, y float64) {
			verts := lShapeVertices(x+10, y-size)
			w.addCloudyPolygon(polygonBounds(verts, 15), verts, 1, 1, color.Black, color.White)
		}},
	}

	w.yPos -= 10
	for i, it := range items {
		col := col1
		if i%2 == 1 {
			col = col2
		}
		if i%2 == 0 && i > 0 {
			w.yPos -= rowGap
		}
		it.draw(col, w.yPos)
		w.drawLabel(col+size/2-15, w.yPos-size-15, it.label)
		if i%2 == 1 {
			w.yPos -= size
		}
	}
	w.yPos -= size + 20
}

// pageFillStroke shows different fill/stroke combinations.
func (w *writer) pageFillStroke() {
	w.newPage("Cloudy Borders \u2014 Fill and Stroke")

	x0 := margin + 20.0
	size := 100.0
	lightBlue := color.DeviceRGB{0.7, 0.85, 1.0}

	// stroke only
	w.yPos -= 20
	rect := pdf.Rectangle{
		LLx: x0,
		LLy: w.yPos - size,
		URx: x0 + size,
		URy: w.yPos,
	}
	w.addCloudySquare(rect, 1, 1, color.Black, nil)
	w.drawLabel(x0+size+30, w.yPos-size/2-3, "stroke only")
	w.yPos -= size

	// fill only
	w.yPos -= 30
	rect = pdf.Rectangle{
		LLx: x0,
		LLy: w.yPos - size,
		URx: x0 + size,
		URy: w.yPos,
	}
	w.addCloudySquare(rect, 1, 1, nil, lightBlue)
	w.drawLabel(x0+size+30, w.yPos-size/2-3, "fill only")
	w.yPos -= size

	// fill + stroke
	w.yPos -= 30
	rect = pdf.Rectangle{
		LLx: x0,
		LLy: w.yPos - size,
		URx: x0 + size,
		URy: w.yPos,
	}
	w.addCloudySquare(rect, 1, 1, color.Black, lightBlue)
	w.drawLabel(x0+size+30, w.yPos-size/2-3, "fill + stroke")
	w.yPos -= size
}

// regularPolygonVertices returns flat x,y pairs for a regular n-gon
// centered at (cx, cy) with the given radius.
func regularPolygonVertices(cx, cy, r float64, n int) []float64 {
	vv := make([]float64, 2*n)
	for i := range n {
		// start at top, go counter-clockwise
		angle := math.Pi/2 + float64(i)*2*math.Pi/float64(n)
		vv[2*i] = cx + r*math.Cos(angle)
		vv[2*i+1] = cy + r*math.Sin(angle)
	}
	return vv
}

// lShapeVertices returns flat x,y pairs for an L-shaped polygon.
// The shape is positioned with its bottom-left at (x0, y0).
func lShapeVertices(x0, y0 float64) []float64 {
	return []float64{
		x0, y0,
		x0 + 60, y0,
		x0 + 60, y0 + 40,
		x0 + 30, y0 + 40,
		x0 + 30, y0 + 90,
		x0, y0 + 90,
	}
}

// polygonBounds returns a bounding rectangle for flat x,y vertex pairs,
// expanded by the given padding.
func polygonBounds(verts []float64, padding float64) pdf.Rectangle {
	if len(verts) < 2 {
		return pdf.Rectangle{}
	}
	minX, maxX := verts[0], verts[0]
	minY, maxY := verts[1], verts[1]
	for i := 2; i+1 < len(verts); i += 2 {
		minX = min(minX, verts[i])
		maxX = max(maxX, verts[i])
		minY = min(minY, verts[i+1])
		maxY = max(maxY, verts[i+1])
	}
	return pdf.Rectangle{
		LLx: minX - padding,
		LLy: minY - padding,
		URx: maxX + padding,
		URy: maxY + padding,
	}
}
