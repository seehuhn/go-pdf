package cff

import (
	"bufio"
	"errors"
	"io"

	"seehuhn.de/go/pdf/font/parser"
)

func readIndex(p *parser.Parser) ([][]byte, error) {
	count, err := p.ReadUInt16()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, nil
	}

	offSize, err := p.ReadUInt8()
	if err != nil {
		return nil, err
	}

	var offsets []uint32
	prevOffset := uint32(1)
	size := p.Size()
	for i := 0; i <= int(count); i++ {
		blob, err := p.ReadBlob(int(offSize))
		if err != nil {
			return nil, err
		}

		var offs uint32
		for _, x := range blob {
			offs = offs<<8 + uint32(x)
		}
		if offs < prevOffset || int64(offs) >= size {
			return nil, p.Error("invalid CFF INDEX")
		}
		offsets = append(offsets, offs-1)
		prevOffset = offs
	}

	buf := make([]byte, offsets[count])
	_, err = p.Read(buf)
	if err != nil {
		return nil, err
	}

	res := make([][]byte, count)
	for i := 0; i < int(count); i++ {
		res[i] = buf[offsets[i]:offsets[i+1]]
	}

	return res, nil
}

func writeIndex(w io.Writer, data [][]byte) (int, error) {
	count := len(data)
	if count >= 1<<16 {
		return 0, errors.New("too many items for CFF INDEX")
	}
	if count == 0 {
		return w.Write([]byte{0, 0})
	}

	bodyLength := 0
	for _, blob := range data {
		bodyLength += len(blob)
	}

	offSize := 1
	for bodyLength+1 >= 1<<(8*offSize) {
		offSize++
	}
	if offSize > 4 {
		return 0, errors.New("too much data for CFF INDEX")
	}

	total := 0
	out := bufio.NewWriter(w)

	n, _ := out.Write([]byte{
		byte(count >> 8), byte(count), // count
		byte(offSize), // offSize
	})
	total += n

	// offset
	var buf [4]byte
	pos := uint32(1)
	for i := 0; i <= count; i++ {
		for j := 0; j < offSize; j++ {
			buf[j] = byte(pos >> (8 * (offSize - j - 1)))
		}
		n, _ = out.Write(buf[:offSize])
		total += n
		if i < count {
			pos += uint32(len(data[i]))
		}
	}

	// data
	for i := 0; i < count; i++ {
		n, _ = out.Write(data[i])
		total += n
	}

	return total, out.Flush()
}
