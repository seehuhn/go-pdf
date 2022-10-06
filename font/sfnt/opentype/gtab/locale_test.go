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
