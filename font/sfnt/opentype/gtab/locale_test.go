package gtab

import (
	"fmt"
	"testing"

	"golang.org/x/text/language"
)

func TestGetScript(t *testing.T) {
	cases := []struct{ in, out string }{
		{"en", "latn"},
		{"en-US", "latn"},
		{"de", "latn"},
		{"el", "grek"},
		{"el", "grek"},
		{"sr", "cyrl"},
		{"sr-Latn", "latn"},
	}
	for _, test := range cases {
		out, err := getOtfScript(test.in)
		if err != nil {
			t.Error(err)
		}
		if out != test.out {
			t.Errorf("expected %q but got %q", test.out, out)
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
