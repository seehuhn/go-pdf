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
	"regexp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/postscript/type1"
)

// type1Ctx represents a parsed Type1 font for traversal.
type type1Ctx struct {
	font *type1.Font
}

// newType1Ctx creates a new Type1 font context by reading and parsing the font program.
func newType1Ctx(getter pdf.Getter, fontRef pdf.Reference) (*type1Ctx, error) {
	if fontRef == 0 {
		return nil, errors.New("invalid font reference for `load`")
	}

	stm, err := pdf.GetStream(getter, fontRef)
	if err != nil {
		return nil, fmt.Errorf("getting font program stream for `load`: %w", err)
	}
	if stm == nil {
		return nil, errors.New("missing font program stream for `load`")
	}

	decoded, err := pdf.DecodeStream(getter, stm, 0)
	if err != nil {
		return nil, fmt.Errorf("decoding font program stream for `load`: %w", err)
	}
	defer decoded.Close()

	t1font, err := type1.Read(decoded)
	if err != nil {
		return nil, fmt.Errorf("parsing type1 font for `load`: %w", err)
	}

	return &type1Ctx{font: t1font}, nil
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
