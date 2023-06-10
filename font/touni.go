// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package font

import (
	"sort"
	"unicode/utf16"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/tounicode"
	"seehuhn.de/go/sfnt/type1"
)

// SimpleMapping describes the unicode text corresponding to a character code
// in a simple font.
type SimpleMapping struct {
	Code byte
	Text []rune
}

// WriteToUnicodeSimple writes the ToUnicode stream for a simple font.
// This modifies mm.
func WriteToUnicodeSimple(w pdf.Putter, ref pdf.Reference, ordering string, mm []SimpleMapping) error {
	data := &tounicode.Info{
		Name: "Seehuhn-" + pdf.Name(ordering) + "-UCS2",
		ROS: &type1.CIDSystemInfo{
			Registry:   "Seehuhn",
			Ordering:   ordering,
			Supplement: 0,
		},
		CodeSpace: []tounicode.CodeSpaceRange{{First: 0x00, Last: 0xFF}},
	}

	sort.Slice(mm, func(i, j int) bool { return mm[i].Code < mm[j].Code })

	canDeltaRange := make([]bool, len(mm))
	step := make([]byte, len(mm))
	var prevDelta int
	var prevCharCode byte
	for i, m := range mm {
		delta := int(m.Text[0]) - int(m.Code)
		charCode := m.Code
		if i > 0 {
			canDeltaRange[i] = delta == prevDelta && len(m.Text) == 1
			step[i] = charCode - prevCharCode
		}
		prevDelta = delta
		prevCharCode = charCode
	}

	pos := 0
	for pos < len(mm) {
		next := pos + 1
		for next < len(mm) && canDeltaRange[next] {
			next++
		}
		if next > pos+1 {
			bf := tounicode.Range{
				First: cmap.CID(mm[pos].Code),
				Last:  cmap.CID(mm[next-1].Code),
				UTF16: [][]uint16{utf16.Encode(mm[pos].Text)},
			}
			data.Ranges = append(data.Ranges, bf)
			pos = next
			continue
		}

		next = pos + 1
		for next < len(mm) && step[next] < 2 {
			next++
		}
		if next > pos+1 {
			var repl [][]uint16
			for i := pos; i < next; i++ {
				if i > pos && step[i] > 1 {
					for j := 0; j < int(step[i]-1); j++ {
						repl = append(repl, utf16.Encode([]rune{0xFFFD}))
					}
				}
				repl = append(repl, utf16.Encode(mm[i].Text))
			}
			bf := tounicode.Range{
				First: cmap.CID(mm[pos].Code),
				Last:  cmap.CID(mm[next-1].Code),
				UTF16: repl,
			}
			data.Ranges = append(data.Ranges, bf)
			pos = next
			continue
		}

		data.Singles = append(data.Singles, tounicode.Single{
			Code:  cmap.CID(mm[pos].Code),
			UTF16: utf16.Encode(mm[pos].Text),
		})
		pos++
	}

	return data.Embed(ref, w)
}

// CIDMapping describes the unicode text corresponding to a character code
// in a CIDFont.
type CIDMapping struct {
	CharCode uint16 // TODO(voss): match the type in cff.Outlines.Gid2cid
	Text     []rune
}

// WriteToUnicodeCID writes the ToUnicode stream for a CIDFont.
// This modifies mm.
func WriteToUnicodeCID(ref pdf.Reference, w pdf.Putter, mm []CIDMapping) error {
	data := &tounicode.Info{
		Name: "Adobe-Identity-UCS",
		ROS: &type1.CIDSystemInfo{
			Registry:   "Adobe",
			Ordering:   "UCS",
			Supplement: 0,
		},
		CodeSpace: []tounicode.CodeSpaceRange{{First: 0x0000, Last: 0xFFFF}},
	}

	sort.Slice(mm, func(i, j int) bool { return mm[i].CharCode < mm[j].CharCode })

	canDeltaRange := make([]bool, len(mm))
	step := make([]uint16, len(mm))
	var prevDelta int
	var prevCharCode uint16
	for i, m := range mm {
		delta := int(m.Text[0]) - int(m.CharCode)
		charCode := m.CharCode
		if i > 0 {
			canDeltaRange[i] = delta == prevDelta && len(m.Text) == 1
			step[i] = charCode - prevCharCode
		}
		prevDelta = delta
		prevCharCode = charCode
	}

	pos := 0
	for pos < len(mm) {
		next := pos + 1
		for next < len(mm) && canDeltaRange[next] {
			next++
		}
		if next > pos+1 {
			start := mm[pos].CharCode
			end := mm[next-1].CharCode
			bf := tounicode.Range{
				First: cmap.CID(start),
				Last:  cmap.CID(end),
				UTF16: [][]uint16{utf16.Encode(mm[pos].Text)},
			}
			data.Ranges = append(data.Ranges, bf)
			pos = next
			continue
		}

		next = pos + 1
		for next < len(mm) && step[next] < 2 {
			next++
		}
		if next > pos+1 {
			var repl [][]uint16
			for i := pos; i < next; i++ {
				if i > pos && step[i] > 1 {
					for j := 0; j < int(step[i]-1); j++ {
						repl = append(repl, utf16.Encode([]rune{0xFFFD}))
					}
				}
				repl = append(repl, utf16.Encode(mm[i].Text))
			}
			from := mm[pos].CharCode
			to := mm[next-1].CharCode
			bf := tounicode.Range{
				First: cmap.CID(from),
				Last:  cmap.CID(to),
				UTF16: repl,
			}
			data.Ranges = append(data.Ranges, bf)
			pos = next
			continue
		}

		code := mm[pos].CharCode
		data.Singles = append(data.Singles, tounicode.Single{
			Code:  cmap.CID(code),
			UTF16: utf16.Encode(mm[pos].Text),
		})
		pos++
	}

	return data.Embed(ref, w)
}
