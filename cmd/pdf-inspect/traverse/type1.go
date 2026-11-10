// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package traverse

import (
	"fmt"
	"regexp"

	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/type1glyphs"
	"seehuhn.de/go/postscript/type1"
)

// type1Ctx represents a parsed Type1 font for traversal.
type type1Ctx struct {
	font *type1.Font
}

// newType1Ctx creates a new Type1 font context by reading and parsing the font file.
func newType1Ctx(fontFile *glyphdata.Stream) (*type1Ctx, error) {
	t1Font, err := type1glyphs.FromStream(fontFile)
	if err != nil {
		return nil, err
	}

	return &type1Ctx{font: t1Font}, nil
}

// Show displays basic information about the Type1 font.
func (c *type1Ctx) Show() error {
	if c.font == nil {
		fmt.Println("type1.Font: (nil)")
		return nil
	}

	fmt.Printf("FontName: %s\n", c.font.FontName)
	if c.font.FullName != "" {
		fmt.Printf("FullName: %s\n", c.font.FullName)
	}
	if c.font.FamilyName != "" {
		fmt.Printf("FamilyName: %s\n", c.font.FamilyName)
	}
	if c.font.Weight != "" {
		fmt.Printf("Weight: %s\n", c.font.Weight)
	}
	fmt.Printf("ItalicAngle: %.1f°\n", c.font.ItalicAngle)
	fmt.Printf("IsFixedPitch: %t\n", c.font.IsFixedPitch)
	fmt.Printf("UnderlinePosition: %.1f\n", c.font.UnderlinePosition)
	fmt.Printf("UnderlineThickness: %.1f\n", c.font.UnderlineThickness)

	if c.font.Glyphs != nil {
		fmt.Printf("Number of Glyphs: %d\n", len(c.font.Glyphs))
	}

	if !c.font.CreationDate.IsZero() {
		fmt.Printf("CreationDate: %s\n", c.font.CreationDate.Format("2006-01-02 15:04:05"))
	}

	return nil
}

// Next returns available steps for this context.
func (c *type1Ctx) Next() []Step {
	return []Step{{
		Match: regexp.MustCompile(`^glyphs$`),
		Desc:  "`glyphs`",
		Next: func(key string) (Context, error) {
			return newType1GlyphListCtx(c.font)
		},
	}}
}
