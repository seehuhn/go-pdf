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

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
	"seehuhn.de/go/pdf/locale"
)

// ScriptLang is a pair of script and language.
type ScriptLang struct {
	script locale.Script
	lang   locale.Language
}

// Features describes the mandatory and optional features for a script/language.
type Features struct {
	Required FeatureIndex // 0xFFFF, if no required feature
	Optional []FeatureIndex
}

// ScriptListInfo contains the information of a ScriptList table.
type ScriptListInfo map[ScriptLang]*Features

// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#script-list-table-and-script-record
func readScriptList(p *parser.Parser, pos int64) (ScriptListInfo, error) {
	err := p.SeekPos(pos)
	if err != nil {
		return nil, err
	}

	scriptCount, err := p.ReadUInt16()
	if err != nil {
		return nil, err
	}
	if 6*int64(scriptCount) > p.Size() {
		return nil, &font.InvalidFontError{
			SubSystem: "sfnt/gtab",
			Reason:    "invalid scriptCount",
		}
	}

	type scriptTableEntry struct {
		offset uint16
		script locale.Script
	}

	var entries []scriptTableEntry
	for i := 0; i < int(scriptCount); i++ {
		buf, err := p.ReadBytes(6)
		if err != nil {
			return nil, err
		}

		script, ok := otfScript[string(buf[:4])]
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
		if int(entry.offset) < 2+6*len(entries) {
			return nil, &font.InvalidFontError{
				SubSystem: "sfnt/gtab",
				Reason:    "invalid script table offset",
			}
		}
	}

	info := ScriptListInfo{}
	for _, entry := range entries {
		err = readScriptTable(p, pos+int64(entry.offset), entry.script, info)
		if err != nil {
			return nil, err
		}
	}

	return info, nil
}

// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#script-table-and-language-system-record
func readScriptTable(p *parser.Parser, pos int64, script locale.Script, info ScriptListInfo) error {
	err := p.SeekPos(pos)
	if err != nil {
		return err
	}

	data, err := p.ReadBytes(4)
	if err != nil {
		return err
	}

	defaultLangSysOffset := uint16(data[0])<<8 + uint16(data[1])
	langSysCount := uint16(data[2])<<8 + uint16(data[3])

	if defaultLangSysOffset > 0 && defaultLangSysOffset < 4+6*langSysCount {
		return &font.InvalidFontError{
			SubSystem: "sfnt/gtab",
			Reason:    "invalid defaultLangSysOffset",
		}
	}
	if 8+int64(langSysCount)*12 > p.Size() {
		return &font.InvalidFontError{
			SubSystem: "sfnt/gtab",
			Reason:    "invalid langSysCount",
		}
	}

	type langSysRecord struct {
		offset uint16
		lang   locale.Language
	}

	var records []langSysRecord
	for i := 0; i < int(langSysCount); i++ {
		buf, err := p.ReadBytes(6)
		if err != nil {
			return err
		}

		lang, ok := otfLanguage[string(buf[:4])]
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
		ff, err := readLangSysTable(p, pos+int64(defaultLangSysOffset))
		if err != nil {
			return err
		}
		info[ScriptLang{script: script, lang: locale.LangUndefined}] = ff
	}
	for _, record := range records {
		ff, err := readLangSysTable(p, pos+int64(record.offset))
		if err != nil {
			return err
		}
		info[ScriptLang{script: script, lang: record.lang}] = ff
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
	lookupOrderOffset := uint16(data[0])<<8 + uint16(data[1])
	requiredFeatureIndex := FeatureIndex(data[2])<<8 + FeatureIndex(data[3])
	featureIndexCount := uint16(data[4])<<8 + uint16(data[5])
	if lookupOrderOffset != 0 {
		return nil, &font.NotSupportedError{
			SubSystem: "sfnt/gtab",
			Feature:   "use of reordering tables",
		}
	}

	featureIndices := make([]FeatureIndex, featureIndexCount)
	for i := 0; i < int(featureIndexCount); i++ {
		idx, err := p.ReadUInt16()
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
	rScript := map[locale.Script]string{}
	for tag, code := range otfScript {
		rScript[code] = tag
	}
	rLang := map[locale.Language]string{}
	for tag, code := range otfLanguage {
		rLang[code] = tag
	}

	scripts := map[locale.Script][]locale.Language{}
	for key := range info {
		scripts[key.script] = append(scripts[key.script], key.lang)
	}

	type scriptRecord struct {
		tag    string
		script locale.Script
		offs   uint16
	}
	var scriptList []*scriptRecord
	for script := range scripts {
		tag, ok := rScript[script]
		if !ok || len(tag) != 4 {
			panic("invalid script")
		}
		scriptList = append(scriptList, &scriptRecord{
			tag:    tag,
			script: script,
		})
	}
	sort.Slice(scriptList, func(i, j int) bool {
		return scriptList[i].tag < scriptList[j].tag
	})

	totalSize := 2 + 6*len(scripts) // scriptCount, scriptRecords
	for _, sRec := range scriptList {
		sRec.offs = uint16(totalSize)
		langs := scripts[sRec.script]
		langCount := 0
		for _, lang := range langs {
			if lang != locale.LangUndefined {
				langCount++
			}
			langSys := info[ScriptLang{script: sRec.script, lang: lang}]
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
		copy(buf[p:p+4], []byte(rec.tag))
		buf[p+4] = byte(rec.offs >> 8)
		buf[p+5] = byte(rec.offs)
	}
	for _, sRec := range scriptList {
		scriptTablePos := int(sRec.offs)
		langs := scripts[sRec.script]

		type langSysRecord struct {
			langSys *Features
			tag     string
			offs    uint16
		}
		var defaultRecord *langSysRecord
		var langSysRecords []*langSysRecord
		for _, lang := range langs {
			langSys := info[ScriptLang{script: sRec.script, lang: lang}]
			if lang == locale.LangUndefined {
				defaultRecord = &langSysRecord{
					langSys: langSys,
				}
				continue
			}
			tag, ok := rLang[lang]
			if !ok || len(tag) != 4 {
				panic("invalid language")
			}
			langSysRecords = append(langSysRecords, &langSysRecord{
				langSys: langSys,
				tag:     tag,
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
