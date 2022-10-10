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
	"fmt"
	"testing"

	"golang.org/x/text/language"
)

func TestOtfToBCP(t *testing.T) {
	cases := []struct {
		script otfScript
		lang   otfLang
	}{
		{"DFLT", ""},

		{"latn", ""},
		{"cyrl", ""},
		{"grek", ""},
		{"hebr", ""},
		{"arab", ""},
		{"thai", ""},
		{"hani", ""},
		{"kana", ""},
		{"latn", "ENG "},
		{"latn", "ROM "},
		{"latn", "TRK "},
		{"latn", "DEU "},
		{"cyrl", "SRB "},
		{"cyrl", "RUS "},
		{"hani", "ZHT "},
	}
	for _, test := range cases {
		tag, err := otfToBCP47(test.script, test.lang)
		if err != nil {
			t.Error(err)
			continue
		}
		script, lang, err := bcp47ToOtf(tag)
		if err != nil {
			t.Error(err)
			continue
		}
		if script != test.script || lang != test.lang {
			t.Errorf("got %s, %s; want %s, %s", script, lang, test.script, test.lang)
		}
	}
}

func TestBcpToOtf(t *testing.T) {
	cases := []struct {
		tag    language.Tag
		script otfScript
		lang   otfLang
	}{
		{language.MustParse("und-Zzzz"), "DFLT", ""},
		{language.MustParse("und-Latn"), "latn", ""},

		{language.English, "latn", "ENG "},
		{language.AmericanEnglish, "latn", "ENG "},
		{language.BritishEnglish, "latn", "ENG "},
		{language.German, "latn", "DEU "},
		{language.Greek, "grek", "ELL "},
		{language.Russian, "cyrl", "RUS "},
		{language.SimplifiedChinese, "hani", "ZHS "},
		{language.TraditionalChinese, "hani", "ZHT "},
		{language.Chinese, "hani", "ZHP "},
	}
	for i, test := range cases {
		script, lang, err := bcp47ToOtf(test.tag)
		if err != nil {
			t.Error(err)
			continue
		}
		if script != test.script || lang != test.lang {
			t.Errorf("%d: got %s, %s; want %s, %s", i, script, lang, test.script, test.lang)
		}
	}
}

func TestScriptTags(t *testing.T) {
	for _, bcpTag := range scriptBcp47 {
		_, err := language.ParseScript(bcpTag)
		if err != nil {
			t.Error(err)
		}
	}
}

func TestLangTags(t *testing.T) {
	for _, bcpTag := range langBcp47 {
		tag, err := language.Parse(bcpTag)
		if err != nil {
			t.Error(err)
		}
		if tag.String() != bcpTag {
			fmt.Println(bcpTag, "->", tag.String())
		}
	}
}
