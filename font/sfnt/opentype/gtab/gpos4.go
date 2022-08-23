package gtab

import (
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
	"seehuhn.de/go/pdf/font/sfnt/opentype/anchor"
	"seehuhn.de/go/pdf/font/sfnt/opentype/coverage"
	"seehuhn.de/go/pdf/font/sfnt/opentype/markarray"
)

// Gpos4_1 is a Mark-to-Base Attachment Positioning Subtable (format 1)
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#mark-to-base-attachment-positioning-format-1-mark-to-base-attachment-point
type Gpos4_1 struct {
	MarkCov   coverage.Table
	BaseCov   coverage.Table
	MarkArray []markarray.Record // indexed by mark coverage index
	BaseArray [][]anchor.Table   // indexed by base coverage index, then by mark class
}

// Apply implements the Subtable interface.
func (l *Gpos4_1) Apply(keep KeepGlyphFn, seq []font.Glyph, a, b int) *Match {
	// TODO(voss): does this apply to the base or the mark?
	markIdx, ok := l.MarkCov[seq[a].Gid]
	if !ok {
		return nil
	}
	markRecord := l.MarkArray[markIdx]

	if a == 0 {
		return nil
	}
	p := a - 1
	var baseIdx int
	for p >= 0 {
		baseIdx, ok = l.BaseCov[seq[p].Gid]
		if ok {
			break
		}
		p--
	}
	if p < 0 {
		return nil
	}
	baseRecord := l.BaseArray[baseIdx][markRecord.Class]
	if baseRecord.IsEmpty() {
		// TODO(voss): verify that this is what others do, too.
		return nil
	}

	dx := baseRecord.X - markRecord.X
	dy := baseRecord.Y - markRecord.Y
	for i := p; i < a; i++ {
		dx -= seq[i].Advance
	}
	g := seq[a]
	g.XOffset += dx
	g.YOffset += dy
	_ = dy
	return &Match{
		InputPos: []int{a},
		Replace:  []font.Glyph{g},
		Next:     a + 1,
	}
}

func readGpos4_1(p *parser.Parser, subtablePos int64) (Subtable, error) {
	buf, err := p.ReadBytes(10)
	if err != nil {
		return nil, err
	}
	markCoverageOffset := int64(buf[0])<<8 | int64(buf[1])
	baseCoverageOffset := int64(buf[2])<<8 | int64(buf[3])
	markClassCount := int(buf[4])<<8 | int(buf[5])
	markArrayOffset := int64(buf[6])<<8 | int64(buf[7])
	baseArrayOffset := int64(buf[8])<<8 | int64(buf[9])

	markCov, err := coverage.Read(p, subtablePos+markCoverageOffset)
	if err != nil {
		return nil, err
	}
	baseCov, err := coverage.Read(p, subtablePos+baseCoverageOffset)
	if err != nil {
		return nil, err
	}

	markArray, err := markarray.Read(p, subtablePos+markArrayOffset, len(markCov))
	if err != nil {
		return nil, err
	}
	if len(markCov) > len(markArray) {
		markCov.Prune(len(markArray))
	} else {
		markArray = markArray[:len(markCov)]
	}

	baseArrayPos := subtablePos + baseArrayOffset
	err = p.SeekPos(baseArrayPos)
	if err != nil {
		return nil, err
	}

	baseCount, err := p.ReadUint16()
	if err != nil {
		return nil, err
	}
	if int(baseCount) > len(baseCov) {
		baseCount = uint16(len(baseCov))
	} else {
		baseCov.Prune(int(baseCount))
	}
	numOffsets := uint(baseCount) * uint(markClassCount)
	if numOffsets > (65536-6-2)/2 {
		// Offsets are 16-bit from baseArrayPos, and there must still be
		// space for at least one achor table.
		return nil, &font.InvalidFontError{
			SubSystem: "sfnt/opentype/gtab",
			Reason:    "GPOS4.1 table too large",
		}
	}
	offsets := make([]uint16, numOffsets)
	for i := range offsets {
		offsets[i], err = p.ReadUint16()
		if err != nil {
			return nil, err
		}
	}

	baseArray := make([][]anchor.Table, baseCount)
	for i := range baseArray {
		row := make([]anchor.Table, markClassCount)
		for j := range row {
			if offsets[j] == 0 {
				continue
			}
			row[j], err = anchor.Read(p, baseArrayPos+int64(offsets[j]))
			if err != nil {
				return nil, err
			}
		}
		baseArray[i] = row
		offsets = offsets[markClassCount:]
	}

	return &Gpos4_1{
		MarkCov:   markCov,
		BaseCov:   baseCov,
		MarkArray: markArray,
		BaseArray: baseArray,
	}, nil
}

func (l *Gpos4_1) countMarkClasses() int {
	if len(l.BaseArray) > 0 {
		return len(l.BaseArray[0])
	}

	var maxClass uint16
	for _, rec := range l.MarkArray {
		if rec.Class > maxClass {
			maxClass = rec.Class
		}
	}
	return int(maxClass) + 1
}

// EncodeLen implements the Subtable interface.
func (l *Gpos4_1) EncodeLen() int {
	total := 12
	total += l.MarkCov.EncodeLen()
	total += l.BaseCov.EncodeLen()
	total += 2 + (4+6)*len(l.MarkArray)

	total += 2
	for _, row := range l.BaseArray {
		for _, rec := range row {
			total += 2
			if !rec.IsEmpty() {
				total += 6
			}
		}
	}
	return total
}

// Encode implements the Subtable interface.
func (l *Gpos4_1) Encode() []byte {
	markCount := len(l.MarkArray)
	markClassCount := l.countMarkClasses()
	baseCount := len(l.BaseArray)

	total := 12
	markCoverageOffset := total
	total += l.MarkCov.EncodeLen()
	baseCoverageOffset := total
	total += l.BaseCov.EncodeLen()
	markArrayOffset := total
	total += 2 + (4+6)*markCount
	baseArrayOffset := total
	total += 2
	for _, row := range l.BaseArray {
		for _, rec := range row {
			total += 2
			if !rec.IsEmpty() {
				total += 6
			}
		}
	}
	res := make([]byte, 0, total)

	res = append(res,
		0, 1, // posFormat
		byte(markCoverageOffset>>8), byte(markCoverageOffset),
		byte(baseCoverageOffset>>8), byte(baseCoverageOffset),
		byte(markClassCount>>8), byte(markClassCount),
		byte(markArrayOffset>>8), byte(markArrayOffset),
		byte(baseArrayOffset>>8), byte(baseArrayOffset),
	)

	res = append(res, l.MarkCov.Encode()...)
	res = append(res, l.BaseCov.Encode()...)

	res = append(res,
		byte(markCount>>8), byte(markCount),
	)
	offs := 2 + 4*markCount
	for _, rec := range l.MarkArray {
		res = append(res,
			byte(rec.Class>>8), byte(rec.Class),
			byte(offs>>8), byte(offs),
		)
		offs += 6
	}
	for _, rec := range l.MarkArray {
		res = rec.Append(res)
	}

	res = append(res,
		byte(baseCount>>8), byte(baseCount),
	)
	offs = 2 + 2*baseCount*markClassCount
	for _, row := range l.BaseArray {
		for _, rec := range row {
			if rec.IsEmpty() {
				res = append(res, 0, 0)
				continue
			}
			res = append(res,
				byte(offs>>8), byte(offs),
			)
			offs += 6
		}
	}
	for _, row := range l.BaseArray {
		for _, rec := range row {
			if rec.IsEmpty() {
				continue
			}
			res = rec.Append(res)
		}
	}

	return res
}
