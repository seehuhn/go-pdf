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

// Jpx draws a grid of JPXDecode images, each covering one way in which
// the JPEG 2000 data determines colour, bit depth or opacity.
package main

import (
	"bytes"
	"fmt"
	"math"
	"os"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/jpx"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	pdfimg "seehuhn.de/go/pdf/graphics/image"
	"seehuhn.de/go/pdf/graphics/text"
)

// size is the width and height of most test images, in pixels.
const size = 48

// stripeSize is the size of the sub-sampling test images, in pixels.
const stripeSize = 24

func main() {
	if err := run("test.pdf"); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("wrote test.pdf")
}

// cell is one image of the grid, with its label.
type cell struct {
	label      []string
	img        *pdfimg.Dict
	background bool // draw coloured stripes behind the image
}

func run(filename string) error {
	cells, err := makeCells()
	if err != nil {
		return err
	}

	page, err := document.CreateSinglePage(filename, document.A4, pdf.V2_0, nil)
	if err != nil {
		return err
	}
	helv := font.Must(standard.Helvetica.New())
	F := text.F{Font: helv, Size: 14}
	Fsmall := text.F{Font: helv, Size: 8}

	text.Show(page.Builder, text.M{X: 60, Y: 800}, F, "JPXDecode Images")

	const (
		imgSize = 120.0
		colX0   = 60.0
		colDX   = 170.0
		rowY0   = 640.0
		rowDY   = 175.0
	)
	for i, c := range cells {
		x := colX0 + float64(i%3)*colDX
		y := rowY0 - float64(i/3)*rowDY
		page.PushGraphicsState()
		if c.background {
			drawBackground(page, x, y, imgSize)
		}
		page.Transform(matrix.Translate(x, y))
		page.Transform(matrix.Scale(imgSize, imgSize))
		page.DrawXObject(c.img)
		page.PopGraphicsState()

		args := []any{Fsmall, text.M{X: x, Y: y - 12}, fmt.Sprintf("%d. %s", i+1, c.label[0])}
		for _, l := range c.label[1:] {
			args = append(args, text.NL, l)
		}
		text.Show(page.Builder, args...)
	}

	text.Show(page.Builder, Fsmall, text.M{X: 60, Y: 70},
		text.Wrap(470,
			"Cells 1 to 4 show the same picture, a colour gradient with a yellow disc and a dark block.",
			"Cell 1 is its grey version, cell 4 its CMYK version (with approximate colours), and cell 5 its colour negative.",
			"Cells 8 and 9 must look the same: the disc fades out towards its rim and the stripes behind it show through.",
			"Cells 10 and 11 show sharp red and blue stripes without colour fringes; their chroma is stored at half resolution.",
		),
	)

	return page.Close()
}

func makeCells() ([]cell, error) {
	var cells []cell
	add := func(label []string, im *jpx.Image, d *pdfimg.Dict, background bool) error {
		var buf bytes.Buffer
		if err := jpx.Encode(&buf, im, &jpx.EncodeParams{Wrapper: true}); err != nil {
			return err
		}
		d.Width = im.Width
		d.Height = im.Height
		d.Data = pdfimg.NewJPXSource(buf.Bytes())
		cells = append(cells, cell{label: label, img: d, background: background})
		return nil
	}

	r, g, b := picture(size, size)
	grey := make([]int32, len(r))
	for i := range r {
		grey[i] = clamp8(0.299*float64(r[i]) + 0.587*float64(g[i]) + 0.114*float64(b[i]))
	}
	y, cb, cr := toSYCC(r, g, b)
	c, m, ye, k := toCMYK(r, g, b)

	steps := []struct {
		label      []string
		im         *jpx.Image
		d          *pdfimg.Dict
		background bool
	}{
		{[]string{"8-bit grey, no ColorSpace", "(colr 17)"},
			withSpec(newImage(size, size, 8, grey), 17), &pdfimg.Dict{}, false},
		{[]string{"8-bit RGB, no ColorSpace", "(colr 16)"},
			withSpec(newImage(size, size, 8, r, g, b), 16), &pdfimg.Dict{}, false},
		{[]string{"8-bit sYCC, no ColorSpace (colr 18)", "same as 2"},
			withSpec(newImage(size, size, 8, y, cb, cr), 18), &pdfimg.Dict{}, false},
		{[]string{"8-bit CMYK, no ColorSpace", "(colr 12)"},
			withSpec(newImage(size, size, 8, c, m, ye, k), 12), &pdfimg.Dict{}, false},
		{[]string{"RGB, ColorSpace DeviceRGB,", "Decode [1 0 1 0 1 0]: negative of 2"},
			withSpec(newImage(size, size, 8, r, g, b), 16),
			&pdfimg.Dict{ColorSpace: color.SpaceDeviceRGB, Decode: []float64{1, 0, 1, 0, 1, 0}}, false},
		{[]string{"palette (pclr), no ColorSpace", "becomes Indexed"},
			paletteImage(), &pdfimg.Dict{}, false},
		{[]string{"16-bit RGB gradient, no ColorSpace", "smooth, no banding"},
			gradient16(), &pdfimg.Dict{}, false},
		{[]string{"RGBA, SMaskInData 1", "over stripes"},
			alphaImage(r, g, b, false), &pdfimg.Dict{SMaskInData: 1}, true},
		{[]string{"premultiplied RGBA, SMaskInData 2", "same as 8"},
			alphaImage(r, g, b, true), &pdfimg.Dict{SMaskInData: 2}, true},
		{[]string{"sYCC 4:2:0 (chroma DX = DY = 2)", "clean stripes"},
			subsampled(0), &pdfimg.Dict{}, false},
		{[]string{"sYCC 4:2:0, XOffset = YOffset = 1", "clean stripes, shifted by one pixel"},
			subsampled(1), &pdfimg.Dict{}, false},
	}
	for _, s := range steps {
		if err := add(s.label, s.im, s.d, s.background); err != nil {
			return nil, fmt.Errorf("%s: %w", s.label[0], err)
		}
	}
	return cells, nil
}

// drawBackground fills the cell area with four vertical stripes.
func drawBackground(page *document.Page, x, y, w float64) {
	stripes := []color.Color{
		color.DeviceRGB{0.2, 0.6, 0.2},
		color.DeviceRGB{0.8, 0.2, 0.6},
		color.DeviceRGB{0.2, 0.4, 0.8},
		color.DeviceRGB{0.9, 0.6, 0.1},
	}
	sw := w / float64(len(stripes))
	for i, c := range stripes {
		page.SetFillColor(c)
		page.Rectangle(x+float64(i)*sw, y, sw, w)
		page.Fill()
	}
}

// picture returns the RGB planes of the test picture: a gradient with a
// yellow disc and a dark block.
func picture(w, h int) (r, g, b []int32) {
	r = make([]int32, w*h)
	g = make([]int32, w*h)
	b = make([]int32, w*h)
	for y := range h {
		for x := range w {
			i := y*w + x
			fx := float64(x) / float64(w-1)
			fy := float64(y) / float64(h-1)
			cr, cg, cb := fx, fy, 1-fx
			dx, dy := fx-0.55, fy-0.45
			if dx*dx+dy*dy < 0.25*0.25 {
				cr, cg, cb = 1, 0.9, 0.1
			}
			if fx < 0.25 && fy > 0.75 {
				cr, cg, cb = 0.1, 0.1, 0.15
			}
			r[i], g[i], b[i] = clamp8(255*cr), clamp8(255*cg), clamp8(255*cb)
		}
	}
	return r, g, b
}

// toSYCC converts 8-bit RGB samples to sYCC.  This is the inverse of the
// conversion in [jpx.SYCCToRGB].
func toSYCC(r, g, b []int32) (y, cb, cr []int32) {
	y = make([]int32, len(r))
	cb = make([]int32, len(r))
	cr = make([]int32, len(r))
	for i := range r {
		yy, u, v := sycc(float64(r[i]), float64(g[i]), float64(b[i]))
		y[i], cb[i], cr[i] = clamp8(yy), clamp8(u), clamp8(v)
	}
	return y, cb, cr
}

// sycc converts one 8-bit RGB colour to unrounded sYCC values.
func sycc(r, g, b float64) (y, cb, cr float64) {
	y = 0.299*r + 0.587*g + 0.114*b
	cb = (b-y)/1.772 + 128
	cr = (r-y)/1.402 + 128
	return y, cb, cr
}

// toCMYK converts 8-bit RGB samples to CMYK with full black generation.
func toCMYK(r, g, b []int32) (c, m, y, k []int32) {
	c = make([]int32, len(r))
	m = make([]int32, len(r))
	y = make([]int32, len(r))
	k = make([]int32, len(r))
	for i := range r {
		mx := max(r[i], g[i], b[i])
		k[i] = 255 - mx
		if mx == 0 {
			continue
		}
		c[i] = (mx - r[i]) * 255 / mx
		m[i] = (mx - g[i]) * 255 / mx
		y[i] = (mx - b[i]) * 255 / mx
	}
	return c, m, y, k
}

func clamp8(v float64) int32 {
	return int32(min(max(math.Round(v), 0), 255))
}

// newImage returns an image with one colour channel per plane, all at
// full resolution.
func newImage(w, h, depth int, planes ...[]int32) *jpx.Image {
	im := &jpx.Image{}
	im.Width, im.Height = w, h
	for i, p := range planes {
		im.Components = append(im.Components, jpx.Component{Depth: depth, DX: 1, DY: 1, Width: w, Height: h})
		im.Planes = append(im.Planes, jpx.Plane{Width: w, Height: h, Pix: p})
		im.Channels = append(im.Channels, jpx.Channel{
			Component: i, PaletteColumn: -1, Kind: jpx.ChannelColor, Assoc: i + 1,
		})
	}
	return im
}

// withSpec sets an enumerated colour space on im.
func withSpec(im *jpx.Image, enum int) *jpx.Image {
	im.ColorSpecs = []jpx.ColorSpec{{Method: 1, Enumerated: enum}}
	return im
}

// paletteImage returns the test picture reduced to a six-colour palette.
func paletteImage() *jpx.Image {
	entries := [][]int32{
		{230, 60, 50}, {60, 160, 70}, {50, 90, 200},
		{250, 220, 40}, {30, 30, 40}, {240, 240, 240},
	}
	idx := make([]int32, size*size)
	for y := range size {
		for x := range size {
			fx := float64(x) / float64(size-1)
			fy := float64(y) / float64(size-1)
			v := int32(min(int(3*fx), 2)) // red, green, blue columns
			dx, dy := fx-0.55, fy-0.45
			switch {
			case fx < 0.25 && fy > 0.75:
				v = 4
			case dx*dx+dy*dy < 0.25*0.25:
				v = 3
			case (x/4+y/4)%2 == 0 && fy < 0.2:
				v = 5
			}
			idx[y*size+x] = v
		}
	}
	im := newImage(size, size, 8, idx)
	im.Palette = &jpx.Palette{
		Entries: entries,
		Columns: []jpx.PaletteColumn{{Depth: 8}, {Depth: 8}, {Depth: 8}},
	}
	im.Channels = nil
	for c := range 3 {
		im.Channels = append(im.Channels, jpx.Channel{Component: 0, PaletteColumn: c, Kind: jpx.ChannelColor, Assoc: c + 1})
	}
	return withSpec(im, 16)
}

// gradient16 returns a 16-bit RGB image with a slow gradient, which shows
// banding if the samples are reduced to 8 bits and stretched.
func gradient16() *jpx.Image {
	r := make([]int32, size*size)
	g := make([]int32, size*size)
	b := make([]int32, size*size)
	for y := range size {
		for x := range size {
			i := y*size + x
			fx := float64(x) / float64(size-1)
			fy := float64(y) / float64(size-1)
			r[i] = int32(math.Round(65535 * fx))
			g[i] = int32(math.Round(65535 * (0.3 + 0.4*fy)))
			b[i] = int32(math.Round(65535 * (1 - fx) * fy))
		}
	}
	return withSpec(newImage(size, size, 16, r, g, b), 16)
}

// alphaImage adds an opacity channel to the test picture: the disc is
// opaque in the centre and fades out towards its rim, the rest of the
// image is half transparent.
func alphaImage(r, g, b []int32, premultiplied bool) *jpx.Image {
	alpha := make([]int32, size*size)
	for y := range size {
		for x := range size {
			fx := float64(x) / float64(size-1)
			fy := float64(y) / float64(size-1)
			d := math.Hypot(fx-0.5, fy-0.5)
			a := 0.5
			if d < 0.4 {
				a = 1 - 0.5*d/0.4
			}
			alpha[y*size+x] = clamp8(255 * a)
		}
	}
	planes := [][]int32{r, g, b}
	kind := jpx.ChannelOpacity
	if premultiplied {
		kind = jpx.ChannelPremultipliedOpacity
		for c, p := range planes {
			q := make([]int32, len(p))
			for i, v := range p {
				q[i] = clamp8(float64(v) * float64(alpha[i]) / 255)
			}
			planes[c] = q
		}
	}
	im := newImage(size, size, 8, planes[0], planes[1], planes[2], alpha)
	im.Channels[3] = jpx.Channel{Component: 3, PaletteColumn: -1, Kind: kind, Assoc: 0}
	return withSpec(im, 16)
}

// subsampled returns an sYCC image of red and blue stripes, with the
// chroma components sub-sampled by 2 in both directions.  The image area
// starts at (off, off) on the reference grid, and the stripes follow the
// chroma sample grid, so that every chroma sample covers a single colour.
func subsampled(off int) *jpx.Image {
	const n = stripeSize
	// Chroma sample k covers grid points 2k and 2k+1.  The samples start
	// at k = c0, and a grid point before the first sample takes that one.
	c0 := (off + 1) / 2
	c1 := (off + n + 1) / 2
	split := (n/2 + off + 1) / 2 * 2 // even, so no chroma sample straddles it
	colour := func(x, y int) (r, g, b float64) {
		// chroma sample coordinates
		kx, ky := max((x+off)/2, c0), max((y+off)/2, c0)
		switch {
		case y+off < split && kx%2 == 0:
			return 220, 30, 30
		case y+off < split:
			return 30, 60, 220
		case ky%2 == 0:
			return 250, 220, 40
		default:
			return 40, 160, 60
		}
	}

	yp := make([]int32, n*n)
	for y := range n {
		for x := range n {
			r, g, b := colour(x, y)
			yy, _, _ := sycc(r, g, b)
			yp[y*n+x] = clamp8(yy)
		}
	}

	cn := c1 - c0
	cbp := make([]int32, cn*cn)
	crp := make([]int32, cn*cn)
	for ky := range cn {
		for kx := range cn {
			var sumB, sumR float64
			count := 0
			for gy := 2 * (ky + c0); gy < 2*(ky+c0)+2; gy++ {
				for gx := 2 * (kx + c0); gx < 2*(kx+c0)+2; gx++ {
					x, y := gx-off, gy-off
					if x < 0 || y < 0 || x >= n || y >= n {
						continue
					}
					_, cb, cr := sycc(colour(x, y))
					sumB += cb
					sumR += cr
					count++
				}
			}
			cbp[ky*cn+kx] = clamp8(sumB / float64(count))
			crp[ky*cn+kx] = clamp8(sumR / float64(count))
		}
	}

	im := &jpx.Image{}
	im.XOffset, im.YOffset = off, off
	im.Width, im.Height = n, n
	im.Components = []jpx.Component{
		{Depth: 8, DX: 1, DY: 1, Width: n, Height: n},
		{Depth: 8, DX: 2, DY: 2, Width: cn, Height: cn},
		{Depth: 8, DX: 2, DY: 2, Width: cn, Height: cn},
	}
	im.Planes = []jpx.Plane{
		{Width: n, Height: n, Pix: yp},
		{Width: cn, Height: cn, Pix: cbp},
		{Width: cn, Height: cn, Pix: crp},
	}
	for i := range 3 {
		im.Channels = append(im.Channels, jpx.Channel{
			Component: i, PaletteColumn: -1, Kind: jpx.ChannelColor, Assoc: i + 1,
		})
	}
	return withSpec(im, 18)
}
