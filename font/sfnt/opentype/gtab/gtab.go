package gtab

import (
	"encoding/binary"
	"fmt"
	"sort"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
	"seehuhn.de/go/pdf/locale"
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

// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#script-list-table-and-script-record
func readScriptList(p *parser.Parser, pos int64) error {
	err := p.SeekPos(pos)
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

	type scriptTableEntry struct {
		offset uint16
		script locale.Script
	}

	var entries []scriptTableEntry
	var buf [6]byte
	for i := 0; i < int(scriptCount); i++ {
		_, err = p.Read(buf[:])
		if err != nil {
			return err
		}

		scriptTag := string(buf[:4])
		script, ok := otfScript[scriptTag]
		if !ok {
			continue
		}

		entry := scriptTableEntry{
			offset: uint16(buf[4])<<8 + uint16(buf[5]),
			script: script,
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].offset < entries[j].offset
	})

	for _, entry := range entries {
		readScriptTable(p, pos+int64(entry.offset), entry.script)
	}

	return nil
}

// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#script-table-and-language-system-record
func readScriptTable(p *parser.Parser, pos int64, script locale.Script) error {
	err := p.SeekPos(pos)
	if err != nil {
		return err
	}

	data, err := p.ReadBlob(4)
	if err != nil {
		return err
	}

	defaultLangSysOffset := uint16(data[0])<<8 + uint16(data[1])
	langSysCount := uint16(data[2])<<8 + uint16(data[3])

	type langSysRecord struct {
		offset uint16
		lang   locale.Language
	}

	var records []langSysRecord
	var buf [6]byte
	for i := 0; i < int(langSysCount); i++ {
		_, err = p.Read(buf[:])
		if err != nil {
			return err
		}

		langTag := string(buf[:4])
		lang, ok := otfLanguage[langTag]
		if !ok {
			continue
		}

		entry := langSysRecord{
			offset: uint16(buf[4])<<8 + uint16(buf[5]),
			lang:   lang,
		}
		records = append(records, entry)
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].offset < records[j].offset
	})

	if defaultLangSysOffset != 0 {
		fmt.Println("*", script)
	}
	for _, record := range records {
		fmt.Println(record.lang, script)
	}

	// panic("not implemented")
	return nil
}
