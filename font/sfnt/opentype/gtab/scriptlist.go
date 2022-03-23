package gtab

import (
	"sort"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
	"seehuhn.de/go/pdf/locale"
)

type scriptLang struct {
	script locale.Script
	lang   locale.Language
}

type features struct {
	required featureIndex // 0xFFFF, if no required feature
	optional []featureIndex
}

type scriptListInfo map[scriptLang]*features

// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#script-list-table-and-script-record
func readScriptList(p *parser.Parser, pos int64) (scriptListInfo, error) {
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
	var buf [6]byte
	for i := 0; i < int(scriptCount); i++ {
		_, err = p.Read(buf[:])
		if err != nil {
			return nil, err
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

	info := scriptListInfo{}
	for _, entry := range entries {
		err = readScriptTable(p, pos+int64(entry.offset), entry.script, info)
		if err != nil {
			return nil, err
		}
	}

	return info, nil
}

// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#script-table-and-language-system-record
func readScriptTable(p *parser.Parser, pos int64, script locale.Script, info scriptListInfo) error {
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
		ff, err := readLangSysTable(p, pos+int64(defaultLangSysOffset))
		if err != nil {
			return err
		}
		info[scriptLang{script: script, lang: locale.LangUndefined}] = ff
	}
	for _, record := range records {
		ff, err := readLangSysTable(p, pos+int64(record.offset))
		if err != nil {
			return err
		}
		info[scriptLang{script: script, lang: record.lang}] = ff
	}

	return nil
}

// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#language-system-table
func readLangSysTable(p *parser.Parser, pos int64) (*features, error) {
	err := p.SeekPos(pos)
	if err != nil {
		return nil, err
	}

	data, err := p.ReadBlob(6)
	if err != nil {
		return nil, err
	}
	lookupOrderOffset := uint16(data[0])<<8 + uint16(data[1])
	requiredFeatureIndex := featureIndex(data[2])<<8 + featureIndex(data[3])
	featureIndexCount := uint16(data[4])<<8 + uint16(data[5])
	if lookupOrderOffset != 0 {
		return nil, &font.NotSupportedError{
			SubSystem: "sfnt/gtab",
			Feature:   "use of reordering tables",
		}
	}

	featureIndices := make([]featureIndex, featureIndexCount)
	for i := 0; i < int(featureIndexCount); i++ {
		idx, err := p.ReadUInt16()
		if err != nil {
			return nil, err
		}
		if idx == 0xFFFF {
			continue
		}
		featureIndices[i] = featureIndex(idx)
	}

	return &features{
		required: requiredFeatureIndex,
		optional: featureIndices,
	}, nil
}

func (info scriptListInfo) Encode() []byte {
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

	for script := range scripts {
		type tagLang struct {
			tag  string
			lang locale.Language
		}
		var langs []tagLang
		var def []byte

		for _, lang := range scripts[script] {
			if lang == locale.LangUndefined {
				def = info[scriptLang{
					script: script,
					lang:   lang,
				}].Encode()
			} else {
				langs = append(langs, tagLang{
					tag:  rLang[lang],
					lang: lang,
				})
			}
		}

		_ = def
	}
	panic("not implemented")
}

func (ff *features) Encode() []byte {
	n := len(ff.optional)
	buf := make([]byte, 6+2*n)
	buf[2] = byte(ff.required >> 8)
	buf[3] = byte(ff.required)
	buf[4] = byte(n >> 8)
	buf[5] = byte(n)
	for i, idx := range ff.optional {
		buf[6+2*i] = byte(idx >> 8)
		buf[6+2*i+1] = byte(idx)
	}
	return buf
}
