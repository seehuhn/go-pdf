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

// Command filter-chains generates a one-page PDF which draws the same
// small 1-bit image many times, each copy stored behind a different
// /Filter chain, to see how viewers cope with long chains and with
// chains that use the same filter more than once.
//
// The PDF specification sets no limit on the length of a filter chain,
// and only a note in Table 5 says that repeating a filter "is not
// recommended".  A viewer that stops at some chain length may be
// enforcing a length limit, or may be refusing repeated filters; the
// page separates the two.  The first section uses chains without
// repeats, up to the six distinct lossless filters (CCITTFax innermost,
// which is why the image is 1-bit).  The second section uses short
// chains that repeat a filter.  The third section has long chains that
// cycle through the five general-purpose lossless filters.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/text"
)

const (
	imageDim = 16 // pixels

	pageWidth  = 595.0
	pageHeight = 842.0
	margin     = 50.0
	columns    = 4
	cellWidth  = (pageWidth - 2*margin) / columns
	cellHeight = 86.0
	imageSize  = 56.0
	sectionGap = 16.0
)

// a filter together with its caption abbreviation
type namedFilter struct {
	short  string
	filter pdf.Filter
}

var (
	flate = namedFilter{"Flate", pdf.FilterFlate{}}
	lzw   = namedFilter{"LZW", pdf.FilterLZW{OffByOne: true}}
	rl    = namedFilter{"RL", pdf.FilterRunLength{}}
	ahx   = namedFilter{"AHx", pdf.FilterASCIIHex{}}
	a85   = namedFilter{"A85", pdf.FilterASCII85{}}
	ccitt = namedFilter{"CCITT", pdf.FilterCCITTFax{K: -1, Columns: imageDim, Rows: imageDim}}
)

// lossless lists the general-purpose lossless filters in the order in
// which the long chains cycle through them.
var lossless = []namedFilter{flate, lzw, rl, ahx, a85}

// cycle returns a chain of n filters, cycling through lossless starting
// at position start.
func cycle(start, n int) []namedFilter {
	chain := make([]namedFilter, n)
	for i := range chain {
		chain[i] = lossless[(start+i)%len(lossless)]
	}
	return chain
}

type section struct {
	title  string
	chains [][]namedFilter
}

var sections = []section{
	{
		"A. No repeated filters",
		[][]namedFilter{
			nil,
			cycle(0, 1),
			cycle(0, 2),
			cycle(0, 3),
			cycle(0, 4),
			cycle(0, 5),
			append(cycle(0, 5), ccitt),
		},
	},
	{
		"B. Short chains with a repeated filter",
		[][]namedFilter{
			{flate, flate},
			{lzw, lzw},
			{rl, rl},
			{ahx, ahx},
			{a85, a85},
			{ahx, a85, ahx},
			{flate, ahx, flate},
			{flate, lzw, rl, flate},
			{flate, ahx, flate, ahx, flate},
		},
	},
	{
		"C. Long chains",
		[][]namedFilter{
			cycle(0, 6),
			cycle(1, 8),
			cycle(2, 10),
			cycle(3, 15),
		},
	},
}

func main() {
	if err := run("test.pdf"); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(filename string) error {
	paper := &pdf.Rectangle{URx: pageWidth, URy: pageHeight}
	page, err := document.CreateSinglePage(filename, paper, pdf.V1_7, nil)
	if err != nil {
		return err
	}

	titleFont := font.Must(standard.HelveticaBold.New())
	bodyFont := font.Must(standard.Helvetica.New())
	title := text.F{Font: titleFont, Size: 14, Color: color.DeviceGray(0)}
	heading := text.F{Font: titleFont, Size: 10, Color: color.DeviceGray(0)}
	body := text.F{Font: bodyFont, Size: 10, Color: color.DeviceGray(0.1)}
	label := text.F{Font: bodyFont, Size: 7, Color: color.DeviceGray(0)}

	text.Show(page.Builder,
		text.M{X: margin, Y: pageHeight - margin - 14},
		title, "Long filter chains and repeated filters",
	)
	text.Show(page.Builder,
		text.M{X: margin, Y: pageHeight - margin - 34},
		body,
		text.Wrap(pageWidth-2*margin,
			"Each square below is the same 16×16 pixel 1-bit image: a black",
			"frame with a black triangle in the lower left half.  The copies",
			"differ only in the /Filter chain used to store the image data;",
			"the caption gives the number of filters and lists the chain in",
			"decoding order.  Section A uses no filter twice, section B",
			"repeats a filter in a short chain, section C has long chains",
			"that cycle through Flate, LZW, RunLength, ASCIIHex and",
			"ASCII85.  A conforming viewer shows all squares identical.",
			"A blank or garbled square in A marks a chain length limit; one",
			"in B marks a refusal of repeated filters.",
		),
	)

	page.SetLineWidth(0.5)
	page.SetStrokeColor(color.DeviceGray(0.5))

	pixels := makePixels()
	y := pageHeight - margin - 34 - 10*10 - sectionGap
	for _, sec := range sections {
		text.Show(page.Builder, text.M{X: margin, Y: y}, heading, sec.title)
		y -= 14

		for i, chain := range sec.chains {
			col := i % columns
			row := i / columns
			x := margin + float64(col)*cellWidth + (cellWidth-imageSize)/2
			yImg := y - float64(row)*cellHeight - imageSize

			var filters []pdf.Filter
			var names []string
			for _, f := range chain {
				filters = append(filters, f.filter)
				names = append(names, f.short)
			}

			page.PushGraphicsState()
			page.Transform(matrix.Matrix{imageSize, 0, 0, imageSize, x, yImg})
			page.DrawXObject(&chainImage{pixels: pixels, filters: filters})
			page.PopGraphicsState()
			page.Rectangle(x-0.5, yImg-0.5, imageSize+1, imageSize+1)
			page.Stroke()

			caption := fmt.Sprintf("n = %d", len(chain))
			if len(chain) > 0 {
				caption += ": " + strings.Join(names, " ")
			} else {
				caption += " (no filter)"
			}
			text.Show(page.Builder,
				text.M{X: x, Y: yImg - 11},
				label, text.Wrap(cellWidth-20, caption),
			)
		}
		rows := (len(sec.chains) + columns - 1) / columns
		y -= float64(rows)*cellHeight + sectionGap
	}

	return page.Close()
}

// makePixels returns the rows of a 16×16 1-bit DeviceGray image (0 is
// black): a two pixel black frame and a black triangle filling the lower
// left half of the interior.
func makePixels() []byte {
	var pixels []byte
	for y := range imageDim {
		var row uint16
		for x := range imageDim {
			frame := x < 2 || y < 2 || x >= imageDim-2 || y >= imageDim-2
			triangle := x < y
			if !frame && !triangle {
				row |= 1 << (15 - x) // white
			}
		}
		pixels = append(pixels, byte(row>>8), byte(row))
	}
	return pixels
}

// chainImage is an Image XObject whose data is stored behind the given
// filter chain, filters[0] being the outermost.
type chainImage struct {
	pixels  []byte
	filters []pdf.Filter
}

var _ graphics.XObject = (*chainImage)(nil)

func (m *chainImage) Subtype() pdf.Name      { return "Image" }
func (m *chainImage) ResourceName() pdf.Name { return "" }

func (m *chainImage) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	dict := pdf.Dict{
		"Type":             pdf.Name("XObject"),
		"Subtype":          pdf.Name("Image"),
		"Width":            pdf.Integer(imageDim),
		"Height":           pdf.Integer(imageDim),
		"ColorSpace":       pdf.Name("DeviceGray"),
		"BitsPerComponent": pdf.Integer(1),
	}

	// OpenStream refuses to repeat a filter in a chain, so the chain is
	// applied by hand and recorded in the dict.
	v := pdf.GetVersion(rm.Out())
	var buf bytes.Buffer
	var w io.WriteCloser = nopCloser{&buf}
	var names pdf.Array
	var parms pdf.Array
	hasParms := false
	for _, f := range m.filters {
		name, p, err := f.Info(v)
		if err != nil {
			return nil, err
		}
		names = append(names, name)
		if len(p) > 0 {
			parms = append(parms, p)
			hasParms = true
		} else {
			parms = append(parms, nil)
		}
		w, err = f.Encode(v, w)
		if err != nil {
			return nil, err
		}
	}
	if _, err := w.Write(m.pixels); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	if len(names) > 0 {
		dict["Filter"] = names
		if hasParms {
			dict["DecodeParms"] = parms
		}
	}

	ref := rm.Alloc()
	stm, err := rm.Out().OpenStream(ref, dict)
	if err != nil {
		return nil, err
	}
	if _, err := stm.Write(buf.Bytes()); err != nil {
		return nil, err
	}
	if err := stm.Close(); err != nil {
		return nil, err
	}
	return ref, nil
}

type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }
