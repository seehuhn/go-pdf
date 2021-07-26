package parser

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

type markRecord struct {
	markClass uint16
	anchor
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
	if format != 1 {
		return g.error("Anchor Table format %d not supported", format)
	}

	res.X = int16(data[1])
	res.Y = int16(data[2])
	return nil
}

type anchor struct {
	X int16
	Y int16
}
