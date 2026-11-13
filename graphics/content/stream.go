// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package content

import (
	"errors"
	"fmt"
	"io"
	"iter"
	"math"
	"strconv"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/imagedict"
)

// Stream is an immutable content stream exposed in two views: an
// operator iterator and a byte reader.  Both views describe the same
// content stream and parse to the same operator sequence; the exact byte
// form returned by [Stream.RawBytes] is implementation-dependent (see
// [NewScanner] and [Operators.RawBytes] for the two concrete cases).
//
// Each call to NewIter or RawBytes returns an independent reader.
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

	// RawBytes returns a reader over the stream's content-stream bytes.
	// The bytes do not depend on the PDF version of an output file;
	// use [WriteStream] to write the stream into a PDF file.
	// The caller is responsible for closing the returned reader.
	RawBytes() (io.ReadCloser, error)
}

// Iter is a single-use iterator over content stream operators.
//
// The expected usage is: consume [Iter.All]; check [Iter.Err] for any IO
// error.  The yielded operator stream is raw — no closer synthesis is
// performed by the iterator itself.  Consumers that need to balance
// unclosed contexts (q/Q, BT/ET, BMC/EMC, BX/EX, open paths) should
// drive [State.ClosingOperators] from their own [State] (as
// [seehuhn.de/go/pdf/reader.Reader.ProcessIter] does).  The args slice
// yielded by All may be reused on the next step (the scanner backend
// shares its internal buffer, like [bufio.Scanner.Bytes]); callers that
// need to retain args must clone them.
type Iter interface {
	// All iterates the operators in the stream.
	All() iter.Seq2[OpName, []pdf.Object]

	// Err returns any IO error encountered during iteration.
	// It is always nil for in-memory iterators (e.g. the one returned by
	// [Operators.NewIter]).
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

// streamFactory is the immutable [Stream] implementation that creates independent
// iterators via [streamFactory.NewIter].
type streamFactory struct {
	open func() (io.ReadCloser, error)
}

var _ Stream = (*streamFactory)(nil)

// NewScanner returns a [Stream] that lazily scans a content stream.
// The open function is called each time [Stream.NewIter] creates a new
// iterator, so the returned Stream supports multiple independent iterations.
//
// The yielded operator stream is verbatim — no version filtering, no
// context fix-ups, no missing-resource drops.  Consumers are responsible
// for handling malformed input safely (see e.g.
// [seehuhn.de/go/pdf/reader.Reader], which drops operators rejected by
// [State.ApplyOperator] before dispatch).
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

// RawBytes returns a reader over the scanner's underlying content-stream
// bytes.  The reader follows the same permissive policy as [NewIter]:
// malformed input (corrupt flate, unknown filter, …) yields an empty or
// truncated byte sequence rather than a propagated error.
func (sc *streamFactory) RawBytes() (io.ReadCloser, error) {
	rc, err := sc.open()
	if err != nil {
		if pdf.IsMalformed(err) {
			return io.NopCloser(emptyReader{}), nil
		}
		return nil, err
	}
	return &permissiveReader{rc: rc}, nil
}

// emptyReader is an io.Reader that always returns EOF.
type emptyReader struct{}

func (emptyReader) Read(_ []byte) (int, error) { return 0, io.EOF }

// permissiveReader wraps a content-stream byte reader and converts
// mid-read malformed-input errors into a clean EOF, matching the
// permissive scanner policy in [pumpScanner].
type permissiveReader struct {
	rc  io.ReadCloser
	eof bool
}

func (p *permissiveReader) Read(buf []byte) (int, error) {
	if p.eof {
		return 0, io.EOF
	}
	n, err := p.rc.Read(buf)
	if err != nil && err != io.EOF && pdf.IsMalformed(err) {
		p.eof = true
		return n, io.EOF
	}
	return n, err
}

func (p *permissiveReader) Close() error {
	return p.rc.Close()
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
			if pdf.IsReadError(err) {
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
				if err != nil {
					// end of available input: the token bytes are already
					// read, so finalize it.  Any sticky error (EOF or
					// malformed filter) resurfaces on the next Scan.
					break
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

// inlineImageLength returns the Length entry of an inline image dictionary,
// or -1 if the entry is absent or not a non-negative integer.  A value too
// large for an int32 is returned as [math.MaxInt32], so that the result fits
// an int on all platforms.
func inlineImageLength(dict pdf.Dict) int {
	l, ok := imagedict.InlineValue(dict, "Length").(pdf.Integer)
	switch {
	case !ok || l < 0:
		return -1
	case l > math.MaxInt32:
		return math.MaxInt32
	}
	return int(l)
}

// maxValueDepth limits nesting of arrays and dicts to prevent stack overflow.
const maxValueDepth = 10

// readValueDepth reads one complete PDF value (atomic or composite) from the
// stream.  It handles arrays ([...]) and dictionaries (<<...>>) recursively;
// depth is the number of enclosing arrays and dictionaries.
func (s *scanner) readValueDepth(depth int) (pdf.Object, error) {
	tok, err := s.ScanToken()
	if err != nil {
		return nil, err
	}

	switch tok {
	case pdf.Operator("ID"):
		return nil, errImageData
	case pdf.Operator("EI"):
		return nil, errImageEnd
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

// errImageData and errImageEnd report that the ID or EI operator was found
// where an inline image dictionary expects a key or a value.  Neither error
// leaves [scanner.readInlineImage].
var (
	errImageData = errors.New("ID operator inside inline image dictionary")
	errImageEnd  = errors.New("EI operator inside inline image dictionary")
)

// readInlineImage reads a BI...ID...EI sequence and returns it as a %image%
// pseudo-operator.
//
// The image entries are not checked here: the data is bounded by the length
// of the content stream, and DecodeInlineImage checks the entries before
// decoding it.  An image with a malformed dictionary is dropped, but its data
// is still read, so that it is not parsed as operators.  An EI operator before
// ID ends an image without data.
func (s *scanner) readInlineImage() (Operator, error) {
	dict, err := s.readDictBody("ID", 0)
	if err != nil {
		switch {
		case errors.Is(err, errImageEnd):
			// the EI operator has been consumed
			return Operator{}, parseError{}
		case errors.Is(err, errImageData):
			// the ID operator has been consumed
		case errors.Is(err, parseError{}):
			found, err := s.skipToImageData()
			if err != nil {
				return Operator{}, err
			}
			if !found {
				return Operator{}, parseError{}
			}
		default:
			return Operator{}, err
		}
		if _, err := s.readImageData(nil); err != nil {
			return Operator{}, err
		}
		return Operator{}, parseError{}
	}

	data, err := s.readImageData(dict)
	if err != nil {
		return Operator{}, err
	}
	return Operator{
		Name: OpInlineImage,
		Args: []pdf.Object{dict, pdf.String(data)},
	}, nil
}

// skipToImageData skips the remaining tokens of a malformed inline image
// dictionary, up to and including the ID operator.  It reports whether ID was
// found; the search also stops at EI, which ends an image without data.
func (s *scanner) skipToImageData() (bool, error) {
	for {
		tok, err := s.ScanToken()
		if errors.Is(err, parseError{}) {
			continue
		} else if err != nil {
			return false, err
		}
		switch tok {
		case pdf.Operator("ID"):
			return true, nil
		case pdf.Operator("EI"):
			return false, nil
		}
	}
}

// readImageData reads the data of an inline image which follows the ID
// operator, and consumes the EI operator after it.  dict is the image
// dictionary.  The Length entry of the dictionary is set to the actual length
// of the data, and is added if absent.
//
// Data longer than [maxInlineImageBytes] is skipped, and the image is
// dropped with a parseError.  Scanning then resumes after the data.
func (s *scanner) readImageData(dict pdf.Dict) ([]byte, error) {
	length := inlineImageLength(dict)

	// the ID operator is followed by a single white-space byte
	b, _ := s.Peek()
	spaceAfterID := class[b] == space
	if spaceAfterID {
		s.ReadByte()
	}

	data := []byte{}
	tooLarge := false
	if length > maxInlineImageBytes {
		for range length {
			if _, err := s.ReadByte(); err != nil {
				return nil, err
			}
		}
		s.skipSpaceBytes()
		if s.checkEI() {
			s.SkipN(2)
			return nil, parseError{}
		}
		// The Length entry is wrong: the data ends as for an image without
		// a Length entry.
		tooLarge = true
	} else if length >= 0 {
		// The buffer grows with the data actually read, so that a Length
		// beyond the end of the stream cannot force a large allocation.
		data = make([]byte, 0, min(length, 64<<10))
		for range length {
			b, err := s.ReadByte()
			if err != nil {
				return nil, err
			}
			data = append(data, b)
		}

		// the data is followed by optional white space and EI
		var ws []byte
		for {
			b, err := s.Peek()
			if err != nil || class[b] != space {
				break
			}
			s.SkipByte()
			if len(data)+len(ws) < maxInlineImageBytes {
				ws = append(ws, b)
			} else {
				tooLarge = true
			}
		}
		if s.checkEI() {
			s.SkipN(2)
			setInlineImageLength(dict, len(data))
			return data, nil
		}

		if i := inlineImageEnd(data); i >= 0 {
			if i+2 == len(data) && (len(ws) > 0 || s.atDelimiter()) {
				// The Length entry is too large, but only the EI marker and
				// the white space before it have been read.
				data = data[:max(i-1, 0)]
				setInlineImageLength(dict, len(data))
				return data, nil
			} else if i+2 < len(data) {
				// The Length entry is too large: the data ended at an EI
				// marker already read, and the bytes after the marker have
				// been consumed too.
				return nil, parseError{}
			}
			// An "EI" at the end of data, followed by a regular character,
			// is not an EI marker.
		}
		if len(ws) == 0 && s.endsBeforeEI(data) {
			// The Length entry is too large, but only the white space and
			// "E" of the EI marker have been read.
			data = data[:len(data)-2]
			s.SkipByte()
			setInlineImageLength(dict, len(data))
			return data, nil
		}
		// The Length entry is too small: the data ends as for an image
		// without a Length entry.
		data = append(data, ws...)
	}

	// The data ends at a white-space byte followed by "EI" and a delimiter.
	// The white space after ID counts, so that the data may be empty.
	if !(len(data) == 0 && !tooLarge && spaceAfterID && s.checkEI()) {
		for {
			b, err := s.ReadByte()
			if err != nil {
				return nil, err
			}
			if class[b] == space && s.checkEI() {
				break
			}
			if !tooLarge && len(data) < maxInlineImageBytes {
				data = append(data, b)
			} else {
				tooLarge = true
			}
		}
	}
	s.SkipN(2)
	if tooLarge {
		return nil, parseError{}
	}

	setInlineImageLength(dict, len(data))
	return data, nil
}

// skipSpaceBytes consumes white-space bytes.  Unlike
// [scanner.SkipWhiteSpace], it leaves comments in place.
func (s *scanner) skipSpaceBytes() {
	for {
		b, err := s.Peek()
		if err != nil || class[b] != space {
			return
		}
		s.SkipByte()
	}
}

// setInlineImageLength sets the Length entry of an inline image dictionary
// to n, adding the entry if it is absent.  The entry is what
// [Operator.Format] writes, so that the image reads back unchanged.  Where
// the dictionary gives both L and Length, L takes precedence and the
// redundant Length is dropped.  A nil dict, used for an image which is
// dropped, is left alone.
func setInlineImageLength(dict pdf.Dict, n int) {
	if dict == nil {
		return
	}
	key, ok := imagedict.InlineKey(dict, "Length")
	if !ok {
		key = "L"
	}
	if key == "L" {
		delete(dict, "Length")
	}
	dict[key] = pdf.Integer(n)
}

// endsBeforeEI reports whether data ends with a white-space byte followed by
// "E", and the input continues with "I" and a delimiter, so that an EI marker
// straddles the end of data.
func (s *scanner) endsBeforeEI(data []byte) bool {
	n := len(data)
	if n < 2 || class[data[n-2]] != space || data[n-1] != 'E' {
		return false
	}
	buf := s.PeekN(2)
	return len(buf) > 0 && buf[0] == 'I' && (len(buf) == 1 || class[buf[1]] != regular)
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

// ContainsInlineImageEnd reports whether data, used as the data of an inline
// image without a Length entry, would be taken to end early.  This is the
// case if data contains "EI" followed by a byte other than a regular
// character, or by the end of data, where the "EI" is at the start of data
// or follows a white-space byte.  At the start of data, the white space
// after the ID operator precedes the "EI".
func ContainsInlineImageEnd(data []byte) bool {
	return inlineImageEnd(data) >= 0
}

// inlineImageEnd returns the index of the first "EI" in data which
// [ContainsInlineImageEnd] would take as the end of the data, or -1 if there
// is none.
func inlineImageEnd(data []byte) int {
	for i := 0; i+1 < len(data); i++ {
		if (i == 0 || class[data[i-1]] == space) && data[i] == 'E' && data[i+1] == 'I' &&
			(i+2 == len(data) || class[data[i+2]] != regular) {
			return i
		}
	}
	return -1
}

// atDelimiter reports whether the input continues with a byte other than a
// regular character, or has ended.
func (s *scanner) atDelimiter() bool {
	b, err := s.Peek()
	return err != nil || class[b] != regular
}

// LookingAt checks if the next n bytes match the given string
func (s *scanner) LookingAt(str string) bool {
	buf := s.PeekN(len(str))
	return string(buf) == str
}

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
	// a name cannot contain a null byte, so "#00" stays literal
	if high == 0 && low == 0 {
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
	// maxInlineImageBytes caps the byte length of inline image data.
	maxInlineImageBytes = 16 * 1024 * 1024
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

// class classifies each byte per PDF 2.0 section 7.2.3 (whitespace and
// delimiters).
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
	'/': delimiter,
	'%': delimiter,
}
