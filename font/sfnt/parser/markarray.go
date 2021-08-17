package parser

type markRecord struct {
	markClass uint16
	anchor
}

type anchor struct {
	X int16
	Y int16
}

func (g *gTab) readMarkArrayTable(pos int64) ([]markRecord, error) {
	// TODO(voss): is caching needed here?

	s := &State{
		A: pos,
	}
	err := g.Exec(s,
		CmdSeek,
		CmdRead16, TypeUInt, // markCount
		CmdStoreInto, 0,
		CmdLoop,
		CmdStash, // markRecords[i].markClass
		CmdStash, // markRecords[i].markAnchorOffset
		CmdEndLoop,
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

func (g *gTab) readAnchor(pos int64, res *anchor) error {
	s := &State{
		A: pos,
	}
	err := g.Exec(s,
		CmdSeek,
		CmdStash, // anchorFormat
		CmdStash, // xCoordinate
		CmdStash, // yCoordinate
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
		xDeviceOffset, err := g.ReadUInt16()
		if err != nil {
			return err
		}
		yDeviceOffset, err := g.ReadUInt16()
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
		return g.error("Anchor Table format %d not supported", format)
	}

	return nil
}

func (g *gTab) readDeviceTable(pos int64, offs uint16) (int16, error) {
	if offs == 0 {
		return 0, nil
	}

	s := &State{
		A: pos + int64(offs),
	}
	err := g.Exec(s,
		CmdSeek,
		CmdStash, // startSize
		CmdStash, // endSize
		CmdStash, // deltaFormat
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
