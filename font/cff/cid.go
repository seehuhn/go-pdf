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

package cff

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"

	"golang.org/x/text/language"

	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	sfntcmap "seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/opentype/gtab"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/font/tounicode"
)

// FontComposite is a CFF font for embedding into a PDF file as a composite font.
type FontComposite struct {
	info        *sfnt.Font
	cmap        sfntcmap.Subtable
	gsubLookups []gtab.LookupIndex
	gposLookups []gtab.LookupIndex
	*font.Geometry

	cs  charcode.CodeSpaceRange
	ros *type1.CIDSystemInfo
}

// FontOptions allows to customize details of the font embedding.
type FontOptions struct {
	Language language.Tag
	CS       charcode.CodeSpaceRange
	ROS      *type1.CIDSystemInfo
}

var defaultFontOptions = FontOptions{
	Language: language.Und,
	CS:       charcode.UCS2,
}

// NewComposite allocates a new CFF font for embedding into a PDF file as a composite font.
func NewComposite(info *sfnt.Font, opt *FontOptions) (*FontComposite, error) {
	if !info.IsCFF() {
		return nil, errors.New("wrong font type")
	}

	if opt == nil {
		opt = &defaultFontOptions
	}

	geometry := &font.Geometry{
		UnitsPerEm:   info.UnitsPerEm,
		GlyphExtents: info.GlyphBBoxes(),
		Widths:       info.Widths(),

		Ascent:             info.Ascent,
		Descent:            info.Descent,
		BaseLineSkip:       info.Ascent - info.Descent + info.LineGap,
		UnderlinePosition:  info.UnderlinePosition,
		UnderlineThickness: info.UnderlineThickness,
	}

	cmap, err := info.CMapTable.GetBest()
	if err != nil {
		return nil, err
	}

	loc := opt.Language
	CS := opt.CS
	if CS == nil {
		CS = charcode.UCS2
	}
	ROS := opt.ROS
	if ROS == nil {
		ROS = info.Outlines.(*cff.Outlines).ROS
	}
	if ROS == nil {
		ROS = &type1.CIDSystemInfo{ // TODO(voss): is this ok?
			Registry:   "Adobe",
			Ordering:   "Identity",
			Supplement: 0,
		}
	}

	res := &FontComposite{
		info:        info,
		cmap:        cmap,
		gsubLookups: info.Gsub.FindLookups(loc, gtab.GsubDefaultFeatures),
		gposLookups: info.Gpos.FindLookups(loc, gtab.GposDefaultFeatures),
		Geometry:    geometry,
		cs:          CS,
		ros:         ROS,
	}
	return res, nil
}

// Embed implements the [font.Font] interface.
func (f *FontComposite) Embed(w pdf.Putter, resName pdf.Name) (font.Embedded, error) {
	err := pdf.CheckVersion(w, "use of OpenType fonts", pdf.V1_6)
	if err != nil {
		return nil, err
	}
	res := &embeddedComposite{
		FontComposite: f,
		w:             w,
		Resource:      pdf.Resource{Ref: w.Alloc(), Name: resName},
		CIDEncoder:    cmap.NewCustomCIDEncoder(f.cs, f.ros),
	}
	w.AutoClose(res)
	return res, nil
}

// Layout implements the [font.Font] interface.
func (f *FontComposite) Layout(s string, ptSize float64) glyph.Seq {
	return f.info.Layout(f.cmap, f.gsubLookups, f.gposLookups, s)
}

type embeddedComposite struct {
	*FontComposite
	w pdf.Putter
	pdf.Resource

	cmap.CIDEncoder
	closed bool
}

func (f *embeddedComposite) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true

	// subset the font
	encoding := f.CIDEncoder.Encoding()
	var ss []subset.Glyph
	ss = append(ss, subset.Glyph{OrigGID: 0, CID: 0})
	for _, p := range encoding {
		ss = append(ss, subset.Glyph{OrigGID: p.GID, CID: p.CID})
	}
	subsetInfo, err := subset.CID(f.info, ss, f.ros)
	if err != nil {
		return fmt.Errorf("font subset: %w", err)
	}
	subsetTag := subset.Tag(ss, f.info.NumGlyphs())

	cmap := make(map[charcode.CharCode]type1.CID)
	for _, e := range encoding {
		cmap[e.Code] = e.CID
	}

	m := make(map[charcode.CharCode][]rune)
	for _, e := range encoding {
		m[e.Code] = e.Text
	}
	toUnicode := tounicode.FromMapping(f.cs, m)

	info := EmbedInfoComposite{
		Font:      subsetInfo.AsCFF(),
		SubsetTag: subsetTag,
		CS:        f.cs,
		ROS:       f.ros,
		CMap:      cmap,
		ToUnicode: toUnicode,

		UnitsPerEm: f.info.UnitsPerEm,
		Ascent:     f.info.Ascent,
		Descent:    f.info.Descent,
		CapHeight:  f.info.CapHeight,
		IsSerif:    f.info.IsSerif,
		IsScript:   f.info.IsScript,
	}
	return info.Embed(f.w, f.Ref)
}

// EmbedInfoComposite contains the information needed to embed a CFF font as a composite font into a PDF file.
type EmbedInfoComposite struct {
	// Font is the font to embed (already subsetted, if needed).
	Font *cff.Font

	// SubsetTag should be a unique tag for the font subset,
	// or the empty string if this is the full font.
	SubsetTag string

	CS   charcode.CodeSpaceRange
	ROS  *type1.CIDSystemInfo
	CMap map[charcode.CharCode]type1.CID

	UnitsPerEm uint16 // TODO(voss): get this from the font matrix instead?

	Ascent    funit.Int16
	Descent   funit.Int16
	CapHeight funit.Int16
	IsSerif   bool
	IsScript  bool

	IsAllCap   bool
	IsSmallCap bool

	// ToUnicode (optional) is a map from character codes to unicode strings.
	ToUnicode *tounicode.Info
}

// Embed writes the PDF objects needed to embed the font into the PDF file.
// This is the reverse of [ExtractComposite]
func (info *EmbedInfoComposite) Embed(w pdf.Putter, fontDictRef pdf.Reference) error {
	err := pdf.CheckVersion(w, "composite CFF fonts", pdf.V1_3)
	if err != nil {
		return err
	}

	cff := info.Font

	// CidFontName shall be the value of the CIDFontName entry in the CIDFont program.
	// The name may have a subset prefix if appropriate.
	cidFontName := cff.FontInfo.FontName
	if info.SubsetTag != "" {
		cidFontName = info.SubsetTag + "+" + cidFontName
	}

	// make a CMap
	cmapInfo := cmap.New(info.ROS, info.CS, info.CMap)
	var encoding pdf.Object
	if cmap.IsPredefined(cmapInfo) {
		encoding = pdf.Name(cmapInfo.Name)
	} else {
		encoding = w.Alloc()
	}

	unitsPerEm := info.UnitsPerEm

	var ww []font.CIDWidth
	widths := cff.Widths()
	if cff.Gid2Cid != nil { // CID-keyed CFF font
		for gid, w := range widths {
			ww = append(ww, font.CIDWidth{CID: cff.Gid2Cid[gid], GlyphWidth: w})
		}
	} else { // simple CFF font
		for gid, w := range widths {
			ww = append(ww, font.CIDWidth{CID: type1.CID(gid), GlyphWidth: w})
		}
	}
	DW, W := font.EncodeCIDWidths(ww, info.UnitsPerEm)

	q := 1000 / float64(unitsPerEm)
	bbox := cff.BBox()
	fontBBox := &pdf.Rectangle{
		LLx: bbox.LLx.AsFloat(q),
		LLy: bbox.LLy.AsFloat(q),
		URx: bbox.URx.AsFloat(q),
		URy: bbox.URy.AsFloat(q),
	}

	isSymbolic := true

	cidFontRef := w.Alloc()
	var toUnicodeRef pdf.Reference
	fontDescriptorRef := w.Alloc()
	fontFileRef := w.Alloc()

	fontDict := pdf.Dict{
		"Type":            pdf.Name("Font"),
		"Subtype":         pdf.Name("Type0"),
		"BaseFont":        pdf.Name(cidFontName + "-" + cmapInfo.Name),
		"Encoding":        encoding,
		"DescendantFonts": pdf.Array{cidFontRef},
	}
	if info.ToUnicode != nil {
		toUnicodeRef = w.Alloc()
		fontDict["ToUnicode"] = toUnicodeRef
	}

	ROS := pdf.Dict{
		"Registry":   pdf.String(info.ROS.Registry),
		"Ordering":   pdf.String(info.ROS.Ordering),
		"Supplement": pdf.Integer(info.ROS.Supplement),
	}

	cidFontDict := pdf.Dict{
		"Type":           pdf.Name("Font"),
		"Subtype":        pdf.Name("CIDFontType0"),
		"BaseFont":       pdf.Name(cidFontName),
		"CIDSystemInfo":  ROS,
		"FontDescriptor": fontDescriptorRef,
	}
	if DW != 1000 {
		cidFontDict["DW"] = DW
	}
	if W != nil {
		// TODO(voss): use an indirect object?
		cidFontDict["W"] = W
	}

	fd := &font.Descriptor{
		FontName:     cidFontName,
		IsFixedPitch: cff.IsFixedPitch,
		IsSerif:      info.IsSerif,
		IsSymbolic:   isSymbolic,
		IsScript:     info.IsScript,
		IsItalic:     cff.ItalicAngle != 0,
		IsAllCap:     info.IsAllCap,
		IsSmallCap:   info.IsSmallCap,
		ForceBold:    cff.Private[0].ForceBold,
		FontBBox:     fontBBox,
		ItalicAngle:  cff.ItalicAngle,
		Ascent:       info.Ascent.AsFloat(q),
		Descent:      info.Descent.AsFloat(q),
		CapHeight:    info.CapHeight.AsFloat(q),
	}
	fontDescriptor := fd.AsDict()
	fontDescriptor["FontFile3"] = fontFileRef

	compressedRefs := []pdf.Reference{fontDictRef, cidFontRef, fontDescriptorRef}
	compressedObjects := []pdf.Object{fontDict, cidFontDict, fontDescriptor}
	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return pdf.Wrap(err, "composite CFF font dicts")
	}

	// See section 9.9 of PDF 32000-1:2008 for details.
	fontFileDict := pdf.Dict{
		"Subtype": pdf.Name("CIDFontType0C"),
	}
	fontFileStream, err := w.OpenStream(fontFileRef, fontFileDict, pdf.FilterCompress{})
	if err != nil {
		return err
	}
	err = cff.Write(fontFileStream)
	if err != nil {
		return fmt.Errorf("embedding CFF CIDFont %q: %w", cidFontName, err)
	}
	err = fontFileStream.Close()
	if err != nil {
		return err
	}

	if ref, ok := encoding.(pdf.Reference); ok {
		err = cmapInfo.Embed(w, ref, nil)
		if err != nil {
			return err
		}
	}

	if toUnicodeRef != 0 {
		err = info.ToUnicode.Embed(w, toUnicodeRef)
		if err != nil {
			return err
		}
	}

	return nil
}

// ExtractComposite extracts information about a composite CFF font from a PDF file.
// This is the reverse of [EmbedInfoComposite.Embed].
func ExtractComposite(r pdf.Getter, dicts *font.Dicts) (*EmbedInfoComposite, error) {
	if err := dicts.Type.MustBe(font.CFFComposite); err != nil {
		return nil, err
	}
	res := &EmbedInfoComposite{}

	stm, err := pdf.DecodeStream(r, dicts.FontProgram, 0)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(stm)
	if err != nil {
		return nil, err
	}
	cff, err := cff.Read(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	res.Font = cff

	postScriptName, _ := pdf.GetName(r, dicts.CIDFontDict["BaseFont"])
	if m := subset.TagRegexp.FindStringSubmatch(string(postScriptName)); m != nil {
		res.SubsetTag = m[1]
	}

	cmap, err := cmap.Extract(r, dicts.FontDict["Encoding"])
	if err != nil {
		return nil, err
	}
	var ROS *type1.CIDSystemInfo
	if rosDict, _ := pdf.GetDict(r, dicts.CIDFontDict["CIDSystemInfo"]); rosDict != nil {
		registry, _ := pdf.GetString(r, rosDict["Registry"])
		ordering, _ := pdf.GetString(r, rosDict["Ordering"])
		supplement, _ := pdf.GetInteger(r, rosDict["Supplement"])
		ROS = &type1.CIDSystemInfo{
			Registry:   string(registry),
			Ordering:   string(ordering),
			Supplement: int32(supplement),
		}
	}
	if ROS == nil {
		ROS = cmap.ROS
	}
	res.CS = cmap.CS
	res.ROS = ROS
	res.CMap = cmap.GetMapping()

	if info, _ := tounicode.Extract(r, dicts.FontDict["ToUnicode"], cmap.CS); info != nil {
		// TODO(voss): check that the codespace ranges are compatible with the cmap.
		res.ToUnicode = info
	}

	// TODO(voss): be more robust here
	unitsPerEm := uint16(math.Round(1 / float64(cff.FontMatrix[0])))
	q := 1000 / float64(unitsPerEm)

	res.UnitsPerEm = unitsPerEm
	res.Ascent = funit.Int16(math.Round(dicts.FontDescriptor.Ascent / q))
	res.Descent = funit.Int16(math.Round(dicts.FontDescriptor.Descent / q))
	res.CapHeight = funit.Int16(math.Round(dicts.FontDescriptor.CapHeight / q))
	res.IsSerif = dicts.FontDescriptor.IsSerif
	res.IsScript = dicts.FontDescriptor.IsScript
	res.IsAllCap = dicts.FontDescriptor.IsAllCap
	res.IsSmallCap = dicts.FontDescriptor.IsSmallCap

	return res, nil
}
