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
	"io"
	"reflect"
	"regexp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/graphics/extract"
)

// fontDictCtx represents a PDF font dictionary.
type fontDictCtx struct {
	r    pdf.Getter
	dict dict.Dict
}

// newFontDictCtx creates a new font dictionary context.
func newFontDictCtx(r pdf.Getter, pdfDict pdf.Dict) (*fontDictCtx, error) {
	x := pdf.NewExtractor(r)
	d, err := extract.Dict(x, pdfDict)
	if err != nil {
		return nil, err
	}
	return &fontDictCtx{r: r, dict: d}, nil
}

// Next returns available steps for this context.
func (c *fontDictCtx) Next() []Step {
	var steps []Step

	// Check if any font type has an embedded font file for @raw
	var hasEmbeddedFont bool
	switch f := c.dict.(type) {
	case *dict.Type1:
		hasEmbeddedFont = f.FontFile != nil
	case *dict.TrueType:
		hasEmbeddedFont = f.FontFile != nil
	case *dict.CIDFontType0:
		hasEmbeddedFont = f.FontFile != nil
	case *dict.CIDFontType2:
		hasEmbeddedFont = f.FontFile != nil
	}

	if hasEmbeddedFont {
		// Add @raw step to extract raw font data
		steps = append(steps, Step{
			Match: regexp.MustCompile(`^@raw$`),
			Desc:  "`@raw`",
			Next: func(key string) (Context, error) {
				// Extract FontFile based on font type
				var fontFile *glyphdata.Stream
				switch f := c.dict.(type) {
				case *dict.Type1:
					fontFile = f.FontFile
				case *dict.TrueType:
					fontFile = f.FontFile
				case *dict.CIDFontType0:
					fontFile = f.FontFile
				case *dict.CIDFontType2:
					fontFile = f.FontFile
				}

				if fontFile == nil {
					return nil, fmt.Errorf("no embedded font file available")
				}

				// Use a pipe to stream the font data
				pr, pw := io.Pipe()
				go func() {
					err := fontFile.WriteTo(pw, nil)
					pw.CloseWithError(err)
				}()

				return &rawStreamCtx{r: pr}, nil
			},
		})

		// Add load step for appropriate font types
		switch fontDict := c.dict.(type) {
		case *dict.Type1:
			steps = append(steps, Step{
				Match: regexp.MustCompile(`^load$`),
				Desc:  "`load`",
				Next: func(key string) (Context, error) {
					if fontDict.FontFile == nil {
						return nil, fmt.Errorf("no embedded font file available")
					}
					t1ctx, err := newType1Ctx(fontDict.FontFile)
					if err != nil {
						return nil, fmt.Errorf("creating type1 context for `load`: %w", err)
					}
					return t1ctx, nil
				},
			})
		case *dict.TrueType:
			if fontDict.FontFile != nil && (fontDict.FontFile.Type == glyphdata.TrueType || fontDict.FontFile.Type == glyphdata.OpenTypeGlyf) {
				steps = append(steps, Step{
					Match: regexp.MustCompile(`^load$`),
					Desc:  "`load`",
					Next: func(key string) (Context, error) {
						sfntCtx, err := newSfntCtx(fontDict.FontFile)
						if err != nil {
							return nil, fmt.Errorf("creating sfnt context for `load`: %w", err)
						}
						return sfntCtx, nil
					},
				})
			}
		case *dict.CIDFontType2:
			if fontDict.FontFile != nil && (fontDict.FontFile.Type == glyphdata.TrueType || fontDict.FontFile.Type == glyphdata.OpenTypeGlyf) {
				steps = append(steps, Step{
					Match: regexp.MustCompile(`^load$`),
					Desc:  "`load`",
					Next: func(key string) (Context, error) {
						sfntCtx, err := newSfntCtx(fontDict.FontFile)
						if err != nil {
							return nil, fmt.Errorf("creating sfnt context for `load`: %w", err)
						}
						return sfntCtx, nil
					},
				})
			}
		}
	}

	// Add cmap step for CID font types
	switch fontDict := c.dict.(type) {
	case *dict.CIDFontType0:
		if fontDict.CMap != nil {
			steps = append(steps, Step{
				Match: regexp.MustCompile(`^cmap$`),
				Desc:  "`cmap`",
				Next: func(key string) (Context, error) {
					return newCMapCtx(fontDict.CMap), nil
				},
			})
		}
	case *dict.CIDFontType2:
		if fontDict.CMap != nil {
			steps = append(steps, Step{
				Match: regexp.MustCompile(`^cmap$`),
				Desc:  "`cmap`",
				Next: func(key string) (Context, error) {
					return newCMapCtx(fontDict.CMap), nil
				},
			})
		}
	}

	return steps
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
		if dict.FontFile != nil {
			fmt.Println("Font Program:", dict.FontFile.Type)
		} else {
			fmt.Println("Font Program: external")
		}

	case *dict.TrueType:
		fmt.Println("TrueType font:")
		fmt.Printf("PostScript Name: %s\n", dict.PostScriptName)
		if dict.SubsetTag != "" {
			fmt.Printf("Subset Tag: %s\n", dict.SubsetTag)
		}
		showFontDescriptor(dict.Descriptor)
		if dict.FontFile != nil {
			fmt.Println("Font Program:", dict.FontFile.Type)
		} else {
			fmt.Println("Font Program: external")
		}

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
