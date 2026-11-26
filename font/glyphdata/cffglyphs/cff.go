// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package cffglyphs provides support for encoding and decoding CFF font data
// in PDF.
package cffglyphs

import (
	"bytes"
	"fmt"
	"io"
	"slices"

	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/parser"
)

// FromStream extracts a CFF font from a font file stream.
//
// The stream must have type [glyphdata.CFF] or [glyphdata.CFFSimple].
// Returns an error if the stream has the wrong type or contains
// invalid CFF data.
func FromStream(stream *glyphdata.Stream) (*cff.Font, error) {
	if stream.Type != glyphdata.CFF && stream.Type != glyphdata.CFFSimple {
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

// CIDToGID returns the glyph ID of a CID in a CID-keyed CFF font.  Where the
// font program is not CID-keyed, the glyph ID equals the CID.  A CID which the
// program does not contain selects glyph 0.  Where several glyphs have the
// same CID, the one with the lowest glyph ID is selected.
func CIDToGID(cf *cff.Font) func(cid.CID) glyph.ID {
	if !cf.IsCIDKeyed() {
		return func(c cid.CID) glyph.ID {
			if int(c) >= len(cf.Glyphs) {
				return 0
			}
			return glyph.ID(c)
		}
	}
	m := make(map[cid.CID]glyph.ID, len(cf.GIDToCID))
	for gid, v := range slices.Backward(cf.GIDToCID) {
		m[v] = glyph.ID(gid)
	}
	return func(c cid.CID) glyph.ID { return m[c] }
}
