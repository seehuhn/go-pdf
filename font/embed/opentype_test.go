// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package embed_test

import (
	"testing"

	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/embed"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/internal/debug/makefont"
)

func TestOpenTypeFontCMap(t *testing.T) {
	type kind string
	const (
		simple    kind = "simple"
		composite kind = "composite"
	)
	classify := func(F font.Layouter) (outlines string, k kind) {
		switch F.(type) {
		case *cff.Simple:
			return "cff", simple
		case *cff.Composite:
			return "cff", composite
		case *truetype.Simple:
			return "glyf", simple
		case *truetype.Composite:
			return "glyf", composite
		}
		t.Fatalf("unexpected font type %T", F)
		return "", simple
	}

	fonts := map[string]func() *sfnt.Font{
		"cff":  makefont.OpenType,
		"glyf": makefont.TrueType,
	}
	cases := []struct {
		name string
		opt  *embed.Options
		want kind
	}{
		{"nil options", nil, simple},
		{"empty options", &embed.Options{}, simple},
		{"IdentityH", &embed.Options{CMap: cmap.IdentityH}, composite},
		{"IdentityV", &embed.Options{CMap: cmap.IdentityV}, composite},
		{"UTF8H", &embed.Options{CMap: cmap.UTF8H}, composite},
	}
	for outlines, mk := range fonts {
		for _, c := range cases {
			F, err := embed.OpenTypeFont(mk(), c.opt)
			if err != nil {
				t.Errorf("%s/%s: %v", outlines, c.name, err)
				continue
			}
			gotOutlines, got := classify(F)
			if gotOutlines != outlines || got != c.want {
				t.Errorf("%s/%s: got %s kind %s, want kind %s", outlines, c.name, gotOutlines, got, c.want)
			}
		}
	}
}
