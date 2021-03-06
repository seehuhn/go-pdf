package coverage

import (
	"fmt"
	"sort"

	"golang.org/x/exp/maps"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
)

// Set is a coverage table, but with the coverage indices omitted.
type Set map[font.GlyphID]bool

// Glyphs returned the glyphs covered by the Set, in order of increasing glyph ID.
func (set Set) Glyphs() []font.GlyphID {
	glyphs := maps.Keys(set)
	sort.Slice(glyphs, func(i, j int) bool { return glyphs[i] < glyphs[j] })
	return glyphs
}

// ToTable converts the Set to a Coverage table.
func (set Set) ToTable() Table {
	glyphs := set.Glyphs()
	table := make(Table)
	for i, gid := range glyphs {
		table[gid] = i
	}
	return table
}

// ReadSet reads a coverage table from a parser.
// This function allows for some duplicate glyphs to be included.  This is
// forbidden by the spec, but occurs in some widely used fonts.
func ReadSet(p *parser.Parser, pos int64) (Set, error) {
	err := p.SeekPos(pos)
	if err != nil {
		return nil, err
	}

	format, err := p.ReadUint16()
	if err != nil {
		return nil, err
	}

	table := make(Set)

	switch format {
	case 1: // Coverage Format 1
		glyphCount, err := p.ReadUint16()
		if err != nil {
			return nil, err
		}
		for i := 0; i < int(glyphCount); i++ {
			gid, err := p.ReadUint16()
			if err != nil {
				return nil, err
			}
			table[font.GlyphID(gid)] = true
		}

	case 2: // Coverage Format 2
		rangeCount, err := p.ReadUint16()
		if err != nil {
			return nil, err
		}
		pos := 0
		prev := -1
		for i := 0; i < int(rangeCount); i++ {
			buf, err := p.ReadBytes(6)
			if err != nil {
				return nil, err
			}
			startGlyphID := int(buf[0])<<8 | int(buf[1])
			endGlyphID := int(buf[2])<<8 | int(buf[3])
			startCoverageIndex := int(buf[4])<<8 | int(buf[5])
			if startCoverageIndex != pos ||
				startGlyphID < prev ||
				endGlyphID < startGlyphID {
				// Some fonts list individual glyphs twice.  To cover most of
				// these cases, we allow startGlyphID to be equal to prev.
				return nil, &font.InvalidFontError{
					SubSystem: "sfnt/opentype/coverage",
					Reason:    "invalid coverage table (format 2)",
				}
			}
			for gid := startGlyphID; gid <= endGlyphID; gid++ {
				table[font.GlyphID(gid)] = true
				pos++
			}
			prev = endGlyphID
		}

	default:
		return nil, &font.NotSupportedError{
			SubSystem: "sfnt/opentype/coverage",
			Feature:   fmt.Sprintf("coverage format %d", format),
		}
	}

	return table, nil
}
