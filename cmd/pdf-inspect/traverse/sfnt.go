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
	"fmt"
	"regexp"
	"sort"
	"strings"

	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/sfntglyphs"
)

// sfntCtx represents a parsed font for traversal.
type sfntCtx struct {
	font *sfnt.Font
}

// newSfntCtx creates a new font context by reading and parsing the font file.
func newSfntCtx(fontFile *glyphdata.Stream) (*sfntCtx, error) {
	sfont, err := sfntglyphs.FromStream(fontFile)
	if err != nil {
		return nil, err
	}

	return &sfntCtx{font: sfont}, nil
}

// Show displays basic information about the font.
func (c *sfntCtx) Show() error {
	if c.font == nil {
		fmt.Println("sfnt.Font: (nil)")
		return nil
	}
	fmt.Printf("Family Name: %s\n", c.font.FamilyName)
	if name := c.font.PostScriptName(); name != "" {
		fmt.Printf("PostScript Name: %s\n", name)
	}
	fmt.Printf("Number of Glyphs: %d\n", c.font.NumGlyphs())
	fmt.Printf("Units Per Em: %d\n", c.font.UnitsPerEm)
	fmt.Printf("IsCFF: %t\n", c.font.IsCFF())
	fmt.Printf("IsGlyf: %t\n", c.font.IsGlyf())

	if c.font.CMapTable != nil {
		var cmapKeys []string
		for key := range c.font.CMapTable {
			cmapKeys = append(cmapKeys, fmt.Sprintf("(%d,%d)", key.PlatformID, key.EncodingID))
		}
		sort.Strings(cmapKeys)
		fmt.Printf("Cmap tables: %s\n", strings.Join(cmapKeys, ", "))
	}

	return nil
}

// Next returns available steps for this context.
func (c *sfntCtx) Next() []Step {
	return []Step{{
		Match: regexp.MustCompile(`^glyphs$`),
		Desc:  "`glyphs`",
		Next: func(key string) (Context, error) {
			return newSfntGlyphListCtx(c.font)
		},
	}}
}
