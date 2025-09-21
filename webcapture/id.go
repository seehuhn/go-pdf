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

// Identifier represents a digital identifier for a web capture content set.
type Identifier struct {
	// ID represents the digital identifier of the content set.
	// This is an MD5 hash of the content (16 bytes).
	ID []byte
}

// ExtractIdentifier extracts an identifier from a PDF byte string object.
func ExtractIdentifier(x *pdf.Extractor, obj pdf.Object) (*Identifier, error) {
	str, err := pdf.Optional(pdf.GetString(x.R, obj))
	if err != nil {
		return nil, err
	}

	hash := []byte(str)
	if len(hash) != 16 {
		return nil, pdf.Errorf("invalid identifier length: %d != 16", len(hash))
	}

	id := &Identifier{
		ID: hash,
	}

	return id, nil
}

// Embed converts the identifier to a PDF byte string as an indirect object.
func (id *Identifier) Embed(rm *pdf.EmbedHelper) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	// Validate the ID
	if len(id.ID) != 16 {
		return nil, zero, fmt.Errorf("invalid identifier length: %d != 16", len(id.ID))
	}

	// Create PDF byte string
	pdfStr := pdf.String(id.ID)

	// Always create as indirect object
	ref := rm.Alloc()
	err := rm.Out().Put(ref, pdfStr)
	if err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
}
