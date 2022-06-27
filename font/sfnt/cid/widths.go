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

package cid

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/funit"
)

func mostFrequent(w []funit.Int16) funit.Int16 {
	hist := make(map[funit.Int16]int)
	for _, wi := range w {
		hist[wi]++
	}

	bestCount := 0
	bestVal := funit.Int16(0)
	for wi, count := range hist {
		if count > bestCount || count == 1000 && count == bestCount {
			bestCount = count
			bestVal = wi
		}
	}
	return bestVal
}

type seq struct {
	start  int
	values []funit.Int16
}

// encodeWidths constructs the /W array for PDF CIDFonts.
// See section 9.7.4.3 of PDF 32000-1:2008 for details.
func encodeWidths(w []funit.Int16, scale float64) (pdf.Integer, pdf.Array) {
	n := len(w)

	dw := mostFrequent(w)

	// remove any occurence of two or more consecutive dw values
	var seqs []*seq
	a := 0
	for {
		for a < n && w[a] == dw {
			a++
		}
		if a >= n {
			break
		}

		b := a + 1
		for b < n && !(w[b] == dw && (b == n-1 || w[b+1] == dw)) {
			b++
		}
		seqs = append(seqs, &seq{
			start:  a,
			values: w[a:b],
		})

		a = b + 2
	}

	// find any runs of length > 4
	var res pdf.Array
	for _, seq := range seqs {
		v := seq.values
		n = len(v)
		a := 0
		i := 0
		for i < n-4 {
			if v[i] != v[i+1] || v[i] != v[i+2] || v[i] != v[i+3] || v[i] != v[i+4] {
				i++
				continue
			}

			if i > a {
				var ww pdf.Array
				for _, wi := range v[a:i] {
					ww = append(ww, wi.AsInteger(scale))
				}
				res = append(res, pdf.Integer(seq.start+a), ww)
			}
			a, i = i, i+4

			for i < n && v[i] == v[a] {
				i++
			}
			res = append(res,
				pdf.Integer(seq.start+a), pdf.Integer(seq.start+i-1), v[a].AsInteger(scale))
			a = i
		}
		if i < n {
			i = n
		}
		if i > a {
			var ww pdf.Array
			for _, wi := range v[a:i] {
				ww = append(ww, wi.AsInteger(scale))
			}
			res = append(res, pdf.Integer(seq.start+a), ww)
		}
	}

	return dw.AsInteger(scale), res
}
