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
)

// Info contains information about the font.
type Info struct {
	FamilyName string
	Width      os2.Width
	Weight     os2.Weight

	Version          head.Version
	CreationTime     time.Time
	ModificationTime time.Time

	Copyright string
	Trademark string
	PermUse   os2.Permissions

	UnitsPerEm uint16

	Ascent  int16
	Descent int16
	LineGap int16

	ItalicAngle        float64 // Italic angle (degrees counterclockwise from vertical)
	UnderlinePosition  int16   // Underline position (negative)
	UnderlineThickness int16   // Underline thickness

	IsBold    bool
	IsRegular bool
	IsOblique bool

	// TODO(voss): should this have a separate field for advance widths?
	CMap cmap.Subtable
	Font interface{} // either *cff.Outlines or *TTInfo
}

// TTInfo contains information specific to TrueType fonts.
type TTInfo struct {
	Widths []uint16
	Glyphs glyf.Glyphs
	Tables map[string][]byte
}

func (info *Info) NumGlyphs() int {
	switch outlines := info.Font.(type) {
	case *cff.Outlines:
		return len(outlines.Glyphs)
	case *TTInfo:
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
	case *TTInfo:
		return f.Widths
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
	case *TTInfo:
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
	if info.Width != 0 && info.Width != os2.WidthNormal {
		words = append(words, info.Width.String())
	}
	if info.Weight != 0 && info.Weight != os2.WeightNormal {
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
