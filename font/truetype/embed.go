package truetype

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
)

// Embed embeds a TrueType font into a pdf file and returns a reference to
// the font descriptor dictionary.
func (tt *Font) Embed(w *pdf.Writer) (*pdf.Reference, error) {
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
	stm, fontStream, err := w.OpenStream(dict, nil, opt)
	if err != nil {
		return nil, err
	}
	n, err := tt.export(stm, func(name string) bool {
		return name != "vhea" && name != "vmtx" && name != "PCLT"
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

	// step 3: write the font descriptor
	fdesc := pdf.Dict{
		"Type":        pdf.Name("FontDescriptor"),
		"FontName":    pdf.Name(tt.Info.FontName),
		"Flags":       pdf.Integer(getFlags(tt.Info)),
		"FontBBox":    tt.Info.BBox,
		"ItalicAngle": pdf.Number(tt.Info.ItalicAngle),
		"Ascent":      pdf.Number(tt.Info.Ascent),
		"Descent":     pdf.Number(tt.Info.Descent),
		"CapHeight":   pdf.Number(tt.Info.CapHeight),

		// TrueType files don't contain this information, so make up a value.
		// The coefficients were found using linear regression over the fonts
		// found in a large collection of PDF files.  The fit is not good, but
		// I guess this is still better than just saying 70.
		"StemV": pdf.Integer(0.0838*float64(tt.Info.Weight) + 36.0198 + 0.5),

		"FontFile2": fontStream,
	}

	fdescRef, err := w.Write(fdesc, nil)
	if err != nil {
		return nil, err
	}

	return fdescRef, nil
}

// EmbedAsType0 embeds a TrueType font into a pdf file and returns a reference
// to the CIDFont dictionary.
func (tt *Font) EmbedAsType0(w *pdf.Writer) (*pdf.Reference, error) {
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
		return true // name != "cmap"
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

	// step 2: get information about the font
	FontName := pdf.Name(tt.Info.FontName)

	FontDescriptor := pdf.Dict{
		"Type":        pdf.Name("FontDescriptor"),
		"FontName":    FontName,
		"Flags":       pdf.Integer(getFlags(tt.Info)),
		"FontBBox":    tt.Info.BBox,
		"ItalicAngle": pdf.Number(tt.Info.ItalicAngle),
		"Ascent":      pdf.Number(tt.Info.Ascent),
		"Descent":     pdf.Number(tt.Info.Descent),
		"CapHeight":   pdf.Number(tt.Info.CapHeight),

		// TrueType files don't contain this information, so make up a value.
		// The coefficients were found using linear regression over the fonts
		// found in a large collection of PDF files.  The fit is not good, but
		// I guess this is still better than just saying 70.
		"StemV": pdf.Integer(0.0838*float64(tt.Info.Weight) + 36.0198 + 0.5),

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

	DW := mostFrequent(tt.Info.Width)
	W := encodeWidths(tt.Info.Width, DW)
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

	return CIDFontRef, nil
}

type fontFlags int

const (
	fontFlagFixedPitch  fontFlags = 1 << (1 - 1)  // All glyphs have the same width (as opposed to proportional or variable-pitch fonts, which have different widths).
	fontFlagSerif       fontFlags = 1 << (2 - 1)  // Glyphs have serifs, which are short strokes drawn at an angle on the top and bottom of glyph stems. (Sans serif fonts do not have serifs.)
	fontFlagSymbolic    fontFlags = 1 << (3 - 1)  // Font contains glyphs outside the Adobe standard Latin character set. This flag and the Nonsymbolic flag shall not both be set or both be clear.
	fontFlagScript      fontFlags = 1 << (4 - 1)  // Glyphs resemble cursive handwriting.
	fontFlagNonsymbolic fontFlags = 1 << (6 - 1)  // Font uses the Adobe standard Latin character set or a subset of it.
	fontFlagItalic      fontFlags = 1 << (7 - 1)  // Glyphs have dominant vertical strokes that are slanted.
	fontFlagAllCap      fontFlags = 1 << (17 - 1) // Font contains no lowercase letters; typically used for display purposes, such as for titles or headlines.
	fontFlagSmallCap    fontFlags = 1 << (18 - 1) // Font contains both uppercase and lowercase letters.  The uppercase letters are similar to those in the regular version of the same typeface family. The glyphs for the lowercase letters have the same shapes as the corresponding uppercase letters, but they are sized and their proportions adjusted so that they have the same size and stroke weight as lowercase glyphs in the same typeface family.
	fontFlagForceBold   fontFlags = 1 << (19 - 1) // ...
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
