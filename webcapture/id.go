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
func ExtractIdentifier(x *pdf.Extractor, obj pdf.Object) (*Identifier, error) {
	str, err := pdf.Optional(x.GetString(obj))
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
