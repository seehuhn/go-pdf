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
	"seehuhn.de/go/pdf/font/funit"
	"seehuhn.de/go/pdf/font/parser"
)

// GposValueRecord describes an adjustment to the position of a glyph or set of glyphs.
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#value-record
type GposValueRecord struct {
	XPlacement        funit.Int16 // Horizontal adjustment for placement
	YPlacement        funit.Int16 // Vertical adjustment for placement
	XAdvance          funit.Int16 // Horizontal adjustment for advance
	YAdvance          funit.Int16 // Vertical adjustment for advance
	XPlacementDevOffs uint16      // Offset to Device table/VariationIndex table for horizontal placement
	YPlacementDevOffs uint16      // Offset to Device table/VariationIndex table for vertical placement
	XAdvanceDevOffs   uint16      // Offset to Device table/VariationIndex table for horizontal advance
	YAdvanceDevOffs   uint16      // Offset to Device table/VariationIndex table for vertical advance
}

// readValueRecord reads the binary representation of a valueRecord.  The
// valueFormat determines which fields are present in the binary
// representation.
func readValueRecord(p *parser.Parser, valueFormat uint16) (*GposValueRecord, error) {
	if valueFormat == 0 {
		return nil, nil
	}

	res := &GposValueRecord{}
	var err error
	if valueFormat&0x0001 != 0 {
		tmp, err := p.ReadInt16()
		if err != nil {
			return nil, err
		}
		res.XPlacement = funit.Int16(tmp)
	}
	if valueFormat&0x0002 != 0 {
		tmp, err := p.ReadInt16()
		if err != nil {
			return nil, err
		}
		res.YPlacement = funit.Int16(tmp)
	}
	if valueFormat&0x0004 != 0 {
		tmp, err := p.ReadInt16()
		if err != nil {
			return nil, err
		}
		res.XAdvance = funit.Int16(tmp)
	}
	if valueFormat&0x0008 != 0 {
		tmp, err := p.ReadInt16()
		if err != nil {
			return nil, err
		}
		res.YAdvance = funit.Int16(tmp)
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

func (vr *GposValueRecord) getFormat() uint16 {
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

func (*GposValueRecord) encodeLen(format uint16) int {
	return 2 * bits.OnesCount16(format)
}

func (vr *GposValueRecord) encode(format uint16) []byte {
	bufSize := vr.encodeLen(format)
	buf := make([]byte, 0, bufSize)

	if vr == nil && format != 0 {
		vr = &GposValueRecord{}
	}

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
	return buf
}

func (vr *GposValueRecord) String() string {
	if vr == nil {
		return "<nil>"
	}

	var adjust []string
	if vr.XPlacement != 0 {
		adjust = append(adjust, fmt.Sprintf("x%+d", vr.XPlacement))
	}
	if vr.YPlacement != 0 {
		adjust = append(adjust, fmt.Sprintf("y%+d", vr.YPlacement))
	}
	if vr.XAdvance != 0 {
		adjust = append(adjust, fmt.Sprintf("dx%+d", vr.XAdvance))
	}
	if vr.YAdvance != 0 {
		adjust = append(adjust, fmt.Sprintf("dy%+d", vr.YAdvance))
	}
	if vr.XPlacementDevOffs != 0 {
		adjust = append(adjust, fmt.Sprintf("xdev%+d", vr.XPlacementDevOffs))
	}
	if vr.YPlacementDevOffs != 0 {
		adjust = append(adjust, fmt.Sprintf("ydev%+d", vr.YPlacementDevOffs))
	}
	if vr.XAdvanceDevOffs != 0 {
		adjust = append(adjust, fmt.Sprintf("dxdev%+d", vr.XAdvanceDevOffs))
	}
	if vr.YAdvanceDevOffs != 0 {
		adjust = append(adjust, fmt.Sprintf("dydev%+d", vr.YAdvanceDevOffs))
	}
	if len(adjust) == 0 {
		return "_"
	}
	return strings.Join(adjust, ",")
}

// Apply adjusts the position of a glyph according to the value record.
func (vr *GposValueRecord) Apply(glyph *font.Glyph) {
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
	glyph.Advance += vr.XAdvance
}
