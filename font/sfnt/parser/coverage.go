package parser

import (
	"errors"

	"seehuhn.de/go/pdf/font"
)

func (g *gTab) readCoverageTable(pos int64) (coverage, error) {
	res, ok := g.coverageCache[pos]
	if ok {
		return res, nil
	}

	s := &State{
		A: pos,
	}
	err := g.Exec(s,
		CmdSeek,
		CmdRead16, TypeUInt, // format
	)
	if err != nil {
		return nil, err
	}
	format := int(s.A)

	res = make(coverage)

	switch format {
	case 1: // coverage table format 1
		err = g.Exec(s,
			CmdRead16, TypeUInt, // glyphCount
			CmdLoop,
			CmdRead16, TypeUInt, // glyphArray[i]
			CmdStash,
			CmdEndLoop,
		)
		if err != nil {
			return nil, err
		}
		for k, gid := range s.GetStash() {
			res[font.GlyphID(gid)] = k
		}
	case 2: // coverage table format 2
		err = g.Exec(s,
			CmdRead16, TypeUInt, // rangeCount
		)
		if err != nil {
			return nil, err
		}
		rangeCount := int(s.A)
		for i := 0; i < rangeCount; i++ {
			err = g.Exec(s,
				CmdRead16, TypeUInt, // startfont.GlyphIndex
				CmdStash,
				CmdRead16, TypeUInt, // endfont.GlyphIndex
				CmdStash,
				CmdRead16, TypeUInt, // startCoverageIndex
				CmdStash,
			)
			if err != nil {
				return nil, err
			}
			xx := s.GetStash()
			for k := int(xx[0]); k <= int(xx[1]); k++ {
				res[font.GlyphID(k)] = k - int(xx[0]) + int(xx[2])
			}
		}
	default:
		return nil, g.error("unsupported coverage format %d", format)
	}

	g.coverageCache[pos] = res
	return res, nil
}

type coverage map[font.GlyphID]int

func (cov coverage) check(size int) error {
	for _, k := range cov {
		if k < 0 || size >= 0 && k >= size {
			return errors.New("invalid coverage table")
		}
	}
	return nil
}
