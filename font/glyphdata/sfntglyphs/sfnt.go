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

// Package sfntglyphs provides support for encoding and decoding TrueType and
// OpenType fonts in PDF.
package sfntglyphs

import (
	"bytes"
	"fmt"
	"io"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/sfnt"
)

// FromStream extracts an OpenType/TrueType font from a font file stream.
//
// The stream must have type [glyphdata.TrueType], [glyphdata.OpenTypeGlyf],
// [glyphdata.OpenTypeCFF], or [glyphdata.OpenTypeCFFSimple].
// Returns an error if the stream is nil, has the wrong type, or contains
// invalid font data.
func FromStream(stream *glyphdata.Stream) (*sfnt.Font, error) {
	if stream == nil || (stream.Type != glyphdata.TrueType &&
		stream.Type != glyphdata.OpenTypeGlyf &&
		stream.Type != glyphdata.OpenTypeCFF &&
		stream.Type != glyphdata.OpenTypeCFFSimple) {
		return nil, fmt.Errorf("expected OpenType/TrueType stream")
	}

	var buf bytes.Buffer
	err := stream.WriteTo(&buf, nil)
	if err != nil {
		return nil, fmt.Errorf("extracting font data: %w", err)
	}

	font, err := sfnt.Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		return nil, fmt.Errorf("parsing font: %w", err)
	}

	return font, nil
}

// ToStream creates a font file stream from SFNT font data.
//
// The tp parameter must be one of [glyphdata.TrueType],
// [glyphdata.OpenTypeGlyf], [glyphdata.OpenTypeCFF], or
// [glyphdata.OpenTypeCFFSimple]. The function panics if an invalid type is
// provided or if the font type is incompatible with the requested stream type.
func ToStream(font *sfnt.Font, tp glyphdata.Type) *glyphdata.Stream {
	switch tp {
	case glyphdata.TrueType, glyphdata.OpenTypeGlyf, glyphdata.OpenTypeCFF, glyphdata.OpenTypeCFFSimple:
		// pass
	default:
		panic(fmt.Sprintf("invalid SFNT stream type: %v", tp))
	}

	isCFF := font.IsCFF()
	switch {
	case isCFF && (tp == glyphdata.TrueType || tp == glyphdata.OpenTypeGlyf):
		panic(fmt.Sprintf("CFF font cannot be used with type %v", tp))
	case !isCFF && (tp == glyphdata.OpenTypeCFF || tp == glyphdata.OpenTypeCFFSimple):
		panic(fmt.Sprintf("glyf font cannot be used with type %v", tp))
	}

	return &glyphdata.Stream{
		Type: tp,
		WriteTo: func(w io.Writer, length *glyphdata.Lengths) error {
			if isCFF {
				return font.WriteOpenTypeCFFPDF(w)
			} else {
				l1, err := font.WriteTrueTypePDF(w)
				if length != nil {
					length.Length1 = pdf.Integer(l1)
				}
				return err
			}
		},
	}
}
