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
	"seehuhn.de/go/sfnt/parser"
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

	font, err := sfnt.Read(bytes.NewReader(buf.Bytes()), parser.NewBudget(int64(buf.Len())))
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
// provided or if the font type is incompatible with the requested stream
// type.
//
// A CFF2 font never reaches this function through the normal embedding
// path (embedding always instances a variable font first, which yields
// static CFF or glyf outlines), so a caller passing one is misusing the
// API. Rather than panicking immediately, ToStream defers to the writer
// for the requested tp, which cleanly rejects CFF2 outlines: this keeps
// the caller-bug reporting uniform across all four tp values instead of
// panicking for some and erroring for others.
func ToStream(font *sfnt.Font, tp glyphdata.Type) *glyphdata.Stream {
	switch tp {
	case glyphdata.TrueType, glyphdata.OpenTypeGlyf, glyphdata.OpenTypeCFF, glyphdata.OpenTypeCFFSimple:
		// pass
	default:
		panic(fmt.Sprintf("invalid SFNT stream type: %v", tp))
	}

	isCFF := font.IsCFF()
	isCFF2 := font.IsCFF2()
	switch {
	case isCFF && (tp == glyphdata.TrueType || tp == glyphdata.OpenTypeGlyf):
		panic(fmt.Sprintf("CFF font cannot be used with type %v", tp))
	case !isCFF && !isCFF2 && (tp == glyphdata.OpenTypeCFF || tp == glyphdata.OpenTypeCFFSimple):
		panic(fmt.Sprintf("glyf font cannot be used with type %v", tp))
	}

	return &glyphdata.Stream{
		Type: tp,
		WriteTo: func(w io.Writer, length *glyphdata.Lengths) error {
			switch tp {
			case glyphdata.TrueType, glyphdata.OpenTypeGlyf:
				l1, err := font.WriteTrueTypePDF(w)
				if length != nil {
					length.Length1 = pdf.Integer(l1)
				}
				return err
			default: // OpenTypeCFF, OpenTypeCFFSimple
				return font.WriteOpenTypeCFFPDF(w)
			}
		},
	}
}
