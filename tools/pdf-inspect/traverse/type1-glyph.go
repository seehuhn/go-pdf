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

	"seehuhn.de/go/geom/path"
	"seehuhn.de/go/postscript/type1"
)

// type1GlyphCtx represents an individual Type1 glyph for traversal.
type type1GlyphCtx struct {
	font      *type1.Font
	glyphName string
	glyph     *type1.Glyph
}

// newType1GlyphCtx creates a new Type1 glyph context.
func newType1GlyphCtx(font *type1.Font, glyphName string, glyph *type1.Glyph) (*type1GlyphCtx, error) {
	if font == nil {
		return nil, errors.New("cannot create glyph context from nil font")
	}
	if glyph == nil {
		return nil, fmt.Errorf("glyph %q not found", glyphName)
	}
	return &type1GlyphCtx{
		font:      font,
		glyphName: glyphName,
		glyph:     glyph,
	}, nil
}

// Show displays detailed information about the individual glyph.
func (c *type1GlyphCtx) Show() error {
	fmt.Printf("Glyph: %s\n", c.glyphName)
	fmt.Printf("WidthX: %g\n", c.glyph.WidthX)
	fmt.Printf("WidthY: %g\n", c.glyph.WidthY)

	// Show bounding box
	if !c.glyph.IsBlank() {
		bbox := c.font.GlyphBBoxPDF(c.glyphName)
		if !bbox.IsZero() {
			fmt.Printf("BBox: (%g,%g)-(%g,%g)\n", bbox.LLx, bbox.LLy, bbox.URx, bbox.URy)
		}
	}

	// Show stems
	if len(c.glyph.HStem) > 0 {
		fmt.Printf("Horizontal Stems: %v\n", c.glyph.HStem)
	}
	if len(c.glyph.VStem) > 0 {
		fmt.Printf("Vertical Stems: %v\n", c.glyph.VStem)
	}

	// Show outline path
	if c.glyph.Outline != nil && len(c.glyph.Outline.Cmds) > 0 {
		fmt.Println("\nOutline Path:")
		currentX, currentY := 0.0, 0.0
		coordIdx := 0

		for i, cmd := range c.glyph.Outline.Cmds {
			switch cmd {
			case path.CmdMoveTo:
				currentX, currentY = c.glyph.Outline.Coords[coordIdx].X, c.glyph.Outline.Coords[coordIdx].Y
				coordIdx++
				fmt.Printf("  %d: MoveTo(%g, %g)\n", i, currentX, currentY)
			case path.CmdLineTo:
				currentX, currentY = c.glyph.Outline.Coords[coordIdx].X, c.glyph.Outline.Coords[coordIdx].Y
				coordIdx++
				fmt.Printf("  %d: LineTo(%g, %g)\n", i, currentX, currentY)
			case path.CmdCubeTo:
				x1, y1 := c.glyph.Outline.Coords[coordIdx].X, c.glyph.Outline.Coords[coordIdx].Y
				x2, y2 := c.glyph.Outline.Coords[coordIdx+1].X, c.glyph.Outline.Coords[coordIdx+1].Y
				currentX, currentY = c.glyph.Outline.Coords[coordIdx+2].X, c.glyph.Outline.Coords[coordIdx+2].Y
				coordIdx += 3
				fmt.Printf("  %d: CurveTo(%g, %g, %g, %g, %g, %g)\n", i, x1, y1, x2, y2, currentX, currentY)
			case path.CmdClose:
				fmt.Printf("  %d: ClosePath()\n", i)
			}
		}
	} else {
		fmt.Println("\nOutline Path: (empty)")
	}

	return nil
}

// Next returns available steps for this context.
func (c *type1GlyphCtx) Next() []Step {
	return nil
}
