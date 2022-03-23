package gtab

import (
	"encoding/binary"
	"fmt"

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
