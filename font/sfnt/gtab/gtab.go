// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package gtab

import (
	"errors"
	"io"
	"sort"

	"seehuhn.de/go/pdf/font/parser"
	"seehuhn.de/go/pdf/font/sfnt/table"
	"seehuhn.de/go/pdf/locale"
)

// GTab represents a GSUB or GPOS table.
type GTab struct {
	r   io.ReaderAt
	toc *table.Header
	*parser.Parser

	script, language string

	gdef *GdefInfo

	lookupListOffset int64
	lookups          []uint16
	subtableReader   stReaderFn

	classDefCache map[int64]ClassDef
	coverageCache map[int64]Coverage
}

type stReaderFn func(s *parser.State, format uint16, subtablePos int64) (LookupSubtable, error)

// OldRead wraps a parser with a helper to read GSUB and GPOS tables. The loc
// argument determines the writing script and language.
// If loc is nil, the default script and language of the font are used.
func OldRead(toc *table.Header, r io.ReaderAt, loc *locale.Locale) (*GTab, error) {
	g := &GTab{
		r:        r,
		toc:      toc,
		script:   otfDefaultScript,
		language: otfDefaultLanguage,
	}
	if loc != nil {
		if s, ok := otfScript[loc.Script]; ok {
			g.script = s
		}
		if l, ok := otfLanguage[loc.Language]; ok {
			g.language = l
		}
	}

	gdef, err := g.ReadGdefTable()
	if err != nil && !table.IsMissing(err) {
		return nil, err
	}
	g.gdef = gdef

	return g, nil
}

// selectLookups returns the selected lookups as indices into the lookupList.
func (g *GTab) selectLookups(tableName string, includeFeature map[string]bool) ([]uint16, error) {
	info, err := g.toc.Find(tableName)
	if err != nil {
		return nil, err
	}
	g.Parser = parser.New(tableName,
		io.NewSectionReader(g.r, int64(info.Offset), int64(info.Length)))

	s := &parser.State{}

	err = g.Exec(s,
		parser.CmdStash16, // majorVersion
		parser.CmdStash16, // minorVersion
		parser.CmdStash16, // scriptListOffset
		parser.CmdStash16, // featureListOffset
		parser.CmdStash16, // lookupListOffset
	)
	if err != nil {
		return nil, err
	}
	data := s.GetStash()
	if data[0] != 1 || data[1] > 1 {
		return nil, g.Error("unsupported %s version %d.%d", tableName, data[0], data[1])
	}
	scriptListOffset := int64(data[2])
	featureListOffset := int64(data[3])
	lookupListOffset := int64(data[4])
	var featureVariationsOffset uint16
	if data[1] > 0 {
		featureVariationsOffset, err = g.ReadUint16()
		if err != nil {
			return nil, err
		}
	}
	if featureVariationsOffset != 0 {
		return nil, g.Error("VariationIndex support not implemented")
	}

	// Read the script list and pick a script.
	// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#script-list-table-and-script-record
	s.A = scriptListOffset
	err = g.Exec(s,
		parser.CmdSeek,
		parser.CmdRead16, parser.TypeUInt, // scriptCount
	)
	if err != nil {
		return nil, err
	}
	scriptCount := s.A
	var bestScriptOffs int64 = -1
	for i := 0; i < int(scriptCount); i++ {
		err = g.Exec(s,
			parser.CmdRead32, parser.TypeTag, // scriptTag
			parser.CmdRead16, parser.TypeUInt, // scriptOffset
		)
		if err != nil {
			return nil, err
		}
		if s.Tag == g.script || s.Tag == "DFLT" && bestScriptOffs < 0 {
			bestScriptOffs = s.A
		}
	}
	if bestScriptOffs <= 0 {
		return nil, nil
	}

	// Read the language list for the script, and pick a LangSys
	// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#script-table-and-language-system-record
	scriptTablePos := int64(scriptListOffset) + bestScriptOffs
	s.A = scriptTablePos
	err = g.Exec(s,
		parser.CmdSeek,
		parser.CmdStash16, // defaultLangSysOffset
		parser.CmdStash16, // langSysCount
	)
	if err != nil {
		return nil, err
	}
	data = s.GetStash()
	defaultLangSysOffset := data[0]
	langSysCount := int(data[1])
	bestLangSysOffs := int64(defaultLangSysOffset)
	for i := 0; i < langSysCount; i++ {
		err = g.Exec(s,
			parser.CmdRead32, parser.TypeTag, // langSysTag
			parser.CmdRead16, parser.TypeUInt, // langSysOffset
		)
		if err != nil {
			return nil, err
		}
		if s.Tag == g.language {
			bestLangSysOffs = s.A
			break
		}
	}
	if bestLangSysOffs == 0 {
		// no language-specific script behavior is defined
		return nil, nil
	}

	// Read the LangSys table and get the feature indices
	// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#language-system-table
	s.A = scriptTablePos + bestLangSysOffs
	err = g.Exec(s,
		parser.CmdSeek,
		parser.CmdRead16, parser.TypeUInt, // lookupOrderOffset
		parser.CmdAssertEq, 0,
		parser.CmdStash16,                 // requiredFeatureIndex
		parser.CmdRead16, parser.TypeUInt, // featureIndexCount
		parser.CmdLoop,
		parser.CmdStash16, // featureIndices[i]
		parser.CmdEndLoop,
	)
	if err != nil {
		return nil, err
	}
	featureIndices := s.GetStash()
	requiredFeature := 0
	if featureIndices[0] == 0xFFFF {
		// no required feature
		featureIndices = featureIndices[1:]
		requiredFeature = -1
	}

	// Read the feature list table
	// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#feature-list-table
	s.A = featureListOffset
	err = g.Exec(s,
		parser.CmdSeek,
		parser.CmdRead16, parser.TypeUInt, // featureCount
	)
	if err != nil {
		return nil, err
	}
	numFeatures := int(s.A)
	var featureOffsets []uint16
	for i, fi := range featureIndices {
		if int(fi) >= numFeatures {
			return nil, g.Error("feature index %d out of range", fi)
		}
		s.A = featureListOffset + 2 + 6*int64(fi)
		err = g.Exec(s,
			parser.CmdSeek,
			parser.CmdRead32, parser.TypeTag, // featureTag
			parser.CmdRead16, parser.TypeUInt, // featureOffset
		)
		if err != nil {
			return nil, err
		}
		if includeFeature[s.Tag] || i == requiredFeature {
			featureOffsets = append(featureOffsets, uint16(s.A))
		}
	}

	// read the Feature Tables to find the lookup indices
	// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#feature-table
	var lookupIndices []uint16
	for _, offs := range featureOffsets {
		s.A = int64(offs) + featureListOffset
		err = g.Exec(s,
			parser.CmdSeek,                    // start of Feature table
			parser.CmdStash16,                 // featureParamsOffset
			parser.CmdRead16, parser.TypeUInt, // lookupIndexCount
			parser.CmdLoop,
			parser.CmdStash16, // lookupIndex
			parser.CmdEndLoop,
		)
		if err != nil {
			return nil, err
		}
		data := s.GetStash()
		if data[0] != 0 {
			return nil, g.Error("FeatureParams tables not supported")
		}
		lookupIndices = append(lookupIndices, data[1:]...)
	}

	// sort and remove duplicates
	sort.Slice(lookupIndices, func(i, j int) bool {
		return lookupIndices[i] < lookupIndices[j]
	})
	i := 1
	for i < len(lookupIndices) {
		if lookupIndices[i] == lookupIndices[i-1] {
			lookupIndices = append(lookupIndices[:i], lookupIndices[i+1:]...)
		} else {
			i++
		}
	}

	s.A = lookupListOffset
	err = g.Exec(s,
		parser.CmdSeek,
		parser.CmdRead16, parser.TypeUInt, // lookupCount
		parser.CmdLoop,
		parser.CmdStash16, // lookupOffset[i]
		parser.CmdEndLoop,
	)
	if err != nil {
		return nil, err
	}
	g.lookupListOffset = lookupListOffset
	g.lookups = s.GetStash()

	return lookupIndices, nil
}

// readLookups reads the selected lookup tables.
func (g *GTab) readLookups(lookupIndices []uint16) (Lookups, error) {
	var res Lookups
	for _, idx := range lookupIndices {
		l, err := g.getGtabLookup(idx)
		if err != nil {
			return nil, err
		}
		res = append(res, l)
	}
	return res, nil
}

func (g *GTab) getGtabLookup(idx uint16) (*OldLookupTable, error) {
	if int(idx) >= len(g.lookups) {
		return nil, g.Error("lookup index %d out of range", idx)
	}
	base := g.lookupListOffset + int64(g.lookups[idx])

	// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#lookup-table
	s := &parser.State{}
	s.A = base
	err := g.Exec(s,
		parser.CmdSeek,
		parser.CmdStash16,                 // lookupType
		parser.CmdStash16,                 // lookupFlag
		parser.CmdRead16, parser.TypeUInt, // subtableCount
		parser.CmdLoop,
		parser.CmdStash16, // subtableOffset
		parser.CmdEndLoop,
	)
	if err != nil {
		return nil, err
	}
	data := s.GetStash()
	lookupType := data[0]
	flags := data[1]
	subtables := data[2:]
	var markFilteringSet uint16
	if flags&useMarkFilteringSet != 0 {
		// TODO(voss): implement this
		return nil, errors.New("USE_MARK_FILTERING_SET not supported")

		// markFilteringSet, err = g.ReadUInt16()
		// if err != nil {
		// 	return nil, err
		// }
	}

	lookup := &OldLookupTable{
		Filter:           g.makeFilter(flags),
		rtl:              flags&0x0001 != 0,
		markFilteringSet: markFilteringSet,
	}
	for _, offs := range subtables {
		res, err := g.subtableReader(s, lookupType, base+int64(offs))
		if err != nil {
			return nil, err
		}

		if res != nil {
			lookup.Subtables = append(lookup.Subtables, res)
		}
	}

	return lookup, nil
}
