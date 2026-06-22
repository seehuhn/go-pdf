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

package ghostscript

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
)

var keepTempFiles = false

// RenderImage renders a single PDF page to an image using the ghostscript
// command-line tool.  It returns ErrNoGhostscript if ghostscript is not
// available.
//
// This is intended for use in code generators (see go:generate directives),
// which may compare ghostscript's idea of a PDF page against our own.
func RenderImage(pdfWidth, pdfHeight float64, v pdf.Version, f func(page *document.Page) error) (image.Image, error) {
	if !isAvailable() {
		return nil, ErrNoGhostscript
	}

	paper := &pdf.Rectangle{
		URx: pdfWidth,
		URy: pdfHeight,
	}
	r, err := newGSRenderer(paper, v)
	if err != nil {
		return nil, err
	}
	err = f(r.Page)
	if err != nil {
		return nil, err
	}
	return r.Close()
}

type gsRenderer struct {
	Dir     string
	PDFName string

	*document.Page
}

func newGSRenderer(paper *pdf.Rectangle, v pdf.Version) (*gsRenderer, error) {
	var dir string
	var err error
	if !keepTempFiles {
		dir, err = os.MkdirTemp("", "pdf")
		if err != nil {
			return nil, err
		}
	} else {
		const dirName = "./render-files"
		err = os.Mkdir(dirName, 0755)
		if err != nil && !os.IsExist(err) {
			return nil, err
		}
		dir, err = filepath.Abs(dirName)
		if err != nil {
			return nil, err
		}
	}

	idx := <-gsIndex
	gsIndex <- idx + 1

	pdfName := filepath.Join(dir, fmt.Sprintf("test%03d.pdf", idx))
	doc, err := document.CreateSinglePage(pdfName, paper, v, nil)
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

	pngName := strings.TrimSuffix(r.PDFName, ".pdf") + ".png"

	cmd := exec.Command(
		"gs", "-q",
		"-sDEVICE=png16m", fmt.Sprintf("-r%d", Resolution),
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

	if !keepTempFiles {
		err = os.RemoveAll(r.Dir)
		if err != nil {
			return nil, err
		}
	}

	return img, nil
}

// isAvailable returns true if the ghostscript command-line tool is available.
func isAvailable() bool {
	gsScriptOnce.Do(func() {
		out, err := exec.Command("gs", "-h").Output()
		if err != nil {
			gsScriptFound = false
			return
		}
		gsScriptFound = gsScriptPNGRe.Match(out)
		gsIndex <- 1
	})
	return gsScriptFound
}

// ErrNoGhostscript is returned if the ghostscript command-line tool is not
// available.
var ErrNoGhostscript = errors.New("cannot run ghostscript")

var (
	gsScriptOnce  sync.Once
	gsScriptPNGRe = regexp.MustCompile(`\bpng16m\b`)
	gsScriptFound bool
	gsIndex       = make(chan int, 1)
)

// Resolution is the image resolution in pixels per inch used for rendering.
const Resolution = 4 * 72
