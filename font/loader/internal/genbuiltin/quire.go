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

package main

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"seehuhn.de/go/postscript/afm"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/glyf"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/opentype/coverage"
	"seehuhn.de/go/sfnt/opentype/gtab"
	"seehuhn.de/go/sfnt/parser"
)

// tier3 lists the characters kept in the Quire fonts: Latin, Greek and
// Cyrillic, with the punctuation and symbols used alongside them.
var tier3 = []struct{ lo, hi rune }{
	{0x0020, 0x007E}, {0x00A0, 0x00FF}, // basic Latin, Latin-1
	{0x0100, 0x017F},                   // Latin Extended-A
	{0x0192, 0x0192},                   // florin
	{0x0218, 0x021B},                   // Romanian S/T with comma below
	{0x02C6, 0x02DD},                   // spacing accents
	{0x0370, 0x03FF},                   // Greek
	{0x0400, 0x04FF},                   // Cyrillic
	{0x1E80, 0x1E85}, {0x1EF2, 0x1EF3}, // Welsh
	{0x2000, 0x206F}, // general punctuation
	{0x20A0, 0x20CF}, // currency
	{0x2100, 0x218F}, // letterlike symbols, number forms
	{0x2190, 0x22FF}, // arrows, mathematical operators
	{0x2500, 0x25FF}, // box drawing, blocks, shapes
	{0xFB00, 0xFB06}, // Latin ligatures
}

const (
	oflURL          = "https://openfontlicense.org"
	quireDescLatin  = "Subset and renamed derivative of a font licensed under the SIL Open Font License 1.1; see the font's copyright notice."
	quireLicenseOFL = "This Font Software is licensed under the SIL Open Font License, Version 1.1."
)

// quireStyles lists the style suffixes of the Quire Latin faces.
var quireStyles = []string{"Regular", "Bold", "Italic", "BoldItalic"}

// quireFamilies lists the Liberation families, in the order of the output.
var quireFamilies = []string{"Sans", "Serif", "Mono"}

// quireFile is a generated font file.
type quireFile struct {
	name string // base name, without the compression suffix
	data []byte // uncompressed content
}

// makeQuireLatin builds the twelve Quire Latin fonts from the Liberation
// TrueType files in lib.  Each font gets a glyph for every glyph in the Adobe
// metrics of the standard font it replaces.  Both lib and afms are keyed by
// file name.
func makeQuireLatin(lib, afms map[string][]byte) ([]quireFile, error) {
	var res []quireFile
	for _, family := range quireFamilies {
		for _, style := range quireStyles {
			fontName := "Quire" + family + "-" + style
			required, err := afmGlyphNames(afms[standardName(fontName)+".afm"])
			if err != nil {
				return nil, fmt.Errorf("%s: %w", fontName, err)
			}
			src := lib["Liberation"+family+"-"+style+".ttf"]
			data, name, err := makeQuireFace(src, family, style, required)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", fontName, err)
			}
			res = append(res, quireFile{name + ".ttf", data})
		}
	}
	return res, nil
}

// afmGlyphNames returns the names of the glyphs in an AFM file, in sorted
// order.  Data without glyph metrics, including a missing file, is an error.
func afmGlyphNames(data []byte) ([]string, error) {
	metrics, err := afm.Read(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if len(metrics.Glyphs) == 0 {
		return nil, errors.New("no glyph metrics")
	}
	return slices.Sorted(maps.Keys(metrics.Glyphs)), nil
}

// makeQuireFace subsets and renames one Liberation face.  The result has a
// glyph for each of the required glyph names.
func makeQuireFace(src []byte, family, style string, required []string) (data []byte, fontName string, err error) {
	orig, err := sfnt.Read(bytes.NewReader(src), parser.NewBudget(int64(len(src))))
	if err != nil {
		return nil, "", err
	}
	outl, ok := orig.Outlines.(*glyf.Outlines)
	if !ok {
		return nil, "", fmt.Errorf("unexpected outline type %T", orig.Outlines)
	}
	cm, err := orig.CMapTable.GetBest()
	if err != nil {
		return nil, "", err
	}

	gids := []glyph.ID{0}
	seen := map[glyph.ID]bool{0: true}
	add := func(gid glyph.ID) {
		if gid != 0 && !seen[gid] {
			seen[gid] = true
			gids = append(gids, gid)
		}
	}
	for _, rg := range tier3 {
		for r := rg.lo; r <= rg.hi; r++ {
			add(cm.Lookup(r))
		}
	}

	// required glyphs, by name or via Unicode; those which Liberation
	// lacks are added below
	for _, name := range required {
		add(lookupGlyph(outl.Names, cm, name))
	}

	sub, err := orig.Subset(gids)
	if err != nil {
		return nil, "", err
	}
	if family != "Mono" {
		if err := addLiga(sub); err != nil {
			return nil, "", err
		}
	}
	if err := setNotdefGlyf(sub); err != nil {
		return nil, "", err
	}
	if err := renameMacron(sub); err != nil {
		return nil, "", err
	}
	if err := addCommaAccent(sub, family); err != nil {
		return nil, "", err
	}
	subOutl := sub.Outlines.(*glyf.Outlines)
	subCMap, err := sub.CMapTable.GetBest()
	if err != nil {
		return nil, "", err
	}
	for _, name := range required {
		if lookupGlyph(subOutl.Names, subCMap, name) == 0 {
			return nil, "", fmt.Errorf("glyph %q not found", name)
		}
	}

	styleName := style
	if style == "BoldItalic" {
		styleName = "Bold Italic"
	}
	sub.FamilyName = "Quire " + family
	sub.Subfamily = styleName
	sub.FullName = "Quire " + family + " " + styleName
	sub.FontName = "Quire" + family + "-" + style
	if strings.Contains(strings.ToLower(sub.Trademark), "liberation") {
		sub.Trademark = ""
	}
	sub.License = quireLicenseOFL
	sub.LicenseURL = oflURL
	sub.Description = quireDescLatin

	buf := &bytes.Buffer{}
	if _, err := sub.Write(buf); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), sub.FontName, nil
}

// addLiga adds a GSUB "liga" feature which forms the fi and fl ligatures,
// for all scripts and languages of the font.  Liberation has the ligature
// glyphs, but no feature which produces them.  The standard fonts Helvetica
// and Times-Roman, which the Quire Sans and Serif faces replace, define both
// ligatures in their metrics.
func addLiga(f *sfnt.Font) error {
	cm, err := f.CMapTable.GetBest()
	if err != nil {
		return err
	}
	gid := make(map[rune]glyph.ID)
	for _, r := range []rune{'f', 'i', 'l', '\uFB01', '\uFB02'} {
		gid[r] = cm.Lookup(r)
		if gid[r] == 0 {
			return fmt.Errorf("no glyph for %U", r)
		}
	}
	if f.Gsub == nil {
		return errors.New("no GSUB table")
	}
	gsub := f.Gsub
	if gsub.Variations != nil || gsub.VariationsRaw != nil {
		return errors.New("unexpected GSUB feature variations")
	}
	for _, feat := range gsub.FeatureList {
		if feat.Tag == "liga" {
			return errors.New("GSUB already has a liga feature")
		}
	}

	lookup := &gtab.LookupTable{
		Meta: &gtab.LookupMetaInfo{LookupType: 4},
		Subtables: []gtab.Subtable{
			&gtab.Gsub4_1{
				Cov: coverage.Table{gid['f']: 0},
				Repl: [][]gtab.Ligature{{
					{In: []glyph.ID{gid['i']}, Out: gid['\uFB01']},
					{In: []glyph.ID{gid['l']}, Out: gid['\uFB02']},
				}},
			},
		},
	}
	lookupIdx := gtab.LookupIndex(len(gsub.LookupList))
	gsub.LookupList = append(gsub.LookupList, lookup)

	// The feature records are kept in alphabetical order of their tags, so
	// the feature indices of the language systems need to be renumbered.
	features := append(slices.Clone(gsub.FeatureList), &gtab.Feature{
		Tag:     "liga",
		Lookups: []gtab.LookupIndex{lookupIdx},
	})
	perm := make([]int, len(features))
	for i := range perm {
		perm[i] = i
	}
	slices.SortStableFunc(perm, func(a, b int) int {
		return strings.Compare(features[a].Tag, features[b].Tag)
	})
	newIndex := make([]gtab.FeatureIndex, len(features))
	sorted := make(gtab.FeatureListInfo, len(features))
	for to, from := range perm {
		newIndex[from] = gtab.FeatureIndex(to)
		sorted[to] = features[from]
	}
	gsub.FeatureList = sorted
	ligaIdx := newIndex[len(features)-1]

	for _, ls := range gsub.ScriptList {
		if ls.Required != 0xFFFF {
			ls.Required = newIndex[ls.Required]
		}
		for i, idx := range ls.Optional {
			ls.Optional[i] = newIndex[idx]
		}
		ls.Optional = append(ls.Optional, ligaIdx)
		slices.Sort(ls.Optional)
	}
	return nil
}

// renameMacron gives the name macron to the glyph of U+02C9, the spacing
// accent which the glyph of this name is in the Adobe metrics.  Liberation
// uses the name for the glyph of U+00AF, a bar as wide as the advance width;
// this glyph keeps its character code and is renamed uni00AF.
func renameMacron(f *sfnt.Font) error {
	outl := f.Outlines.(*glyf.Outlines)
	cm, err := f.CMapTable.GetBest()
	if err != nil {
		return err
	}
	bar, accent := cm.Lookup(0x00AF), cm.Lookup(0x02C9)
	if bar == 0 || accent == 0 || bar == accent || outl.Names[bar] != "macron" ||
		slices.Contains(outl.Names, "uni00AF") {
		return errors.New("unexpected macron glyphs")
	}
	outl.Names[bar] = "uni00AF"
	outl.Names[accent] = "macron"
	return nil
}

// addCommaAccent gives the font a glyph named commaaccent, the spacing comma
// below of the Adobe metrics, which Liberation Sans has under this name.
// Liberation Serif has the glyph under the name uniF004.  Liberation Mono
// has only the combining comma below; the glyph is built from it, placed as
// in scommaaccent, which has the same advance width as every glyph of the
// fixed-pitch font.
func addCommaAccent(f *sfnt.Font, family string) error {
	outl := f.Outlines.(*glyf.Outlines)
	if slices.Contains(outl.Names, "commaaccent") {
		return nil
	}

	switch family {
	case "Serif":
		i := slices.Index(outl.Names, "uniF004")
		if i < 0 {
			return errors.New("glyph uniF004 not found")
		}
		outl.Names[i] = "commaaccent"
		return nil

	case "Mono":
		i := slices.Index(outl.Names, "scommaaccent")
		if i < 0 {
			return errors.New("glyph scommaaccent not found")
		}
		comp, ok := outl.Glyphs[i].Data.(glyf.CompositeGlyph)
		if !ok || len(comp.Components) != 2 {
			return errors.New("scommaaccent is not a composite of two glyphs")
		}
		u, err := comp.Components[1].Unpack()
		if err != nil {
			return err
		}
		child := outl.Glyphs[u.Child]
		if outl.Names[u.Child] != "uni0326" || child == nil || u.AlignPoints ||
			u.Trfm[0] != 1 || u.Trfm[1] != 0 || u.Trfm[2] != 0 || u.Trfm[3] != 1 {
			return errors.New("unexpected comma below in scommaaccent")
		}
		dx, dy := funit.Int16(u.Trfm[4]), funit.Int16(u.Trfm[5])
		u.UseMyMetrics = false
		g := &glyf.Glyph{
			LLx:  child.LLx + dx,
			LLy:  child.LLy + dy,
			URx:  child.URx + dx,
			URy:  child.URy + dy,
			Data: glyf.CompositeGlyph{Components: []glyf.GlyphComponent{u.Pack()}},
		}
		outl.Glyphs = append(outl.Glyphs, g)
		outl.Widths = append(outl.Widths, outl.Widths[i])
		outl.Names = append(outl.Names, "commaaccent")
		return nil
	}
	return fmt.Errorf("no commaaccent for family %q", family)
}

// lookupGlyph finds the glyph for an Adobe glyph name, first in the post
// names of the font and then via the character code.
func lookupGlyph(glyphNames []string, cm cmap.Subtable, name string) glyph.ID {
	if i := slices.Index(glyphNames, name); i > 0 {
		return glyph.ID(i)
	}
	rr := []rune(names.ToUnicode(name, ""))
	if len(rr) != 1 {
		return 0
	}
	return cm.Lookup(rr[0])
}

// gzipBytes compresses data deterministically.
func gzipBytes(data []byte) ([]byte, error) {
	buf := &bytes.Buffer{}
	zw, err := gzip.NewWriterLevel(buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(data); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
