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

package page

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
)

// PDF 2.0 sections: 7.8

// Content represents a single content stream that can be shared
// across multiple pages. It implements [pdf.Embedder] for deduplication.
type Content struct {
	Operators content.Stream
}

var _ pdf.Embedder = (*Content)(nil)

// Embed writes the content stream to the PDF file.
// No resource validation is performed; validation is done at the Page level.
func (pc *Content) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	ref := rm.Alloc()
	stm, err := rm.Out().OpenStream(ref, nil, pdf.FilterCompress{})
	if err != nil {
		return nil, err
	}

	for _, op := range pc.Operators {
		if err := content.WriteOperator(stm, op); err != nil {
			stm.Close()
			return nil, err
		}
	}

	if err := stm.Close(); err != nil {
		return nil, err
	}

	return ref, nil
}

// ExtractContent reads a single content stream from a PDF object.
func ExtractContent(x *pdf.Extractor, obj pdf.Object) (*Content, error) {
	resolved, err := x.Resolve(obj)
	if err != nil {
		return nil, err
	}

	stream, ok := resolved.(*pdf.Stream)
	if !ok {
		return nil, pdf.Errorf("expected stream, got %T", resolved)
	}

	stm, err := pdf.DecodeStream(x.R, stream, 0)
	if err != nil {
		return nil, err
	}
	defer stm.Close()

	operators, err := content.ReadStream(stm, pdf.GetVersion(x.R), content.Page)
	if err != nil {
		return nil, err
	}

	return &Content{Operators: operators}, nil
}
