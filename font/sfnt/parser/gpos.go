package parser

import (
	"fmt"

	"seehuhn.de/go/pdf/font"
)

type GposInfo []*GposLookup

// ReadGposInfo reads the "GSUB" table of a font, for a given writing script
// and language.
func (p *Parser) ReadGposInfo(script, lang string, extraFeatures ...string) (GposInfo, error) {
	gtab, err := newGTab(p, script, lang)
	if err != nil {
		return nil, err
	}

	includeFeature := map[string]bool{
		"kern": true,
		"mark": true,
		"mkmk": true,
	}
	for _, feature := range extraFeatures {
		includeFeature[feature] = true
	}
	err = gtab.Init("GPOS", includeFeature)
	if err != nil {
		return nil, err
	}

	var res GposInfo
	for _, idx := range gtab.LookupIndices {
		l, err := gtab.GetGposLookup(idx)
		if err != nil {
			return nil, err
		}
		res = append(res, l)
	}
	return res, nil
}

type GposLookup struct {
	Format uint16 // TODO(voss): remove?
	Flags  uint16

	subtables        []gposLookupSubtable
	markFilteringSet uint16
}

func (g *gTab) GetGposLookup(idx uint16) (*GposLookup, error) {
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

	lookup := &GposLookup{
		Format:           format,
		Flags:            flags,
		markFilteringSet: markFilteringSet,
	}
	for _, offs := range subtables {
		res, err := g.readGposSubtable(s, format, base+int64(offs))
		if err != nil {
			return nil, err
		}

		lookup.subtables = append(lookup.subtables, res)
	}

	return lookup, nil
}

func (g *gTab) readGposSubtable(s *State, format uint16, subtablePos int64) (gposLookupSubtable, error) {
	// TODO(voss): is this called more than once for the same subtablePos? -> use caching?
	s.A = subtablePos
	err := g.Exec(s,
		CmdSeek,
		CmdRead16, TypeUInt, // format
	)
	if err != nil {
		return nil, err
	}
	subFormat := s.A

	var res gposLookupSubtable

	// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#table-organization
	// switch 10*format + uint16(subFormat) {
	switch 10*format + uint16(subFormat) {
	case 4_1: // Mark-to-Base Attachment Positioning Format 1: Mark-to-base Attachment Point
		err = g.Exec(s,
			CmdStash, // markCoverageOffset
			CmdStash, // baseCoverageOffset
			CmdStash, // markClassCount
			CmdStash, // markArrayOffset
			CmdStash, // baseArrayOffset
		)
		data := s.GetStash()
		markCoverageOffset := data[0]
		baseCoverageOffset := data[1]
		markClassCount := data[2]
		markArrayOffset := data[3]
		baseArrayOffset := data[4]

		markCoverage, err := g.readCoverageTable(subtablePos + int64(markCoverageOffset))
		if err != nil {
			return nil, err
		}
		baseCoverage, err := g.readCoverageTable(subtablePos + int64(baseCoverageOffset))
		if err != nil {
			return nil, err
		}
		fmt.Println(data)
		fmt.Println(markCoverage)
		fmt.Println(baseCoverage)
		_ = markClassCount
		_ = markArrayOffset
		_ = baseArrayOffset
	}

	if res == nil {
		return nil, g.error("unsupported lookup type %d.%d\n", format, subFormat)
	}

	return res, nil
}

type gposLookupSubtable interface {
	Position(uint16, []font.GlyphPos, int)
}
