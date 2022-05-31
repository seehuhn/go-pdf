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

func TestGsubParser(t *testing.T) {
	fontInfo := debug.MakeSimpleFont()
	lookups, err := Parse(fontInfo, `
	GSUB1: A->B, M->N
	GSUB1: A-C -> B-D, M->N, N->O
	GSUB1: A->X, B->X, C->X, M->X, N->X
	GSUB2: A -> "AA", B -> "AA", C -> "ABAAC"
	GSUB3: A -> [ "BCD" ]
	GSUB4: -marks A A A -> B, A -> D, A A -> C
	GSUB5:
		"AAA" -> 1@0 2@1 1@0, "AAB" -> 1@0 1@1 2@0 ||
		class :alpha: = [A-K]
		class :digits: = [L-Z]
		/A B C/ :alpha: :digits: -> 2@1, :alpha: :: :digits: -> 2@2 ||
		[A B C] [A C] [A D] -> 3@0
	GSUB6:
		A B | C D | E F -> 1@0 2@1, B | C D E | F -> 1@2 ||
		inputclass :ABC: = ["ABC"]
		backtrackclass :DEF: = ["DEF"]
		lookaheadclass :DEF: = ["DEF"]
		/A B C/ :DEF: :: | :ABC: | :: :DEF: -> 1@0 ||
		[A] [A B C] | [A B] [A C] [B C] | [A B C] [A B C] -> 1@0 1@1 1@2
	`)
	if err != nil {
		t.Fatal(err)
	}
	fontInfo.Gsub = &gtab.Info{LookupList: lookups}
	ExplainGsub(fontInfo)
}

func FuzzGsub1(f *testing.F) {
	f.Add("A->B, M->N")
	f.Add("A-C -> B-D, M->N, N->O")
	f.Add("A->X, B->X, C->X, M->X, N->X")
	f.Fuzz(func(t *testing.T, desc string) {
		fontInfo := debug.MakeSimpleFont()
		lookups, err := Parse(fontInfo, "GSUB1: "+desc)
		if err != nil || len(lookups) != 1 {
			return
		}
		fontInfo.Gsub = &gtab.Info{LookupList: lookups}
		desc2 := ExplainGsub(fontInfo)
		lookups2, err := Parse(fontInfo, desc2)
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
		lookups, err := Parse(fontInfo, "GSUB2: "+desc)
		if err != nil || len(lookups) != 1 {
			return
		}
		fontInfo.Gsub = &gtab.Info{LookupList: lookups}
		desc2 := ExplainGsub(fontInfo)
		lookups2, err := Parse(fontInfo, desc2)
		if err != nil {
			t.Fatal(err)
		}
		if d := cmp.Diff(lookups, lookups2); d != "" {
			t.Error(d)
		}
	})
}

func FuzzGsub3(f *testing.F) {
	f.Add("A->[B], M->[N]")
	f.Add(`A -> ["AA"], B -> ["AA"], C -> ["ABAAC"]`)
	f.Fuzz(func(t *testing.T, desc string) {
		fontInfo := debug.MakeSimpleFont()
		lookups, err := Parse(fontInfo, "GSUB3: "+desc)
		if err != nil || len(lookups) != 1 {
			return
		}
		fontInfo.Gsub = &gtab.Info{LookupList: lookups}
		desc2 := ExplainGsub(fontInfo)
		lookups2, err := Parse(fontInfo, desc2)
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
	f.Add(`
		class :alpha: = [A-K]
		class :digits: = [L-Z]
    	/A B C/ :alpha: :digits: -> 2@1, :alpha: :: :digits: -> 2@2`)
	f.Add(`[A B C] [A C] [A D] -> 3@0`)
	f.Add(`"AAA" -> 1@0 2@1 1@0, "AAB" -> 1@0 1@1 2@0 ||
		class :alpha: = [A-K]
		class :digits: = [L-Z]
		/A B C/ :alpha: :digits: -> 2@1, :alpha: :: :digits: -> 2@2 ||
		[A B C] [A C] [A D] -> 3@0`)
	f.Fuzz(func(t *testing.T, desc string) {
		fontInfo := debug.MakeSimpleFont()
		lookups, err := Parse(fontInfo, "GSUB5: "+desc)
		if err != nil || len(lookups) != 1 {
			return
		}
		fontInfo.Gsub = &gtab.Info{LookupList: lookups}
		desc2 := ExplainGsub(fontInfo)
		lookups2, err := Parse(fontInfo, desc2)
		if err != nil {
			t.Fatal(err)
		}
		if d := cmp.Diff(lookups, lookups2); d != "" {
			t.Error(d)
		}
	})
}

func FuzzGsub6(f *testing.F) {
	f.Add(`A B | C D | E F -> 1@0 2@1, B | C D E | F -> 1@2`)
	f.Add(`
		backtrackclass :DEF: = ["DEF"]
		inputclass :ABC: = ["ABC"]
		lookaheadclass :GHI: = ["GHI"]
		/A B C/ :DEF: :: | :ABC: | :: :GHI: -> 1@0`)
	f.Add(`[A] [A B C] | [A B] [A C] [B C] | [A B C] [A B C] -> 1@0 1@1 1@2`)
	f.Add(`
		A B | C D | E F -> 1@0 2@1, B | C D E | F -> 1@2 ||
		inputclass :ABC: = ["ABC"]
		backtrackclass :DEF: = ["DEF"]
		lookaheadclass :DEF: = ["DEF"]
		/A B C/ :DEF: :: | :ABC: | :: :DEF: -> 1@0 ||
		[A] [A B C] | [A B] [A C] [B C] | [A B C] [A B C] -> 1@0 1@1 1@2`)
	f.Fuzz(func(t *testing.T, desc string) {
		fontInfo := debug.MakeSimpleFont()
		lookups, err := Parse(fontInfo, "GSUB6: "+desc)
		if err != nil || len(lookups) != 1 {
			return
		}
		fontInfo.Gsub = &gtab.Info{LookupList: lookups}
		desc2 := ExplainGsub(fontInfo)
		lookups2, err := Parse(fontInfo, desc2)
		if err != nil {
			t.Fatal(err)
		}
		if d := cmp.Diff(lookups, lookups2); d != "" {
			t.Error(d)
		}
	})
}

func TestGposParser(t *testing.T) {
	fontInfo := debug.MakeSimpleFont()
	lookups, err := Parse(fontInfo, `
	GPOS1: [A-C] -> dy+10 ||
		D -> dx-1, E -> dx+1, F -> dy-1, G -> dy+1, H -> x+1, I -> y+1
	`)
	if err != nil {
		t.Fatal(err)
	}
	fontInfo.Gpos = &gtab.Info{LookupList: lookups}

	explain := ExplainGpos(fontInfo)
	fmt.Println(explain)
}

func FuzzGpos1(f *testing.F) {
	f.Add("A-> dx+0")
	f.Add("[A-C] -> dy+10")
	f.Add("D -> dx-1, E -> dx+1, F -> dy-1, G -> dy+1, H -> x+1, I -> y+1")
	f.Add(`[A-C] -> dy+10 ||
	D -> dx-1, E -> dx+1, F -> dy-1, G -> dy+1, H -> x+1, I -> y+1`)
	f.Fuzz(func(t *testing.T, desc string) {
		fontInfo := debug.MakeSimpleFont()
		lookups, err := Parse(fontInfo, "GPOS1: "+desc)
		if err != nil || len(lookups) != 1 {
			return
		}
		fontInfo.Gpos = &gtab.Info{LookupList: lookups}
		desc2 := ExplainGpos(fontInfo)
		lookups2, err := Parse(fontInfo, desc2)
		if err != nil {
			t.Fatal(err)
		}
		if d := cmp.Diff(lookups, lookups2); d != "" {
			t.Error(d)
		}
	})
}
