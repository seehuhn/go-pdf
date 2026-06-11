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

package annotation

import "seehuhn.de/go/pdf"

// PDF 2.0 sections: 12.5.6.5 12.5.6.19

// Highlight is the highlighting mode of a link or widget annotation,
// the visual effect used while the mouse button is pressed or held down
// inside the annotation's active area.
// The value must be one of the constants below.
type Highlight pdf.Name

// Valid values for the [Highlight] type.
const (
	HighlightNone    Highlight = "N" // no highlighting
	HighlightInvert  Highlight = "I" // invert the annotation rectangle
	HighlightOutline Highlight = "O" // invert the border
	HighlightPush    Highlight = "P" // push-down effect
)

// encodeEntry adds the /H entry to an annotation dictionary.
// The default mode, [HighlightInvert], and the empty shorthand for it
// are not written.
func (h Highlight) encodeEntry(rm *pdf.ResourceManager, dict pdf.Dict, what string) error {
	if h == "" || h == HighlightInvert {
		return nil
	}
	if err := pdf.CheckVersion(rm.Out, what, pdf.V1_2); err != nil {
		return err
	}
	dict["H"] = pdf.Name(h)
	return nil
}
