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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/sfnt/cmap"
	"seehuhn.de/go/pdf/font/sfnt/glyf"
	"seehuhn.de/go/pdf/font/sfnt/head"
	"seehuhn.de/go/pdf/font/sfnt/maxp"
	"seehuhn.de/go/pdf/font/sfnt/os2"
	"seehuhn.de/go/pdf/font/type1"
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

	CMap     cmap.Subtable
	Outlines interface{} // either *cff.Outlines or *TTFOutlines
}

// TTFOutlines stores the glyph data of a TrueType font.
type TTFOutlines struct {
	Glyphs glyf.Glyphs
	Widths []uint16
	Names  []string
	Tables map[string][]byte
	Maxp   *maxp.TTFInfo
}

// GetFontInfo returns an Adobe FontInfo structure for the given font.
func (info *Info) GetFontInfo() *type1.FontInfo {
	q := 1 / float64(info.UnitsPerEm)
	fontInfo := &type1.FontInfo{
		FullName:   info.FullName(),
		FamilyName: info.FamilyName,
		Weight:     info.Weight.String(),
		FontName:   info.PostscriptName(),
		Version:    info.Version.String(),

		Copyright: strings.ReplaceAll(info.Copyright, "©", "(c)"),
		Notice:    info.Trademark,

		FontMatrix: []float64{q, 0, 0, q, 0, 0}, // TODO(voss): is this right?

		ItalicAngle:  info.ItalicAngle,
		IsFixedPitch: info.IsFixedPitch(),

		UnderlinePosition:  info.UnderlinePosition,
		UnderlineThickness: info.UnderlineThickness,
	}
	return fontInfo
}

// IsTrueType returns true if the font contains TrueType glyph outlines.
func (info *Info) IsTrueType() bool {
	_, ok := info.Outlines.(*TTFOutlines)
	return ok
}

// IsOpenType returns true if the font contains CFF glyph outlines.
func (info *Info) IsOpenType() bool {
	_, ok := info.Outlines.(*cff.Outlines)
	return ok
}

// FullName returns the full name of the font.
func (info *Info) FullName() string {
	return info.FamilyName + " " + info.Subfamily()
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
	} else if info.IsItalic {
		words = append(words, "Italic")
	}
	if len(words) == 0 {
		return "Regular"
	}
	return strings.Join(words, " ")
}

// PostscriptName returns the Postscript name of the font.
func (info *Info) PostscriptName() pdf.Name {
	name := info.FamilyName + "-" + info.Subfamily()
	re := regexp.MustCompile(`[^!-$&-'*-.0-;=?-Z\\^-z|~]+`)
	return pdf.Name(re.ReplaceAllString(name, ""))
}

// BBox returns the bounding box of the font.
func (info *Info) BBox() (bbox font.Rect) {
	first := true
	for i := 0; i < info.NumGlyphs(); i++ {
		ext := info.GlyphExtent(font.GlyphID(i))
		if ext.IsZero() {
			continue
		}

		if first {
			bbox = ext
			continue
		}

		bbox.Extend(ext)
	}
	return
}

// NumGlyphs returns the number of glyphs in the font.
func (info *Info) NumGlyphs() int {
	switch outlines := info.Outlines.(type) {
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
	switch f := info.Outlines.(type) {
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

// Extents returns the glyph bounding boxes for the font.
func (info *Info) Extents() []font.Rect {
	switch f := info.Outlines.(type) {
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

// GlyphWidth returns the advance width of the glyph with the given glyph ID.
func (info *Info) GlyphWidth(gid font.GlyphID) uint16 {
	switch f := info.Outlines.(type) {
	case *cff.Outlines:
		return f.Glyphs[gid].Width
	case *TTFOutlines:
		if f.Widths == nil {
			return 0
		}
		return f.Widths[gid]
	default:
		panic("unexpected font type")
	}
}

// GlyphExtent returns the glyph bounding box for one glyph.
func (info *Info) GlyphExtent(gid font.GlyphID) font.Rect {
	switch f := info.Outlines.(type) {
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

// GlyphName returns the name if a glyph.  If the name is not known,
// the empty string is returned.
func (info *Info) GlyphName(gid font.GlyphID) pdf.Name {
	switch f := info.Outlines.(type) {
	case *cff.Outlines:
		return f.Glyphs[gid].Name
	case *TTFOutlines:
		if f.Names == nil {
			return ""
		}
		return pdf.Name(f.Names[gid])
	default:
		panic("unexpected font type")
	}
}
