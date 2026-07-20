// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package printprep

import (
	"errors"
	"math"
	"slices"

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/sfntglyphs"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/graphics/extract"
)

var (
	errFontNotIndirect = errors.New("font embed did not return a reference")
	errNoDescriptor    = errors.New("font has no descriptor")
)

// convFont is the converted form of one font resource.
type convFont struct {
	// ref is the output font dictionary reference.  For a pass-through font it
	// is set immediately; for a converted glyf font it is filled by finalize.
	ref pdf.Reference

	// codeToGID is non-nil when the source font was converted to a composite
	// Identity-H font.  It maps each source one-byte code to the source glyph
	// identifier, which the converted font also uses as its two-byte character
	// code (its CID).  Content showing text in this font is re-encoded
	// accordingly, and each glyph used is recorded in used together with the
	// width of the code that showed it.
	codeToGID map[byte]uint16
	used      map[uint16]float64

	// glyf holds the state needed to finalize a converted glyf font once its
	// used glyphs are known; nil for pass-through fonts.
	glyf *glyfConv
}

// glyfConv carries the deferred state for a glyf-to-Identity-H conversion.
type glyfConv struct {
	src    *sfnt.Font
	psName string
	desc   *font.Descriptor
	width  [256]float64 // per-code widths from the source font
}

// reencode maps a shown string to the two-byte codes of the converted font and
// records, for each glyph used, the width of the code that showed it (so that
// unused codes mapping to the same glyph cannot contribute a wrong width).
// Unmapped bytes become the notdef glyph (0).
func (cf *convFont) reencode(s pdf.String) pdf.String {
	out := make([]byte, 0, len(s)*2)
	for _, b := range []byte(s) {
		gid := cf.codeToGID[b]
		cf.used[gid] = cf.glyf.width[b]
		out = append(out, byte(gid>>8), byte(gid))
	}
	return pdf.String(out)
}

// fontContext manages font conversion for one content context (a page or a
// form XObject).  Fonts are converted lazily on first use, and the converted
// resource dictionary is built once the content walk is complete.
type fontContext struct {
	c    *converter
	src  pdf.Dict // source /Font resource subdictionary
	conv map[pdf.Name]*convFont
}

func (c *converter) newFontContext(srcRes pdf.Dict) *fontContext {
	var src pdf.Dict
	if srcRes != nil {
		src, _ = pdf.CursorAt(c.x, nil).Dict(srcRes["Font"])
	}
	return &fontContext{c: c, src: src, conv: make(map[pdf.Name]*convFont)}
}

// use returns the converted font for the resource name, converting it on first
// use.  It returns nil when the name is unknown or the font cannot be read.
func (fc *fontContext) use(name pdf.Name) *convFont {
	if cf, ok := fc.conv[name]; ok {
		return cf
	}
	var cf *convFont
	if fc.src != nil {
		cf = fc.c.convertFont(fc.src[name])
	}
	fc.conv[name] = cf
	return cf
}

// subdict finalizes the used fonts and returns the converted /Font resource
// subdictionary, or nil if no fonts were used.
func (fc *fontContext) subdict() (pdf.Object, error) {
	out := pdf.Dict{}
	for name, cf := range fc.conv {
		if cf == nil {
			continue
		}
		if cf.glyf != nil {
			ref, err := fc.c.finalizeGlyf(cf)
			if err != nil {
				// on failure, drop the font rather than abort the document
				continue
			}
			cf.ref = ref
		}
		if cf.ref != 0 {
			out[name] = cf.ref
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// convertFont converts a single source font.
//
// Embedded simple TrueType (glyf) fonts are converted to composite
// CIDFontType2 fonts with Identity-H encoding, which removes the simple-font
// code-to-glyph resolution that some renderers implement incorrectly.  All
// other fonts are copied unchanged.
func (c *converter) convertFont(fontObj pdf.Object) *convFont {
	if fontObj == nil {
		return nil
	}
	d, err := extract.Dict(pdf.CursorAt(c.x, nil), fontObj, false)
	if err != nil || d == nil {
		return c.passThroughFont(fontObj)
	}
	if tt, ok := d.(*dict.TrueType); ok && tt.FontFile != nil {
		cf, err := c.convertGlyfSimple(tt)
		if err != nil {
			return c.passThroughFont(fontObj)
		}
		return cf
	}
	return c.passThroughFont(fontObj)
}

// convertGlyfSimple begins converting an embedded simple TrueType font to a
// composite CIDFontType2 font with Identity-H encoding.  The returned font
// carries a re-encoding map and deferred state; the font program is subset and
// embedded by finalizeGlyf once the used glyphs are known.
func (c *converter) convertGlyfSimple(d *dict.TrueType) (*convFont, error) {
	if d.Descriptor == nil {
		return nil, errNoDescriptor
	}
	sf, err := sfntglyphs.FromStream(d.FontFile)
	if err != nil {
		return nil, err
	}

	symbolic := d.Descriptor.IsSymbolic
	sel := sfntglyphs.NewTrueTypeSelector(sf, symbolic, d.Encoding)

	codeToGID := make(map[byte]uint16)
	for code := range 256 {
		gid, ok := sel(cid.CID(code) + 1)
		if !ok {
			continue
		}
		codeToGID[byte(code)] = uint16(gid)
	}

	return &convFont{
		codeToGID: codeToGID,
		used:      make(map[uint16]float64),
		glyf: &glyfConv{
			src:    sf,
			psName: d.PostScriptName,
			desc:   d.Descriptor,
			width:  d.Width,
		},
	}, nil
}

// finalizeGlyf subsets the glyf font to its used glyphs and embeds it as a
// CIDFontType2 font, returning its reference.
func (c *converter) finalizeGlyf(cf *convFont) (pdf.Reference, error) {
	g := cf.glyf

	// glyph list: notdef plus every used glyph, in increasing order so that the
	// subset places glyph i at position i
	gidSet := map[glyph.ID]struct{}{0: {}}
	for id := range cf.used {
		gidSet[glyph.ID(id)] = struct{}{}
	}
	glyphs := make([]glyph.ID, 0, len(gidSet))
	for id := range gidSet {
		glyphs = append(glyphs, id)
	}
	slices.Sort(glyphs)

	embedSrc := g.src.Clone()
	embedSrc.CMapTable = nil
	embedSrc.Gsub = nil
	embedSrc.Gpos = nil
	embedSrc.Gdef = nil
	subsetFont, err := embedSrc.Subset(glyphs)
	if err != nil {
		return 0, err
	}

	// CIDToGIDMap: CID is the source glyph id, mapping to the subset position
	maxCID := glyphs[len(glyphs)-1]
	cidToGID := make([]glyph.ID, maxCID+1)
	identity := true
	for pos, srcGID := range glyphs {
		cidToGID[srcGID] = glyph.ID(pos)
		if int(srcGID) != pos {
			identity = false
		}
	}

	ww := make(map[cmap.CID]float64, len(cf.used))
	for id, w := range cf.used {
		ww[cmap.CID(id)] = w
	}

	cmapFile, err := cmap.Predefined("Identity-H")
	if err != nil {
		return 0, err
	}

	subsetTag := subset.Tag(glyphs, g.src.NumGlyphs())

	// composite fonts require MissingWidth to be zero; the descriptor's font
	// name must match the (new) subset tag
	fd := *g.desc
	fd.MissingWidth = 0
	fd.FontName = subset.Join(subsetTag, g.psName)

	fontDict := &dict.CIDFontType2{
		PostScriptName:  g.psName,
		SubsetTag:       subsetTag,
		Descriptor:      &fd,
		ROS:             cmap.NewGIDToCIDIdentity().ROS(),
		CMap:            cmapFile,
		Width:           ww,
		DefaultWidth:    math.Round(subsetFont.GlyphWidthPDF(0)),
		DefaultVMetrics: dict.DefaultVMetricsDefault,
		FontFile:        sfntglyphs.ToStream(subsetFont, glyphdata.TrueType),
	}
	if !identity {
		fontDict.CIDToGID = cidToGID
	}

	obj, err := c.rm.Embed(fontDict)
	if err != nil {
		return 0, err
	}
	ref, ok := obj.(pdf.Reference)
	if !ok {
		return 0, errFontNotIndirect
	}
	return ref, nil
}

// passThroughFont copies a source font unchanged, returning a reference to the
// copy.
func (c *converter) passThroughFont(fontObj pdf.Object) *convFont {
	nv, ok := fontObj.(pdf.Native)
	if !ok {
		return nil
	}
	copied, err := c.copy.Copy(nv)
	if err != nil {
		return nil
	}
	if ref, ok := copied.(pdf.Reference); ok {
		return &convFont{ref: ref}
	}
	ref := c.out.Alloc()
	if err := c.out.Put(ref, copied); err != nil {
		return nil
	}
	return &convFont{ref: ref}
}
