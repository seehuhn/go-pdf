// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package pdf

// TextAlign represents the justification of text, used for the /Q entry of
// free text annotations, redaction annotations, and interactive form fields
// containing variable text.
type TextAlign int

const (
	// TextAlignLeft left-justifies text against the left edge.
	TextAlignLeft TextAlign = 0

	// TextAlignCenter centres text between the left and right edges.
	TextAlignCenter TextAlign = 1

	// TextAlignRight right-justifies text against the right edge.
	TextAlignRight TextAlign = 2
)
