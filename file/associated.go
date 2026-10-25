// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package file

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/limits"
)

// ExtractAssociatedFiles reads the AF entry of an object (PDF 2.0, section
// 14.13).  Entries which are not file specifications, or which cannot be
// written back as associated files, are dropped.  The result is nil if no
// usable entry remains, so that the object reads as though it had no AF
// entry.  A list with more than [limits.MaxAssociatedFiles] entries is
// dropped as a whole rather than truncated.
func ExtractAssociatedFiles(c pdf.Cursor, obj pdf.Object) ([]*Specification, error) {
	afArray, err := pdf.Optional(c.Array(obj))
	if err != nil {
		return nil, err
	}
	if len(afArray) > limits.MaxAssociatedFiles {
		return nil, nil
	}

	var res []*Specification
	for _, afObj := range afArray {
		spec, err := c.DecodeOptional(afObj, ExtractSpecification)
		if err != nil {
			return nil, err
		}
		if spec != nil && spec.CanBeAF(pdf.V2_0) == nil {
			res = append(res, spec)
		}
	}
	return res, nil
}
