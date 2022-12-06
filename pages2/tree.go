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

package pages2

import "seehuhn.de/go/pdf"

// Tree represents a PDF page tree.
type Tree struct {
	w      *pdf.Writer
	parent *Tree

	ready []*pageDict
}

// dicts contains /Page or /Pages objects which are complete
// except for the /Parent field.
// New pages are appended at the end.
// Refs contains references to the /Page or /Pages objects.
type pageDict struct {
	dict pdf.Dict
	ref  *pdf.Reference
}

func NewTree(w *pdf.Writer) *Tree {
	return &Tree{w: w}
}

func (t *Tree) Close() error {
	panic("not implemented")
}

func (t *Tree) AddPage(page pdf.Dict) (*pdf.Reference, error) {
	panic("not implemented")
}

func (t *Tree) NewRange() (*Tree, error) {
	return &Tree{w: t.w, parent: t}, nil
}

func (t *Tree) FirstPageIndex() pdf.Object {
	panic("not implemented")
}

// DefaultAttributes specifies inheritable Page Attributes.
//
// These attributes are documented in sections 7.7.3.3 and 7.7.3.4 of
// PDF 32000-1:2008.
type DefaultAttributes struct {
	Resources *pdf.Resources

	// Mediabox defines the boundaries of the physical
	// medium on which the page shall be displayed or printed.
	MediaBox *pdf.Rectangle

	// Cropbox defines the visible region of default user space.  When the page
	// is displayed or printed, its contents shall be clipped (cropped) to this
	// rectangle and then shall be imposed on the output medium in some
	// implementation-defined manner.  Default value: the value of MediaBox.
	CropBox *pdf.Rectangle

	// Rotate gives the number of degrees by which the page shall be rotated
	// clockwise when displayed or printed.  The value shall be a multiple of
	// 90.
	Rotate int
}
