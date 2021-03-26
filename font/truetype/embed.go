package truetype

import (
	"errors"
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
)

// Embed embeds a TrueType font into a pdf file.
func Embed(w *pdf.Writer, fname string, subset map[rune]bool) (*font.Font, error) {
	tt, err := Open(fname)
	if err != nil {
		return nil, err
	}
	defer tt.Close()

	info, err := tt.GetInfo()
	if err != nil {
		return nil, err
	}
	FontName := pdf.Name(info.FontName)

	// determine the character set
	if subset != nil && !info.IsSuperset(subset) {
		var missing []rune
		for r, ok := range subset {
			if !ok {
				continue
			}
			if info.CMap[r] == 0 {
				missing = append(missing, r)
			}
		}
		msg := fmt.Sprintf("missing glyphs: %q", string(missing))
		return nil, errors.New(msg)
	}
	glyphs := []font.GlyphIndex{0} // always include the placeholder glyph
	for r, idx := range info.CMap {
		if subset == nil || subset[r] {
			glyphs = append(glyphs, idx)
		}
	}
	// TODO(voss): use this for subsetting the font

	// TODO(voss): if len(glyphs) < 256, write a Type 2 font.
	// This will require synthesizing a new cmap table.
	if w.Version < pdf.V1_3 {
		return nil, &pdf.VersionError{
			Earliest:  pdf.V1_2,
			Operation: "use of TrueType-based CIDFonts",
		}
	}

	// step 1: write a copy of the font file into the font stream.
	size := w.NewPlaceholder(10)
	dict := pdf.Dict{
		"Length1": size,
	}
	opt := &pdf.StreamOptions{
		Filters: []*pdf.FilterInfo{
			{Name: "FlateDecode"},
		},
	}
	stm, FontFile, err := w.OpenStream(dict, nil, opt)
	if err != nil {
		return nil, err
	}
	n, err := tt.export(stm, func(name string) bool {
		// the list of tables to include is from PDF 32000-1:2008, table 126
		switch name {
		case "glyf", "head", "hhea", "hmtx", "loca", "maxp", "cvt ", "fpgm", "prep":
			return true
		default:
			fmt.Println("dropping", name)
			return false
		}
	})
	if err != nil {
		return nil, err
	}
	err = size.Set(pdf.Integer(n))
	if err != nil {
		return nil, err
	}
	err = stm.Close()
	if err != nil {
		return nil, err
	}

	// step 2: write the dictionaries to describe the font.
	FontDescriptor := pdf.Dict{
		"Type":        pdf.Name("FontDescriptor"),
		"FontName":    FontName,
		"Flags":       pdf.Integer(getFlags(info)),
		"FontBBox":    info.FontBBox,
		"ItalicAngle": pdf.Number(info.ItalicAngle),
		"Ascent":      pdf.Number(info.Ascent),
		"Descent":     pdf.Number(info.Descent),
		"CapHeight":   pdf.Number(info.CapHeight),

		// TrueType files don't contain this information, so make up a value.
		// The coefficients were found using linear regression over the fonts
		// found in a large collection of PDF files.  The fit is not good, but
		// I guess this is still better than just saying 70.
		"StemV": pdf.Integer(0.0838*float64(info.Weight) + 36.0198 + 0.5),

		"FontFile2": FontFile,
	}
	FontDescriptorRef, err := w.Write(FontDescriptor, nil)
	if err != nil {
		return nil, err
	}

	// TODO(voss): make sure there is only one copy of this per PDF file.
	CIDSystemInfoRef, err := w.Write(pdf.Dict{
		"Registry":   pdf.String("Adobe"),
		"Ordering":   pdf.String("Identity"),
		"Supplement": pdf.Integer(0),
	}, nil)
	if err != nil {
		return nil, err
	}

	DW := mostFrequent(info.Width)
	W := encodeWidths(info.Width, DW)
	WRefs, err := w.WriteCompressed(nil, W)
	if err != nil {
		return nil, err
	}

	CIDFont := pdf.Dict{
		"Type":           pdf.Name("Font"),
		"Subtype":        pdf.Name("CIDFontType2"),
		"BaseFont":       FontName,
		"CIDSystemInfo":  CIDSystemInfoRef,
		"FontDescriptor": FontDescriptorRef,
		"W":              WRefs[0],
	}
	if DW != 0 {
		CIDFont["DW"] = pdf.Integer(DW)
	}

	CIDFontRef, err := w.Write(CIDFont, nil)
	if err != nil {
		return nil, err
	}

	FontRef, err := w.Write(pdf.Dict{
		"Type":            pdf.Name("Font"),
		"Subtype":         pdf.Name("Type0"),
		"BaseFont":        pdf.Name(info.FontName), // TODO(voss): make sure this is consistent
		"Encoding":        pdf.Name("Identity-H"),
		"DescendantFonts": pdf.Array{CIDFontRef},
	}, nil)
	if err != nil {
		return nil, err
	}

	font := &font.Font{
		Ref:  FontRef,
		CMap: info.CMap,
		Enc: func(ii ...font.GlyphIndex) []byte {
			res := make([]byte, 0, 2*len(ii))
			for _, idx := range ii {
				res = append(res, byte(idx>>8), byte(idx))
			}
			return res
		},
		Ligatures:   map[font.GlyphPair]font.GlyphIndex{},
		Kerning:     map[font.GlyphPair]int{},
		GlyphExtent: info.GlyphExtent,
		Width:       info.Width,
		Ascent:      info.Ascent,
		Descent:     info.Descent,
		LineGap:     info.LineGap,
	}

	return font, nil
}

type fontFlags int

const (
	fontFlagFixedPitch  fontFlags = 1 << 0  // All glyphs have the same width (as opposed to proportional or variable-pitch fonts, which have different widths).
	fontFlagSerif       fontFlags = 1 << 1  // Glyphs have serifs, which are short strokes drawn at an angle on the top and bottom of glyph stems. (Sans serif fonts do not have serifs.)
	fontFlagSymbolic    fontFlags = 1 << 2  // Font contains glyphs outside the Adobe standard Latin character set. This flag and the Nonsymbolic flag shall not both be set or both be clear.
	fontFlagScript      fontFlags = 1 << 3  // Glyphs resemble cursive handwriting.
	fontFlagNonsymbolic fontFlags = 1 << 5  // Font uses the Adobe standard Latin character set or a subset of it.
	fontFlagItalic      fontFlags = 1 << 6  // Glyphs have dominant vertical strokes that are slanted.
	fontFlagAllCap      fontFlags = 1 << 16 // Font contains no lowercase letters; typically used for display purposes, such as for titles or headlines.
	fontFlagSmallCap    fontFlags = 1 << 17 // Font contains both uppercase and lowercase letters.  The uppercase letters are similar to those in the regular version of the same typeface family. The glyphs for the lowercase letters have the same shapes as the corresponding uppercase letters, but they are sized and their proportions adjusted so that they have the same size and stroke weight as lowercase glyphs in the same typeface family.
	fontFlagForceBold   fontFlags = 1 << 18 // ...
)

func getFlags(info *font.Info) fontFlags {
	var flags fontFlags
	if info.IsFixedPitch {
		flags |= fontFlagFixedPitch
	}
	if info.IsSerif {
		flags |= fontFlagSerif
	}
	if info.IsScript {
		flags |= fontFlagScript
	}
	if info.IsItalic {
		flags |= fontFlagItalic
	}
	if info.IsAdobeLatin {
		flags |= fontFlagNonsymbolic
	} else {
		flags |= fontFlagSymbolic
	}
	// FontFlagAllCap
	// FontFlagSmallCap
	return flags
}
