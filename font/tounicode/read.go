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

package tounicode

import (
	"bytes"
	"errors"
	"io"
	"regexp"
	"strconv"

	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/pdf"
)

// Read reads and parses a ToUnicode CMap from the given reader.
func Read(r io.Reader) (*Info, error) {
	outer, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	// check that we have a valid CMap
	m := bodyRegexp.FindSubmatch(outer)
	if len(m) != 2 {
		return nil, ErrInvalid
	}
	body := m[1]

	m = typeRegexp.FindSubmatch(body)
	if len(m) == 2 {
		if string(m[1]) != "2" {
			return nil, ErrInvalid
		}
	}

	info := &Info{
		ROS: &type1.CIDSystemInfo{},
	}

	// read the code space ranges
	mm := codespaceRegexp.FindAllSubmatch(body, -1)
	for _, m := range mm {
		inner := m[1]
		for {
			var first, last type1.CID

			inner = skipComments(inner)
			if len(inner) == 0 {
				break
			}

			inner, first, err = parseCharCode(inner)
			if err != nil {
				return nil, err
			}

			inner = skipComments(inner)

			inner, last, err = parseCharCode(inner)
			if err != nil {
				return nil, err
			}

			info.CodeSpace = append(info.CodeSpace, CodeSpaceRange{
				First: first,
				Last:  last,
			})
		}
	}

	// read the bfchar mappings
	mm = bfcharRegexp.FindAllSubmatch(body, -1)
	for _, m := range mm {
		inner := m[1]
		for {
			var code type1.CID
			var rr []uint16

			inner = skipComments(inner)
			if len(inner) == 0 {
				break
			}

			inner, code, err = parseCharCode(inner)
			if err != nil {
				return nil, err
			}

			inner = skipComments(inner)

			inner, rr, err = parseString(inner)
			if err != nil {
				return nil, err
			}

			if info.containsCode(code) {
				info.Singles = append(info.Singles, Single{
					Code:  code,
					UTF16: rr,
				})
			}
		}
	}

	// read the bfrange mappings
	mm = bfrangeRegexp.FindAllSubmatch(body, -1)
	for _, m := range mm {
		inner := m[1]
		for {
			var first, last type1.CID
			var rr []uint16

			inner = skipComments(inner)
			if len(inner) == 0 {
				break
			}

			inner, first, err = parseCharCode(inner)
			if err != nil {
				return nil, err
			}

			inner = skipComments(inner)

			inner, last, err = parseCharCode(inner)
			if err != nil {
				return nil, err
			}

			inner = skipComments(inner)

			nextRange := Range{
				First: first,
				Last:  last,
			}

			m = arrayRegexp.FindSubmatch(inner)
			if m != nil {
				inner = inner[len(m[0]):]
				array := m[1]
				for {
					array = skipComments(array)
					if len(array) == 0 {
						break
					}

					array, rr, err = parseString(array)
					if err != nil {
						return nil, err
					}

					nextRange.UTF16 = append(nextRange.UTF16, rr)
				}
			} else {
				inner, rr, err = parseString(inner)
				if err != nil {
					return nil, err
				}
				nextRange.UTF16 = [][]uint16{rr}
			}

			if nextRange.First <= nextRange.Last && info.containsRange(nextRange.First, nextRange.Last) {
				info.Ranges = append(info.Ranges, nextRange)
			}
		}
	}

	// read meta information
	m = nameRegexp.FindSubmatch(body)
	if len(m) == 2 {
		n, err := pdf.ParseName(m[1])
		if err == nil {
			info.Name = n
		}
	}
	m = registryRegexp.FindSubmatch(body)
	if len(m) == 2 {
		sRaw, err := pdf.ParseString(m[1])
		s := sRaw.AsTextString()
		if err == nil && isValidVCString(s) {
			info.ROS.Registry = s
		}
	}
	m = orderingRegexp.FindSubmatch(body)
	if len(m) == 2 {
		sRaw, err := pdf.ParseString(m[1])
		s := sRaw.AsTextString()
		if err == nil && isValidVCString(s) {
			info.ROS.Ordering = s
		}
	}
	m = supplementRegexp.FindSubmatch(body)
	if len(m) == 2 {
		x, err := strconv.ParseInt(string(m[1]), 10, 32)
		if err == nil {
			info.ROS.Supplement = int32(x)
		}
	}

	return info, nil
}

func isValidVCString(s string) bool {
	// According to CIDFont spec (Adobe tech note #5014):
	// Version control strings (Registry and Ordering) must consist only
	// of alphanumeric ASCII characters and the underscore character.
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r == '_') {
			return false
		}
	}
	return true
}

func skipComments(buf []byte) []byte {
	for {
		m := commentRegexp.FindSubmatch(buf)
		if m == nil {
			return buf
		}
		buf = buf[len(m[0]):]
	}
}

func parseCharCode(buf []byte) ([]byte, type1.CID, error) {
	m := charCodeRegexp.FindSubmatch(buf)
	if m == nil {
		return nil, 0, ErrInvalid
	}

	x, err := strconv.ParseUint(string(m[1]), 16, 32)
	if err != nil {
		return nil, 0, ErrInvalid
	}

	return buf[len(m[0]):], type1.CID(x), nil
}

func parseString(buf []byte) ([]byte, []uint16, error) {
	m := stringRegexp.FindSubmatch(buf)
	if m == nil {
		return nil, nil, ErrInvalid
	}

	q := bytes.ReplaceAll(m[1], []byte{' '}, []byte{})
	if len(q)%4 != 0 {
		return nil, nil, ErrInvalid
	}

	var s []uint16
	for len(q) > 0 {
		x, err := strconv.ParseUint(string(q[:4]), 16, 16)
		if err != nil {
			return nil, nil, ErrInvalid
		}
		s = append(s, uint16(x))
		q = q[4:]
	}

	return buf[len(m[0]):], s, nil
}

var (
	bodyRegexp = regexp.MustCompile(`(?is)\bbegincmap\b\s*(.+?)\s*\bendcmap\b`)
	typeRegexp = regexp.MustCompile(`(?is)/CMapType\b\s*(.+?)\s*\bdef\b`)

	codespaceRegexp = regexp.MustCompile(`(?is)\bbegincodespacerange\b\s*(.*?)\bendcodespacerange\b`)
	bfcharRegexp    = regexp.MustCompile(`(?is)\bbeginbfchar\b\s*(.*?)\bendbfchar\b`)
	bfrangeRegexp   = regexp.MustCompile(`(?is)\bbeginbfrange\b\s*(.*?)\bendbfrange\b`)

	commentRegexp  = regexp.MustCompile(`^%.*?(?:\n|\r)\s*`)
	charCodeRegexp = regexp.MustCompile(`^<([0-9a-fA-F]*)>\s*`)
	stringRegexp   = regexp.MustCompile(`^<([0-9a-fA-F ]*)>\s*`)
	arrayRegexp    = regexp.MustCompile(`^\[(.*?)\]\s*`)

	nameRegexp       = regexp.MustCompile(`(?is)/CMapName\b\s*(/.+?)\s*\bdef\b`)
	registryRegexp   = regexp.MustCompile(`(?is)/CIDSystemInfo\s*<<.*?/Registry\s*(\(.+?\))`)
	orderingRegexp   = regexp.MustCompile(`(?is)/CIDSystemInfo\s*<<.*?/Ordering\s*(\(.+?\))`)
	supplementRegexp = regexp.MustCompile(`(?is)/CIDSystemInfo\s*<<.*?/Supplement\s*([0-9]+)`)
)

// ErrInvalid is returned when a ToUnicode CMap cannot be parsed.
var ErrInvalid = errors.New("invalid ToUnicode CMap")
