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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/tounicode"
)

// SimpleMapping describes the unicode text corresponding to a character code
// in a simple font.
type SimpleMapping struct {
	CharCode byte
	Text     []rune
}

// WriteToUnicodeSimple writes the ToUnicode stream for a simple font.
// This modifies mm.
func WriteToUnicodeSimple(w *pdf.Writer, ordering string, mm []SimpleMapping, toUnicodeRef *pdf.Reference) error {
	data := &tounicode.Info{
		Name:       "Seehuhn-" + pdf.Name(ordering) + "-UCS2",
		Registry:   pdf.String("Seehuhn"),
		Ordering:   pdf.String(ordering),
		Supplement: 0,
		CodeSpace:  []tounicode.CodeSpaceRange{{First: 0, Last: 255}},
	}

	sort.Slice(mm, func(i, j int) bool { return mm[i].CharCode < mm[j].CharCode })

	canDeltaRange := make([]bool, len(mm))
	step := make([]byte, len(mm))
	var prevDelta int
	var prevCharCode byte
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
			bf := tounicode.Range{
				First: tounicode.CharCode(mm[pos].CharCode),
				Last:  tounicode.CharCode(mm[next-1].CharCode),
				Text:  []string{string(mm[pos].Text)},
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
			var repl []string
			for i := pos; i < next; i++ {
				if i > pos && step[i] > 1 {
					for j := 0; j < int(step[i]-1); j++ {
						repl = append(repl, "\uFFFD")
					}
				}
				repl = append(repl, string(mm[i].Text))
			}
			bf := tounicode.Range{
				First: tounicode.CharCode(mm[pos].CharCode),
				Last:  tounicode.CharCode(mm[next-1].CharCode),
				Text:  repl,
			}
			data.Ranges = append(data.Ranges, bf)
			pos = next
			continue
		}

		data.Singles = append(data.Singles, tounicode.Single{
			Code: tounicode.CharCode(mm[pos].CharCode),
			Text: string(mm[pos].Text),
		})
		pos++
	}

	return writeToUnicodeStream(w, data, toUnicodeRef)
}

// CIDMapping describes the unicode text corresponding to a character code
// in a CIDFont.
type CIDMapping struct {
	CharCode uint16
	Text     []rune
}

// WriteToUnicodeCID writes the ToUnicode stream for a CIDFont.
// This modifies mm.
func WriteToUnicodeCID(w *pdf.Writer, mm []CIDMapping, toUnicodeRef *pdf.Reference) error {
	data := &tounicode.Info{
		CodeSpace:  []tounicode.CodeSpaceRange{{First: 0, Last: 65535}},
		Name:       "Adobe-Identity-UCS",
		Registry:   pdf.String("Adobe"),
		Ordering:   pdf.String("UCS"),
		Supplement: 0,
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
				First: tounicode.CharCode(start),
				Last:  tounicode.CharCode(end),
				Text:  []string{string(mm[pos].Text)},
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
			var repl []string
			for i := pos; i < next; i++ {
				if i > pos && step[i] > 1 {
					for j := 0; j < int(step[i]-1); j++ {
						repl = append(repl, "\uFFFD")
					}
				}
				repl = append(repl, string(mm[i].Text))
			}
			from := mm[pos].CharCode
			to := mm[next-1].CharCode
			bf := tounicode.Range{
				First: tounicode.CharCode(from),
				Last:  tounicode.CharCode(to),
				Text:  repl,
			}
			data.Ranges = append(data.Ranges, bf)
			pos = next
			continue
		}

		code := mm[pos].CharCode
		data.Singles = append(data.Singles, tounicode.Single{
			Code: tounicode.CharCode(code),
			Text: string(mm[pos].Text),
		})
		pos++
	}

	return writeToUnicodeStream(w, data, toUnicodeRef)
}

func writeToUnicodeStream(w *pdf.Writer, data *tounicode.Info, toUnicodeRef *pdf.Reference) error {
	compress := &pdf.FilterInfo{
		Name: pdf.Name("LZWDecode"),
	}
	if w.Version >= pdf.V1_2 {
		compress = &pdf.FilterInfo{Name: pdf.Name("FlateDecode")}
	}
	cmapStream, _, err := w.OpenStream(nil, toUnicodeRef, compress)
	if err != nil {
		return err
	}
	err = data.Write(cmapStream)
	if err != nil {
		return err
	}
	err = cmapStream.Close()
	if err != nil {
		return err
	}
	return nil
}
