// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package graphics

import (
	"fmt"

	"seehuhn.de/go/pdf"
)

// This file implements color-related PDF operators.
// The operators implemented here are defined in table 73 of
// ISO 32000-2:2020.

// SetColorSpaceStroke sets the current color space for nonstroking operations.
//
// This implements the PDF graphics operator "CS".
func (w *Writer) SetColorSpaceStroke(cs ColorSpace) {
	if !w.isValid("SetColorSpaceStroke", objPage|objText) {
		return
	}

	csFam := cs.ColorSpaceFamily()
	if minVersion, ok := colMinVersion[csFam]; ok && w.Version < minVersion {
		w.Err = &pdf.VersionError{Operation: "SetColorSpaceStroke", Earliest: minVersion}
	}

	var name pdf.Name
	if n, isName := cs.PDFObject().(pdf.Name); isName {
		name = n
	} else {
		name = w.getResourceName(catExtGState, cs)
	}

	if w.isSet(StateColorStroke) && w.ColorSpaceStroke == name {
		return
	}
	w.ColorSpaceStroke = name
	w.ColorValuesStroke = cs.DefaultValues()
	w.Set |= StateColorStroke

	err := name.PDF(w.Content)
	if err != nil {
		w.Err = err
		return
	}
	_, w.Err = fmt.Fprintln(w.Content, " CS")
}
