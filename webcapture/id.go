// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package webcapture

import (
	"fmt"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 14.10.3

// Identifier represents a digital identifier for a web capture content set.
type Identifier struct {
	// ID represents the digital identifier of the content set.
	// This is an MD5 hash of the content (16 bytes).
	ID []byte

	// SingleUse determines if Embed returns a string (true) or
	// a reference (false).
	SingleUse bool
}

// ExtractIdentifier extracts an identifier from a PDF byte string object.
// Returns nil, nil if the object is absent or malformed.
func ExtractIdentifier(c pdf.Cursor, obj pdf.Object, _ bool) (*Identifier, error) {
	str, err := pdf.Optional(c.String(obj))
	if err != nil {
		return nil, err
	}
	if len(str) != 16 {
		return nil, nil
	}

	return &Identifier{ID: []byte(str)}, nil
}

// Embed converts the identifier to a PDF byte string.
func (id *Identifier) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "web capture identifier", pdf.V1_3); err != nil {
		return nil, err
	}

	if len(id.ID) != 16 {
		return nil, fmt.Errorf("invalid identifier length: %d != 16", len(id.ID))
	}

	pdfStr := pdf.String(id.ID)

	if id.SingleUse {
		return pdfStr, nil
	}

	ref := e.Alloc()
	err := e.Out().Put(ref, pdfStr)
	if err != nil {
		return nil, err
	}
	return ref, nil
}
