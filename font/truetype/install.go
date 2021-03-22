package truetype

import (
	"io"

	"seehuhn.de/go/pdf"
)

// Install embeds a TrueType font into a pdf file and returns a reference to
// the font descriptor dictionary.
func Install(w *pdf.Writer, fname string) (*pdf.Reference, error) {
	// step 1: get  information about the font
	tt, err := Open(fname)
	if err != nil {
		return nil, err
	}
	defer tt.Close()

	stat, err := tt.fd.Stat()
	if err != nil {
		return nil, err
	}
	size := stat.Size()

	FontName := tt.FontName
	// TODO(voss): if FontName == "", invent a name: The name must be no longer
	// than 63 characters and restricted to the printable ASCII subset, codes
	// 33 to 126, except for the 10 characters '[', ']', '(', ')', '{', '}',
	// '<', '>', '/', '%'.
	os2Info, err := tt.GetOS2Info()
	if err != nil {
		return nil, err
	}
	postInfo, err := tt.GetPostInfo()
	if err != nil {
		return nil, err
	}

	// factor for converting from TrueType FUnit to PDF glyph units
	q := 1000 / float64(tt.Head.UnitsPerEm)

	// step 2: write a copy of the font file into the font stream.
	dict := pdf.Dict{
		"Length1": pdf.Integer(size),
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
	_, err = tt.fd.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(stm, tt.fd)
	if err != nil {
		return nil, err
	}
	err = stm.Close()
	if err != nil {
		return nil, err
	}

	// step 3: write the font descriptor
	var flags fontFlags
	if postInfo.IsFixedPitch {
		flags |= fontFlagFixedPitch
	}
	switch os2Info.V0.FamilyClass >> 8 {
	case 1, 2, 3, 4, 5, 7:
		flags |= fontFlagSerif
	case 10:
		flags |= fontFlagScript
	}
	if tt.Head.MacStyle&(1<<1) != 0 {
		flags |= fontFlagItalic
	}
	AdobeStandardLatinOnly := false // TODO(voss)
	if AdobeStandardLatinOnly {
		flags |= fontFlagNonsymbolic
	} else {
		flags |= fontFlagSymbolic
	}
	// FontFlagAllCap
	// FontFlagSmallCap

	fdesc := pdf.Dict{
		"Type":     pdf.Name("FontDescriptor"),
		"FontName": pdf.Name(FontName),
		"Flags":    pdf.Integer(flags),
		"FontBBox": &pdf.Rectangle{
			LLx: float64(tt.Head.XMin) * q,
			LLy: float64(tt.Head.YMin) * q,
			URx: float64(tt.Head.XMax) * q,
			URy: float64(tt.Head.YMax) * q,
		},
		"ItalicAngle": pdf.Number(postInfo.ItalicAngle),
		"FontFile2":   fontStream,
	}

	if os2Info.V0MSValid {
		var ascent, descent float64
		if os2Info.V0.Selection&(1<<7) != 0 {
			ascent = float64(os2Info.V0MS.TypoAscender)
			descent = float64(os2Info.V0MS.TypoDescender)
		} else {
			ascent = float64(os2Info.V0MS.WinAscent)
			descent = -float64(os2Info.V0MS.WinDescent)
		}
		fdesc["Ascent"] = pdf.Number(ascent * q)
		fdesc["Descent"] = pdf.Number(descent * q)
	} else {
		// TODO(voss): use the "hhea" table instead
	}

	if os2Info.V0.Version >= 4 {
		fdesc["XHeight"] = pdf.Number(float64(os2Info.V4.XHeight) * q)
		fdesc["CapHeight"] = pdf.Number(float64(os2Info.V4.CapHeight) * q)
	} else {
		// TODO(voss): XHeight may be set equal to the top of the unscaled and
		// unhinted glyph bounding box of the glyph encoded at U+0078 (LATIN
		// SMALL LETTER X).

		// TODO(voss): CapHeight may be set equal to the top of the unscaled
		// and unhinted glyph bounding box of the glyph encoded at U+0048
		// (LATIN CAPITAL LETTER H)
	}

	// https://stackoverflow.com/a/35543715/648741
	fdesc["StemV"] = pdf.Integer(10 + 220*(float64(os2Info.V0.WeightClass)-50)/900)

	fontDesc, err := w.Write(fdesc, nil)
	if err != nil {
		return nil, err
	}

	return fontDesc, nil
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
