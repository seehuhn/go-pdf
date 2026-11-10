// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package traverse

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"

	"seehuhn.de/go/postscript/type1"
)

// type1GlyphListCtx represents a list of glyphs in a Type1 font.
type type1GlyphListCtx struct {
	font *type1.Font
}

// newType1GlyphListCtx creates a new Type1 glyph list context.
func newType1GlyphListCtx(font *type1.Font) (*type1GlyphListCtx, error) {
	if font == nil {
		return nil, errors.New("cannot create glyph list context from nil font")
	}
	return &type1GlyphListCtx{font: font}, nil
}

// Show displays the list of glyphs with their properties.
func (c *type1GlyphListCtx) Show() error {
	const indent = "  "

	fmt.Println("Glyph List (Type1 font):")
	headerIndent := indent
	fmt.Fprintf(os.Stdout, "%s Name               |   WidthX | BBox (LLx,LLy)-(URx,URy)\n", headerIndent)
	fmt.Fprintf(os.Stdout, "%s--------------------|----------|-------------------------\n", headerIndent)

	// Get all glyph names and sort them
	glyphNames := make([]string, 0, len(c.font.Glyphs))
	for name := range c.font.Glyphs {
		glyphNames = append(glyphNames, name)
	}
	sort.Strings(glyphNames)

	for _, name := range glyphNames {
		glyph := c.font.Glyphs[name]

		bboxStr := ""
		if !glyph.IsBlank() {
			bbox := c.font.GlyphBBoxPDF(name)
			if !bbox.IsZero() {
				bboxStr = fmt.Sprintf("(%g,%g)-(%g,%g)",
					bbox.LLx, bbox.LLy,
					bbox.URx, bbox.URy)
			}
		}

		fmt.Fprintf(os.Stdout, "%s%-19s | %8g | %s\n",
			headerIndent, name, glyph.WidthX, bboxStr)
	}

	return nil
}

// Next returns available steps for this context.
func (c *type1GlyphListCtx) Next() []Step {
	if c.font == nil || len(c.font.Glyphs) == 0 {
		return nil
	}
	return []Step{{
		Match: regexp.MustCompile(`^.+$`),
		Desc:  "glyph name",
		Next: func(key string) (Context, error) {
			if glyph, exists := c.font.Glyphs[key]; exists {
				return newType1GlyphCtx(c.font, key, glyph)
			}
			return nil, &KeyError{Key: key, Ctx: "type1 glyph list"}
		},
	}}
}
