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

// Package type1glyphs provides support for encoding and decoding Type 1 fonts
// in PDF.
package type1glyphs

import (
	"fmt"
	"io"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/postscript/type1"
)

// FromStream extracts a Type 1 font from a font file stream.
//
// The stream must have type [glyphdata.Type1].
// Returns an error if the stream is nil, has the wrong type, or contains
// invalid Type 1 data.
func FromStream(stream *glyphdata.Stream) (*type1.Font, error) {
	if stream == nil || stream.Type != glyphdata.Type1 {
		return nil, fmt.Errorf("expected Type1 stream")
	}

	r, w := io.Pipe()
	var t1Font *type1.Font
	var parseErr error

	go func() {
		defer w.Close()
		err := stream.WriteTo(w, nil)
		if err != nil {
			w.CloseWithError(fmt.Errorf("extracting font data: %w", err))
		}
	}()

	t1Font, parseErr = type1.Read(r)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing Type1 font: %w", parseErr)
	}

	return t1Font, nil
}

// ToStream creates a font file stream from Type 1 font data.
func ToStream(font *type1.Font) *glyphdata.Stream {
	return &glyphdata.Stream{
		Type: glyphdata.Type1,
		WriteTo: func(w io.Writer, length *glyphdata.Lengths) error {
			l1, l2, err := font.WritePDF(w)
			if length != nil {
				length.Length1 = pdf.Integer(l1)
				length.Length2 = pdf.Integer(l2)
				length.Length3 = pdf.Integer(0)
			}
			return err
		},
	}
}
