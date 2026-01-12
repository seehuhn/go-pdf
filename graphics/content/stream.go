// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package content

import (
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"

	"seehuhn.de/go/pdf"
)

// Stream represents a PDF content stream.
//
// Content streams can occur in the following places:
//   - Page contents
//   - Form XObjects
//   - Patterns
//   - Type 3 fonts
//   - Annotation appearances
type Stream []Operator

// Equal determines whether two content streams contain the same sequence of
// operators.
func (s Stream) Equal(other Stream) bool {
	if len(s) != len(other) {
		return false
	}
	for i := range s {
		if !s[i].Equal(other[i]) {
			return false
		}
	}
	return true
}

// Type identifies the type of content stream.
type Type int

const (
	Page              Type = iota // page content stream
	Form                          // Form XObject (includes annotation appearances)
	TransparencyGroup             // transparency group XObject
	PatternColored                // tiling pattern, PaintType 1 (colored)
	PatternUncolored              // tiling pattern, PaintType 2 (uncolored)
	Glyph                         // Type 3 font glyph
)

func (ct Type) String() string {
	switch ct {
	case Page:
		return "page"
	case Form:
		return "form"
	case TransparencyGroup:
		return "transparency group"
	case PatternColored:
		return "pattern (colored)"
	case PatternUncolored:
		return "pattern (uncolored)"
	case Glyph:
		return "glyph"
	default:
		return fmt.Sprintf("Type(%d)", ct)
	}
}

// ReadStream reads a PDF content stream and returns the sequence of operators.
// The version parameter specifies the PDF version to use for validation:
//   - For v <= pdf.MaxVersion, unknown and version-incompatible operators
//     (outside BX/EX compatibility sections) are silently dropped.
//   - For v > pdf.MaxVersion, unknown operators are kept (they may be valid
//     in a newer PDF version).
//
// The content type parameter specifies what kind of content stream is being
// read. This affects which operators are allowed:
//   - Type 3 operators (d0, d1) are only kept for Type3Content.
//
// Parse errors are handled permissively: malformed content is skipped and
// parsing continues. IO errors are returned to the caller.
//
// If the stream has unbalanced state at EOF (unclosed q/Q, BT/ET, BMC/EMC,
// or BX/EX), the missing closing operators are appended automatically.
//
// Operators that are invalid in the current graphics object context are
// either fixed up (text operators get BT auto-inserted) or skipped
// (path operators outside path context).
func ReadStream(r io.Reader, v pdf.Version, ct Type, res *Resources) (Stream, error) {
	s := &streamScanner{
		buf: make([]byte, 512),
	}
	s.src = r

	var stream Stream
	state := NewState(ct, res) // tracks nesting, compatibility, and current object

	for {
		op, err := s.scan()
		switch err {
		case nil:
			// filter based on version validation
			validErr := op.isValidName(v)
			if validErr == ErrUnknown || validErr == ErrVersion {
				if state.InCompatibilitySection() {
					// inside BX/EX: keep unknown operators
				} else if v > pdf.MaxVersion {
					// future PDF version: keep (may be valid)
				} else {
					// known PDF version: drop unknown/version-incompatible
					continue
				}
			}
			// handle deprecated operators by substitution
			if validErr == ErrDeprecated {
				switch op.Name {
				case OpFillCompat:
					op.Name = OpFill // F -> f
				default:
					continue // drop other deprecated operators
				}
			}

			// filter Type 3 operators based on content type
			if ct != Glyph {
				if op.Name == OpType3ColoredGlyph || op.Name == OpType3UncoloredGlyph {
					continue
				}
			}

			// check if operator is allowed in current state and fix up if needed
			if state.CheckOperatorAllowed(op.Name) != nil {
				// not allowed - try to fix up by inserting prefix operators
				if fixOps := fixupOperator(state, op); fixOps != nil {
					for _, fixOp := range fixOps {
						state.ApplyStateChanges(fixOp.Name, fixOp.Args)
						stream = append(stream, fixOp)
					}
					// fall through to process original operator via main path
				} else {
					// can't fix up - skip
					continue
				}
			}

			// skip operators whose required state is not set (e.g., text-showing
			// operators when no font has been set)
			info := operators[op.Name]
			if info != nil && info.Requires&^state.Usable != 0 {
				continue
			}

			// update state and filter improperly nested operators
			if state.ApplyStateChanges(op.Name, op.Args) != nil {
				continue // drop mismatched/unbalanced closing operators
			}

			// update state bits for operators that set new state
			if info != nil && info.Sets != 0 {
				state.Usable |= info.Sets
				state.GState.Set |= info.Sets
			}

			stream = append(stream, op)
		case io.EOF:
			// auto-close open contexts
			stream = append(stream, closeOpenContexts(state)...)
			return stream, nil
		case errParse:
			// permissive: skip malformed content
		default:
			// IO error
			return stream, err
		}
	}
}

// fixupOperator returns prefix operators to insert before op to make its
// context valid, or nil if the operator cannot be fixed up and should be
// skipped. The returned operators do NOT include op itself - the caller
// should process op through the main validation path after applying the
// prefix operators.
func fixupOperator(state *State, op Operator) []Operator {
	info := operators[op.Name]
	if info == nil {
		return nil
	}

	// text operator outside text context: auto-insert BT if allowed
	if info.Allowed == ObjText && state.CurrentObject != ObjText {
		// BT is only allowed in page context
		if state.CurrentObject == ObjPage {
			return []Operator{{Name: OpTextBegin}}
		}
		// can't insert BT in path or other context - skip the operator
		return nil
	}

	// can't fix up other cases
	return nil
}

// closeOpenContexts returns operators to close any open contexts at EOF.
func closeOpenContexts(state *State) []Operator {
	names := state.ClosingOperators()
	ops := make([]Operator, len(names))
	for i, name := range names {
		ops[i] = Operator{Name: name}
	}
	return ops
}

// streamScanner is an internal scanner for content streams
type streamScanner struct {
	line  int // 0-based
	col   int // 0-based
	stack []*scanStackFrame
	args  []pdf.Object

	srcErr error

	src       io.Reader
	buf       []byte
	pos, used int
	crSeen    bool
}

type scanStackFrame struct {
	data   []pdf.Native
	isDict bool
}

// scan reads the next operator from the content stream
func (s *streamScanner) scan() (Operator, error) {
	s.args = s.args[:0]

	// check for comments first
	s.skipWhiteSpaceExceptComments()
	if bb := s.peekN(1); len(bb) > 0 && bb[0] == '%' {
		comment := s.readComment()
		return Operator{
			Name: OpRawContent,
			Args: []pdf.Object{pdf.String(comment)},
		}, nil
	}

tokenLoop:
	for {
		obj, err := s.nextToken()
		if err != nil {
			return Operator{}, err
		}

		switch obj {
		case pdf.Operator("<<"):
			s.stack = append(s.stack, &scanStackFrame{isDict: true})
			continue tokenLoop
		case pdf.Operator(">>"):
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
		case pdf.Operator("["):
			s.stack = append(s.stack, &scanStackFrame{})
			continue tokenLoop
		case pdf.Operator("]"):
			if len(s.stack) == 0 || s.stack[len(s.stack)-1].isDict {
				// unexpected "]"
				continue tokenLoop
			}
			// convert []pdf.Native to pdf.Array ([]pdf.Object)
			arr := make(pdf.Array, len(s.stack[len(s.stack)-1].data))
			for i, elem := range s.stack[len(s.stack)-1].data {
				arr[i] = elem
			}
			obj = arr
			s.stack = s.stack[:len(s.stack)-1]
		}

		if len(s.stack) > 0 { // we are inside a dict or array
			s.stack[len(s.stack)-1].data = append(s.stack[len(s.stack)-1].data, obj)
		} else if op, ok := obj.(pdf.Operator); ok {
			// skip operators with too many arguments
			if len(s.args) >= maxOperatorArgs {
				s.args = s.args[:0]
				continue tokenLoop
			}

			opName := OpName(op)

			// check for BI (inline image)
			if opName == opBeginInlineImage {
				return s.readInlineImage()
			}

			return Operator{Name: opName, Args: slices.Clone(s.args)}, nil
		} else {
			if len(s.args) < maxOperatorArgs {
				s.args = append(s.args, obj)
			}
		}
	}
}

// getInlineImageInt extracts an integer value from an inline image dictionary,
// checking both the abbreviated and full key names.
// Returns -1 if the key is not found or the value is not a valid integer.
func getInlineImageInt(dict pdf.Dict, abbrev, full pdf.Name) int {
	var val pdf.Object
	if v, ok := dict[abbrev]; ok {
		val = v // abbreviated key takes precedence per spec
	} else if v, ok := dict[full]; ok {
		val = v
	}
	if val == nil {
		return -1
	}
	switch v := val.(type) {
	case pdf.Integer:
		return int(v)
	case pdf.Real:
		return int(v)
	}
	return -1
}

// getInlineImageFilter extracts the filter name(s) from an inline image dictionary.
// Returns the final filter in the chain (the one applied last during encoding,
// first during decoding).
func getInlineImageFilter(dict pdf.Dict) pdf.Name {
	var val pdf.Object
	if v, ok := dict["F"]; ok {
		val = v
	} else if v, ok := dict["Filter"]; ok {
		val = v
	}
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case pdf.Name:
		return v
	case pdf.Array:
		// last filter in array is the final/outermost one
		if len(v) > 0 {
			if name, ok := v[len(v)-1].(pdf.Name); ok {
				return name
			}
		}
	}
	return ""
}

// isASCIIFilter returns true if the filter is ASCIIHexDecode or ASCII85Decode.
func isASCIIFilter(filter pdf.Name) bool {
	switch filter {
	case "ASCIIHexDecode", "AHx", "ASCII85Decode", "A85":
		return true
	}
	return false
}

// readInlineImage reads a BI...ID...EI sequence and returns it as a %image% pseudo-operator
func (s *streamScanner) readInlineImage() (Operator, error) {
	// read image dictionary (between BI and ID)
	dict := pdf.Dict{}
	for {
		s.skipWhiteSpace()

		// check if we hit ID
		if s.peekString("ID") {
			s.skipN(2)
			break
		}

		// read key
		b := s.peekN(1)
		if len(b) == 0 {
			return Operator{}, io.EOF
		}
		if b[0] != '/' {
			return Operator{}, errParse
		}
		s.skipN(1)
		key := s.readName()

		// read value
		val, err := s.nextToken()
		if err != nil {
			return Operator{}, err
		}
		if val != nil {
			dict[key] = val
		}
	}

	// validate image dimensions (defense against resource exhaustion)
	width := getInlineImageInt(dict, "W", "Width")
	height := getInlineImageInt(dict, "H", "Height")
	if width <= 0 || height <= 0 || width > maxInlineImageDim || height > maxInlineImageDim {
		return Operator{}, errParse
	}
	if width*height > maxInlineImagePixels {
		return Operator{}, errParse
	}

	// get length (PDF 2.0) and filter
	length := getInlineImageInt(dict, "L", "Length")
	filter := getInlineImageFilter(dict)

	// skip whitespace after ID
	// spec: "the ID operator shall be followed by a single white-space character"
	// for ASCII filters, we may need to skip additional whitespace
	b, _ := s.peek()
	if b <= 32 {
		s.readByte()
	}
	if isASCIIFilter(filter) {
		s.skipWhiteSpace()
	}

	var imageData []byte

	if length > 0 {
		// PDF 2.0: use Length key for efficient reading
		if length > maxInlineImageBytes {
			return Operator{}, errParse
		}
		imageData = make([]byte, length)
		for i := 0; i < length; i++ {
			b, err := s.readByte()
			if err != nil {
				return Operator{}, err
			}
			imageData[i] = b
		}
		// skip optional whitespace before EI
		s.skipWhiteSpace()
	} else {
		// no Length key: read until we find [\r\n]EI pattern
		var prevByte byte
		for len(imageData) < maxInlineImageBytes {
			// check for EI pattern: previous byte is \r or \n, followed by "EI" + delimiter
			if (prevByte == '\r' || prevByte == '\n') && s.checkEI() {
				// remove the trailing newline from image data
				if len(imageData) > 0 {
					imageData = imageData[:len(imageData)-1]
				}
				break
			}

			b, err := s.readByte()
			if err != nil {
				return Operator{}, err
			}
			imageData = append(imageData, b)
			prevByte = b
		}

		if len(imageData) >= maxInlineImageBytes {
			// no valid EI found within limit
			return Operator{}, errParse
		}
	}

	// consume the EI operator
	if !s.peekString("EI") {
		return Operator{}, errParse
	}
	s.skipN(2)

	// verify EI is followed by whitespace, delimiter, or EOF
	nextByte, err := s.peek()
	if err != io.EOF && nextByte > 32 && class[nextByte] != delimiter {
		return Operator{}, errParse
	}

	return Operator{
		Name: OpInlineImage,
		Args: []pdf.Object{dict, pdf.String(imageData)},
	}, nil
}

// checkEI checks if we're at a valid EI terminator.
// Returns true if the next bytes are "EI" followed by whitespace, delimiter, or EOF.
func (s *streamScanner) checkEI() bool {
	buf := s.peekN(3)
	if len(buf) < 2 {
		return false
	}
	if buf[0] != 'E' || buf[1] != 'I' {
		return false
	}
	if len(buf) == 2 {
		return true // EOF after EI is valid
	}
	// EI must be followed by whitespace or delimiter
	return buf[2] <= 32 || class[buf[2]] == delimiter
}

// peekString checks if the next n bytes match the given string
func (s *streamScanner) peekString(str string) bool {
	buf := s.peekN(len(str))
	return string(buf) == str
}

func (s *streamScanner) nextToken() (pdf.Native, error) {
	s.skipWhiteSpace()
	bb := s.peekN(2)
	if len(bb) == 0 {
		return nil, s.srcErr
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
		return pdf.Operator("<<"), nil
	case bb[0] == '<':
		s.skipN(1)
		return s.readHexString()
	case string(bb) == ">>":
		s.skipN(2)
		return pdf.Operator(">>"), nil
	default:
		firstByte := bb[0]
		opBytes := []byte{firstByte}
		s.readByte()
		if class[firstByte] == regular {
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

		if opBytes[0] >= '0' && opBytes[0] <= '9' || opBytes[0] == '.' || opBytes[0] == '-' || opBytes[0] == '+' {
			if x := parseNumber(opBytes); x != nil {
				return x, nil
			}
		}

		switch string(opBytes) {
		case "false":
			return pdf.Boolean(false), nil
		case "true":
			return pdf.Boolean(true), nil
		case "null":
			return nil, nil
		}
		return pdf.Operator(opBytes), nil
	}
}

// readComment reads a comment line and returns it as a byte slice
func (s *streamScanner) readComment() []byte {
	var comment []byte
	for {
		b, err := s.peek()
		if err != nil || b == 10 || b == 13 {
			break
		}
		s.readByte()
		comment = append(comment, b)
	}
	return comment
}

// Reads a PDF string (not including the leading parenthesis).
func (s *streamScanner) readString() (pdf.String, error) {
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

func (s *streamScanner) readHexString() (pdf.String, error) {
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
func (s *streamScanner) readName() pdf.Name {
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

func (s *streamScanner) tryHex() (byte, bool) {
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
func (s *streamScanner) skipWhiteSpace() {
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

// skipWhiteSpaceExceptComments skips whitespace but not comments
func (s *streamScanner) skipWhiteSpaceExceptComments() {
	for {
		b, err := s.peek()
		if err != nil {
			break
		}
		if b <= 32 {
			s.readByte()
		} else {
			break
		}
	}
}

// skipToEOL skips everything up to (but not including) the end of the line.
func (s *streamScanner) skipToEOL() {
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
func (s *streamScanner) readByte() (byte, error) {
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
func (s *streamScanner) peek() (byte, error) {
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
func (s *streamScanner) peekN(n int) []byte {
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
func (s *streamScanner) skipN(n int) {
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
func (s *streamScanner) refill() error {
	if s.srcErr != nil {
		return s.srcErr
	}

	s.used = copy(s.buf, s.buf[s.pos:s.used])
	s.pos = 0

	n, err := s.src.Read(s.buf[s.used:])
	s.used += n
	s.srcErr = err

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
func parseNumber(s []byte) pdf.Native {
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

// limits for defense against resource exhaustion
const (
	maxInlineImageBytes  = 4096       // spec recommendation for inline image data
	maxInlineImagePixels = 256 * 1024 // ~512×512 max
	maxInlineImageDim    = 65536      // max width or height
	maxOperatorArgs      = 64         // PDF spec requires support for 32 DeviceN colorants
)

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
	regular,   // 128-255 (all regular)
	regular, regular, regular, regular, regular, regular, regular, regular,
	regular, regular, regular, regular, regular, regular, regular, regular,
	regular, regular, regular, regular, regular, regular, regular, regular,
	regular, regular, regular, regular, regular, regular, regular, regular,
	regular, regular, regular, regular, regular, regular, regular, regular,
	regular, regular, regular, regular, regular, regular, regular, regular,
	regular, regular, regular, regular, regular, regular, regular, regular,
	regular, regular, regular, regular, regular, regular, regular, regular,
	regular, regular, regular, regular, regular, regular, regular, regular,
	regular, regular, regular, regular, regular, regular, regular, regular,
	regular, regular, regular, regular, regular, regular, regular, regular,
	regular, regular, regular, regular, regular, regular, regular, regular,
	regular, regular, regular, regular, regular, regular, regular, regular,
	regular, regular, regular, regular, regular, regular, regular, regular,
	regular, regular, regular, regular, regular, regular, regular, regular,
}
