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

package scanner

import (
	"errors"
	"io"
	"math"
	"strconv"

	"seehuhn.de/go/pdf"
)

// A Scanner breaks a content stream into tokens.
//
// Parse errors are ignored as much as possible.
type Scanner struct {
	line  int // 0-based
	col   int // 0-based
	stack []*scanStackFrame
	args  []pdf.Object

	// Err is the first error returned by src.Read().
	// Once an error has been returned, all subsequent calls to .refill() will
	// return err.
	err error

	src       io.Reader
	buf       []byte
	pos, used int
	crSeen    bool
}

type scanStackFrame struct {
	data   []pdf.Object
	isDict bool
}

// NewScanner returns a new scanner that reads from r.
func NewScanner() *Scanner {
	return &Scanner{
		buf: make([]byte, 512),
	}
}

// Scan return an iterator over all PDF objects in the content stream.
//
// The []pdf.Object slice passed to the yield function is owned by the scanner
// and is only valid until the yield returns.
func (s *Scanner) Scan(r io.Reader) func(yield func(op string, args []pdf.Object) error) error {
	iterate := func(yield func(string, []pdf.Object) error) error {
		s.err = nil

		// TODO(voss): reset line and col?

		s.src = r
		s.pos = 0
		s.used = 0
		s.crSeen = false

	tokenLoop:
		for {
			obj, err := s.nextToken()
			if err != nil {
				break
			}

			switch obj {
			case operator("<<"):
				s.stack = append(s.stack, &scanStackFrame{isDict: true})
				continue tokenLoop
			case operator(">>"):
				if len(s.stack) == 0 || !s.stack[len(s.stack)-1].isDict {
					// unexpected '>>'
					continue tokenLoop
				}
				entry := s.stack[len(s.stack)-1]
				s.stack = s.stack[:len(s.stack)-1]
				if len(entry.data)%2 != 0 {
					// unexpected '>>'
					continue tokenLoop
				}
				dict := pdf.Dict{}
				for i := 0; i < len(entry.data); i += 2 {
					key, ok := entry.data[i].(pdf.Name)
					if !ok {
						// invalid key
						continue
					}
					val := entry.data[i+1]
					if val == nil {
						continue
					}
					dict[key] = val
				}
				obj = dict
			case operator("["):
				s.stack = append(s.stack, &scanStackFrame{})
				continue tokenLoop
			case operator("]"):
				if len(s.stack) == 0 || s.stack[len(s.stack)-1].isDict {
					// unexpected "]"
					continue tokenLoop
				}
				obj = pdf.Array(s.stack[len(s.stack)-1].data)
				s.stack = s.stack[:len(s.stack)-1]
			}

			if len(s.stack) > 0 { // we are inside a dict or array
				s.stack[len(s.stack)-1].data = append(s.stack[len(s.stack)-1].data, obj)
			} else if op, ok := obj.(operator); ok {
				err := yield(string(op), s.args)
				if err != nil {
					return err
				}
				s.args = s.args[:0]
			} else {
				s.args = append(s.args, obj)
			}
		}

		if s.err == io.EOF {
			if s.col > 0 {
				s.col = 0
				s.line++
			}
			return nil
		}

		return s.err
	}
	return iterate
}

func (s *Scanner) nextToken() (pdf.Object, error) {
	s.skipWhiteSpace()
	bb := s.peekN(2)
	if len(bb) == 0 {
		return nil, s.err
	}

	switch {
	case bb[0] == '/':
		s.skipN(1)
		return s.readName(), nil
	case bb[0] == '(':
		s.skipN(1)
		return s.readString()
	case string(bb) == "<<":
		s.skipN(2)
		return operator("<<"), nil
	case bb[0] == '<':
		s.skipN(1)
		return s.readHexString()
	case string(bb) == ">>":
		s.skipN(2)
		return operator(">>"), nil
	default:
		// TODO(voss): revisit this once
		// https://github.com/pdf-association/pdf-issues/issues/363
		// is resolved.
		opBytes := []byte{bb[0]}
		s.readByte() // skip bb[0] (invalidates bb)
		if class[bb[0]] == regular {
			for {
				b, err := s.peek()
				if err == io.EOF {
					break
				} else if err != nil {
					return nil, err
				}
				if class[b] != regular {
					break
				}
				s.readByte() // skip b
				opBytes = append(opBytes, b)
			}
		}

		if x := parseNumber(opBytes); x != nil {
			return x, nil
		}

		switch string(opBytes) {
		case "false":
			return pdf.Boolean(false), nil
		case "true":
			return pdf.Boolean(true), nil
		case "null":
			return nil, nil
		}
		return operator(opBytes), nil
	}
}

// Reads a PDF string (not including the leading parenthesis).
func (s *Scanner) readString() (pdf.String, error) {
	var res []byte
	bracketLevel := 1
	ignoreLF := false
	for {
		b, err := s.readByte()
		if err != nil {
			return nil, err
		}
		if ignoreLF && b == 10 {
			continue
		}
		ignoreLF = false
		switch b {
		case '(':
			bracketLevel++
			res = append(res, b)
		case ')':
			bracketLevel--
			if bracketLevel == 0 {
				return pdf.String(res), nil
			}
			res = append(res, b)
		case '\\':
			b, err = s.readByte()
			if err != nil {
				return nil, err
			}
			switch b {
			case 'n':
				res = append(res, '\n')
			case 'r':
				res = append(res, '\r')
			case 't':
				res = append(res, '\t')
			case 'b':
				res = append(res, '\b')
			case 'f':
				res = append(res, '\f')
			case '(': // literal (
				res = append(res, '(')
			case ')': // literal )
				res = append(res, ')')
			case '\\': // literal \
				res = append(res, '\\')
			case 10: // LF
				// ignore
			case 13: // CR or CR+LF
				// ignore
				ignoreLF = true
			case '0', '1', '2', '3', '4', '5', '6', '7': // octal
				oct := b - '0'
				for i := 0; i < 2; i++ {
					b, err = s.peek()
					if err == io.EOF {
						break
					} else if err != nil {
						return nil, err
					}
					if b < '0' || b > '7' {
						break
					}
					s.readByte()
					oct = oct*8 + (b - '0')
				}
				res = append(res, oct)
			default:
				res = append(res, b)
			}
		default:
			res = append(res, b)
		}
	}
}

func (s *Scanner) readHexString() (pdf.String, error) {
	var res []byte
	first := true
	var hi byte
readLoop:
	for {
		b, err := s.readByte()
		if err != nil {
			return nil, err
		}
		var lo byte
		switch {
		case b == '>':
			break readLoop
		case b <= 32:
			continue
		case b >= '0' && b <= '9':
			lo = b - '0'
		case b >= 'A' && b <= 'F':
			lo = b - 'A' + 10
		case b >= 'a' && b <= 'f':
			lo = b - 'a' + 10
		default:
			return nil, errParse
		}
		if first {
			hi = lo << 4
			first = false
		} else {
			res = append(res, hi|lo)
			first = true
		}
	}
	if !first {
		res = append(res, hi)
	}

	return pdf.String(res), nil
}

// readName reads a PDF name object (not including the leading slash).
func (s *Scanner) readName() pdf.Name {
	var name []byte
	for {
		b, err := s.peek()
		if err != nil {
			break
		}

		if b == '#' {
			if b, ok := s.tryHex(); ok {
				name = append(name, b)
				continue
			}
			name = append(name, '#')
		} else if class[b] != regular {
			break
		} else {
			name = append(name, b)
		}
		s.readByte()
	}
	return pdf.Name(name)
}

func (s *Scanner) tryHex() (byte, bool) {
	digits := s.peekN(3)
	if len(digits) != 3 {
		return 0, false
	}
	high := hexDigit(digits[1])
	low := hexDigit(digits[2])
	if high == 255 || low == 255 {
		return 0, false
	}
	s.skipN(3)
	return high<<4 | low, true
}

// skipWhiteSpace skips all input (including comments) until a non-whitespace
// character is found.
func (s *Scanner) skipWhiteSpace() {
	for {
		b, err := s.peek()
		if err != nil {
			break
		}
		if b <= 32 {
			s.readByte()
		} else if b == '%' {
			s.skipToEOL()
		} else {
			break
		}
	}
}

// skipToEOL skips everything up to (but not including) the end of the line.
func (s *Scanner) skipToEOL() {
	for {
		b, err := s.peek()
		if b == 10 || b == 13 || err != nil {
			break
		}
		s.readByte()
	}
}

// readByte consumes and returns the next byte of the input stream.
// The function updates the line and column numbers.
func (s *Scanner) readByte() (byte, error) {
	b, err := s.peek()
	if err != nil {
		return 0, err
	}
	s.pos++

	if s.crSeen && b == 10 {
		// LF after CR does not start a new line
	} else if b == 10 || b == 13 {
		s.line++
		s.col = 0
	} else {
		s.col++
	}
	s.crSeen = (b == 13)

	return b, nil
}

// Peek returns the next byte from the input stream without consuming it.
func (s *Scanner) peek() (byte, error) {
	for s.pos >= s.used {
		err := s.refill()
		if err != nil {
			return 0, err
		}
	}
	return s.buf[s.pos], nil
}

// PeekN returns the next n bytes from the input stream without consuming them.
// In case of EOF or of a read error, less than n bytes may be returned.
//
// The returned slice is owned by the scanner and is only valid until the next
// read.
func (s *Scanner) peekN(n int) []byte {
	for s.pos+n > s.used {
		err := s.refill()
		if err != nil {
			break
		}
	}

	a := s.pos
	b := s.pos + n
	if b > s.used {
		b = s.used
	}
	return s.buf[a:b]
}

// skipN consumes n bytes from the input stream.
func (s *Scanner) skipN(n int) {
	for n > 0 {
		if s.pos >= s.used {
			err := s.refill()
			if err != nil {
				break
			}
		}
		if s.pos+n <= s.used {
			s.pos += n
			break
		}
		n -= s.used - s.pos
		s.pos = s.used
	}
}

// refill reads more data from the underlying reader into the buffer.
// This is the only place where the underlying reader is called.
func (s *Scanner) refill() error {
	if s.err != nil {
		return s.err
	}

	s.used = copy(s.buf, s.buf[s.pos:s.used])
	s.pos = 0

	n, err := s.src.Read(s.buf[s.used:])
	s.used += n
	s.err = err

	if n == 0 {
		return err
	}
	return nil
}

func hexDigit(c byte) byte {
	if c >= '0' && c <= '9' {
		return c - '0'
	} else if c >= 'A' && c <= 'F' {
		return c - 'A' + 10
	} else if c >= 'a' && c <= 'f' {
		return c - 'a' + 10
	} else {
		return 255
	}
}

// parseNumber tries to interpret s as a number.
// The function returns [pdf.Integer] or [pdf.Real] in case s is a valid
// number, and nil otherwise.
func parseNumber(s []byte) pdf.Object {
	// TODO(voss): don't use strconv

	x, err := strconv.ParseInt(string(s), 10, 64)
	if err == nil {
		return pdf.Integer(x)
	}

	isSimple := true
	for i, c := range s {
		if i == 0 && (c == '+' || c == '-') {
			continue
		}
		if c == '.' {
			continue
		}
		if c < '0' || c > '9' {
			isSimple = false
			break
		}
	}

	if isSimple {
		y, err := strconv.ParseFloat(string(s), 64)
		if err == nil && !math.IsInf(y, 0) && !math.IsNaN(y) {
			return pdf.Real(y)
		}
	}

	return nil
}

var errParse = errors.New("parse error")

// operator is a PDF operator found in a content stream.
type operator pdf.Name

// PDF implements the [pdf.Object] interface.
func (x operator) PDF(w io.Writer) error {
	_, err := w.Write([]byte(x))
	return err
}

type characterClass byte

const (
	regular characterClass = iota
	space
	delimiter
)

var class = [256]characterClass{
	space,     // 0
	regular,   // 1
	regular,   // 2
	regular,   // 3
	regular,   // 4
	regular,   // 5
	regular,   // 6
	regular,   // 7
	regular,   // 8
	space,     // 9 '\t'
	space,     // 10 '\n'
	regular,   // 11
	space,     // 12 '\f'
	space,     // 13 '\r'
	regular,   // 14
	regular,   // 15
	regular,   // 16
	regular,   // 17
	regular,   // 18
	regular,   // 19
	regular,   // 20
	regular,   // 21
	regular,   // 22
	regular,   // 23
	regular,   // 24
	regular,   // 25
	regular,   // 26
	regular,   // 27
	regular,   // 28
	regular,   // 29
	regular,   // 30
	regular,   // 31
	space,     // 32 ' '
	regular,   // 33 '!'
	regular,   // 34 '"'
	regular,   // 35 '#'
	regular,   // 36 '$'
	delimiter, // 37 '%'
	regular,   // 38 '&'
	regular,   // 39 '\''
	delimiter, // 40 '('
	delimiter, // 41 ')'
	regular,   // 42 '*'
	regular,   // 43 '+'
	regular,   // 44 ','
	regular,   // 45 '-'
	regular,   // 46 '.'
	delimiter, // 47 '/'
	regular,   // 48 '0'
	regular,   // 49 '1'
	regular,   // 50 '2'
	regular,   // 51 '3'
	regular,   // 52 '4'
	regular,   // 53 '5'
	regular,   // 54 '6'
	regular,   // 55 '7'
	regular,   // 56 '8'
	regular,   // 57 '9'
	regular,   // 58 ':'
	regular,   // 59 ';'
	delimiter, // 60 '<'
	regular,   // 61 '='
	delimiter, // 62 '>'
	regular,   // 63 '?'
	regular,   // 64 '@'
	regular,   // 65 'A'
	regular,   // 66 'B'
	regular,   // 67 'C'
	regular,   // 68 'D'
	regular,   // 69 'E'
	regular,   // 70 'F'
	regular,   // 71 'G'
	regular,   // 72 'H'
	regular,   // 73 'I'
	regular,   // 74 'J'
	regular,   // 75 'K'
	regular,   // 76 'L'
	regular,   // 77 'M'
	regular,   // 78 'N'
	regular,   // 79 'O'
	regular,   // 80 'P'
	regular,   // 81 'Q'
	regular,   // 82 'R'
	regular,   // 83 'S'
	regular,   // 84 'T'
	regular,   // 85 'U'
	regular,   // 86 'V'
	regular,   // 87 'W'
	regular,   // 88 'X'
	regular,   // 89 'Y'
	regular,   // 90 'Z'
	delimiter, // 91 '['
	regular,   // 92 '\\'
	delimiter, // 93 ']'
	regular,   // 94 '^'
	regular,   // 95 '_'
	regular,   // 96 '`'
	regular,   // 97 'a'
	regular,   // 98 'b'
	regular,   // 99 'c'
	regular,   // 100 'd'
	regular,   // 101 'e'
	regular,   // 102 'f'
	regular,   // 103 'g'
	regular,   // 104 'h'
	regular,   // 105 'i'
	regular,   // 106 'j'
	regular,   // 107 'k'
	regular,   // 108 'l'
	regular,   // 109 'm'
	regular,   // 110 'n'
	regular,   // 111 'o'
	regular,   // 112 'p'
	regular,   // 113 'q'
	regular,   // 114 'r'
	regular,   // 115 's'
	regular,   // 116 't'
	regular,   // 117 'u'
	regular,   // 118 'v'
	regular,   // 119 'w'
	regular,   // 120 'x'
	regular,   // 121 'y'
	regular,   // 122 'z'
	regular,   // 123 '{'
	regular,   // 124 '|'
	regular,   // 125 '}'
	regular,   // 126 '~'
	regular,   // 127
	regular,   // 128
	regular,   // 129
	regular,   // 130
	regular,   // 131
	regular,   // 132
	regular,   // 133
	regular,   // 134
	regular,   // 135
	regular,   // 136
	regular,   // 137
	regular,   // 138
	regular,   // 139
	regular,   // 140
	regular,   // 141
	regular,   // 142
	regular,   // 143
	regular,   // 144
	regular,   // 145
	regular,   // 146
	regular,   // 147
	regular,   // 148
	regular,   // 149
	regular,   // 150
	regular,   // 151
	regular,   // 152
	regular,   // 153
	regular,   // 154
	regular,   // 155
	regular,   // 156
	regular,   // 157
	regular,   // 158
	regular,   // 159
	regular,   // 160
	regular,   // 161
	regular,   // 162
	regular,   // 163
	regular,   // 164
	regular,   // 165
	regular,   // 166
	regular,   // 167
	regular,   // 168
	regular,   // 169
	regular,   // 170
	regular,   // 171
	regular,   // 172
	regular,   // 173
	regular,   // 174
	regular,   // 175
	regular,   // 176
	regular,   // 177
	regular,   // 178
	regular,   // 179
	regular,   // 180
	regular,   // 181
	regular,   // 182
	regular,   // 183
	regular,   // 184
	regular,   // 185
	regular,   // 186
	regular,   // 187
	regular,   // 188
	regular,   // 189
	regular,   // 190
	regular,   // 191
	regular,   // 192
	regular,   // 193
	regular,   // 194
	regular,   // 195
	regular,   // 196
	regular,   // 197
	regular,   // 198
	regular,   // 199
	regular,   // 200
	regular,   // 201
	regular,   // 202
	regular,   // 203
	regular,   // 204
	regular,   // 205
	regular,   // 206
	regular,   // 207
	regular,   // 208
	regular,   // 209
	regular,   // 210
	regular,   // 211
	regular,   // 212
	regular,   // 213
	regular,   // 214
	regular,   // 215
	regular,   // 216
	regular,   // 217
	regular,   // 218
	regular,   // 219
	regular,   // 220
	regular,   // 221
	regular,   // 222
	regular,   // 223
	regular,   // 224
	regular,   // 225
	regular,   // 226
	regular,   // 227
	regular,   // 228
	regular,   // 229
	regular,   // 230
	regular,   // 231
	regular,   // 232
	regular,   // 233
	regular,   // 234
	regular,   // 235
	regular,   // 236
	regular,   // 237
	regular,   // 238
	regular,   // 239
	regular,   // 240
	regular,   // 241
	regular,   // 242
	regular,   // 243
	regular,   // 244
	regular,   // 245
	regular,   // 246
	regular,   // 247
	regular,   // 248
	regular,   // 249
	regular,   // 250
	regular,   // 251
	regular,   // 252
	regular,   // 253
	regular,   // 254
	regular,   // 255
}
