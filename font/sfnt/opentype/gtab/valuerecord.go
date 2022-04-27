// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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
	"fmt"
	"math/bits"
	"strings"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
)

// ValueRecord describes an adjustment to the position of a glyph or set of glyphs.
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#value-record
type ValueRecord struct {
	XPlacement        int16  // Horizontal adjustment for placement
	YPlacement        int16  // Vertical adjustment for placement
	XAdvance          int16  // Horizontal adjustment for advance
	YAdvance          int16  // Vertical adjustment for advance
	XPlacementDevOffs uint16 // Offset to Device table/VariationIndex table for horizontal placement
	YPlacementDevOffs uint16 // Offset to Device table/VariationIndex table for vertical placement
	XAdvanceDevOffs   uint16 // Offset to Device table/VariationIndex table for horizontal advance
	YAdvanceDevOffs   uint16 // Offset to Device table/VariationIndex table for vertical advance
}

// readValueRecord reads the binary representation of a valueRecord.  The
// valueFormat determines which fields are present in the binary
// representation.
func readValueRecord(p *parser.Parser, valueFormat uint16) (*ValueRecord, error) {
	if valueFormat == 0 {
		return nil, nil
	}

	res := &ValueRecord{}
	var err error
	if valueFormat&0x0001 != 0 {
		res.XPlacement, err = p.ReadInt16()
		if err != nil {
			return nil, err
		}
	}
	if valueFormat&0x0002 != 0 {
		res.YPlacement, err = p.ReadInt16()
		if err != nil {
			return nil, err
		}
	}
	if valueFormat&0x0004 != 0 {
		res.XAdvance, err = p.ReadInt16()
		if err != nil {
			return nil, err
		}
	}
	if valueFormat&0x0008 != 0 {
		res.YAdvance, err = p.ReadInt16()
		if err != nil {
			return nil, err
		}
	}
	if valueFormat&0x0010 != 0 {
		res.XPlacementDevOffs, err = p.ReadUint16()
		if err != nil {
			return nil, err
		}
	}
	if valueFormat&0x0020 != 0 {
		res.YPlacementDevOffs, err = p.ReadUint16()
		if err != nil {
			return nil, err
		}
	}
	if valueFormat&0x0040 != 0 {
		res.XAdvanceDevOffs, err = p.ReadUint16()
		if err != nil {
			return nil, err
		}
	}
	if valueFormat&0x0080 != 0 {
		res.YAdvanceDevOffs, err = p.ReadUint16()
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}

func (vr *ValueRecord) getFormat() uint16 {
	if vr == nil {
		return 0
	}

	var format uint16
	if vr.XPlacement != 0 {
		format |= 0x0001
	}
	if vr.YPlacement != 0 {
		format |= 0x0002
	}
	if vr.XAdvance != 0 {
		format |= 0x0004
	}
	if vr.YAdvance != 0 {
		format |= 0x0008
	}
	if vr.XPlacementDevOffs != 0 {
		format |= 0x0010
	}
	if vr.YPlacementDevOffs != 0 {
		format |= 0x0020
	}
	if vr.XAdvanceDevOffs != 0 {
		format |= 0x0040
	}
	if vr.YAdvanceDevOffs != 0 {
		format |= 0x0080
	}

	if format == 0 {
		// set one of the fields to mark the difference between
		// a nil *ValueRecord and a zero value.
		format = 0x0004
	}

	return format
}

func (*ValueRecord) encodeLen(format uint16) int {
	return 2 * bits.OnesCount16(format)
}

func (vr *ValueRecord) encode(format uint16) []byte {
	bufSize := vr.encodeLen(format)
	buf := make([]byte, 0, bufSize)
	if format&0x0001 != 0 {
		buf = append(buf, byte(vr.XPlacement>>8), byte(vr.XPlacement))
	}
	if format&0x0002 != 0 {
		buf = append(buf, byte(vr.YPlacement>>8), byte(vr.YPlacement))
	}
	if format&0x0004 != 0 {
		buf = append(buf, byte(vr.XAdvance>>8), byte(vr.XAdvance))
	}
	if format&0x0008 != 0 {
		buf = append(buf, byte(vr.YAdvance>>8), byte(vr.YAdvance))
	}
	if format&0x0010 != 0 {
		buf = append(buf, byte(vr.XPlacementDevOffs>>8), byte(vr.XPlacementDevOffs))
	}
	if format&0x0020 != 0 {
		buf = append(buf, byte(vr.YPlacementDevOffs>>8), byte(vr.YPlacementDevOffs))
	}
	if format&0x0040 != 0 {
		buf = append(buf, byte(vr.XAdvanceDevOffs>>8), byte(vr.XAdvanceDevOffs))
	}
	if format&0x0080 != 0 {
		buf = append(buf, byte(vr.YAdvanceDevOffs>>8), byte(vr.YAdvanceDevOffs))
	}
	if len(buf) != bufSize {
		panic("unexpected buffer size") // TODO(voss): remove
	}
	return buf
}

func (vr *ValueRecord) String() string {
	if vr == nil {
		return "<nil>"
	}

	var adjust []string
	if vr.XPlacement != 0 {
		adjust = append(adjust, fmt.Sprintf("xpos%+d", vr.XPlacement))
	}
	if vr.YPlacement != 0 {
		adjust = append(adjust, fmt.Sprintf("ypos%+d", vr.YPlacement))
	}
	if vr.XAdvance != 0 {
		adjust = append(adjust, fmt.Sprintf("xadv%+d", vr.XAdvance))
	}
	if vr.YAdvance != 0 {
		adjust = append(adjust, fmt.Sprintf("yadv%+d", vr.YAdvance))
	}
	if vr.XPlacementDevOffs != 0 {
		adjust = append(adjust, fmt.Sprintf("xposdev%+d", vr.XPlacementDevOffs))
	}
	if vr.YPlacementDevOffs != 0 {
		adjust = append(adjust, fmt.Sprintf("yposdev%+d", vr.YPlacementDevOffs))
	}
	if vr.XAdvanceDevOffs != 0 {
		adjust = append(adjust, fmt.Sprintf("xadvdev%+d", vr.XAdvanceDevOffs))
	}
	if vr.YAdvanceDevOffs != 0 {
		adjust = append(adjust, fmt.Sprintf("yadvdev%+d", vr.YAdvanceDevOffs))
	}
	if len(adjust) == 0 {
		return "_"
	}
	return strings.Join(adjust, ",")
}

// Apply adjusts the position of a glyph according to the value record.
func (vr *ValueRecord) Apply(glyph *font.Glyph) {
	if vr == nil {
		return
	}

	if vr.YAdvance != 0 ||
		vr.XPlacementDevOffs != 0 ||
		vr.YPlacementDevOffs != 0 ||
		vr.XAdvanceDevOffs != 0 ||
		vr.YAdvanceDevOffs != 0 {
		panic("not implemented")
	}

	glyph.XOffset += vr.XPlacement
	glyph.YOffset += vr.YPlacement
	glyph.Advance += int32(vr.XAdvance)
}
