// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package cffglyphs provides support for encoding and decoding CFF font data
// in PDF.
package cffglyphs

import (
	"bytes"
	"fmt"
	"io"

	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/parser"
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

	font, err := cff.Read(bytes.NewReader(buf.Bytes()), parser.NewBudget(int64(buf.Len())))
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
