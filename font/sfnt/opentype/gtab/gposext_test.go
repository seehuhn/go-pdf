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

package gtab_test

import (
	"testing"

	"seehuhn.de/go/pdf/font/debug"
	"seehuhn.de/go/pdf/font/sfnt/opentype/coverage"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gtab"
)

func TestGpos(t *testing.T) {
	fontInfo := debug.MakeSimpleFont()

	gidA := fontInfo.CMap.Lookup('A')

	lookupType := uint16(1)
	subtables := gtab.Subtables{
		&gtab.Gpos1_1{
			Cov:    coverage.Table{gidA: 0},
			Adjust: &gtab.GposValueRecord{YPlacement: 500},
		},
	}

	gpos := &gtab.Info{
		ScriptList: map[gtab.ScriptLang]*gtab.Features{
			{}: {}, // Required: 0
		},
		FeatureList: []*gtab.Feature{
			{Tag: "test", Lookups: []gtab.LookupIndex{0}},
		},
		LookupList: gtab.LookupList{
			{
				Meta: &gtab.LookupMetaInfo{
					LookupType: lookupType,
				},
				Subtables: subtables,
			},
		},
	}
	fontInfo.Gpos = gpos

	exportFont(fontInfo, 1234, "")
}
