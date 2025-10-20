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

package memfile

import "seehuhn.de/go/pdf"

// NewPDFWriter creates a new PDF writer which writes to a MemFile.
// The returned writer can still be used as a reader after it has been closed.
func NewPDFWriter(v pdf.Version, opt *pdf.WriterOptions) (*pdf.Writer, *MemFile) {
	tmpFile := New()
	w, err := pdf.NewWriter(tmpFile, v, opt)
	if err != nil {
		panic(err)
	}
	return w, tmpFile
}

// AddBlankPage adds a minimal page tree with a single blank page to the writer.
// This creates the minimum structure required for a valid PDF document that can
// be read by pdf.NewReader.
func AddBlankPage(w *pdf.Writer) error {
	pageRef := w.Alloc()
	pagesRef := w.Alloc()

	pageDict := pdf.Dict{
		"Type":      pdf.Name("Page"),
		"Parent":    pagesRef,
		"Resources": pdf.Dict{},
		"MediaBox":  &pdf.Rectangle{URx: 100, URy: 100},
	}
	err := w.Put(pageRef, pageDict)
	if err != nil {
		return err
	}

	pagesDict := pdf.Dict{
		"Type":  pdf.Name("Pages"),
		"Kids":  pdf.Array{pageRef},
		"Count": pdf.Integer(1),
	}
	err = w.Put(pagesRef, pagesDict)
	if err != nil {
		return err
	}

	w.GetMeta().Catalog.Pages = pagesRef
	return nil
}
