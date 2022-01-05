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
// different methods to write the PDF content stream for the page.
// After the content stream has been written, the .Close() method must be called.
type Page struct {
	BBox *pdf.Rectangle
	w    *bufio.Writer
	stm  io.WriteCloser
	dict pdf.Dict
	pp   *PageRange
}

// NewPage adds a new page to the page range and returns an object which
// can be used to write the content stream for the page.
func (pp *PageRange) NewPage(attr *Attributes) (*Page, error) {
	if pp.inPage {
		return nil, errors.New("previous page not closed")
	}

	tree := pp.tree

	var mediaBox *pdf.Rectangle
	inherited := true
	if attr != nil && attr.MediaBox != nil {
		mediaBox = attr.MediaBox
		inherited = false
	} else if pp.attr != nil && pp.attr.MediaBox != nil {
		mediaBox = pp.attr.MediaBox
	} else if tree.attr != nil && tree.attr.MediaBox != nil {
		mediaBox = tree.attr.MediaBox
	} else {
		return nil, errors.New("missing MediaBox")
	}

	defaultAttr := pp.attr
	if defaultAttr == nil {
		defaultAttr = tree.attr
	}

	pageDict := pdf.Dict{
		"Type": pdf.Name("Page"),
	}
	if !inherited {
		pageDict["MediaBox"] = mediaBox
	}
	if attr != nil {
		if attr.Resources != nil {
			pageDict["Resources"] = pdf.AsDict(attr.Resources)
		}
		if attr.CropBox != nil &&
			(defaultAttr == nil ||
				defaultAttr.CropBox == nil ||
				!defaultAttr.CropBox.NearlyEqual(attr.CropBox, 1)) {
			pageDict["CropBox"] = attr.CropBox
		}
		if attr.Rotate != 0 && (defaultAttr == nil || defaultAttr.Rotate != attr.Rotate) {
			pageDict["Rotate"] = pdf.Integer(attr.Rotate)
		}
	}

	compress := &pdf.FilterInfo{Name: pdf.Name("LZWDecode")}
	if tree.w.Version >= pdf.V1_2 {
		compress = &pdf.FilterInfo{Name: pdf.Name("FlateDecode")}
	}
	stream, contentRef, err := tree.w.OpenStream(nil, nil, compress)
	if err != nil {
		return nil, err
	}
	pageDict["Contents"] = contentRef

	pp.inPage = true

	return &Page{
		BBox: mediaBox,
		w:    bufio.NewWriter(stream),
		stm:  stream,
		dict: pageDict,
		pp:   pp,
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
	err = p.stm.Close()
	if err != nil {
		return err
	}
	p.pp.inPage = false
	return p.pp.Append(p.dict)
}

// Write writes the contents of buf to the content stream.  It returns the
// number of bytes written.  If n < len(buf), it also returns an error
// explaining why the write is short.
func (p *Page) Write(buf []byte) (n int, err error) {
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
