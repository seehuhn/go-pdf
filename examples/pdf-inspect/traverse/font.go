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
	"reflect"
	"sort"
	"strings"
	"unicode"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/glyph"
)

// fontDictCtx represents a PDF font dictionary.
type fontDictCtx struct {
	r    pdf.Getter
	dict font.Dict
}

// newFontDictCtx creates a new font dictionary context.
func newFontDictCtx(r pdf.Getter, pdfDict pdf.Dict) (*fontDictCtx, error) {
	dict, err := dict.Read(r, pdfDict)
	if err != nil {
		return nil, err
	}
	return &fontDictCtx{r: r, dict: dict}, nil
}

// Next returns a context for the given key.
func (c *fontDictCtx) Next(key string) (Context, error) {
	if key == "@raw" {
		// Check if any font type has a non-zero FontRef
		var fontRef pdf.Reference

		switch f := c.dict.(type) {
		case *dict.Type1:
			fontRef = f.FontRef
		case *dict.TrueType:
			fontRef = f.FontRef
		case *dict.CIDFontType0:
			fontRef = f.FontRef
		case *dict.CIDFontType2:
			fontRef = f.FontRef
		default:
			return nil, &KeyError{Key: key, Ctx: "font dict"}
		}

		if fontRef == 0 {
			return nil, &KeyError{Key: key, Ctx: "font dict"}
		}

		// Get the stream for the font program
		stm, err := pdf.GetStream(c.r, fontRef)
		if err != nil {
			return nil, err
		} else if stm == nil {
			return nil, errors.New("missing font program stream")
		}
		decoded, err := pdf.DecodeStream(c.r, stm, 0)
		if err != nil {
			return nil, err
		}

		return &rawStreamCtx{r: decoded}, nil
	}

	if key == "@load" {
		if ttDict, ok := c.dict.(*dict.TrueType); ok {
			if ttDict.FontRef != 0 && ttDict.FontType == glyphdata.TrueType {
				sctx, err := newSfntCtx(c.r, ttDict.FontRef)
				if err != nil {
					return nil, fmt.Errorf("creating sfnt context for @load: %w", err)
				}
				return sctx, nil
			}
		}
	}

	return nil, &KeyError{Key: key, Ctx: "font dict"}
}

// Keys returns the available keys for navigation.
func (c *fontDictCtx) Keys() []string {
	var keys []string

	var hasEmbeddedFont bool
	switch f := c.dict.(type) {
	case *dict.Type1:
		hasEmbeddedFont = f.FontRef != 0
	case *dict.TrueType:
		hasEmbeddedFont = f.FontRef != 0
	case *dict.CIDFontType0:
		hasEmbeddedFont = f.FontRef != 0
	case *dict.CIDFontType2:
		hasEmbeddedFont = f.FontRef != 0
	}

	if hasEmbeddedFont {
		keys = append(keys, "`@raw`")
	}

	if ttDict, ok := c.dict.(*dict.TrueType); ok {
		if ttDict.FontRef != 0 && ttDict.FontType == glyphdata.TrueType {
			keys = append(keys, "`@load`")
		}
	}

	return keys
}

// Show displays information about the font dictionary.
func (c *fontDictCtx) Show() error {
	switch dict := c.dict.(type) {
	case *dict.Type1:
		fmt.Println("Type1 font:")
		fmt.Printf("PostScript Name: %s\n", dict.PostScriptName)
		if dict.SubsetTag != "" {
			fmt.Printf("Subset Tag: %s\n", dict.SubsetTag)
		}
		showFontDescriptor(dict.Descriptor)
		fmt.Println("Font Program:", dict.FontType)

	case *dict.TrueType:
		fmt.Println("TrueType font:")
		fmt.Printf("PostScript Name: %s\n", dict.PostScriptName)
		if dict.SubsetTag != "" {
			fmt.Printf("Subset Tag: %s\n", dict.SubsetTag)
		}
		showFontDescriptor(dict.Descriptor)
		fmt.Println("Font Program:", dict.FontType)

	case *dict.Type3:
		fmt.Println("Type3 font:")
		showFontDescriptor(dict.Descriptor)
		fmt.Printf("Font Matrix: %v\n", dict.FontMatrix)
		if dict.CharProcs != nil {
			fmt.Printf("Character Procedures: present (%d glyphs)\n", len(dict.CharProcs))
		}

	case *dict.CIDFontType0:
		fmt.Println("CIDFontType0:")
		fmt.Printf("PostScript Name: %s\n", dict.PostScriptName)
		if dict.SubsetTag != "" {
			fmt.Printf("Subset Tag: %s\n", dict.SubsetTag)
		}
		showFontDescriptor(dict.Descriptor)
		if dict.ROS != nil {
			fmt.Printf("Registry-Ordering-Supplement: %s-%s-%d\n", dict.ROS.Registry, dict.ROS.Ordering, dict.ROS.Supplement)
		}

	case *dict.CIDFontType2:
		fmt.Println("CIDFontType2:")
		fmt.Printf("PostScript Name: %s\n", dict.PostScriptName)
		if dict.SubsetTag != "" {
			fmt.Printf("Subset Tag: %s\n", dict.SubsetTag)
		}
		showFontDescriptor(dict.Descriptor)
		if dict.ROS != nil {
			fmt.Printf("Registry-Ordering-Supplement: %s-%s-%d\n", dict.ROS.Registry, dict.ROS.Ordering, dict.ROS.Supplement)
		}

	default:
		fmt.Printf("%T font\n", dict)
	}

	return nil
}

// sfntCtx represents a parsed font for traversal.
type sfntCtx struct {
	font *sfnt.Font
}

// newSfntCtx creates a new font context by reading and parsing the font program.
func newSfntCtx(getter pdf.Getter, fontRef pdf.Reference) (*sfntCtx, error) {
	if fontRef == 0 {
		return nil, errors.New("invalid font reference for @load")
	}

	stm, err := pdf.GetStream(getter, fontRef)
	if err != nil {
		return nil, fmt.Errorf("getting font program stream for @load: %w", err)
	}
	if stm == nil {
		return nil, errors.New("missing font program stream for @load")
	}

	decoded, err := pdf.DecodeStream(getter, stm, 0)
	if err != nil {
		return nil, fmt.Errorf("decoding font program stream for @load: %w", err)
	}
	defer decoded.Close()

	sfont, err := sfnt.Read(decoded)
	if err != nil {
		return nil, fmt.Errorf("parsing sfnt font for @load: %w", err)
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

// Next returns a new context for the given key.
func (c *sfntCtx) Next(key string) (Context, error) {
	if key == "glyphs" {
		return newSfntGlyphListCtx(c.font)
	}
	return nil, &KeyError{Key: key, Ctx: "sfnt font"}
}

// Keys returns the available keys.
func (c *sfntCtx) Keys() []string {
	return []string{"`glyphs`"}
}

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
	for i := 0; i < numGlyphs; i++ {
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

		displayName := name
		if len(displayName) > 20 {
			displayName = displayName[:17] + "..."
		}
		if displayName == "" && i == 0 {
			displayName = ".notdef"
		}

		fmt.Fprintf(os.Stdout, "%s%4d | %-19s | %-24s | %s\n",
			headerIndent, i, charStr, bboxStr, displayName)
	}

	return nil
}

// Next returns the context for the given key.
func (c *sfntGlyphListCtx) Next(key string) (Context, error) {
	return nil, fmt.Errorf("no key %q in sfnt glyph list context", key)
}

// Keys returns the list of navigable keys.
func (c *sfntGlyphListCtx) Keys() []string {
	return nil
}

// showFontDescriptor displays font descriptor information.
func showFontDescriptor(desc *font.Descriptor) {
	if desc == nil {
		return
	}

	fmt.Println("Font Descriptor:")

	if desc.FontName != "" {
		fmt.Printf("  • Font Name: %s\n", desc.FontName)
	}
	if desc.FontFamily != "" {
		fmt.Printf("  • Font Family: %s\n", desc.FontFamily)
	}
	if desc.FontStretch != 0 {
		fmt.Printf("  • Font Stretch: %v\n", desc.FontStretch)
	}
	if desc.FontWeight != 0 {
		fmt.Printf("  • Font Weight: %v\n", desc.FontWeight)
	}
	if desc.IsFixedPitch {
		fmt.Println("  • Fixed Pitch: true")
	}
	if desc.IsSerif {
		fmt.Println("  • Serif: true")
	}
	if desc.IsSymbolic {
		fmt.Println("  • Symbolic: true")
	}
	if desc.IsScript {
		fmt.Println("  • Script: true")
	}
	if desc.IsItalic {
		fmt.Println("  • Italic: true")
	}
	if desc.IsAllCap {
		fmt.Println("  • All Cap: true")
	}
	if desc.IsSmallCap {
		fmt.Println("  • Small Cap: true")
	}
	if desc.ForceBold {
		fmt.Println("  • Force Bold: true")
	}
	if !reflect.DeepEqual(desc.FontBBox, font.Descriptor{}.FontBBox) {
		fmt.Printf("  • Font BBox: %v\n", desc.FontBBox)
	}
	if desc.ItalicAngle != 0 {
		fmt.Printf("  • Italic Angle: %.1f°\n", desc.ItalicAngle)
	}
	if desc.Ascent != 0 {
		fmt.Printf("  • Ascent: %.1f\n", desc.Ascent)
	}
	if desc.Descent != 0 {
		fmt.Printf("  • Descent: %.1f\n", desc.Descent)
	}
	if desc.Leading != 0 {
		fmt.Printf("  • Leading: %.1f\n", desc.Leading)
	}
	if desc.CapHeight != 0 {
		fmt.Printf("  • Cap Height: %.1f\n", desc.CapHeight)
	}
	if desc.XHeight != 0 {
		fmt.Printf("  • X Height: %.1f\n", desc.XHeight)
	}
	if desc.StemV != 0 {
		fmt.Printf("  • Stem V: %.1f\n", desc.StemV)
	}
	if desc.StemH != 0 {
		fmt.Printf("  • Stem H: %.1f\n", desc.StemH)
	}
	if desc.MaxWidth != 0 {
		fmt.Printf("  • Max Width: %.1f\n", desc.MaxWidth)
	}
	if desc.AvgWidth != 0 {
		fmt.Printf("  • Avg Width: %.1f\n", desc.AvgWidth)
	}
	if desc.MissingWidth != 0 {
		fmt.Printf("  • Missing Width: %.1f\n", desc.MissingWidth)
	}
}
