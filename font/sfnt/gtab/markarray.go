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

import "seehuhn.de/go/pdf/font/parser"

type markRecord struct {
	markClass uint16
	anchor
}

type anchor struct {
	X int16
	Y int16
}

func (g *GTab) readMarkArrayTable(pos int64) ([]markRecord, error) {
	// TODO(voss): is caching needed here?

	s := &parser.State{
		A: pos,
	}
	err := g.Exec(s,
		parser.CmdSeek,
		parser.CmdRead16, parser.TypeUInt, // markCount
		parser.CmdStoreInto, 0,
		parser.CmdLoop,
		parser.CmdStash16, // markRecords[i].markClass
		parser.CmdStash16, // markRecords[i].markAnchorOffset
		parser.CmdEndLoop,
	)
	if err != nil {
		return nil, err
	}
	markCount := int(s.R[0])
	data := s.GetStash()

	res := make([]markRecord, markCount)
	idx := 0
	for len(data) > 0 {
		class := data[0]
		offs := data[1]
		data = data[2:]

		res[idx].markClass = class
		err = g.readAnchor(pos+int64(offs), &res[idx].anchor)
		if err != nil {
			return nil, err
		}
		idx++
	}

	return res, nil
}

func (g *GTab) readAnchor(pos int64, res *anchor) error {
	s := &parser.State{
		A: pos,
	}
	err := g.Exec(s,
		parser.CmdSeek,
		parser.CmdStash16, // anchorFormat
		parser.CmdStash16, // xCoordinate
		parser.CmdStash16, // yCoordinate
	)
	if err != nil {
		return err
	}

	data := s.GetStash()
	format := data[0]
	res.X = int16(data[1])
	res.Y = int16(data[2])
	switch format {
	case 1:
		// nothing to do
	case 3:
		xDeviceOffset, err := g.ReadUint16()
		if err != nil {
			return err
		}
		yDeviceOffset, err := g.ReadUint16()
		if err != nil {
			return err
		}
		dx, err := g.readDeviceTable(pos, xDeviceOffset)
		if err != nil {
			return err
		}
		dy, err := g.readDeviceTable(pos, yDeviceOffset)
		if err != nil {
			return err
		}
		res.X += dx
		res.Y += dy
	default:
		return g.Error("Anchor Table format %d not supported", format)
	}

	return nil
}

func (g *GTab) readDeviceTable(pos int64, offs uint16) (int16, error) {
	if offs == 0 {
		return 0, nil
	}

	s := &parser.State{
		A: pos + int64(offs),
	}
	err := g.Exec(s,
		parser.CmdSeek,
		parser.CmdStash16, // startSize
		parser.CmdStash16, // endSize
		parser.CmdStash16, // deltaFormat
	)
	if err != nil {
		return 0, err
	}
	data := s.GetStash()
	format := data[2]

	var size int
	switch format {
	case 1: // LOCAL_2_BIT_DELTAS
		size = 2
	case 2: // LOCAL_4_BIT_DELTAS
		size = 4
	case 3: // LOCAL_8_BIT_DELTAS
		size = 8
	default: // 0x8000 = VARIATION_INDEX / 0x7FFC = reserved
		// not implemented
		return 0, nil
	}

	var delta int16
	// TODO(voss): implement this
	_ = size

	return delta, nil
}
