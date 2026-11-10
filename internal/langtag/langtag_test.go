// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package langtag

import (
	"testing"

	"golang.org/x/text/language"
)

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want language.Tag
	}{
		{"en-US", language.AmericanEnglish},
		{"de", language.German},
		{"EN-us", language.AmericanEnglish},
		{"und-DE", language.MustParse("und-DE")},
		{"", language.Und},

		// malformed
		{"en-US-foobarbazqux", language.Und},
		{"en--US", language.Und},
		{"en-US-", language.Und},
		{"123-de", language.Und},
		{"zz-ZZ-qqqqqqqqq", language.Und},

		// unknown subtags after the primary language
		{"en-XYZ", language.English},
		{"de-1234", language.German},
		{"en-abcde", language.English},
		{"en-US-abcde-abcde", language.AmericanEnglish},

		// private-use and special tags survive
		{"x-foo", language.MustParse("x-foo")},
		{"en-x-foo", language.MustParse("en-x-foo")},
		{"qaa-DE", language.MustParse("qaa-DE")},
		{"zxx", language.MustParse("zxx")},

		// unknown primary language
		{"qq", language.Und},
		{"qq-en", language.Und},
		{"qq-DE", language.Und},
		{"xx-Latn", language.Und},
	}
	for _, tc := range cases {
		if got := Parse(tc.in); got != tc.want {
			t.Errorf("%q: got %v, want %v", tc.in, got, tc.want)
		}
	}
}
