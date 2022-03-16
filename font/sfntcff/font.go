// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package sfntcff

import (
	"regexp"
	"strings"
	"time"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/sfnt/cmap"
	"seehuhn.de/go/pdf/font/sfnt/glyf"
	"seehuhn.de/go/pdf/font/sfnt/head"
	"seehuhn.de/go/pdf/font/sfnt/os2"
	"seehuhn.de/go/pdf/font/sfnt/table"
)

// Info contains information about the font.
type Info struct {
	FamilyName string
	Width      font.Width
	Weight     font.Weight

	Version          head.Version
	CreationTime     time.Time
	ModificationTime time.Time

	Copyright string
	Trademark string
	PermUse   os2.Permissions

	UnitsPerEm uint16

	Ascent    int16
	Descent   int16 // negative
	LineGap   int16
	CapHeight int16
	XHeight   int16

	ItalicAngle        float64 // Italic angle (degrees counterclockwise from vertical)
	UnderlinePosition  int16   // Underline position (negative)
	UnderlineThickness int16   // Underline thickness

	IsItalic  bool // Glyphs have dominant vertical strokes that are slanted.
	IsBold    bool
	IsRegular bool
	IsOblique bool
	IsSerif   bool
	IsScript  bool // Glyphs resemble cursive handwriting.

	CMap cmap.Subtable
	Font interface{} // either *cff.Outlines or *TTInfo
}

// TTFOutlines contains information specific to TrueType fonts.
type TTFOutlines struct {
	Widths []uint16
	Glyphs glyf.Glyphs
	Tables map[string][]byte
	Maxp   *table.MaxpTTF
}

// NumGlyphs returns the number of glyphs in the font.
func (info *Info) NumGlyphs() int {
	switch outlines := info.Font.(type) {
	case *cff.Outlines:
		return len(outlines.Glyphs)
	case *TTFOutlines:
		return len(outlines.Glyphs)
	default:
		panic("unexpected font type")
	}
}

// Widths return the advance widths of the glyphs of the font.
func (info *Info) Widths() []uint16 {
	switch f := info.Font.(type) {
	case *cff.Outlines:
		widths := make([]uint16, len(f.Glyphs))
		for i, g := range f.Glyphs {
			widths[i] = g.Width
		}
		return widths
	case *TTFOutlines:
		return f.Widths
	default:
		panic("unexpected font type")
	}
}

// Extent returns the glyph bounding box for one glyph.
func (info *Info) Extent(gid font.GlyphID) font.Rect {
	switch f := info.Font.(type) {
	case *cff.Outlines:
		return f.Glyphs[gid].Extent()
	case *TTFOutlines:
		g := f.Glyphs[gid]
		if g == nil {
			return font.Rect{}
		}
		return g.Rect
	default:
		panic("unexpected font type")
	}
}

// Extents returns the glyph bounding boxes for the font.
func (info *Info) Extents() []font.Rect {
	switch f := info.Font.(type) {
	case *cff.Outlines:
		extents := make([]font.Rect, len(f.Glyphs))
		for i, g := range f.Glyphs {
			extents[i] = g.Extent()
		}
		return extents
	case *TTFOutlines:
		extents := make([]font.Rect, len(f.Glyphs))
		for i, g := range f.Glyphs {
			if g == nil {
				continue
			}
			extents[i] = g.Rect
		}
		return extents
	default:
		panic("unexpected font type")
	}
}

// FullName returns the full name of the font.
func (info *Info) FullName() string {
	return info.FamilyName + " " + info.Subfamily()
}

// PostscriptName returns the Postscript name of the font.
func (info *Info) PostscriptName() string {
	name := info.FamilyName + "-" + info.Subfamily()
	re := regexp.MustCompile(`[^!-$&-'*-.0-;=?-Z\\^-z|~]+`)
	return re.ReplaceAllString(name, "")
}

// Subfamily returns the subfamily name of the font.
func (info *Info) Subfamily() string {
	var words []string
	if info.Width != 0 && info.Width != font.WidthNormal {
		words = append(words, info.Width.String())
	}
	if info.Weight != 0 && info.Weight != font.WeightNormal {
		words = append(words, info.Weight.String())
	} else if info.IsBold {
		words = append(words, "Bold")
	}
	if info.IsOblique {
		words = append(words, "Oblique")
	} else if info.ItalicAngle != 0 {
		words = append(words, "Italic")
	}
	if len(words) == 0 {
		return "Regular"
	}
	return strings.Join(words, " ")
}
