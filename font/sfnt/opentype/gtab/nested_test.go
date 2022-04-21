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
	"testing"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt/opentype/coverage"
)

func FuzzSeqContext1(f *testing.F) {
	sub := &SeqContext1{}
	f.Add(sub.Encode())
	sub.Cov = coverage.Table{3: 0, 5: 1}
	sub.Rules = [][]SequenceRule{
		{},
		{},
	}
	f.Add(sub.Encode())
	sub.Rules = [][]SequenceRule{
		{
			{
				In: []font.GlyphID{4},
				Actions: []SeqLookup{
					{SequenceIndex: 0, LookupListIndex: 1},
					{SequenceIndex: 1, LookupListIndex: 5},
					{SequenceIndex: 0, LookupListIndex: 4},
				},
			},
		},
		{
			{
				In: []font.GlyphID{6, 7},
				Actions: []SeqLookup{
					{SequenceIndex: 0, LookupListIndex: 2},
				},
			},
			{
				In: []font.GlyphID{6},
				Actions: []SeqLookup{
					{SequenceIndex: 2, LookupListIndex: 1},
					{SequenceIndex: 1, LookupListIndex: 2},
					{SequenceIndex: 0, LookupListIndex: 3},
				},
			},
		},
	}
	f.Add(sub.Encode())

	f.Fuzz(func(t *testing.T, data []byte) {
		doFuzz(t, 1, 1, readSeqContext1, data)
	})
}

func FuzzSeqContext2(f *testing.F) {
	sub := &SeqContext2{}
	f.Add(sub.Encode())
	sub.Cov = coverage.Table{3: 0, 5: 1}
	sub.Rules = [][]ClassSequenceRule{
		{},
		{},
	}
	f.Add(sub.Encode())
	sub.Rules = [][]ClassSequenceRule{
		{
			{
				In: []uint16{4},
				Actions: []SeqLookup{
					{SequenceIndex: 0, LookupListIndex: 1},
					{SequenceIndex: 1, LookupListIndex: 5},
					{SequenceIndex: 0, LookupListIndex: 4},
				},
			},
		},
		{
			{
				In: []uint16{6, 7},
				Actions: []SeqLookup{
					{SequenceIndex: 0, LookupListIndex: 2},
				},
			},
			{
				In: []uint16{6},
				Actions: []SeqLookup{
					{SequenceIndex: 2, LookupListIndex: 1},
					{SequenceIndex: 1, LookupListIndex: 2},
					{SequenceIndex: 0, LookupListIndex: 3},
				},
			},
		},
	}
	f.Add(sub.Encode())

	f.Fuzz(func(t *testing.T, data []byte) {
		doFuzz(t, 1, 2, readSeqContext2, data)
	})
}

func FuzzSeqContext3(f *testing.F) {
	sub := &SeqContext3{}
	f.Add(sub.Encode())
	sub.Cov = append(sub.Cov, coverage.Table{3: 0, 4: 1})
	sub.Actions = []SeqLookup{
		{SequenceIndex: 0, LookupListIndex: 1},
		{SequenceIndex: 1, LookupListIndex: 5},
		{SequenceIndex: 0, LookupListIndex: 4},
	}
	f.Add(sub.Encode())
	sub.Cov = append(sub.Cov, coverage.Table{1: 0, 3: 1, 5: 2})
	f.Add(sub.Encode())

	f.Fuzz(func(t *testing.T, data []byte) {
		doFuzz(t, 1, 3, readSeqContext3, data)
	})
}
