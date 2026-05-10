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

package halftone

import (
	"seehuhn.de/go/pdf"
)

// maxHalftoneAxis bounds the per-axis size of any halftone threshold array
// read from a PDF file, to protect against memory exhaustion via malicious
// input. The cap applies to the read path only; writers are not restricted.
const maxHalftoneAxis = 1 << 11

// validateType6Dims checks the Width and Height of a Type 6 halftone read
// from a PDF file.
func validateType6Dims(width, height int) error {
	if width <= 0 || width > maxHalftoneAxis {
		return pdf.Errorf("Type 6 Width %d out of range [1, %d]", width, maxHalftoneAxis)
	}
	if height <= 0 || height > maxHalftoneAxis {
		return pdf.Errorf("Type 6 Height %d out of range [1, %d]", height, maxHalftoneAxis)
	}
	return nil
}

// validateType10Dims checks the Xsquare and Ysquare of a Type 10 halftone
// read from a PDF file.
func validateType10Dims(size1, size2 int) error {
	if size1 <= 0 || size1 > maxHalftoneAxis {
		return pdf.Errorf("Type 10 Xsquare %d out of range [1, %d]", size1, maxHalftoneAxis)
	}
	if size2 <= 0 || size2 > maxHalftoneAxis {
		return pdf.Errorf("Type 10 Ysquare %d out of range [1, %d]", size2, maxHalftoneAxis)
	}
	return nil
}

// validateType16Dims checks the four axis fields of a Type 16 halftone read
// from a PDF file and reports whether a second rectangle is present
// (Width2 > 0 or Height2 > 0). When hasSecondRect is true, both Width2 and
// Height2 must be in range.
func validateType16Dims(width, height, width2, height2 int) (hasSecondRect bool, err error) {
	if width <= 0 || width > maxHalftoneAxis {
		return false, pdf.Errorf("Type 16 Width %d out of range [1, %d]", width, maxHalftoneAxis)
	}
	if height <= 0 || height > maxHalftoneAxis {
		return false, pdf.Errorf("Type 16 Height %d out of range [1, %d]", height, maxHalftoneAxis)
	}
	hasSecondRect = width2 > 0 || height2 > 0
	if hasSecondRect {
		if width2 <= 0 || width2 > maxHalftoneAxis {
			return false, pdf.Errorf("Type 16 Width2 %d out of range [1, %d]", width2, maxHalftoneAxis)
		}
		if height2 <= 0 || height2 > maxHalftoneAxis {
			return false, pdf.Errorf("Type 16 Height2 %d out of range [1, %d]", height2, maxHalftoneAxis)
		}
	}
	return hasSecondRect, nil
}

// rejectNestedType5 returns an error if obj resolves to an inline halftone
// dict whose HalftoneType is 5. This prevents unbounded recursion when
// extracting a Type 5 halftone whose Default or colorant entry is itself
// Type 5: indirect-reference cycles among Type 5 dicts are caught by
// pdf.ExtractorGet's path.Seen check, but inline nesting and deep linear
// chains of distinct refs are not — without this check, a malicious file
// can grow the call stack proportionally to file size.
func rejectNestedType5(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, role string) error {
	resolved, err := pdf.Resolve(x.R, obj)
	if err != nil || resolved == nil {
		return nil // let the main extract path surface the error
	}
	d, ok := resolved.(pdf.Dict)
	if !ok {
		return nil // streams (Types 6/10/16) cannot be Type 5
	}
	t, _ := x.GetInteger(path, d["HalftoneType"])
	if t != 5 {
		return nil
	}
	return pdf.Errorf("invalid %s halftone: Type 5 cannot be nested", role)
}
