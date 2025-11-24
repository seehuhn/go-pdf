// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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
	"io"

	"seehuhn.de/go/pdf"
)

// ContentStream represents a file-independent PDF content stream object.
// It contains a resource dictionary and a slice of PDF graphics operators.
type ContentStream struct {
	// Resources contains the resource dictionary for this content stream.
	Resources *pdf.Resources

	// Operators contains the sequence of graphics operators in this stream.
	Operators []Operator
}

// NewContentStream creates a new empty content stream.
func NewContentStream() *ContentStream {
	return &ContentStream{
		Resources: &pdf.Resources{},
		Operators: []Operator{},
	}
}

// ExtractContentStream extracts a content stream from a PDF file.
func ExtractContentStream(x *pdf.Extractor, obj pdf.Object) (*ContentStream, error) {
	// TODO: Implement content stream extraction
	// This would parse the content stream and extract operators
	return nil, pdf.Error("ExtractContentStream not yet implemented")
}

// Embed adds the content stream to a PDF file.
//
// This implements the [pdf.Embedder] interface.
func (cs *ContentStream) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	// TODO: Implement embedding
	// This would write the operators to a stream and embed the resources
	return nil, pdf.Error("ContentStream.Embed not yet implemented")
}

// WriteTo writes the content stream operators to an io.Writer.
// This is used when the content stream needs to be written to a PDF stream.
func (cs *ContentStream) WriteTo(w io.Writer, opt pdf.OutputOptions) error {
	for _, op := range cs.Operators {
		// Write arguments
		for i, arg := range op.Args {
			if i > 0 {
				_, err := w.Write([]byte(" "))
				if err != nil {
					return err
				}
			}
			err := pdf.Format(w, opt, arg)
			if err != nil {
				return err
			}
		}
		// Write operator name
		if len(op.Args) > 0 {
			_, err := w.Write([]byte(" "))
			if err != nil {
				return err
			}
		}
		_, err := w.Write([]byte(op.Name))
		if err != nil {
			return err
		}
		_, err = w.Write([]byte("\n"))
		if err != nil {
			return err
		}
	}
	return nil
}
