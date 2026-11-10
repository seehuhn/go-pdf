// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package type1

import (
	"math"

	"seehuhn.de/go/pdf/font/encoding/simpleenc"
	"seehuhn.de/go/pdf/internal/stdmtx"
)

// IsStandard returns true if the font is one of the standard 14 PDF fonts.
// This is determined by the font name, the set of glyphs used, and the glyph
// widths.
func isStandard(fontName string, t *simpleenc.Simple) bool {
	m, ok := stdmtx.Metrics[fontName]
	if !ok {
		return false
	}

	for code, info := range t.MappedCodes() {
		gid := t.GID(code)
		glyphName := t.GlyphName(gid)
		wStd, ok := m.Width[glyphName]
		if !ok {
			// The glyph is not in the standard font.
			return false
		}

		wOurs := info.Width

		if math.Abs(wStd-wOurs) > 0.5 {
			return false
		}
	}
	return true
}
