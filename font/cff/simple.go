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

package cff

import (
	"errors"
	"math"
	"slices"

	"golang.org/x/text/language"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/encoding/simpleenc"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/cffglyphs"
	"seehuhn.de/go/pdf/font/internal/fontdesc"
	"seehuhn.de/go/pdf/font/internal/fontgeom"
	"seehuhn.de/go/pdf/font/internal/vfinstance"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/internal/fontname"
)

type OptionsSimple struct {
	Language     language.Tag
	GsubFeatures map[string]bool
	GposFeatures map[string]bool

	// Variations pins the axes of a variable font before embedding.  Keys are
	// variation axis tags; omitted axes keep their default value.  A CFF2 or
	// otherwise variable font is always instanced, even when this is nil.
	Variations map[string]float64
}

// Simple represents a CFF font which can be embedded in a PDF file
// as a simple font.
type Simple struct {
	cffFont *cff.Font

	// unitsPerEm caches the source sfnt.Font's UnitsPerEm so Layout can scale
	// layouter output (which is in these units) without holding the sfnt.Font.
	unitsPerEm uint16

	// Descriptor describes the font as designed, in PDF glyph space units.
	// It is filled in from the font program and may be adjusted before the
	// font is embedded; entries which depend on the document instead of the
	// font (FontName, IsSymbolic and MissingWidth) are ignored and filled in
	// at embedding time.
	Descriptor *font.Descriptor

	*font.Geometry
	layouter *sfnt.Layouter

	*simpleenc.Simple

	// Name is the PDF resource-dictionary key under which this font is
	// referenced in content streams.  If non-empty, the builder uses this
	// value as the /Font subdictionary key; the spec requires the two to
	// match (PDF 2.0 Table 109).  Required in PDF 1.0; optional in PDF
	// 1.1–1.7; deprecated (forbidden by this library's writer) in PDF 2.0.
	Name pdf.Name
}

var _ font.Layouter = (*Simple)(nil)

// PostScriptName returns the name by which the PDF file refers to this font.
// A font program need not name itself, so the name may be one derived here
// rather than one the font gave.
func (f *Simple) PostScriptName() string {
	return fontname.ForCFF(f.cffFont)
}

// ResourceName returns the preferred resource-dictionary key for this font.
// See [font.Instance.ResourceName].
func (f *Simple) ResourceName() pdf.Name {
	return f.Name
}

// NewSimple turns a sfnt.Font into a PDF CFF font.
//
// The font can be embedded as a simple font or as a composite font,
// depending on the options used.
//
// The sfnt.Font info must be an OpenType font with CFF outlines.
func NewSimple(info *sfnt.Font, opt *OptionsSimple) (*Simple, error) {
	if opt == nil {
		opt = &OptionsSimple{}
	}

	info, err := vfinstance.Apply(info, opt.Variations)
	if err != nil {
		return nil, err
	}

	cffFont := info.AsCFF()
	if cffFont == nil {
		return nil, errors.New("no CFF outlines in font")
	}

	geom, fontBBox := fontgeom.FromSFNT(info)

	layouter, err := info.NewLayouter(opt.Language, opt.GsubFeatures, opt.GposFeatures)
	if err != nil {
		return nil, err
	}

	notdefWidth := math.Round(info.GlyphWidthPDF(0))

	f := &Simple{
		cffFont:    cffFont,
		unitsPerEm: info.UnitsPerEm,

		Descriptor: fontdesc.FromSFNT(info, fontBBox),

		Geometry: geom,
		layouter: layouter,

		Simple: simpleenc.NewSimple(notdefWidth, fontname.ForCFF(cffFont), &pdfenc.WinAnsi),
	}

	return f, nil
}

// FontInfo returns information required to load the font file and to
// extract the the glyph corresponding to a character identifier. The
// result is a pointer to one of the FontInfo* types defined in the
// font/dict package.
func (f *Simple) FontInfo() any {
	dict, err := f.makeFontDict()
	if err != nil {
		return nil
	}
	return dict.FontInfo()
}

// Encode converts a glyph ID to a character code (for use with the
// instance's codec).  The arguments width and text are hints for choosing
// an appropriate advance width and text representation for the character
// code, in case a new code is allocated.
//
// The function returns the character code, and a boolean indicating
// whether the encoding was successful.  If the function returns false, the
// glyph ID cannot be encoded with this font instance.
//
// Use the Codec to append the character code to PDF strings.
//
// Encode converts a glyph ID to a character code.
func (f *Simple) Encode(gid glyph.ID, text string) (charcode.Code, bool) {
	if c, ok := f.Simple.GetCode(gid, text); ok {
		return charcode.Code(c), true
	}

	// Allocate new code
	width := math.Round(f.cffFont.GlyphWidthPDF(gid))
	c, err := f.Simple.Encode(gid, f.cffFont.Glyphs[gid].Name, text, width)
	return charcode.Code(c), err == nil
}

// Layout appends a string to a glyph sequence.  The string is typeset at
// the given point size and the resulting GlyphSeq is returned.
//
// If seq is nil, a new glyph sequence is allocated.  If seq is not
// nil, the return value is guaranteed to be equal to seq.
func (f *Simple) Layout(seq *font.GlyphSeq, ptSize float64, s string) *font.GlyphSeq {
	if seq == nil {
		seq = &font.GlyphSeq{}
	}

	// Layouter advances/offsets are in UnitsPerEm; scale uniformly to points.
	q := ptSize / float64(f.unitsPerEm)

	buf := f.layouter.Layout(s)
	seq.Seq = slices.Grow(seq.Seq, len(buf))
	for _, g := range buf {
		xOffset := float64(g.XOffset) * q
		if len(seq.Seq) == 0 {
			seq.Skip += xOffset
		} else {
			seq.Seq[len(seq.Seq)-1].Advance += xOffset
		}
		seq.Seq = append(seq.Seq, font.Glyph{
			GID:     g.GID,
			Advance: float64(g.Advance) * q,
			Rise:    float64(g.YOffset) * q,
			Text:    string(g.Text),
		})
	}
	return seq
}

// Embed converts the Go representation of the object into a PDF object,
// corresponding to the PDF version of the output file.
//
// The first return value is the PDF representation of the object.
// If the object is embedded in the PDF file, this may be a reference.
//
// The second return value is a Go representation of the embedded object.
// In most cases, this value is not used and T can be set to [Unused].
func (f *Simple) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "simple CFF fonts", pdf.V1_2); err != nil {
		return nil, err
	}

	ref := e.Alloc()
	e.Defer(func(eh *pdf.EmbedHelper) error {
		dict, err := f.makeFontDict()
		if err != nil {
			return err
		}
		_, err = eh.EmbedAt(ref, dict)
		return err
	})

	return ref, nil
}

func (f *Simple) makeFontDict() (*dict.Type1, error) {
	if err := f.Simple.Error(); err != nil {
		return nil, pdf.Errorf("font %q: %w", f.PostScriptName(), err)
	}

	outlines := f.cffFont.Outlines
	srcTag, postScriptName := subset.Split(f.PostScriptName())

	// subset the font, if needed
	glyphs := f.Simple.Glyphs()
	subsetTag := subset.Retag(subset.Tag(glyphs, outlines.NumGlyphs()), srcTag)

	// TagFontInfo copies, so the font information can be modified below without
	// reaching the font this subset was made from
	fontInfo := subset.TagFontInfo(f.cffFont.FontInfo, subsetTag, postScriptName)

	var subsetOutlines *cff.Outlines
	if subsetTag != "" {
		subsetOutlines = outlines.Subset(glyphs)
	} else {
		subsetOutlines = outlines.Clone()
	}

	// convert to a simple font, if needed:
	if len(subsetOutlines.Private) != 1 {
		return nil, errors.New("font must be embedded as a composite font")
	}
	subsetOutlines.ROS = nil
	subsetOutlines.GIDToCID = nil
	if len(subsetOutlines.FontMatrices) > 0 && subsetOutlines.FontMatrices[0] != matrix.Identity {
		fontInfo.FontMatrix = subsetOutlines.FontMatrices[0].Mul(fontInfo.FontMatrix)
	}
	subsetOutlines.FontMatrices = nil
	for gid, origGID := range glyphs { // fill in the glyph names
		subsetOutlines.SetGlyphName(glyph.ID(gid), f.Simple.GlyphName(origGID))
	}
	// The real encoding is set in the PDF font dictionary, so that readers can
	// know the meaning of codes without having to parse the font file. Here we
	// set the built-in encoding of the font to the standard encoding, to
	// minimise font size.
	subsetOutlines.Encoding = cff.StandardEncoding(subsetOutlines.Glyphs)

	subsetCFF := &cff.Font{
		FontInfo: fontInfo,
		Outlines: subsetOutlines,
	}

	// construct the font dictionary and font descriptor
	isSymbolic := false
	for _, gid := range glyphs {
		if gid == 0 {
			continue
		}
		if !pdfenc.StandardLatin.Has[f.Simple.GlyphName(gid)] {
			isSymbolic = true
			break
		}
	}

	// the descriptor describes the design; only these entries depend on how
	// the document uses the font
	fd := *f.Descriptor
	fd.FontName = subset.Join(subsetTag, postScriptName)
	fd.IsSymbolic = isSymbolic
	fd.MissingWidth = f.Simple.DefaultWidth()

	dict := &dict.Type1{
		PostScriptName: postScriptName,
		SubsetTag:      subsetTag,
		Name:           f.Name,
		Descriptor:     &fd,
		Encoding:       f.Simple.Encoding(),
		FontFile:       cffglyphs.ToStream(subsetCFF, glyphdata.CFFSimple),
		ToUnicode:      f.Simple.ToUnicode(),
	}
	for c, info := range f.Simple.MappedCodes() {
		dict.Width[c] = info.Width
	}

	return dict, nil
}
