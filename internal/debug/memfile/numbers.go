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

package memfile

import (
	"bytes"
	"fmt"
	"regexp"
)

// longDecimalRe matches real numbers whose fractional part carries
// implausible float64 artefacts, such as "137.63799999999998".  Such
// numbers indicate computed coordinates or dimensions which have been
// written without rounding.  Two signatures are used: eight or more
// significant fractional digits (admitting legitimate constants like the
// Lab white point, 0.9504559, or calibration ratios like 0.000189394,
// where leading zeros do not count as precision), and a long run of
// zeros followed by another digit -- dust on small magnitudes, such as
// 7.000000000000001 from 0.07*100, which has almost all of its
// significant content before the first nonzero digit.  Shortest-form
// float64 output never ends in zeros, so a genuine zero run is always
// an artefact.
var longDecimalRe = regexp.MustCompile(
	`[0-9]\.0*[1-9][0-9]{7,}` + // too much precision
		`|[0-9]\.[0-9]*0{8,}[0-9]`) // dust hidden behind leading zeros

// maxReports limits how many hits CheckNumbers returns.
const maxReports = 5

// CheckNumbers scans the raw bytes of a PDF file for real numbers with an
// implausibly long fractional part (see [Round]).  It returns a short
// description for each hit, at most maxReports entries.
//
// Binary stream contents are skipped, so compressed or encoded data cannot
// produce spurious hits.  Printable stream contents are scanned, since
// uncompressed content streams contain the operator numbers we want to
// check.
func CheckNumbers(data []byte) []string {
	var res []string
	add := func(region []byte) {
		for _, loc := range longDecimalRe.FindAllIndex(region, -1) {
			lo := loc[0] - 40
			if lo < 0 {
				lo = 0
			}
			hi := loc[1] + 12
			if hi > len(region) {
				hi = len(region)
			}
			res = append(res, fmt.Sprintf("%.60q", region[lo:hi]))
			if len(res) == maxReports {
				return
			}
		}
	}

	plain, streams := splitRegions(data)
	for _, r := range plain {
		add(r)
		if len(res) == maxReports {
			return res
		}
	}
	for _, r := range streams {
		if !printable(r) {
			continue
		}
		add(r)
		if len(res) == maxReports {
			return res
		}
	}
	return res
}

// splitRegions partitions data into the regions outside stream keywords and
// the raw contents of streams.
func splitRegions(data []byte) (plain, streams [][]byte) {
	for i := 0; i < len(data); {
		j := findToken(data, i, "stream")
		if j < 0 {
			plain = append(plain, data[i:])
			return plain, streams
		}
		plain = append(plain, data[i:j])

		k := j + len("stream")
		if k < len(data) && data[k] == '\r' {
			k++
		}
		if k < len(data) && data[k] == '\n' {
			k++
		} else if k == j+len("stream") {
			// no EOL after the keyword: this is not a stream start
			plain = append(plain, data[j:k])
			i = k
			continue
		}

		e := findToken(data, k, "endstream")
		if e < 0 {
			streams = append(streams, data[k:])
			return plain, streams
		}
		streams = append(streams, data[k:e])
		i = e + len("endstream")
	}
	return plain, streams
}

// findToken returns the position of the first occurrence of tok at or after
// pos which is not preceded by an ASCII letter, or -1 if there is none.
func findToken(data []byte, pos int, tok string) int {
	for i := pos; ; {
		j := bytes.Index(data[i:], []byte(tok))
		if j < 0 {
			return -1
		}
		j += i
		if j == 0 || !isLetter(data[j-1]) {
			return j
		}
		i = j + 1
	}
}

func isLetter(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}

// printable reports whether b consists only of printable ASCII characters
// and common whitespace.
func printable(b []byte) bool {
	for _, c := range b {
		if c != '\n' && c != '\r' && c != '\t' && (c < 0x20 || c > 0x7e) {
			return false
		}
	}
	return true
}
