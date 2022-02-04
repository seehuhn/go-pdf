package table

import (
	"errors"
	"io"
)

// ReadMaxp reads the number of Glyphs from the "maxp" table.
// All other information is ignored.
func ReadMaxp(r io.Reader) (int, error) {
	var buf [6]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}

	version := uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3])
	if version != 0x00005000 && version != 0x00010000 {
		return 0, errors.New("sfnt/maxp: unknown version")
	}

	numGlyphs := int(buf[4])<<8 | int(buf[5])
	return numGlyphs, nil
}

// EncodeMaxp encodes the number of Glyphs in a "maxp" table.
func EncodeMaxp(numGlyphs int) ([]byte, error) {
	if numGlyphs < 0 || numGlyphs >= 1<<16 {
		return nil, errors.New("sfnt/maxp: numGlyphs out of range")
	}
	return []byte{0x00, 0x00, 0x50, 0x00, byte(numGlyphs >> 8), byte(numGlyphs)}, nil
}
