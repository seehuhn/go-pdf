// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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
	"sort"

	"golang.org/x/text/language"
	"seehuhn.de/go/pdf/sfnt/parser"
)

// ScriptListInfo contains the information of a ScriptList table.
// It maps BCP 47 tags to OpenType font features.
type ScriptListInfo map[language.Tag]*Features

// Features describes the mandatory and optional features for a script/language.
type Features struct {
	Required FeatureIndex // 0xFFFF, if no required feature
	Optional []FeatureIndex
}

type scriptTableEntry struct {
	script otfScript
	offset uint16
}

// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#script-list-table-and-script-record
func readScriptList(p *parser.Parser, pos int64) (ScriptListInfo, error) {
	err := p.SeekPos(pos)
	if err != nil {
		return nil, err
	}

	scriptCount, err := p.ReadUint16()
	if err != nil {
		return nil, err
	}
	if 6*int64(scriptCount) > p.Size() {
		return nil, &parser.InvalidFontError{
			SubSystem: "sfnt/gtab",
			Reason:    "invalid scriptCount",
		}
	}

	var entries []scriptTableEntry
	for i := 0; i < int(scriptCount); i++ {
		buf, err := p.ReadBytes(6)
		if err != nil {
			return nil, err
		}

		entry := scriptTableEntry{
			script: otfScript(buf[:4]),
			offset: uint16(buf[4])<<8 | uint16(buf[5]),
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].offset < entries[j].offset
	})

	for _, entry := range entries {
		if int(entry.offset) < 2+6*len(entries) {
			return nil, &parser.InvalidFontError{
				SubSystem: "sfnt/gtab",
				Reason:    "invalid script table offset",
			}
		}
	}

	info := ScriptListInfo{}
	for _, entry := range entries {
		err = info.readScriptTable(entry.script, p, pos+int64(entry.offset))
		if err != nil {
			return nil, err
		}
	}

	return info, nil
}

// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#script-table-and-language-system-record
func (info ScriptListInfo) readScriptTable(script otfScript, p *parser.Parser, pos int64) error {
	err := p.SeekPos(pos)
	if err != nil {
		return err
	}

	data, err := p.ReadBytes(4)
	if err != nil {
		return err
	}

	defaultLangSysOffset := uint16(data[0])<<8 | uint16(data[1])
	langSysCount := uint16(data[2])<<8 | uint16(data[3])

	if defaultLangSysOffset > 0 && defaultLangSysOffset < 4+6*langSysCount {
		return &parser.InvalidFontError{
			SubSystem: "sfnt/gtab",
			Reason:    "invalid defaultLangSysOffset",
		}
	}
	if 8+int64(langSysCount)*12 > p.Size() {
		return &parser.InvalidFontError{
			SubSystem: "sfnt/gtab",
			Reason:    "invalid langSysCount",
		}
	}

	type langSysRecord struct {
		lang   otfLang
		offset uint16
	}

	var records []langSysRecord
	if defaultLangSysOffset != 0 {
		records = append(records, langSysRecord{
			offset: defaultLangSysOffset,
		})
	}
	for i := 0; i < int(langSysCount); i++ {
		buf, err := p.ReadBytes(6)
		if err != nil {
			return err
		}

		lang := otfLang(buf[:4])

		records = append(records, langSysRecord{
			lang:   lang,
			offset: uint16(buf[4])<<8 | uint16(buf[5]),
		})
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].offset < records[j].offset
	})

	for _, record := range records {
		ff, err := readLangSysTable(p, pos+int64(record.offset))
		if err != nil {
			return err
		}
		tag, err := otfToBCP47(script, record.lang)
		if err != nil {
			// TODO(voss): what to do here?
			continue
		}
		info[tag] = ff
	}

	return nil
}

// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#language-system-table
func readLangSysTable(p *parser.Parser, pos int64) (*Features, error) {
	err := p.SeekPos(pos)
	if err != nil {
		return nil, err
	}

	data, err := p.ReadBytes(6)
	if err != nil {
		return nil, err
	}
	lookupOrderOffset := uint16(data[0])<<8 | uint16(data[1])
	requiredFeatureIndex := FeatureIndex(data[2])<<8 | FeatureIndex(data[3])
	featureIndexCount := uint16(data[4])<<8 | uint16(data[5])
	if lookupOrderOffset != 0 {
		return nil, &parser.NotSupportedError{
			SubSystem: "sfnt/gtab",
			Feature:   "use of reordering tables",
		}
	}

	featureIndices := make([]FeatureIndex, featureIndexCount)
	for i := 0; i < int(featureIndexCount); i++ {
		idx, err := p.ReadUint16()
		if err != nil {
			return nil, err
		} else if idx == 0xFFFF {
			continue
		}
		featureIndices[i] = FeatureIndex(idx)
	}

	return &Features{
		Required: requiredFeatureIndex,
		Optional: featureIndices,
	}, nil
}

func (info ScriptListInfo) encode() []byte {
	if info == nil {
		return nil
	}

	scriptLangs := map[otfScript][]language.Tag{}
	for tag := range info {
		script, _, err := bcp47ToOtf(tag)
		if err != nil {
			// TODO(voss): what to do here?
			continue
		}
		scriptLangs[script] = append(scriptLangs[script], tag)
	}

	var scriptList []*scriptTableEntry
	for otfScript := range scriptLangs {
		scriptList = append(scriptList, &scriptTableEntry{
			script: otfScript,
		})
	}
	sort.Slice(scriptList, func(i, j int) bool {
		return scriptList[i].script < scriptList[j].script
	})

	totalSize := 2 + 6*len(scriptLangs) // scriptCount, scriptRecords
	for _, sRec := range scriptList {
		sRec.offset = uint16(totalSize)
		langCount := 0
		for _, tag := range scriptLangs[sRec.script] {
			_, lang, _ := bcp47ToOtf(tag)
			if lang != "" {
				langCount++
			}
			langSys := info[tag]
			// lookupOrderOffset, requiredFeatureIndex, featureIndexCount, featureIndices:
			totalSize += 6 + len(langSys.Optional)*2
		}
		totalSize += 4 + 6*langCount // defaultLangSysOffset, langSysCount, langSysRecords
	}

	buf := make([]byte, totalSize)

	// write the ScriptList table
	buf[0] = byte(len(scriptList) >> 8)
	buf[1] = byte(len(scriptList))
	for i, rec := range scriptList {
		p := 2 + i*6
		copy(buf[p:p+4], []byte(rec.script))
		buf[p+4] = byte(rec.offset >> 8)
		buf[p+5] = byte(rec.offset)
	}
	for _, sRec := range scriptList {
		scriptTablePos := int(sRec.offset)

		type langSysRecord struct {
			langSys *Features
			tag     otfLang
			offs    uint16
		}
		var defaultRecord *langSysRecord
		var langSysRecords []*langSysRecord
		for _, tag := range scriptLangs[sRec.script] {
			langSys := info[tag]
			_, lang, _ := bcp47ToOtf(tag)
			if lang == "" {
				defaultRecord = &langSysRecord{
					langSys: langSys,
				}
				continue
			}
			if len(lang) != 4 {
				panic("invalid language " + string(lang))
			}
			langSysRecords = append(langSysRecords, &langSysRecord{
				langSys: langSys,
				tag:     lang,
			})
		}

		sort.Slice(langSysRecords, func(i, j int) bool {
			return langSysRecords[i].tag < langSysRecords[j].tag
		})
		pos := 4 + 6*len(langSysRecords)
		if defaultRecord != nil {
			defaultRecord.offs = uint16(pos)
			pos += 6 + len(defaultRecord.langSys.Optional)*2
		}
		for _, lRec := range langSysRecords {
			lRec.offs = uint16(pos)
			pos += 6 + len(lRec.langSys.Optional)*2
		}

		// write the Script table for sRec.tag
		if defaultRecord != nil {
			buf[scriptTablePos] = byte(defaultRecord.offs >> 8)
			buf[scriptTablePos+1] = byte(defaultRecord.offs)
		}
		buf[scriptTablePos+2] = byte(len(langSysRecords) >> 8)
		buf[scriptTablePos+3] = byte(len(langSysRecords))
		for i, lRec := range langSysRecords {
			p := scriptTablePos + 4 + i*6
			copy(buf[p:p+4], []byte(lRec.tag))
			buf[p+4] = byte(lRec.offs >> 8)
			buf[p+5] = byte(lRec.offs)
		}

		if defaultRecord != nil {
			p := scriptTablePos + int(defaultRecord.offs)
			ff := defaultRecord.langSys
			buf[p+2] = byte(ff.Required >> 8)
			buf[p+3] = byte(ff.Required)
			buf[p+4] = byte(len(ff.Optional) >> 8)
			buf[p+5] = byte(len(ff.Optional))
			for i, idx := range ff.Optional {
				buf[p+6+2*i] = byte(idx >> 8)
				buf[p+6+2*i+1] = byte(idx)
			}
		}
		for _, lRec := range langSysRecords {
			p := scriptTablePos + int(lRec.offs)
			ff := lRec.langSys
			buf[p+2] = byte(ff.Required >> 8)
			buf[p+3] = byte(ff.Required)
			buf[p+4] = byte(len(ff.Optional) >> 8)
			buf[p+5] = byte(len(ff.Optional))
			for i, idx := range ff.Optional {
				buf[p+6+2*i] = byte(idx >> 8)
				buf[p+6+2*i+1] = byte(idx)
			}
		}
	}

	return buf
}
