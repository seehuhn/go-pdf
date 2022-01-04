// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package pages

import (
	"bufio"
	"errors"
	"fmt"
	"io"

	"seehuhn.de/go/pdf"
)

// Page represents the contents of a page in the PDF file.  The object provides
// .Write() and .WriteString() methods to write the PDF content stream for the
// page.  The .Close() method must be called after the content stream has been
// written completely.
type Page struct {
	LLx, LLy, URx, URy float64 // The media box for the page

	w   *bufio.Writer
	stm io.WriteCloser
}

// AddPage adds a new page to the page tree and returns an object which
// can be used to write the content stream for the page.
func (tree *PageTree) AddPage(attr *Attributes) (*Page, error) {
	contentRef, mediaBox, err := tree.addPageInternal(attr)
	if err != nil {
		return nil, err
	}

	return tree.newPage(contentRef, mediaBox)
}

func (tree *PageTree) addPageInternal(attr *Attributes) (*pdf.Reference, *pdf.Rectangle, error) {
	var mediaBox *pdf.Rectangle
	def := tree.defaults
	if def != nil {
		mediaBox = def.MediaBox
	}
	if attr != nil && attr.MediaBox != nil {
		mediaBox = attr.MediaBox
	}
	if mediaBox == nil {
		return nil, nil, errors.New("missing MediaBox")
	}

	contentRef := tree.w.Alloc()

	pageDict := pdf.Dict{
		"Type":     pdf.Name("Page"),
		"Contents": contentRef,
	}
	if attr != nil {
		if attr.Resources != nil {
			pageDict["Resources"] = pdf.AsDict(attr.Resources)
		}
		if attr.MediaBox != nil &&
			(def == nil ||
				def.MediaBox == nil ||
				!def.MediaBox.NearlyEqual(attr.MediaBox, 1)) {
			pageDict["MediaBox"] = attr.MediaBox
		}
		if attr.CropBox != nil &&
			(def == nil ||
				def.CropBox == nil ||
				!def.CropBox.NearlyEqual(attr.CropBox, 1)) {
			pageDict["CropBox"] = attr.CropBox
		}
		if attr.Rotate != 0 && def.Rotate != attr.Rotate {
			pageDict["Rotate"] = pdf.Integer(attr.Rotate)
		}
	}
	err := tree.Ship(pageDict, nil)
	if err != nil {
		return nil, nil, err
	}

	return contentRef, mediaBox, nil
}

func (tree *PageTree) newPage(contentRef *pdf.Reference, mediaBox *pdf.Rectangle) (*Page, error) {
	compress := &pdf.FilterInfo{Name: pdf.Name("LZWDecode")}
	if tree.w.Version >= pdf.V1_2 {
		compress = &pdf.FilterInfo{Name: pdf.Name("FlateDecode")}
	}
	stream, _, err := tree.w.OpenStream(nil, contentRef, compress)
	if err != nil {
		return nil, err
	}
	return &Page{
		LLx: mediaBox.LLx,
		LLy: mediaBox.LLy,
		URx: mediaBox.URx,
		URy: mediaBox.URy,

		w:   bufio.NewWriter(stream),
		stm: stream,
	}, nil
}

// Close writes any buffered data to the content stream and then closes the
// stream.  The Page object cannot be used any more after .Close() has been
// called.
func (p *Page) Close() error {
	err := p.w.Flush()
	if err != nil {
		return err
	}
	p.w = nil
	return p.stm.Close()
}

// Write writes the contents of buf to the content stream.  It returns the
// number of bytes written.  If nn < len(p), it also returns an error
// explaining why the write is short.
func (p *Page) Write(buf []byte) (int, error) {
	return p.w.Write(buf)
}

// Print formats the arguments using their default formats and writes the
// resulting string to the content stream.  Spaces are added between operands
// when neither is a string.
func (p *Page) Print(a ...interface{}) (int, error) {
	return p.w.WriteString(fmt.Sprint(a...))
}

// Printf formats the arguments according to a format specifier and writes the
// resulting string to the content stream.
func (p *Page) Printf(format string, a ...interface{}) (int, error) {
	return p.w.WriteString(fmt.Sprintf(format, a...))
}

// Println formats its arguments using their default formats and writes the
// resulting string to the content stream.  Spaces are always added between
// operands and a newline is appended.
func (p *Page) Println(a ...interface{}) (int, error) {
	return p.w.WriteString(fmt.Sprintln(a...))
}
