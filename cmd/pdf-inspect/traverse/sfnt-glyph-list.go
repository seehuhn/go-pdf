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

package traverse

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/glyph"
)

// sfntGlyphListCtx represents a list of glyphs in a font.
type sfntGlyphListCtx struct {
	font *sfnt.Font
}

// newSfntGlyphListCtx creates a new glyph list context.
func newSfntGlyphListCtx(font *sfnt.Font) (*sfntGlyphListCtx, error) {
	if font == nil {
		return nil, errors.New("cannot create glyph list context from nil font")
	}
	return &sfntGlyphListCtx{font: font}, nil
}

// Show displays the list of glyphs with their properties.
func (c *sfntGlyphListCtx) Show() error {
	const indent = "  "

	fontType := "Unknown"
	if c.font.IsCFF() {
		fontType = "CFF"
	} else if c.font.IsGlyf() {
		fontType = "TrueType (glyf)"
	}

	fmt.Printf("Glyph List (%s font):\n", fontType)
	headerIndent := indent
	fmt.Fprintf(os.Stdout, "%s GID | Characters          | BBox (LLx,LLy)-(URx,URy) | Name\n", headerIndent)
	fmt.Fprintf(os.Stdout, "%s-----|---------------------|--------------------------|------\n", headerIndent)

	// Build a reverse mapping from GID to character codes
	gidToRunes := make(map[glyph.ID][]rune)
	if c.font.CMapTable != nil {
		// Get the best available cmap subtable
		subtable, err := c.font.CMapTable.GetBest()
		if err == nil && subtable != nil {
			// Get the range of characters covered by this subtable
			low, high := subtable.CodeRange()

			// Iterate through the character range and build reverse mapping
			for r := low; r <= high; r++ {
				gid := subtable.Lookup(r)
				if gid != 0 {
					gidToRunes[gid] = append(gidToRunes[gid], r)
				}
			}
		}
	}

	numGlyphs := c.font.NumGlyphs()
	for i := range numGlyphs {
		gid := glyph.ID(i)
		name := c.font.GlyphName(gid)
		bbox := c.font.GlyphBBox(gid)

		isBlank := bbox.IsZero()

		charStr := ""
		if runes, ok := gidToRunes[gid]; ok && len(runes) > 0 {
			var parts []string
			for j, r := range runes {
				if j >= 3 && len(runes) > 3 {
					parts = append(parts, "...")
					break
				}
				if unicode.IsPrint(r) {
					parts = append(parts, fmt.Sprintf("'%c'", r))
				} else {
					if r <= 0xFFFF {
						parts = append(parts, fmt.Sprintf("U+%04X", r))
					} else {
						parts = append(parts, fmt.Sprintf("U+%06X", r))
					}
				}
			}
			charStr = strings.Join(parts, ", ")
		}

		bboxStr := ""
		if !isBlank {
			bboxStr = fmt.Sprintf("(%d,%d)-(%d,%d)", bbox.LLx, bbox.LLy, bbox.URx, bbox.URy)
		}

		fmt.Fprintf(os.Stdout, "%s%4d | %-19s | %-24s | %s\n",
			headerIndent, i, charStr, bboxStr, name)
	}

	return nil
}

// Next returns available steps for this context.
func (c *sfntGlyphListCtx) Next() []Step {
	return nil
}
