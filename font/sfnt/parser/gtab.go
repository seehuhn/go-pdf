// seehuhn.de/go/pdf - support for reading and writing PDF files
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

package parser

import (
	"sort"

	"seehuhn.de/go/pdf/font/sfnt/table"
	"seehuhn.de/go/pdf/locale"
)

// gTab represents a GSUB or GPOS table.
type gTab struct {
	*Parser

	gdef *GdefInfo

	lookupIndices    []uint16
	lookupListOffset int64
	lookups          []uint16
	lookupReader     stReaderFn

	coverageCache map[int64]coverage
}

// newGTab wraps a parser with a helper to read GSUB and GPOS tables.
// This modifies p.Funcs!
func newGTab(p *Parser, tableName string, loc *locale.Locale, includeFeature map[string]bool) (*gTab, error) {
	script := otfScript[loc.Script]
	lang := otfLanguage[loc.Language]

	scriptSeen := false
	chooseScript := func(s *State) {
		if s.Tag == script {
			s.R[4] = s.A
			scriptSeen = true
		} else if (s.R[4] == 0 || s.Tag == "DFLT") && !scriptSeen {
			s.R[4] = s.A
		}
	}
	chooseLang := func(s *State) {
		if s.Tag == lang {
			s.R[0] = s.A
		}
	}
	p.Funcs = []func(*State){
		chooseScript,
		chooseLang,
	}

	gdef, err := p.ReadGdefInfo()
	if err != nil && !table.IsMissing(err) {
		return nil, err
	}

	g := &gTab{
		Parser:        p,
		gdef:          gdef,
		coverageCache: make(map[int64]coverage),
	}
	switch tableName {
	case "GPOS":
		g.lookupReader = g.readGposSubtable
	case "GSUB":
		g.lookupReader = g.readGsubSubtable
	default:
		panic("invalid table type " + tableName)
	}

	err = g.init(tableName, includeFeature)
	if err != nil {
		return nil, err
	}

	return g, nil
}

func (g *gTab) init(tableName string, includeFeature map[string]bool) error {
	err := g.OpenTable(tableName)
	if err != nil {
		return err
	}

	s := &State{}

	err = g.Exec(s,
		// Read the table header.
		CmdRead16, TypeUInt, // majorVersion
		CmdAssertEq, 1,
		CmdRead16, TypeUInt, // minorVersion
		CmdAssertLe, 1,
		CmdStoreInto, 0,
		CmdRead16, TypeUInt, // scriptListOffset
		CmdStoreInto, 1,
		CmdRead16, TypeUInt, // featureListOffset
		CmdStoreInto, 2,
		CmdRead16, TypeUInt, // lookupListOffset
		CmdStoreInto, 3,
		CmdLoad, 0,
		CmdCmpLt, 1,
		CmdJNZ, JumpOffset(2),
		CmdRead16, TypeUInt, // featureVariationsOffset (only version 1.1)

		// Read the script list and pick a script.
		CmdLoadI, 0,
		CmdStoreInto, 4,
		CmdLoad, 1, // scriptListOffset
		CmdSeek,
		CmdRead16, TypeUInt, // scriptCount
		CmdLoop,
		CmdReadTag,          // scriptTag
		CmdRead16, TypeUInt, // scriptOffset
		CmdCall, 0, // chooseScript(Tag=tag, A=offs) -> R4=best offs
		CmdEndLoop,
	)
	if err != nil {
		return err
	}

	bestScriptOffs := s.R[4]
	if bestScriptOffs == 0 {
		// no scripts defined
		return nil
	}

	s.R[1] += bestScriptOffs
	err = g.Exec(s,
		// Read the language list for the script, and pick a LangSys
		CmdLoad, 1,
		CmdSeek,
		CmdRead16, TypeUInt, // defaultLangSysOffset
		CmdStoreInto, 0,
		CmdRead16, TypeUInt, // langSysCount
		CmdLoop,
		CmdReadTag,          // langSysTag
		CmdRead16, TypeUInt, // langSysOffset
		CmdCall, 1, // chooseLang(Tag=tag, A=offs), updates R0
		CmdEndLoop,
	)
	if err != nil {
		return err
	}

	bestLangSysOffs := s.R[0]
	if bestLangSysOffs == 0 {
		// no language-specific script behavior is defined
		return nil
	}

	s.A = s.R[1] + bestLangSysOffs
	err = g.Exec(s, // Read the LangSys table
		CmdSeek,
		CmdRead16, TypeUInt, // lookupOrderOffset
		CmdAssertEq, 0,
		CmdStash,            // requiredFeatureIndex
		CmdRead16, TypeUInt, // featureIndexCount
		CmdLoop,
		CmdStash, // featureIndices[i]
		CmdEndLoop,

		// Read the number of features in the feature list
		CmdLoad, 2, // featureListOffset
		CmdSeek,
		CmdRead16, TypeUInt, // featureCount
	)
	if err != nil {
		return err
	}
	numFeatures := int(s.A)
	featureIndices := s.GetStash()
	required := 0
	if featureIndices[0] == 0xFFFF {
		// no required feature
		featureIndices = featureIndices[1:]
		required = -1
	}

	var featureOffsets []int64
	for i, fi := range featureIndices {
		if int(fi) >= numFeatures {
			return g.error("feature index %d out of range", fi)
		}
		s.A = s.R[2] + 2 + 6*int64(fi)
		err = g.Exec(s,
			CmdSeek,
			CmdReadTag,          // featureTag
			CmdRead16, TypeUInt, // featureOffset
			CmdAdd, 2, // add the base address (featureListOffset)
		)
		if err != nil {
			return err
		}

		tag := s.Tag
		if includeFeature[tag] || i == required {
			featureOffsets = append(featureOffsets, s.A)
		}
	}

	var lookupIndices []uint16
	for _, offs := range featureOffsets {
		s.A = offs
		err = g.Exec(s,
			CmdSeek,             // start of Feature table
			CmdRead16, TypeUInt, // featureParamsOffset
			CmdAssertEq, 0,
			CmdRead16, TypeUInt, // lookupIndexCount
			CmdLoop,
			CmdStash, // lookupIndex
			CmdEndLoop,
		)
		if err != nil {
			return err
		}
		lookupIndices = append(lookupIndices, s.GetStash()...)
	}
	sort.Slice(lookupIndices, func(i, j int) bool {
		return lookupIndices[i] < lookupIndices[j]
	})

	// remove duplicates
	i := 1
	for i < len(lookupIndices) {
		if lookupIndices[i] == lookupIndices[i-1] {
			lookupIndices = append(lookupIndices[:i], lookupIndices[i+1:]...)
		} else {
			i++
		}
	}
	g.lookupIndices = lookupIndices

	// Since more lookups might be required for nested lookups, we
	// keep the complete list of lookupOffsets.
	err = g.Exec(s,
		CmdLoad, 3, // lookupListOffset
		CmdSeek,
		CmdRead16, TypeUInt, // lookupCount
		CmdLoop,
		CmdStash, // lookupOffset[i]
		CmdEndLoop,
	)
	if err != nil {
		return err
	}
	g.lookupListOffset = s.R[3]
	g.lookups = s.GetStash()

	return nil
}

type stReaderFn func(s *State, format uint16, subtablePos int64) (lookupSubtable, error)

// ReadLookups reads the selected lookup tables.
func (g *gTab) ReadLookups() (Lookups, error) {
	var res Lookups
	for _, idx := range g.lookupIndices {
		l, err := g.getGtabLookup(idx)
		if err != nil {
			return nil, err
		}
		res = append(res, l)
	}
	return res, nil
}

func (g *gTab) getGtabLookup(idx uint16) (*lookupTable, error) {
	if int(idx) >= len(g.lookups) {
		return nil, g.error("lookup index %d out of range", idx)
	}
	base := g.lookupListOffset + int64(g.lookups[idx])

	// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#lookup-table
	s := &State{}
	s.A = base
	err := g.Exec(s,
		CmdSeek,
		CmdStash,            // lookupType
		CmdStash,            // lookupFlag
		CmdRead16, TypeUInt, // subtableCount
		CmdLoop,
		CmdStash, // subtableOffset
		CmdEndLoop,
	)
	if err != nil {
		return nil, err
	}
	data := s.GetStash()
	format := data[0]
	flags := data[1]
	subtables := data[2:]
	var markFilteringSet uint16
	if flags&0x0010 != 0 {
		err = g.Exec(s, CmdRead16, TypeUInt) // markFilteringSet
		if err != nil {
			return nil, err
		}
		markFilteringSet = uint16(s.A)
	}

	lookup := &lookupTable{
		rtl:              flags&0x0001 != 0,
		filter:           g.makeFilter(flags),
		markFilteringSet: markFilteringSet,
	}
	for _, offs := range subtables {
		res, err := g.lookupReader(s, format, base+int64(offs))
		if err != nil {
			return nil, err
		}

		if res != nil {
			lookup.subtables = append(lookup.subtables, res)
		}
	}

	return lookup, nil
}
