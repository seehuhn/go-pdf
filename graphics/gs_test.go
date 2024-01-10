// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package graphics_test

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
)

func TestLineWidth(t *testing.T) {
	if !haveGhostScript() {
		t.Skip("ghostscript not found")
	}
	img := gsRender(t, 20, 5, pdf.V1_7, func(r *document.Page) {
		r.SetLineWidth(6.0)
		r.MoveTo(10, 0)
		r.LineTo(10, 5)
		r.Stroke()
	})

	rect := img.Bounds()
	for i := rect.Min.X; i < rect.Max.X; i++ {
		for j := rect.Min.Y; j < rect.Max.Y; j++ {
			r, g, b, a := img.At(i, j).RGBA()
			if i >= 4*7 && i < 4*13 {
				// should be black
				if r != 0 || g != 0 || b != 0 || a != 0xffff {
					t.Errorf("pixel (%d,%d) should be black, but is %d,%d,%d,%d", i, j, r, g, b, a)
				}
			} else {
				// should be white
				if r != 0xffff || g != 0xffff || b != 0xffff || a != 0xffff {
					t.Errorf("pixel (%d,%d) should be white, but is %d,%d,%d,%d", i, j, r, g, b, a)
				}
			}
		}
	}
}

func gsRender(t *testing.T, pdfWidth, pdfHeight float64, v pdf.Version, f func(page *document.Page)) image.Image {
	t.Helper()

	r, err := newGSRenderer(t, pdfWidth, pdfHeight, v)
	if err != nil {
		t.Fatal(err)
	}
	f(r.Page)
	img, err := r.Close()
	if err != nil {
		t.Fatal(err)
	}
	return img
}

type gsRenderer struct {
	Dir     string
	PDFName string

	*document.Page
}

func newGSRenderer(t *testing.T, width, height float64, v pdf.Version) (*gsRenderer, error) {
	t.Helper()

	// dir, _ := filepath.Abs("./xxx")
	dir := t.TempDir()

	pdfName := filepath.Join(dir, "test.pdf")
	paper := &pdf.Rectangle{
		URx: width,
		URy: height,
	}
	opt := &pdf.WriterOptions{Version: v}
	doc, err := document.CreateSinglePage(pdfName, paper, opt)
	if err != nil {
		return nil, err
	}

	res := &gsRenderer{
		Dir:     dir,
		PDFName: pdfName,
		Page:    doc,
	}

	return res, nil
}

func (r *gsRenderer) Close() (image.Image, error) {
	err := r.Page.Close()
	if err != nil {
		return nil, err
	}

	pngName := filepath.Join(r.Dir, "test.png")

	cmd := exec.Command(
		"gs", "-q",
		"-sDEVICE=png16m", fmt.Sprintf("-r%d", gsResolution),
		"-dTextAlphaBits=4", "-dGraphicsAlphaBits=4",
		"-o", pngName,
		r.PDFName)
	cmd.Dir = r.Dir
	cmd.Stdin = nil
	cmd.Stderr = nil
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	if len(out) > 0 {
		fmt.Println("unexpected ghostscript output:")
		fmt.Println(string(out))
	}

	fd, err := os.Open(pngName)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	img, err := png.Decode(fd)
	if err != nil {
		return nil, err
	}

	return img, nil
}

func haveGhostScript() bool {
	gsScriptOnce.Do(func() {
		out, err := exec.Command("gs", "-h").Output()
		if err != nil {
			gsScriptFound = false
			return
		}
		gsScriptFound = gsScriptPNGRe.Match(out)
	})
	return gsScriptFound
}

var (
	gsScriptOnce  sync.Once
	gsScriptPNGRe = regexp.MustCompile(`\bpng16m\b`)
	gsScriptFound bool
)

const gsResolution = 4 * 72
