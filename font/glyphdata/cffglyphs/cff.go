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

// Package cffglyphs provides support for encoding and decoding CFF font data
// in PDF.
package cffglyphs

import (
	"bytes"
	"fmt"
	"io"

	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/sfnt/cff"
)

// FromStream extracts a CFF font from a font file stream.
//
// The stream must have type [glyphdata.CFF] or [glyphdata.CFFSimple].
// Returns an error if the stream is nil, has the wrong type, or contains
// invalid CFF data.
func FromStream(stream *glyphdata.Stream) (*cff.Font, error) {
	if stream == nil || (stream.Type != glyphdata.CFF && stream.Type != glyphdata.CFFSimple) {
		return nil, fmt.Errorf("expected CFF stream")
	}

	var buf bytes.Buffer
	err := stream.WriteTo(&buf, nil)
	if err != nil {
		return nil, fmt.Errorf("extracting font data: %w", err)
	}

	font, err := cff.Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		return nil, fmt.Errorf("parsing CFF font: %w", err)
	}

	return font, nil
}

// ToStream creates a font file stream from CFF font data.
//
// The tp parameter must be either [glyphdata.CFFSimple] or [glyphdata.CFF].
// The function panics if an invalid type is provided.
func ToStream(font *cff.Font, tp glyphdata.Type) *glyphdata.Stream {
	if tp != glyphdata.CFFSimple && tp != glyphdata.CFF {
		panic(fmt.Sprintf("invalid CFF stream type: %v", tp))
	}

	return &glyphdata.Stream{
		Type: tp,
		WriteTo: func(w io.Writer, length *glyphdata.Lengths) error {
			return font.Write(w)
		},
	}
}
