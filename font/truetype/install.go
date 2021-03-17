package truetype

import (
	"fmt"
	"io"
	"os"

	"seehuhn.de/go/pdf"
)

// Install embeds a TrueType font into a pdf file and returns a reference to
// the font descriptor dictionary.
func Install(w *pdf.Writer, fname string) (*pdf.Reference, error) {
	fd, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	stat, err := fd.Stat()
	if err != nil {
		return nil, err
	}
	size := stat.Size()

	// step 1: get some basic information about the font
	header, err := ReadHeader(fd)
	if err != nil {
		return nil, err
	}
	FontName, err := header.GetFontName(fd)
	// TODO(voss): if err == errNoName, invent a name somehow
	if err != nil {
		return nil, err
	}

	fmt.Printf("%08X\n", header.ScalerType)
	for tag, info := range header.Tables {
		fmt.Println(tag, info)
	}

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
	_, err = fd.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(stm, fd)
	if err != nil {
		return nil, err
	}
	err = stm.Close()
	if err != nil {
		return nil, err
	}

	// step 3: write the font descriptor
	var flags fontFlags
	fdesc := pdf.Dict{
		"Type":      pdf.Name("FontDescriptor"),
		"FontName":  pdf.Name(FontName),
		"Flags":     pdf.Integer(flags),
		"FontFile2": fontStream,
	}

	fontDesc, err := w.Write(fdesc, nil)
	if err != nil {
		return nil, err
	}

	return fontDesc, nil
}

type fontFlags int

const (
	FontFlagFixedPitch  fontFlags = 1 << (1 - 1)  // All glyphs have the same width (as opposed to proportional or variable-pitch fonts, which have different widths).
	FontFlagSerif       fontFlags = 1 << (2 - 1)  // Glyphs have serifs, which are short strokes drawn at an angle on the top and bottom of glyph stems. (Sans serif fonts do not have serifs.)
	FontFlagSymbolic    fontFlags = 1 << (3 - 1)  // Font contains glyphs outside the Adobe standard Latin character set. This flag and the Nonsymbolic flag shall not both be set or both be clear.
	FontFlagScript      fontFlags = 1 << (4 - 1)  // Glyphs resemble cursive handwriting.
	FontFlagNonsymbolic fontFlags = 1 << (6 - 1)  // Font uses the Adobe standard Latin character set or a subset of it.
	FontFlagItalic      fontFlags = 1 << (7 - 1)  // Glyphs have dominant vertical strokes that are slanted.
	FontFlagAllCap      fontFlags = 1 << (17 - 1) // Font contains no lowercase letters; typically used for display purposes, such as for titles or headlines.
	FontFlagSmallCap    fontFlags = 1 << (18 - 1) // Font contains both uppercase and lowercase letters.  The uppercase letters are similar to those in the regular version of the same typeface family. The glyphs for the lowercase letters have the same shapes as the corresponding uppercase letters, but they are sized and their proportions adjusted so that they have the same size and stroke weight as lowercase glyphs in the same typeface family.
	FontFlagForceBold   fontFlags = 1 << (19 - 1) // See description after Note 1 in this sub-clause.
)
