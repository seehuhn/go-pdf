// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package graphics

import (
	"fmt"
	"io"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/internal/float"
	"seehuhn.de/go/pdf/pages"
)

// Page is a PDF page.
type Page struct {
	w          *pdf.Writer
	content    io.WriteCloser
	contentRef *pdf.Reference
	resources  *pdf.Resources

	tree *pages.Tree

	state state
	stack []state
	err   error

	newFont  font.Dict
	font     *font.Font
	fontSize float64
	textRise pdf.Integer

	fonts      map[font.Dict]pdf.Name
	imageNames map[pdf.Reference]pdf.Name
}

// AppendPage creates a new page and appends it to a page tree.
func AppendPage(tree *pages.Tree) (*Page, error) {
	p, err := NewPage(tree.Out)
	if err != nil {
		return nil, err
	}

	p.tree = tree

	return p, nil
}

// NewPage creates a new page without appending it to the page tree.
// Once the page is finished, the page dictionary returned by the [Close]
// method can be used to add the page to the page tree.
func NewPage(w *pdf.Writer) (*Page, error) {
	compress := &pdf.FilterInfo{Name: pdf.Name("LZWDecode")}
	if w.Version >= pdf.V1_2 {
		compress = &pdf.FilterInfo{Name: pdf.Name("FlateDecode")}
	}

	stream, contentRef, err := w.OpenStream(nil, nil, compress)
	if err != nil {
		return nil, err
	}

	return &Page{
		w:          w,
		content:    stream,
		contentRef: contentRef,

		state: stateGlobal,

		fonts: make(map[font.Dict]pdf.Name),
	}, nil
}

// Close must be called after drawing the page is complete.
// Any error that occurred during drawing is returned here.
// If the page was created with AppendPage, the returned page dictionary
// has already been added to the page tree and must not be modified.
func (p *Page) Close() (pdf.Dict, error) {
	if p.err != nil {
		return nil, p.err
	}

	err := p.content.Close()
	if err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Type":     pdf.Name("Page"),
		"Contents": p.contentRef,
	}
	if p.resources != nil {
		dict["Resources"] = pdf.AsDict(p.resources)
	}

	if p.tree != nil {
		_, err = p.tree.AppendPage(dict)
		if err != nil {
			return nil, err
		}
	}

	return dict, nil
}

func (p *Page) valid(cmd string, ss ...state) bool {
	if p.err != nil {
		return false
	}

	for _, s := range ss {
		if p.state == s {
			return true
		}
	}

	p.err = fmt.Errorf("unexpected state %q for %q", p.state, cmd)
	return false
}

func (p *Page) coord(x float64) string {
	// TODO(voss): think about this some more
	return float.Format(x, 2)
}

func (p *Page) AddExtGState(name pdf.Name, dict pdf.Dict) {
	if p.resources == nil {
		p.resources = &pdf.Resources{}
	}
	if p.resources.ExtGState == nil {
		p.resources.ExtGState = pdf.Dict{}
	}
	p.resources.ExtGState[name] = dict
}

type state int

// See Figure 9 (p. 113) of PDF 32000-1:2008.
const (
	stateNone state = iota
	stateGlobal
	statePath
	stateText
	stateClipped
	stateShading
	stateImage
	stateExternal
)

func (s state) String() string {
	switch s {
	case stateNone:
		return "none"
	case stateGlobal:
		return "global"
	case statePath:
		return "path"
	case stateText:
		return "text"
	case stateClipped:
		return "clipped"
	case stateShading:
		return "shading"
	case stateImage:
		return "image"
	case stateExternal:
		return "external"
	default:
		return fmt.Sprintf("state(%d)", s)
	}
}
