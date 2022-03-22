package gtab

import (
	"encoding/binary"
	"fmt"
	"sort"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
)

type Info struct{}

func Read(tableName string, r parser.ReadSeekSizer) (*Info, error) {
	p := parser.New(tableName, r)

	var header struct {
		MajorVersion      uint16
		MinorVersion      uint16
		ScriptListOffset  uint16
		FeatureListOffset uint16
		LookupListOffset  uint16
	}
	var FeatureVariationsOffset uint32

	err := binary.Read(p, binary.BigEndian, &header)
	if err != nil {
		return nil, err
	}
	if header.MajorVersion != 1 || header.MinorVersion > 1 {
		return nil, &font.NotSupportedError{
			SubSystem: "sfnt/gtab",
			Feature: fmt.Sprintf("%s table version %d.%d",
				tableName, header.MajorVersion, header.MinorVersion),
		}
	}
	endOfHeader := uint32(10)
	if header.MinorVersion == 1 {
		FeatureVariationsOffset, err = p.ReadUInt32()
		if err != nil {
			return nil, err
		}
		endOfHeader += 4
	}

	fileSize := p.Size()
	for _, offset := range []uint32{
		uint32(header.ScriptListOffset),
		uint32(header.FeatureListOffset),
		uint32(header.LookupListOffset),
		FeatureVariationsOffset,
	} {
		if 0 < offset && offset < endOfHeader || int64(offset) > fileSize {
			return nil, &font.NotSupportedError{
				SubSystem: "sfnt/gtab",
				Feature:   fmt.Sprintf("%s header has invalid offset", tableName),
			}
		}
	}

	if header.ScriptListOffset != 0 {
		readScriptList(p, int64(header.ScriptListOffset))
	}

	_ = FeatureVariationsOffset
	return nil, nil
}

func readScriptList(p *parser.Parser, offset int64) error {
	err := p.SeekPos(offset)
	if err != nil {
		return err
	}

	scriptCount, err := p.ReadUInt16()
	if err != nil {
		return err
	}
	if 6*int64(scriptCount) > p.Size() {
		return &font.InvalidFontError{
			SubSystem: "sfnt/gtab",
			Reason:    "invalid scriptCount",
		}
	}

	buf := make([]byte, 6*int(scriptCount))
	_, err = p.Read(buf)
	if err != nil {
		return err
	}

	type scriptTableEntry struct {
		offset uint16
		tag    uint32
	}

	var entries []scriptTableEntry
	for i := 0; i < int(scriptCount); i++ {
		entry := scriptTableEntry{
			// TODO(voss): filter out unrecognized scripts
			tag: uint32(buf[i*6+0])<<24 |
				uint32(buf[i*6+1])<<16 |
				uint32(buf[i*6+2])<<8 |
				uint32(buf[i*6+3]),
			offset: uint16(buf[i*6+4])<<8 + uint16(buf[i*6+5]),
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].offset < entries[j].offset
	})

	for _, entry := range entries {
		pos := offset + int64(entry.offset)
		_ = pos
	}

	return nil
}
