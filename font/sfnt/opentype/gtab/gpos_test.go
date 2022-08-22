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
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
	"seehuhn.de/go/pdf/font/sfnt/opentype/anchor"
	"seehuhn.de/go/pdf/font/sfnt/opentype/classdef"
	"seehuhn.de/go/pdf/font/sfnt/opentype/coverage"
	"seehuhn.de/go/pdf/font/sfnt/opentype/markarray"
)

func TestGpos2_2(t *testing.T) {
	l1 := &Gpos2_2{
		Cov:    coverage.Set{1: true, 12: true},
		Class1: classdef.Table{1: 1, 2: 1, 12: 2},
		Class2: classdef.Table{3: 1, 4: 2},
		Adjust: [][]*PairAdjust{
			{
				{
					First: &GposValueRecord{
						XPlacement: 1,
						YPlacement: 2,
						XAdvance:   3,
						YAdvance:   4,
					},
					Second: &GposValueRecord{
						XPlacement: 5,
						YPlacement: 6,
						XAdvance:   7,
						YAdvance:   8,
					},
				},
				{
					First: &GposValueRecord{
						XPlacement: 9,
						YPlacement: 10,
						XAdvance:   11,
						YAdvance:   12,
					},
					Second: &GposValueRecord{
						XPlacement: 13,
						YPlacement: 14,
						XAdvance:   15,
						YAdvance:   16,
					},
				},
				{
					First: &GposValueRecord{
						XPlacement: 1000,
						YPlacement: 2000,
						XAdvance:   3000,
						YAdvance:   4000,
					},
					Second: &GposValueRecord{
						XPlacement: 5000,
						YPlacement: 6000,
						XAdvance:   7000,
						YAdvance:   8000,
					},
				},
			},
		},
	}
	data := l1.Encode()
	p := parser.New("test", bytes.NewReader(data))
	err := p.Discard(2)
	if err != nil {
		t.Fatal(err)
	}
	l2, err := readGpos2_2(p, 0)
	if err != nil {
		t.Fatal(err)
	}
	if d := cmp.Diff(l1, l2); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}
}

func TestGpos4_1(t *testing.T) {
	l1 := &Gpos4_1{
		MarkCov: coverage.Table{1: 0},
		BaseCov: coverage.Table{2: 0},
		MarkArray: []markarray.Record{
			{
				Class: 0,
				Table: anchor.Table{
					X: 1,
					Y: 2,
				},
			},
		},
		BaseArray: [][]anchor.Table{
			{
				{
					X: 3,
					Y: 4,
				},
			},
		},
	}
	data := l1.Encode()
	p := parser.New("test", bytes.NewReader(data))
	err := p.Discard(2)
	if err != nil {
		t.Fatal(err)
	}
	l2, err := readGpos4_1(p, 0)
	if err != nil {
		t.Fatal(err)
	}
	if d := cmp.Diff(l1, l2); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}
}

func FuzzGpos1_1(f *testing.F) {
	l := &Gpos1_1{
		Cov: map[font.GlyphID]int{8: 0, 9: 1},
		Adjust: &GposValueRecord{
			XAdvance: 100,
		},
	}
	f.Add(l.Encode())

	f.Fuzz(func(t *testing.T, data []byte) {
		doFuzz(t, 1, 1, readGpos1_1, data)
	})
}

func FuzzGpos1_2(f *testing.F) {
	l := &Gpos1_2{}
	f.Add(l.Encode())
	l = &Gpos1_2{
		Cov: map[font.GlyphID]int{8: 0, 9: 1},
		Adjust: []*GposValueRecord{
			{XAdvance: 100},
			{XAdvance: 50, XPlacement: -50},
		},
	}
	f.Add(l.Encode())

	f.Fuzz(func(t *testing.T, data []byte) {
		doFuzz(t, 1, 2, readGpos1_2, data)
	})
}

func FuzzGpos2_1(f *testing.F) {
	l := &Gpos2_1{}
	f.Add(l.Encode())
	l = &Gpos2_1{
		Cov: map[font.GlyphID]int{1: 0, 3: 1},
		Adjust: []map[font.GlyphID]*PairAdjust{
			{
				2: &PairAdjust{
					First: &GposValueRecord{
						XAdvance: -10,
					},
				},
			},
		},
	}
	f.Add(l.Encode())
	l.Adjust = []map[font.GlyphID]*PairAdjust{
		{
			2: &PairAdjust{
				First: &GposValueRecord{
					XAdvance: -10,
				},
			},
			4: &PairAdjust{
				First: &GposValueRecord{
					XAdvance: -10,
				},
				Second: &GposValueRecord{
					XPlacement: 5,
				},
			},
			6: &PairAdjust{
				First: &GposValueRecord{
					XAdvance: -10,
				},
				Second: &GposValueRecord{
					XPlacement:        1,
					YPlacement:        2,
					XAdvance:          3,
					YAdvance:          4,
					XPlacementDevOffs: 5,
					YPlacementDevOffs: 6,
					XAdvanceDevOffs:   7,
					YAdvanceDevOffs:   8,
				},
			},
		},
	}
	f.Add(l.Encode())

	f.Fuzz(func(t *testing.T, data []byte) {
		doFuzz(t, 2, 1, readGpos2_1, data)
	})
}

func FuzzGpos3_1(f *testing.F) {
	l := &Gpos3_1{}
	f.Add(l.Encode())
	l = &Gpos3_1{
		Cov: coverage.Table{
			1: 0,
			3: 1,
			5: 2,
			6: 3,
		},
		Records: []EntryExitRecord{
			{
				Entry: anchor.Table{X: 1, Y: 2},
				Exit:  anchor.Table{X: 3, Y: 4},
			},
			{
				Entry: anchor.Table{X: -1, Y: -2},
				Exit:  anchor.Table{X: -3, Y: -4},
			},
			{
				Entry: anchor.Table{X: 0, Y: 1},
			},
			{
				Exit: anchor.Table{X: 1, Y: 0},
			},
		},
	}
	f.Add(l.Encode())
	f.Fuzz(func(t *testing.T, data []byte) {
		doFuzz(t, 3, 1, readGpos3_1, data)
	})
}

func FuzzGpos4_1(f *testing.F) {
	l := &Gpos4_1{}
	f.Add(l.Encode())
	l = &Gpos4_1{
		MarkCov: coverage.Table{
			1: 0,
			3: 1,
			9: 2,
		},
		BaseCov: coverage.Table{
			2: 0,
			4: 1,
			6: 2,
		},
		MarkArray: []markarray.Record{
			{
				Class: 0,
				Table: anchor.Table{
					X: -32768,
					Y: 0,
				},
			},
			{
				Class: 1,
				Table: anchor.Table{
					X: 32767,
					Y: 0,
				},
			},
			{
				Class: 0,
				Table: anchor.Table{
					X: -1,
					Y: 1,
				},
			},
		},
		BaseArray: [][]anchor.Table{
			{
				{X: -2, Y: -1},
				{X: 0, Y: 1},
			},
			{
				{X: 2, Y: 3},
				{X: 4, Y: 5},
			},
			{
				{X: 6, Y: 7},
				{X: 8, Y: 255},
			},
		},
	}
	f.Add(l.Encode())

	f.Fuzz(func(t *testing.T, data []byte) {
		doFuzz(t, 4, 1, readGpos4_1, data)
	})
}

func FuzzGpos6_1(f *testing.F) {
	l := &Gpos6_1{}
	f.Add(l.Encode())
	l = &Gpos6_1{
		Mark1Cov: coverage.Table{
			1: 0,
			3: 1,
			9: 2,
		},
		Mark2Cov: coverage.Table{
			2: 0,
			4: 1,
			6: 2,
		},
		Mark1Array: []markarray.Record{
			{
				Class: 0,
				Table: anchor.Table{
					X: -32768,
					Y: 0,
				},
			},
			{
				Class: 1,
				Table: anchor.Table{
					X: 32767,
					Y: 0,
				},
			},
			{
				Class: 0,
				Table: anchor.Table{
					X: -1,
					Y: 1,
				},
			},
		},
		Mark2Array: [][]anchor.Table{
			{
				{X: -2, Y: -1},
				{X: 0, Y: 1},
			},
			{
				{X: 2, Y: 3},
				{X: 4, Y: 5},
			},
			{
				{X: 6, Y: 7},
				{X: 8, Y: 255},
			},
		},
	}
	f.Add(l.Encode())

	f.Fuzz(func(t *testing.T, data []byte) {
		doFuzz(t, 6, 1, readGpos6_1, data)
	})
}
