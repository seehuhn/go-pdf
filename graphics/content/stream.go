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
	"iter"
	"math"
	"slices"
	"strconv"

	"seehuhn.de/go/pdf"
)

// Stream is an immutable factory for content stream iterators.
// Each call to NewIter creates an independent [Iter] that can be used
// to iterate over the operators in the stream.
//
// Content streams can occur in the following places:
//   - Page contents
//   - Form XObjects
//   - Patterns
//   - Type 3 fonts
//   - Annotation appearances
type Stream interface {
	// NewIter creates a new iterator over the operators in the stream.
	NewIter() Iter
}

// Iter is a single-use iterator over content stream operators.
//
// The expected usage is: consume [Iter.All]; check [Iter.Err] for any IO
// error.  The yielded operator stream is raw — no closer synthesis is
// performed by the iterator itself.  Consumers that need to balance
// unclosed contexts (q/Q, BT/ET, BMC/EMC, BX/EX, open paths) should
// drive [State.ClosingOperators] from their own [State] (as
// [seehuhn.de/go/pdf/reader.Reader.ProcessIter] does).  The args slice
// yielded by All shares the iterator's internal buffer and is only
// valid until the next step (like [bufio.Scanner.Bytes]); callers that
// need to retain args must clone them.
type Iter interface {
	// All iterates the operators in the stream.
	All() iter.Seq2[OpName, []pdf.Object]

	// Err returns any IO error encountered during iteration.
	// It is always nil for [Operators].
	Err() error
}

// StreamsEqual reports whether two Stream values contain the same sequence of
// operators. Both nil -> true; one nil -> false. If both are Operators, the
// comparison uses Operators.Equal for efficiency.
func StreamsEqual(a, b Stream) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil {
		// treat an empty stream as equivalent to nil
		return streamIsEmpty(b)
	}
	if b == nil {
		return streamIsEmpty(a)
	}

	// fast path: both are in-memory operator slices
	aOps, aOk := a.(*Operators)
	bOps, bOk := b.(*Operators)
	if aOk && bOk {
		return aOps.Equal(bOps)
	}

	// general path: iterate both streams
	itA := a.NewIter()
	itB := b.NewIter()
	nextA, stopA := iter.Pull2(itA.All())
	defer stopA()
	nextB, stopB := iter.Pull2(itB.All())
	defer stopB()

	for {
		nameA, argsA, okA := nextA()
		nameB, argsB, okB := nextB()
		if !okA && !okB {
			break
		}
		if okA != okB {
			return false
		}
		if nameA != nameB {
			return false
		}
		if len(argsA) != len(argsB) {
			return false
		}
		for i := range argsA {
			if !pdf.Equal(argsA[i], argsB[i]) {
				return false
			}
		}
	}

	if itA.Err() != nil || itB.Err() != nil {
		return false
	}
	return true
}

// streamIsEmpty reports whether s yields zero operators.
func streamIsEmpty(s Stream) bool {
	it := s.NewIter()
	for range it.All() {
		return false
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

// ReadStream reads a PDF content stream and returns its raw operator
// sequence.  Parse errors in the stream are handled permissively
// (malformed tokens are skipped and parsing continues); IO errors are
// returned to the caller.  No fix-ups, no version filtering, and no
// closer synthesis happen here — see [reader.Reader] for the consumer
// path that adds State-based context handling on top.
func ReadStream(open func() (io.ReadCloser, error)) ([]Operator, error) {
	it := NewScanner(open).NewIter()
	var ops []Operator
	for name, args := range it.All() {
		ops = append(ops, Operator{Name: name, Args: slices.Clone(args)})
	}
	if err := it.Err(); err != nil {
		return nil, err
	}
	return ops, nil
}

// streamFactory is the immutable [Stream] implementation that creates independent
// iterators via [streamFactory.NewIter].
type streamFactory struct {
	open func() (io.ReadCloser, error)
}

// NewScanner returns a [Stream] that lazily scans a content stream.
// The open function is called each time [Stream.NewIter] creates a new
// iterator, so the returned Stream supports multiple independent iterations.
//
// The yielded operator stream is verbatim — no version filtering, no
// context fix-ups, no missing-resource drops.  Consumers are responsible
// for handling malformed input safely (see e.g. [reader.Reader], which
// drops operators rejected by [State.ApplyOperator] before dispatch).
//
// The args yielded by All are transient: they share the scanner's internal
// buffer and are only valid for the current iteration step. Callers that
// need to retain args must clone them.
func NewScanner(open func() (io.ReadCloser, error)) Stream {
	return &streamFactory{open: open}
}

// NewIter creates a new iterator over the content stream.
func (sc *streamFactory) NewIter() Iter {
	return &scannerIter{open: sc.open}
}

// scannerIter is a single-use [Iter] that yields the raw operator
// stream produced by the scanner, without any fix-ups.  Permissive
// scanner-IO recovery (parseError resync, sticky-malformed short-circuit)
// is preserved, but operator content is passed through verbatim.
type scannerIter struct {
	open func() (io.ReadCloser, error)
	err  error
}

// All returns an iterator over the operators in the stream.
// The open function is called to obtain a reader for the content stream.
//
// Permissive-reader policy: errors caused by malformed PDF data (unknown
// filter, corrupt flate stream, unparsable operators, …) yield an empty
// or truncated iteration with Err reporting nil.  Any other error — real
// IO failures from the underlying byte source, context cancellations,
// programmer errors — is reported via Err so callers can distinguish a
// read failure from a malformed PDF.  See the package-level error model
// in [pdf] for the classification.
func (si *scannerIter) All() iter.Seq2[OpName, []pdf.Object] {
	return func(yield func(OpName, []pdf.Object) bool) {
		r, err := si.open()
		if err != nil {
			if !pdf.IsMalformed(err) {
				si.err = err
			}
			return
		}
		defer r.Close()
		s := &scanner{
			buf: make([]byte, 512),
			src: r,
		}
		_, err = pumpScanner(s, yield)
		if err != nil {
			si.err = err
		}
	}
}

// Err returns any IO error encountered during iteration.
func (si *scannerIter) Err() error {
	return si.err
}

// pumpScanner drains s, yielding raw scanner operators to yield.
// Returns (ok, err): ok is false if yield aborted or a non-malformed IO
// error occurred; err is the captured IO error (nil for malformed-data
// errors and EOF).
func pumpScanner(s *scanner, yield func(OpName, []pdf.Object) bool) (bool, error) {
	for {
		op, err := s.Scan()
		switch {
		case err == nil:
			if !yield(op.Name, op.Args) {
				return false, nil
			}
		case errors.Is(err, io.EOF):
			return true, nil
		case errors.Is(err, parseError{}):
			// scanner-level parse error: non-sticky, so we skip the
			// offending token and keep scanning.  Reset the composite
			// stack so that an aborted mid-composite parse does not
			// leave orphan frames that would silently swallow
			// subsequent operator arguments.  This discards any
			// outer composites we were mid-way through too, which is
			// the correct fail-fast-and-resync behaviour.
			s.stack = s.stack[:0]
		case pdf.IsMalformed(err):
			// filter-level content error (corrupt flate, invalid ASCII85
			// char, …).  These are sticky — the reader will keep returning
			// the same error — so we treat the reader as exhausted.
			return true, nil
		default:
			// real failure (IO, context cancellation, …): surface
			// the error to the caller.
			return false, err
		}
	}
}

// scanner is an internal scanner for content streams.
//
// Composite values (arrays and dictionaries) are assembled on an explicit
// data stack (s.stack) rather than via recursive readArray/readDict
// calls.  The stack persists across [scanner.Scan] calls so that an
// unterminated composite at parseError-recovery time can be reset
// without losing subsequent tokens to dangling frames; see the
// parseError branch in [pumpScanner].
type scanner struct {
	Line  int // 0-based
	Col   int // 0-based
	stack []*scanStackFrame
	args  []pdf.Object

	err error

	src       io.Reader
	buf       []byte
	pos, used int

	tokenBuf []byte // reusable buffer for nextToken
	crSeen   bool
}

type scanStackFrame struct {
	data   []pdf.Object
	isDict bool
}

// Scan reads the next operator from the content stream
func (s *scanner) Scan() (Operator, error) {
	s.args = s.args[:0]

	// check for comments first
	if err := s.skipWhiteSpaceExceptComments(); err != nil {
		return Operator{}, err
	}
	if bb := s.PeekN(1); len(bb) > 0 && bb[0] == '%' {
		comment, err := s.ReadComment()
		if err != nil {
			return Operator{}, err
		}
		return Operator{
			Name: OpRawContent,
			Args: []pdf.Object{pdf.String(comment)},
		}, nil
	}

tokenLoop:
	for {
		obj, err := s.ScanToken()
		if err != nil {
			return Operator{}, err
		}

		switch obj {
		case pdf.Operator("<<"):
			if len(s.stack) >= maxContentNestDepth {
				return Operator{}, parseError{}
			}
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
			if len(s.stack) >= maxContentNestDepth {
				return Operator{}, parseError{}
			}
			s.stack = append(s.stack, &scanStackFrame{})
			continue tokenLoop
		case pdf.Operator("]"):
			if len(s.stack) == 0 || s.stack[len(s.stack)-1].isDict {
				// unexpected "]"
				continue tokenLoop
			}
			// avoid pdf.Array(nil) for "[]"; pdf.Format would emit it as "null"
			arr := pdf.Array(s.stack[len(s.stack)-1].data)
			if arr == nil {
				arr = pdf.Array{}
			}
			obj = arr
			s.stack = s.stack[:len(s.stack)-1]
		}

		if len(s.stack) > 0 { // we are inside a dict or array
			top := s.stack[len(s.stack)-1]
			limit := maxArrayLen
			if top.isDict {
				// data holds interleaved keys+values, so 2× the entry cap
				limit = 2 * maxDictLen
			}
			if len(top.data) >= limit {
				return Operator{}, parseError{}
			}
			top.data = append(top.data, obj)
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

			return Operator{Name: opName, Args: s.args}, nil
		} else {
			if len(s.args) < maxOperatorArgs {
				s.args = append(s.args, obj)
			}
		}
	}
}

func (s *scanner) ScanToken() (pdf.Native, error) {
	if err := s.SkipWhiteSpace(); err != nil {
		return nil, err
	}
	bb := s.PeekN(2)
	if len(bb) == 0 {
		return nil, s.err
	}

	switch {
	case bb[0] == '/':
		s.SkipByte()
		return s.ReadName()
	case bb[0] == '(':
		s.SkipByte()
		return s.ReadString()
	case string(bb) == "<<":
		s.SkipN(2)
		return pdf.Operator("<<"), nil
	case bb[0] == '<':
		s.SkipByte()
		return s.ReadHexString()
	case string(bb) == ">>":
		s.SkipN(2)
		return pdf.Operator(">>"), nil
	default:
		firstByte := bb[0]
		s.tokenBuf = append(s.tokenBuf[:0], firstByte)
		s.ReadByte()
		overflow := false
		if class[firstByte] == regular {
			for {
				b, err := s.Peek()
				if err == io.EOF {
					break
				} else if err != nil {
					return nil, err
				}
				if class[b] != regular {
					break
				}
				s.ReadByte() // skip b
				if len(s.tokenBuf) < maxNameBytes {
					s.tokenBuf = append(s.tokenBuf, b)
				} else {
					overflow = true
				}
			}
		}
		if overflow {
			return nil, parseError{}
		}

		if s.tokenBuf[0] >= '0' && s.tokenBuf[0] <= '9' || s.tokenBuf[0] == '.' || s.tokenBuf[0] == '-' || s.tokenBuf[0] == '+' {
			if x := parseNumber(s.tokenBuf); x != nil {
				return x, nil
			}
		}

		switch string(s.tokenBuf) {
		case "false":
			return pdf.Boolean(false), nil
		case "true":
			return pdf.Boolean(true), nil
		case "null":
			return nil, nil
		}
		if op, ok := operatorTable[string(s.tokenBuf)]; ok {
			return op, nil
		}
		return pdf.Operator(s.tokenBuf), nil
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

// readValue reads one complete PDF value (atomic or composite) from the stream.
// It handles arrays ([...]) and dictionaries (<<...>>) recursively.
func (s *scanner) readValue() (pdf.Object, error) {
	return s.readValueDepth(0)
}

// maxValueDepth limits nesting of arrays and dicts to prevent stack overflow.
const maxValueDepth = 10

func (s *scanner) readValueDepth(depth int) (pdf.Object, error) {
	tok, err := s.ScanToken()
	if err != nil {
		return nil, err
	}

	switch tok {
	case pdf.Operator("["):
		if depth >= maxValueDepth {
			return nil, parseError{}
		}
		var arr pdf.Array
		for {
			if err := s.SkipWhiteSpace(); err != nil {
				return nil, err
			}
			if s.LookingAt("]") {
				s.SkipByte()
				return arr, nil
			}
			if len(arr) >= maxArrayLen {
				return nil, parseError{}
			}
			elem, err := s.readValueDepth(depth + 1)
			if err != nil {
				return nil, err
			}
			arr = append(arr, elem)
		}
	case pdf.Operator("<<"):
		if depth >= maxValueDepth {
			return nil, parseError{}
		}
		return s.readDictBody(">>", depth+1)
	}

	return tok, nil
}

// readDictBody reads key-value pairs until term is encountered, consuming
// term.  Each key and value is read via readValueDepth(valueDepth), so
// callers control whether the entries count against the depth budget.
// Used by both the <<...>> branch of readValueDepth and by readInlineImage
// (which uses "ID" as its terminator).
func (s *scanner) readDictBody(term string, valueDepth int) (pdf.Dict, error) {
	dict := pdf.Dict{}
	for {
		if err := s.SkipWhiteSpace(); err != nil {
			return nil, err
		}
		if s.LookingAt(term) {
			s.SkipN(len(term))
			return dict, nil
		}
		// read key (must be a name)
		keyObj, err := s.readValueDepth(valueDepth)
		if err != nil {
			return nil, err
		}
		key, ok := keyObj.(pdf.Name)
		if !ok {
			return nil, parseError{}
		}
		// read value
		val, err := s.readValueDepth(valueDepth)
		if err != nil {
			return nil, err
		}
		if val != nil {
			if _, exists := dict[key]; !exists && len(dict) >= maxDictLen {
				return nil, parseError{}
			}
			dict[key] = val
		}
	}
}

// readInlineImage reads a BI...ID...EI sequence and returns it as a %image% pseudo-operator
func (s *scanner) readInlineImage() (Operator, error) {
	// read image dictionary (between BI and ID)
	dict, err := s.readDictBody("ID", 0)
	if err != nil {
		return Operator{}, err
	}

	// validate image dimensions (defense against resource exhaustion)
	width := getInlineImageInt(dict, "W", "Width")
	height := getInlineImageInt(dict, "H", "Height")
	if width <= 0 || height <= 0 || width > maxInlineImageDim || height > maxInlineImageDim {
		return Operator{}, parseError{}
	}
	if width*height > maxInlineImagePixels {
		return Operator{}, parseError{}
	}

	// get length (PDF 2.0) and filter
	length := getInlineImageInt(dict, "L", "Length")
	filter := getInlineImageFilter(dict)

	// skip whitespace after ID
	// spec: "the ID operator shall be followed by a single white-space character"
	// for ASCII filters, we may need to skip additional whitespace
	b, _ := s.Peek()
	if class[b] == space {
		s.ReadByte()
	}
	if isASCIIFilter(filter) {
		if err := s.SkipWhiteSpace(); err != nil {
			return Operator{}, err
		}
	}

	var imageData []byte

	if length > 0 {
		// PDF 2.0: use Length key for efficient reading
		if length > maxInlineImageBytes {
			return Operator{}, parseError{}
		}
		imageData = make([]byte, length)
		for i := range length {
			b, err := s.ReadByte()
			if err != nil {
				return Operator{}, err
			}
			imageData[i] = b
		}
		// skip optional whitespace before EI
		if err := s.SkipWhiteSpace(); err != nil {
			return Operator{}, err
		}
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

			b, err := s.ReadByte()
			if err != nil {
				return Operator{}, err
			}
			imageData = append(imageData, b)
			prevByte = b
		}

		if len(imageData) >= maxInlineImageBytes {
			// no valid EI found within limit
			return Operator{}, parseError{}
		}
	}

	// consume the EI operator
	if err := s.SkipString("EI"); err != nil {
		return Operator{}, err
	}

	// verify EI is followed by whitespace, delimiter, or EOF
	nextByte, err := s.Peek()
	if err != io.EOF && class[nextByte] == regular {
		return Operator{}, parseError{}
	}

	return Operator{
		Name: OpInlineImage,
		Args: []pdf.Object{dict, pdf.String(imageData)},
	}, nil
}

// checkEI checks if we're at a valid EI terminator.
// Returns true if the next bytes are "EI" followed by whitespace, delimiter, or EOF.
func (s *scanner) checkEI() bool {
	buf := s.PeekN(3)
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
	return class[buf[2]] != regular
}

// LookingAt checks if the next n bytes match the given string
func (s *scanner) LookingAt(str string) bool {
	buf := s.PeekN(len(str))
	return string(buf) == str
}

const (
	maxInlineImageBytes  = 4096       // spec recommendation for inline image data
	maxInlineImagePixels = 256 * 1024 // ~512×512 max
	maxInlineImageDim    = 65536      // max width or height
)

// ReadComment reads a comment line and returns it as a byte slice.
// A comment longer than maxNameBytes is rejected with parseError; the
// surplus bytes up to the line terminator are drained first so the
// scanner resyncs at the start of the next line.
func (s *scanner) ReadComment() ([]byte, error) {
	var comment []byte
	overflow := false
	for {
		b, err := s.Peek()
		if err != nil || b == 10 || b == 13 {
			break
		}
		s.ReadByte()
		if len(comment) < maxNameBytes {
			comment = append(comment, b)
		} else {
			overflow = true
		}
	}
	if overflow {
		return nil, parseError{}
	}
	return comment, nil
}

// Reads a PDF string (not including the leading parenthesis).
func (s *scanner) ReadString() (pdf.String, error) {
	var res []byte
	bracketLevel := 1
	ignoreLF := false
	for {
		if len(res) >= maxStringBytes {
			return nil, parseError{}
		}
		b, err := s.ReadByte()
		if err != nil {
			return nil, err
		}
		if ignoreLF {
			ignoreLF = false
			if b == 10 {
				continue
			}
		}
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
			b, err = s.ReadByte()
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
			case 10: // LF
				// line continuation
			case 13: // CR or CR+LF
				// line continuation; ignore an immediately following LF
				ignoreLF = true
			case '0', '1', '2', '3', '4', '5', '6', '7': // octal
				oct := b - '0'
				for range 2 {
					b, err = s.Peek()
					if err == io.EOF {
						break
					} else if err != nil {
						return nil, err
					}
					if b < '0' || b > '7' {
						break
					}
					s.ReadByte()
					oct = oct*8 + (b - '0')
				}
				res = append(res, oct)
			default:
				res = append(res, b)
			}
		case 13: // unescaped CR or CR+LF, normalised to LF per PDF 7.3.4.2
			res = append(res, '\n')
			ignoreLF = true
		default:
			res = append(res, b)
		}
	}
}

func (s *scanner) ReadHexString() (pdf.String, error) {
	var res []byte
	first := true
	var hi byte
readLoop:
	for {
		b, err := s.ReadByte()
		if err != nil {
			return nil, err
		}
		var lo byte
		switch {
		case b == '>':
			break readLoop
		case class[b] == space:
			continue
		case b >= '0' && b <= '9':
			lo = b - '0'
		case b >= 'A' && b <= 'F':
			lo = b - 'A' + 10
		case b >= 'a' && b <= 'f':
			lo = b - 'a' + 10
		default:
			return nil, parseError{}
		}
		if first {
			hi = lo << 4
			first = false
		} else {
			if len(res) >= maxStringBytes {
				return nil, parseError{}
			}
			res = append(res, hi|lo)
			first = true
		}
	}
	if !first {
		res = append(res, hi)
	}

	return pdf.String(res), nil
}

// ReadName reads a PDF name object (not including the leading slash).
// Names longer than maxNameBytes are rejected with parseError; the
// surplus regular bytes are drained first so the scanner resyncs at
// the next token boundary when the caller recovers.
func (s *scanner) ReadName() (pdf.Name, error) {
	var name []byte
	overflow := false
	for {
		b, err := s.Peek()
		if err != nil {
			break
		}
		if class[b] != regular {
			break
		}

		var c byte
		if b == '#' {
			if h, ok := s.tryHex(); ok {
				c = h
			} else {
				c = '#'
				s.ReadByte()
			}
		} else {
			c = b
			s.ReadByte()
		}
		if len(name) < maxNameBytes {
			name = append(name, c)
		} else {
			overflow = true
		}
	}
	if overflow {
		return "", parseError{}
	}
	return pdf.Name(name), nil
}

func (s *scanner) tryHex() (byte, bool) {
	digits := s.PeekN(3)
	if len(digits) != 3 {
		return 0, false
	}
	high := hexDigit(digits[1])
	low := hexDigit(digits[2])
	if high == 255 || low == 255 {
		return 0, false
	}
	s.SkipN(3)
	return high<<4 | low, true
}

// SkipWhiteSpace skips all input (including comments) until a non-whitespace
// character is found.
func (s *scanner) SkipWhiteSpace() error {
	for {
		b, err := s.Peek()
		if err != nil {
			return err
		}
		if class[b] == space {
			s.SkipByte()
		} else if b == '%' {
			s.SkipToEOL()
		} else {
			return nil
		}
	}
}

// skipWhiteSpaceExceptComments skips whitespace but not comments.
func (s *scanner) skipWhiteSpaceExceptComments() error {
	for {
		b, err := s.Peek()
		if err != nil {
			return err
		}
		if class[b] == space {
			s.SkipByte()
		} else {
			return nil
		}
	}
}

// SkipToEOL skips everything up to (but not including) the end of the line.
func (s *scanner) SkipToEOL() {
	for {
		b, err := s.Peek()
		if b == 10 || b == 13 || err != nil {
			break
		}
		s.ReadByte()
	}
}

// ReadByte consumes and returns the next byte of the input stream.
// The function updates the line and column numbers.
func (s *scanner) ReadByte() (byte, error) {
	b, err := s.Peek()
	if err != nil {
		return 0, err
	}
	s.pos++

	if s.crSeen && b == 10 {
		// LF after CR does not start a new line
	} else if b == 10 || b == 13 {
		s.Line++
		s.Col = 0
	} else {
		s.Col++
	}
	s.crSeen = (b == 13)

	return b, nil
}

// Peek returns the next byte from the input stream without consuming it.
func (s *scanner) Peek() (byte, error) {
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
func (s *scanner) PeekN(n int) []byte {
	for s.pos+n > s.used {
		err := s.refill()
		if err != nil {
			break
		}
	}

	a := s.pos
	b := min(s.pos+n, s.used)
	return s.buf[a:b]
}

// SkipN consumes n bytes from the input stream.
func (s *scanner) SkipN(n int) {
	for range n {
		s.ReadByte()
	}
}

// SkipByte consumes a single byte from the input.
func (s *scanner) SkipByte() {
	s.ReadByte()
}

// SkipString consumes pat from the input. It returns parseError if the next
// bytes do not match pat, or the underlying read error if input is exhausted
// before pat can be checked.
func (s *scanner) SkipString(pat string) error {
	buf := s.PeekN(len(pat))
	if string(buf) == pat {
		s.SkipN(len(pat))
		return nil
	}
	if len(buf) < len(pat) && s.err != nil {
		return s.err
	}
	return parseError{}
}

// refill reads more data from the underlying reader into the buffer.
// This is the only place where the underlying reader is called.
func (s *scanner) refill() error {
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
func parseNumber(s []byte) pdf.Native {
	hasDecimal := false
	isSimple := true
	for i, c := range s {
		if i == 0 && (c == '+' || c == '-') {
			continue
		}
		if c == '.' {
			hasDecimal = true
			continue
		}
		if c < '0' || c > '9' {
			isSimple = false
			break
		}
	}
	if !isSimple {
		return nil
	}

	str := string(s)

	if !hasDecimal {
		x, err := strconv.ParseInt(str, 10, 64)
		if err == nil {
			return pdf.Integer(x)
		}
	}

	y, err := strconv.ParseFloat(str, 64)
	if err == nil && !math.IsInf(y, 0) && !math.IsNaN(y) {
		return pdf.Real(y)
	}

	return nil
}

// parseError marks a scanner-level content-stream parse failure.  These
// errors never escape the scanner: [scannerIter.scanLoop] recognises them,
// skips the offending token, and keeps scanning.  A fresh empty-struct
// value is returned at each call site — there is no shared state, and no
// mutable global [*pdf.MalformedFileError] that [pdf.Wrap] could accidentally
// modify.
type parseError struct{}

func (parseError) Error() string { return "content stream parse error" }

// limits for defense against resource exhaustion
const (
	maxOperatorArgs     = 64  // PDF spec requires support for 32 DeviceN colorants
	maxContentNestDepth = 256 // cap on `[`/`<<` nesting in scan()
)

// Per-object size bounds, declared as variables so tests can substitute
// smaller values when exercising the bound checks.  In production, all
// are set well above any value seen in legitimate PDF content streams.
var (
	// maxStringBytes caps the byte length of string and hex-string objects.
	maxStringBytes = 16 * 1024 * 1024
	// maxNameBytes caps the byte length of name, operator, and number
	// tokens.  PDF 1.7 requires at least 127 bytes for names.
	maxNameBytes = 4096
	// maxArrayLen caps the number of entries in an array.
	maxArrayLen = 1 << 20
	// maxDictLen caps the number of entries in a dictionary.
	maxDictLen = 64 << 10
)

type characterClass byte

const (
	regular characterClass = iota
	space
	delimiter
)

// operatorTable maps operator names to pre-allocated pdf.Operator values,
// avoiding a []byte to string conversion on every call.
var operatorTable = map[string]pdf.Operator{
	"b": "b", "B": "B", "b*": "b*", "B*": "B*", "BDC": "BDC",
	"BI": "BI", "BMC": "BMC", "BT": "BT", "BX": "BX",
	"c": "c", "cm": "cm", "CS": "CS", "cs": "cs",
	"d": "d", "d0": "d0", "d1": "d1", "Do": "Do", "DP": "DP",
	"EI": "EI", "EMC": "EMC", "ET": "ET", "EX": "EX",
	"f": "f", "F": "F", "f*": "f*",
	"G": "G", "g": "g", "gs": "gs",
	"h": "h",
	"i": "i", "ID": "ID",
	"j": "j", "J": "J",
	"K": "K", "k": "k",
	"l": "l",
	"m": "m", "M": "M", "MP": "MP",
	"n": "n",
	"q": "q", "Q": "Q",
	"re": "re", "RG": "RG", "rg": "rg", "ri": "ri",
	"s": "s", "S": "S",
	"SC": "SC", "sc": "sc", "SCN": "SCN", "scn": "scn", "sh": "sh",
	"T*": "T*", "Tc": "Tc", "Td": "Td", "TD": "TD", "Tf": "Tf",
	"Tj": "Tj", "TJ": "TJ", "TL": "TL", "Tm": "Tm", "Tr": "Tr",
	"Ts": "Ts", "Tw": "Tw", "Tz": "Tz",
	"v": "v",
	"w": "w", "W": "W", "W*": "W*",
	"y": "y",
	"'": "'", "\"": "\"",
}

// class classifies each byte per PDF 7.2.3 (whitespace + delimiters).
// regular is the zero value, so unlisted bytes are regular by default.
var class = [256]characterClass{
	0:   space,
	9:   space,
	10:  space,
	12:  space,
	13:  space,
	32:  space,
	'(': delimiter,
	')': delimiter,
	'<': delimiter,
	'>': delimiter,
	'[': delimiter,
	']': delimiter,
	'{': delimiter,
	'}': delimiter,
	'/': delimiter,
	'%': delimiter,
}
