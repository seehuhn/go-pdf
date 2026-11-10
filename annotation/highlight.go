// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
