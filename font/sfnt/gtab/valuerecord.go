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
	"fmt"
	"strings"

	"seehuhn.de/go/pdf/font"
)

// valueRecord describes all the variables and values used to adjust the
// position of a glyph or set of glyphs.
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#value-record
type valueRecord struct {
	XPlacement       int16  // Horizontal adjustment for placement, in design units.
	YPlacement       int16  // Vertical adjustment for placement, in design units.
	XAdvance         int16  // Horizontal adjustment for advance, in design units — only used for horizontal layout.
	YAdvance         int16  // Vertical adjustment for advance, in design units — only used for vertical layout.
	XPlaDeviceOffset uint16 // Offset to Device table (non-variable font) / VariationIndex table (variable font) for horizontal placement, from beginning of the immediate parent table (SinglePos or PairPosFormat2 lookup subtable, PairSet table within a PairPosFormat1 lookup subtable) — may be NULL.
	YPlaDeviceOffset uint16 // Offset to Device table (non-variable font) / VariationIndex table (variable font) for vertical placement, from beginning of the immediate parent table (SinglePos or PairPosFormat2 lookup subtable, PairSet table within a PairPosFormat1 lookup subtable) — may be NULL.
	XAdvDeviceOffset uint16 // Offset to Device table (non-variable font) / VariationIndex table (variable font) for horizontal advance, from beginning of the immediate parent table (SinglePos or PairPosFormat2 lookup subtable, PairSet table within a PairPosFormat1 lookup subtable) — may be NULL.
	YAdvDeviceOffset uint16 // Offset to Device table (non-variable font) / VariationIndex table (variable font) for vertical advance, from beginning of the immediate parent table (SinglePos or PairPosFormat2 lookup subtable, PairSet table within a PairPosFormat1 lookup subtable) — may be NULL.
}

// readValueRecord reads the binary representation of a valueRecord.  The
// valueFormat determines which fields are present in the binary
// representation.
func (g *GTab) readValueRecord(valueFormat uint16) (*valueRecord, error) {
	if valueFormat == 0 {
		return nil, nil
	}
	res := &valueRecord{}
	var err error
	if valueFormat&0x0001 != 0 {
		res.XPlacement, err = g.ReadInt16()
		if err != nil {
			return nil, err
		}
	}
	if valueFormat&0x0002 != 0 {
		res.YPlacement, err = g.ReadInt16()
		if err != nil {
			return nil, err
		}
	}
	if valueFormat&0x0004 != 0 {
		res.XAdvance, err = g.ReadInt16()
		if err != nil {
			return nil, err
		}
	}
	if valueFormat&0x0008 != 0 {
		res.YAdvance, err = g.ReadInt16()
		if err != nil {
			return nil, err
		}
	}
	if valueFormat&0x0010 != 0 {
		res.XPlaDeviceOffset, err = g.ReadUInt16()
		if err != nil {
			return nil, err
		}
	}
	if valueFormat&0x0020 != 0 {
		res.YPlaDeviceOffset, err = g.ReadUInt16()
		if err != nil {
			return nil, err
		}
	}
	if valueFormat&0x0040 != 0 {
		res.XAdvDeviceOffset, err = g.ReadUInt16()
		if err != nil {
			return nil, err
		}
	}
	if valueFormat&0x0080 != 0 {
		res.YAdvDeviceOffset, err = g.ReadUInt16()
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}

func (vr *valueRecord) String() string {
	var adjust []string
	if vr != nil {
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
		if vr.XPlaDeviceOffset != 0 {
			adjust = append(adjust, fmt.Sprintf("xposdev%+d", vr.XPlaDeviceOffset))
		}
		if vr.YPlaDeviceOffset != 0 {
			adjust = append(adjust, fmt.Sprintf("yposdev%+d", vr.YPlaDeviceOffset))
		}
		if vr.XAdvDeviceOffset != 0 {
			adjust = append(adjust, fmt.Sprintf("xadvdev%+d", vr.XAdvDeviceOffset))
		}
		if vr.YAdvDeviceOffset != 0 {
			adjust = append(adjust, fmt.Sprintf("yadvdev%+d", vr.YAdvDeviceOffset))
		}
	}
	if len(adjust) == 0 {
		return "_"
	}
	return strings.Join(adjust, ",")
}

func (vr *valueRecord) Apply(glyph *font.Glyph) {
	if vr == nil {
		return
	}

	glyph.XOffset += int(vr.XPlacement)
	glyph.YOffset += int(vr.YPlacement)
	glyph.Advance += int(vr.XAdvance)

	if vr.YAdvance != 0 {
		panic("not implemented")
	}
	if vr.XPlaDeviceOffset != 0 {
		panic("not implemented")
	}
	if vr.YPlaDeviceOffset != 0 {
		panic("not implemented")
	}
	if vr.XAdvDeviceOffset != 0 {
		panic("not implemented")
	}
	if vr.YAdvDeviceOffset != 0 {
		panic("not implemented")
	}
}
