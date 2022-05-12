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

package builder

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf/font/debug"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gtab"
)

func TestParser(t *testing.T) {
	fontInfo := debug.MakeSimpleFont()
	lookups, err := parse(fontInfo, `
	GSUB_1: A->B, M->N
	GSUB_1: A-C -> B-D, M->N, N->O
	GSUB_1: A->X, B->X, C->X, M->X, N->X
	GSUB_2: A -> "AA", B -> "AA", C -> "ABAAC"
	GSUB_3: A -> [ "BCD" ]
	GSUB_4: -marks A A A -> B, A -> D, A A -> C
	GSUB_5:
	  class :alpha: = [A-K]
	  class :digits: = [L-Z]
	  "AAA" -> 1@0 2@1 1@0, "AAB" -> 1@0 1@1 2@0 ||
	  /A B C/ :alpha: :digits: -> 2@1, :alpha: :: :digits: -> 2@2 ||
	  [A B C] [A C] [A D] -> 3@0
	GSUB_6:
	  A B | C D | E F -> 1@0 2@1, B | C D E | F -> 1@2
	`)
	if err != nil {
		t.Fatal(err)
	}
	fontInfo.Gsub = &gtab.Info{LookupList: lookups}

	explain := ExplainGsub(fontInfo)
	fmt.Println(explain)
}

func FuzzGsub1(f *testing.F) {
	f.Add("A->B, M->N")
	f.Add("A-C -> B-D, M->N, N->O")
	f.Add("A->X, B->X, C->X, M->X, N->X")
	f.Fuzz(func(t *testing.T, desc string) {
		fontInfo := debug.MakeSimpleFont()
		lookups, err := parse(fontInfo, "GSUB_1: "+desc)
		if err != nil || len(lookups) != 1 {
			return
		}
		fontInfo.Gsub = &gtab.Info{LookupList: lookups}
		desc2 := ExplainGsub(fontInfo)
		lookups2, err := parse(fontInfo, desc2)
		if err != nil {
			t.Fatal(err)
		}
		if d := cmp.Diff(lookups, lookups2); d != "" {
			t.Error(d)
		}
	})
}

func FuzzGsub2(f *testing.F) {
	f.Add("A->B, M->N")
	f.Add(`A -> "AA", B -> "AA", C -> "ABAAC"`)
	f.Fuzz(func(t *testing.T, desc string) {
		fontInfo := debug.MakeSimpleFont()
		lookups, err := parse(fontInfo, "GSUB_2: "+desc)
		if err != nil || len(lookups) != 1 {
			return
		}
		fontInfo.Gsub = &gtab.Info{LookupList: lookups}
		desc2 := ExplainGsub(fontInfo)
		lookups2, err := parse(fontInfo, desc2)
		if err != nil {
			t.Fatal(err)
		}
		if d := cmp.Diff(lookups, lookups2); d != "" {
			t.Error(d)
		}
	})
}

func FuzzGsub5(f *testing.F) {
	f.Add(`"AAA" -> 1@0 2@1 1@0, "AAB" -> 1@0 1@1 2@0 `)
	f.Add(`/A B C/ :alpha: :digits: -> 2@1, :alpha: :: :digits: -> 2@2`)
	f.Add(`[A B C] [A C] [A D] -> 3@0`)
	f.Add(`"AAA" -> 1@0 2@1 1@0, "AAB" -> 1@0 1@1 2@0 ||
	/A B C/ :alpha: :digits: -> 2@1, :alpha: :: :digits: -> 2@2 ||
	[A B C] [A C] [A D] -> 3@0`)
	f.Fuzz(func(t *testing.T, desc string) {
		fontInfo := debug.MakeSimpleFont()
		lookups, err := parse(fontInfo, `
			class :alpha: = [A-K]
			class :digits: = [L-Z]
			GSUB_5: `+desc)
		if err != nil || len(lookups) != 1 {
			return
		}
		fontInfo.Gsub = &gtab.Info{LookupList: lookups}
		desc2 := ExplainGsub(fontInfo)
		lookups2, err := parse(fontInfo, desc2)
		if err != nil {
			t.Fatal(err)
		}
		if d := cmp.Diff(lookups, lookups2); d != "" {
			t.Error(d)
		}
	})
}
