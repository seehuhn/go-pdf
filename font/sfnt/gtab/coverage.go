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

package gtab

import (
	"errors"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt/parser"
)

func (g *GTab) readCoverageTable(pos int64) (coverage, error) {
	res, ok := g.coverageCache[pos]
	if ok {
		return res, nil
	}

	s := &parser.State{
		A: pos,
	}
	err := g.Exec(s,
		parser.CmdSeek,
		parser.CmdRead16, parser.TypeUInt, // format
	)
	if err != nil {
		return nil, err
	}
	format := int(s.A)

	res = make(coverage)

	switch format {
	case 1: // coverage table format 1
		err = g.Exec(s,
			parser.CmdRead16, parser.TypeUInt, // glyphCount
			parser.CmdLoop,
			parser.CmdStash, // glyphArray[i]
			parser.CmdEndLoop,
		)
		if err != nil {
			return nil, err
		}
		for k, gid := range s.GetStash() {
			res[font.GlyphID(gid)] = k
		}
	case 2: // coverage table format 2
		err = g.Exec(s,
			parser.CmdRead16, parser.TypeUInt, // rangeCount
		)
		if err != nil {
			return nil, err
		}
		rangeCount := int(s.A)
		for i := 0; i < rangeCount; i++ {
			err = g.Exec(s,
				parser.CmdStash, // startfont.GlyphIndex
				parser.CmdStash, // endfont.GlyphIndex
				parser.CmdStash, // startCoverageIndex
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
		return nil, g.Error("unsupported coverage format %d", format)
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
