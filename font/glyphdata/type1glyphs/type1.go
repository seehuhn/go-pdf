// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package type1glyphs provides support for encoding and decoding Type 1 fonts
// in PDF.
package type1glyphs

import (
	"errors"
	"fmt"
	"io"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/postscript/type1"
)

// FromStream extracts a Type 1 font from a font file stream.
//
// The stream must have type [glyphdata.Type1].
// Returns an error if the stream has the wrong type or contains
// invalid Type 1 data.
func FromStream(stream *glyphdata.Stream) (*type1.Font, error) {
	if stream.Type != glyphdata.Type1 {
		return nil, errors.New("expected Type1 stream")
	}

	// The parser may stop before the end of the stream.  Closing the read
	// end then makes the writer fail, so that the goroutine always ends.
	r, w := io.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		// A panic in this goroutine cannot be recovered by the caller and would
		// terminate the process; report it to the parser instead.
		defer func() {
			if p := recover(); p != nil {
				w.CloseWithError(fmt.Errorf("extracting font data: panic: %v", p))
			}
		}()
		err := stream.WriteTo(w, nil)
		if err != nil {
			err = fmt.Errorf("extracting font data: %w", err)
		}
		w.CloseWithError(err)
	}()

	t1Font, err := type1.Read(r)
	r.Close()
	<-done
	if err != nil {
		return nil, fmt.Errorf("parsing Type1 font: %w", err)
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
