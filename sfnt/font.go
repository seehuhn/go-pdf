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

package sfnt

import (
	"regexp"
	"strings"
	"time"

	"seehuhn.de/go/pdf/sfnt/cff"
	"seehuhn.de/go/pdf/sfnt/cmap"
	"seehuhn.de/go/pdf/sfnt/funit"
	"seehuhn.de/go/pdf/sfnt/glyf"
	"seehuhn.de/go/pdf/sfnt/glyph"
	"seehuhn.de/go/pdf/sfnt/head"
	"seehuhn.de/go/pdf/sfnt/opentype/gdef"
	"seehuhn.de/go/pdf/sfnt/opentype/gtab"
	"seehuhn.de/go/pdf/sfnt/os2"
	"seehuhn.de/go/pdf/sfnt/type1"
)

// Info contains information about the font.
type Info struct {
	FamilyName string
	Width      os2.Width
	Weight     os2.Weight
	IsItalic   bool // Glyphs have dominant vertical strokes that are slanted.
	IsBold     bool
	IsRegular  bool
	IsOblique  bool
	IsSerif    bool
	IsScript   bool // Glyphs resemble cursive handwriting.

	Description string
	SampleText  string

	Version          head.Version
	CreationTime     time.Time
	ModificationTime time.Time

	Copyright string
	Trademark string
	PermUse   os2.Permissions

	UnitsPerEm uint16

	Ascent    funit.Int16
	Descent   funit.Int16 // negative
	LineGap   funit.Int16 // LineGap = BaseLineSkip - Ascent + Descent
	CapHeight funit.Int16
	XHeight   funit.Int16

	ItalicAngle        float64     // Italic angle (degrees counterclockwise from vertical)
	UnderlinePosition  funit.Int16 // Underline position (negative)
	UnderlineThickness funit.Int16 // Underline thickness

	CMap     cmap.Subtable
	Outlines interface{} // either *cff.Outlines or *glyf.Outlines

	Gdef *gdef.Table
	Gsub *gtab.Info
	Gpos *gtab.Info
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

		FontMatrix: []float64{q, 0, 0, q, 0, 0},

		ItalicAngle:  info.ItalicAngle,
		IsFixedPitch: info.IsFixedPitch(),

		UnderlinePosition:  info.UnderlinePosition,
		UnderlineThickness: info.UnderlineThickness,
	}
	return fontInfo
}

// IsGlyf returns true if the font contains TrueType glyph outlines.
func (info *Info) IsGlyf() bool {
	_, ok := info.Outlines.(*glyf.Outlines)
	return ok
}

// IsCFF returns true if the font contains CFF glyph outlines.
func (info *Info) IsCFF() bool {
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
	if info.Width != 0 && info.Width != os2.WidthNormal {
		words = append(words, info.Width.String())
	}
	if info.Weight != 0 && info.Weight != os2.WeightNormal {
		words = append(words, info.Weight.SimpleString())
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
func (info *Info) PostscriptName() string {
	name := info.FamilyName + "-" + info.Subfamily()
	re := regexp.MustCompile(`[^!-$&-'*-.0-;=?-Z\\^-z|~]+`)
	return re.ReplaceAllString(name, "")
}

// BBox returns the bounding box of the font.
func (info *Info) BBox() (bbox funit.Rect) {
	first := true
	for i := 0; i < info.NumGlyphs(); i++ {
		ext := info.GlyphExtent(glyph.ID(i))
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
	case *glyf.Outlines:
		return len(outlines.Glyphs)
	default:
		panic("unexpected font type")
	}
}

// GlyphWidth returns the advance width of the glyph with the given glyph ID,
// in font design units.
func (info *Info) GlyphWidth(gid glyph.ID) funit.Int16 {
	switch f := info.Outlines.(type) {
	case *cff.Outlines:
		return f.Glyphs[gid].Width
	case *glyf.Outlines:
		if f.Widths == nil {
			return 0
		}
		return f.Widths[gid]
	default:
		panic("unexpected font type")
	}
}

// Widths returns the advance widths of the glyphs in the font.
func (info *Info) Widths() []funit.Int16 {
	switch f := info.Outlines.(type) {
	case *cff.Outlines:
		widths := make([]funit.Int16, info.NumGlyphs())
		for gid, g := range f.Glyphs {
			widths[gid] = g.Width
		}
		return widths
	case *glyf.Outlines:
		return f.Widths
	default:
		panic("unexpected font type")
	}
}

// Extents returns the glyph bounding boxes for the font.
func (info *Info) Extents() []funit.Rect {
	extents := make([]funit.Rect, info.NumGlyphs())
	switch f := info.Outlines.(type) {
	case *cff.Outlines:
		for i, g := range f.Glyphs {
			extents[i] = g.Extent()
		}
		return extents
	case *glyf.Outlines:
		for i, g := range f.Glyphs {
			if g == nil {
				continue
			}
			extents[i] = g.Rect
		}
	default:
		panic("unexpected font type")
	}
	return extents
}

// GlyphExtent returns the glyph bounding box for one glyph in font design
// units.
func (info *Info) GlyphExtent(gid glyph.ID) funit.Rect {
	switch f := info.Outlines.(type) {
	case *cff.Outlines:
		return f.Glyphs[gid].Extent()
	case *glyf.Outlines:
		g := f.Glyphs[gid]
		if g == nil {
			return funit.Rect{}
		}
		return g.Rect
	default:
		panic("unexpected font type")
	}
}

func (info *Info) glyphHeight(gid glyph.ID) funit.Int16 {
	switch f := info.Outlines.(type) {
	case *cff.Outlines:
		return f.Glyphs[gid].Extent().URy
	case *glyf.Outlines:
		g := f.Glyphs[gid]
		if g == nil {
			return 0
		}
		return g.Rect.URy
	default:
		panic("unexpected font type")
	}
}

// GlyphName returns the name if a glyph.  If the name is not known,
// the empty string is returned.
func (info *Info) GlyphName(gid glyph.ID) string {
	switch f := info.Outlines.(type) {
	case *cff.Outlines:
		return f.Glyphs[gid].Name
	case *glyf.Outlines:
		if f.Names == nil {
			return ""
		}
		return f.Names[gid]
	default:
		panic("unexpected font type")
	}
}

// IsFixedPitch returns true if all glyphs in the font have the same width.
func (info *Info) IsFixedPitch() bool {
	ww := info.Widths()
	if len(ww) == 0 {
		return false
	}

	var width funit.Int16
	for _, w := range ww {
		if w == 0 {
			continue
		}
		if width == 0 {
			width = w
		} else if width != w {
			return false
		}
	}

	return true
}
