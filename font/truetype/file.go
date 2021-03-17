package truetype

import (
	"encoding/binary"
	"errors"
	"io"
)

// offset subtable
type offsets struct {
	ScalerType    uint32
	NumTables     uint16
	SearchRange   uint16
	EntrySelector uint16
	RangeShift    uint16
}

type Header struct {
	ScalerType uint32
	Tables     map[string]*TableInfo
}

// table directory entry
type TableInfo struct {
	CheckSum uint32
	Offset   uint32
	Length   uint32
}

func ReadHeader(r io.Reader) (*Header, error) {
	offs := &offsets{}
	err := binary.Read(r, binary.BigEndian, offs)
	if err != nil {
		return nil, err
	}

	tag := offs.ScalerType
	if tag != 0x00010000 && tag != 0x4F54544F {
		return nil, errors.New("unsupported font type")
	}

	res := &Header{
		ScalerType: tag,
		Tables:     map[string]*TableInfo{},
	}
	for i := 0; i < int(offs.NumTables); i++ {
		var tag uint32
		err := binary.Read(r, binary.BigEndian, &tag)
		if err != nil {
			return nil, err
		}
		tagString := string([]byte{
			byte(tag >> 24),
			byte(tag >> 16),
			byte(tag >> 8),
			byte(tag)})
		info := &TableInfo{}
		err = binary.Read(r, binary.BigEndian, info)
		if err != nil {
			return nil, err
		}
		res.Tables[tagString] = info
	}

	return res, nil
}

type nameTableHeader struct {
	Format uint16 // table version number
	Count  uint16 // number of name records
	Offset uint16 // offset to the beginning of strings (bytes)
}

type nameTableRecord struct {
	PlatformID         uint16 // platform identifier code
	PlatformSpecificID uint16 // platform-specific encoding identifier
	LanguageID         uint16 // language identifier
	NameID             uint16 // name identifier
	Length             uint16 // name string length in bytes
	Offset             uint16 // name string offset in bytes
}

func (header *Header) GetFontName(fd io.ReaderAt) (string, error) {
	info := header.Tables["name"]
	if info == nil {
		return "", errNoName
	}

	nameFd := io.NewSectionReader(fd, int64(info.Offset), int64(info.Length))

	nameHeader := &nameTableHeader{}
	err := binary.Read(nameFd, binary.BigEndian, nameHeader)
	if err != nil {
		return "", err
	}

	record := &nameTableRecord{}
	for i := 0; i < int(nameHeader.Count); i++ {
		err := binary.Read(nameFd, binary.BigEndian, record)
		if err != nil {
			return "", err
		}
		if record.NameID != 6 {
			continue
		}

		switch {
		case record.PlatformID == 1 && record.PlatformSpecificID == 0:
			nameFd.Seek(int64(nameHeader.Offset)+int64(record.Offset),
				io.SeekStart)
			buf := make([]byte, record.Length)
			_, err := io.ReadFull(nameFd, buf)
			if err != nil {
				return "", err
			}
			rr := make([]rune, len(buf))
			for i, c := range buf {
				rr[i] = macintosh[c]
			}
			return string(rr), nil
		case record.PlatformID == 3 && record.PlatformSpecificID == 1:
			nameFd.Seek(int64(nameHeader.Offset)+int64(record.Offset),
				io.SeekStart)
			buf := make([]uint16, record.Length/2)
			err := binary.Read(nameFd, binary.BigEndian, buf)
			if err != nil {
				return "", err
			}
			rr := make([]rune, len(buf))
			for i, c := range buf {
				rr[i] = rune(c)
			}
			return string(rr), nil
		}
	}

	return "", errNoName
}
