// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
