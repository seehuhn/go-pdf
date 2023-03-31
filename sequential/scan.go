// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package sequential

import (
	"io"
	"regexp"
	"strconv"
	"strings"
)

type Info struct {
	HeaderVersion string
	XRefBase      int64
	Sections      []*Section
}

type Section struct {
	Objects   []Object
	Start     int64
	XRef      int64
	Trailer   int64
	StartXRef int64
	End       int64
}

type Object struct {
	Pos        int64
	End        int64
	Number     uint32
	Generation uint16
}

func Scan(r io.Reader) (*Info, error) {
	type parserState int
	const (
		start parserState = iota
		inSection
	)
	state := start

	info := &Info{}
	var s *Section
	scanner := newScanner(r, 16*1024, 64)
scanLoop:
	for {
		pos, substr, err := scanner.find(anything)
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		pos += countSpaces(substr[0])

		switch state {
		case start:
			if substr[1] == "" {
				// ignore any material before "%PDF"
				continue
			}
			info.HeaderVersion = substr[1]
			info.XRefBase = pos
			state = inSection

		case inSection:
			if s == nil {
				s = &Section{Start: pos}
			}
			if substr[3] != "" {
				// object
				n, err := strconv.ParseUint(substr[3], 10, 32)
				if err != nil {
					continue scanLoop
				}
				g, err := strconv.ParseUint(substr[4], 10, 16)
				if err != nil {
					continue scanLoop
				}

				s.Objects = append(s.Objects, Object{pos, 0, uint32(n), uint16(g)})
				continue scanLoop
			}

			switch substr[2] {
			case "endobj":
				if len(s.Objects) > 0 && s.Objects[len(s.Objects)-1].End == 0 {
					s.Objects[len(s.Objects)-1].End = pos + int64(len(substr[0]))
				}
			case "xref":
				s.XRef = pos
			case "trailer":
				s.Trailer = pos
			case "startxref":
				s.StartXRef = pos
			case "%%EOF":
				s.End = pos + int64(len(substr[0]))
				info.Sections = append(info.Sections, s)
				s = nil
			}
		}
	}

	return info, nil
}

// isSpace returns true if the byte is a PDF whitespace character.
func isSpace(c byte) bool {
	return c == 0 || c == 9 || c == 10 || c == 12 || c == 13 || c == 32
}

// countSpaces returns the number of leading whitespace characters in s.
func countSpaces(s string) int64 {
	var n int64
	for n < int64(len(s)) && isSpace(s[n]) {
		n++
	}
	return n
}

var (
	whiteSpace = `[\000\011\014 ]+`
	eol        = `(?:\r|\n|\r\n)`
	startPat   = `%PDF-([12]\.[0-9])[^0-9]`
	objectPat  = `([0-9]+)` + whiteSpace + `([0-9]+)` + whiteSpace + `obj`
	markerPat  = eol + `(` + objectPat + `|endobj|xref|trailer|startxref|%%EOF)\b`
	patterns   = []string{startPat, markerPat}
	maxiMompel = "(?:" + strings.Join(patterns, "|") + ")"
	anything   = regexp.MustCompile(maxiMompel)
)
