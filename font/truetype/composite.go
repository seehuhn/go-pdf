// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package truetype

import (
	"errors"
	"maps"
	"math"
	"slices"

	"golang.org/x/text/language"

	"seehuhn.de/go/geom/path"
	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/encoding/cidenc"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/sfntglyphs"
	"seehuhn.de/go/pdf/font/internal/cidsetup"
	"seehuhn.de/go/pdf/font/internal/fontdesc"
	"seehuhn.de/go/pdf/font/internal/fontgeom"
	"seehuhn.de/go/pdf/font/internal/outline"
	"seehuhn.de/go/pdf/font/internal/vfinstance"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/internal/fontname"
)

type OptionsComposite struct {
	Language     language.Tag
	GsubFeatures map[string]bool
	GposFeatures map[string]bool

	// CMap encodes the text in the content stream.  It fixes the writing
	// mode and, through its character collection, the CIDs the glyphs are
	// written as.  A nil CMap selects the predefined Identity-H.  A CMap
	// without mappings, such as [cmap.UTF8H], has codes allocated from its
	// code space as the text requires.
	CMap *cmap.File

	// GIDToCID, if set, decides the CID each glyph is written as; its
	// character collection must be the CMap's, unless the CMap is one of
	// the Identity CMaps or has no mappings.  When nil, the mapping is
	// derived from the CMap, which is only possible for a character
	// collection the library knows the text of.  A mapping which allocates
	// CIDs as glyphs are used must not be shared between fonts.
	GIDToCID cmap.GIDToCID

	// Variations pins the axes of a variable font before embedding.  Keys are
	// variation axis tags; omitted axes keep their default value.  A variable
	// font is always instanced, even when this is nil.
	Variations map[string]float64
}

// Composite represents a TrueType font together with the font options.
// This implements the [font.Layouter] interface.
type Composite struct {
	info *sfnt.Font

	// Descriptor describes the font as designed, in PDF glyph space units.
	// It is filled in from the font program and may be adjusted before the
	// font is embedded; entries which depend on the document instead of the
	// font (FontName, IsSymbolic and MissingWidth) are ignored and filled in
	// at embedding time.
	Descriptor *font.Descriptor

	*font.Geometry
	layouter *sfnt.Layouter

	gidToCID cmap.GIDToCID
	cidenc.CIDEncoder
}

var _ font.Layouter = (*Composite)(nil)

// PostScriptName returns the name by which the PDF file refers to this font.
// A font program need not name itself, so the name may be one derived here
// rather than one the font gave.
func (f *Composite) PostScriptName() string {
	return fontname.ForSFNT(f.info)
}

// ResourceName returns the empty string: composite TrueType fonts produce a
// CIDFontType2 dictionary, which has no /Name entry in the PDF spec.
// See [font.Instance.ResourceName].
func (f *Composite) ResourceName() pdf.Name {
	return ""
}

// NewComposite makes a PDF TrueType font from a sfnt.Font.
// The font info must be an OpenType/TrueType font with glyf outlines.
// The font can be embedded as a simple font or as a composite font.
func NewComposite(info *sfnt.Font, opt *OptionsComposite) (*Composite, error) {
	if opt == nil {
		opt = &OptionsComposite{}
	}

	info, err := vfinstance.Apply(info, opt.Variations)
	if err != nil {
		return nil, err
	}

	if !info.IsGlyf() {
		return nil, errors.New("no glyf outlines in font")
	}

	geometry, fontBBox := fontgeom.FromSFNT(info)
	descriptor := fontdesc.FromSFNT(info, fontBBox)

	layouter, err := info.NewLayouter(opt.Language, opt.GsubFeatures, opt.GposFeatures)
	if err != nil {
		return nil, err
	}

	notdefWidth := math.Round(info.GlyphWidthPDF(0))
	gidToCID, enc, err := cidsetup.FromCMap(opt.CMap, opt.GIDToCID, info, notdefWidth)
	if err != nil {
		return nil, err
	}

	f := &Composite{
		info:     info,
		Geometry: geometry,

		Descriptor: descriptor,
		layouter:   layouter,
		gidToCID:   gidToCID,
		CIDEncoder: enc,
	}

	return f, nil
}

// FontInfo returns information required to load the font file and to
// extract the the glyph corresponding to a character identifier.
// The returned structure is of type [*dict.FontInfoGlyfEmbedded].
func (f *Composite) FontInfo() any {
	dict, _ := f.makeDict()
	if dict == nil {
		return nil
	}
	return dict.FontInfo()
}

// Embed adds the font to a PDF file.
func (f *Composite) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "composite CFF fonts", pdf.V1_3); err != nil {
		return nil, err
	}

	ref := e.Alloc()
	e.Defer(func(rm *pdf.EmbedHelper) error {
		dict, err := f.makeDict()
		if err != nil {
			return err
		}
		_, err = rm.EmbedAt(ref, dict)
		return err
	})

	return ref, nil
}

// Encode converts a glyph ID to a character code.
func (f *Composite) Encode(gid glyph.ID, text string) (charcode.Code, bool) {
	cid := f.gidToCID.CID(gid, text)
	if c, ok := f.CIDEncoder.GetCode(cid, text); ok {
		return c, true
	}

	width := math.Round(f.info.GlyphWidthPDF(gid))
	c, err := f.CIDEncoder.Encode(cid, text, width)
	return c, err == nil
}

// Layout appends a string to a glyph sequence.
func (f *Composite) Layout(seq *font.GlyphSeq, ptSize float64, s string) *font.GlyphSeq {
	if seq == nil {
		seq = &font.GlyphSeq{}
	}

	// Layouter advances/offsets are in UnitsPerEm; scale uniformly to points.
	q := ptSize / float64(f.info.UnitsPerEm)

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

// makeDict creates the PDF font dictionary for this font.
func (f *Composite) makeDict() (*dict.CIDFontType2, error) {
	origFont := f.info
	srcTag, postScriptName := subset.Split(fontname.ForSFNT(origFont))

	origFont = sfntglyphs.StripForEmbedding(origFont)

	// Subset the font, if needed.
	// To minimise file size, we arrange the glyphs in order of increasing CID.
	cidSet := make(map[cid.CID]struct{})
	cidSet[0] = struct{}{} // Always include CID 0 (notdef)
	for _, info := range f.CIDEncoder.MappedCodes() {
		cidSet[info.CID] = struct{}{}
	}
	cidList := slices.Sorted(maps.Keys(cidSet))

	glyphs := make([]glyph.ID, len(cidList))
	for i, cidVal := range cidList {
		glyphs[i] = f.gidToCID.GID(cidVal)
	}
	subsetTag := subset.Retag(subset.Tag(glyphs, origFont.NumGlyphs()), srcTag)

	var subsetFont *sfnt.Font
	if subsetTag != "" {
		sf, err := origFont.Subset(glyphs)
		if err != nil {
			return nil, err
		}
		subsetFont = sf
	} else {
		subsetFont = origFont
	}

	ros := f.gidToCID.ROS()

	// construct the font dictionary and font descriptor
	dw := math.Round(subsetFont.GlyphWidthPDF(0))

	// widths of the CIDs in use
	ww := make(map[cid.CID]float64)
	for _, info := range f.CIDEncoder.MappedCodes() {
		ww[info.CID] = info.Width
	}

	// determine whether the font is symbolic.  Prefer the font's own glyph
	// name; fall back to a name derived from the Unicode text only when the
	// original font lacks a name for this glyph.
	isSymbolic := false
	for _, info := range f.CIDEncoder.MappedCodes() {
		if info.CID == 0 {
			continue
		}
		origGID := f.gidToCID.GID(info.CID)
		name := f.info.GlyphName(origGID)
		if name == "" {
			name = names.FromUnicode(info.Text)
		}
		if !pdfenc.StandardLatin.Has[name] {
			isSymbolic = true
			break
		}
	}

	// The `CIDToGIDMap` entry in the CIDFont dictionary specifies the mapping
	// from CIDs to glyphs.
	cidToGID, isIdentity, err := subset.MakeCIDToGID(cidList)
	if err != nil {
		return nil, err
	}

	// the descriptor describes the design; only these entries depend on how
	// the document uses the font
	fd := *f.Descriptor
	fd.FontName = subset.Join(subsetTag, postScriptName)
	fd.IsSymbolic = isSymbolic

	// the embedded program names itself the same as BaseFont and the
	// descriptor's FontName, which for a subset carry the tag
	subsetFont.FontName = subset.Join(subsetTag, postScriptName)

	fontDict := &dict.CIDFontType2{
		PostScriptName:  postScriptName,
		SubsetTag:       subsetTag,
		Descriptor:      &fd,
		ROS:             ros,
		CMap:            f.CIDEncoder.CMap(ros),
		Width:           ww,
		DefaultWidth:    dw,
		DefaultVMetrics: dict.DefaultVMetricsDefault,
		ToUnicode:       f.CIDEncoder.ToUnicode(),
		FontFile:        sfntglyphs.ToStream(subsetFont, glyphdata.TrueType),
	}
	if !isIdentity {
		fontDict.CIDToGID = cidToGID
	}

	return fontDict, nil
}

var _ font.Outliner = (*Composite)(nil)

// GlyphID returns the glyph a CID selects.
// See [font.Outliner.GlyphID].
func (f *Composite) GlyphID(c cid.CID) (glyph.ID, bool) {
	gid := f.gidToCID.GID(c)
	if int(gid) >= f.info.NumGlyphs() {
		return 0, false
	}
	return gid, gid != 0 || c == 0
}

// Outline returns the outline of a glyph in text space.
// See [font.Outliner.Outline].
func (f *Composite) Outline(gid glyph.ID) path.Path {
	return outline.SFNT(f.info, gid)
}
