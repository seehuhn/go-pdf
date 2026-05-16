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
// The args slice yielded by [Iter.All] is transient: it is only valid
// for the current iteration step (similar to [bufio.Scanner.Bytes]).
// Callers that need to retain args must clone them.
type Iter interface {
	// All returns an iterator over the operators in the stream.
	// The args slice yielded by each step is only valid until the next step.
	All() iter.Seq2[OpName, []pdf.Object]

	// Err returns any IO error encountered during iteration.
	// It is always nil for [Operators].
	Err() error

	// ClosingOperators returns the operator names needed to close any
	// open contexts (unbalanced q/Q, BT/ET, BMC/EMC, BX/EX, or open
	// paths) after iteration has completed.
	// It is always nil for [Operators].
	ClosingOperators() []OpName
}

// StreamsEqual reports whether two Stream values contain the same sequence of
// operators. Both nil -> true; one nil -> false. If both are Operators, the
// comparison uses Operators.Equal for efficiency.
//
// For streams that report [Iter.ClosingOperators] (typically produced by a
// scanner over a file), the closing operators are treated as part of the
// sequence.  This way a stream that ends mid-path compares equal to one
// whose matching close operator is emitted inline — as happens after a
// read/write/read cycle.
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

	// fast path: both are Operators
	aOps, aOk := a.(Operators)
	bOps, bOk := b.(Operators)
	if aOk && bOk {
		return aOps.Equal(bOps)
	}

	// general path: iterate both streams
	itA := a.NewIter()
	itB := b.NewIter()
	nextA, stopA := iter.Pull2(allOps(itA))
	defer stopA()
	nextB, stopB := iter.Pull2(allOps(itB))
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

// allOps yields the operators from [Iter.All], followed by any
// [Iter.ClosingOperators].  This matches the materialisation done by
// [ReadStream].
func allOps(it Iter) iter.Seq2[OpName, []pdf.Object] {
	return func(yield func(OpName, []pdf.Object) bool) {
		for name, args := range it.All() {
			if !yield(name, args) {
				return
			}
		}
		if it.Err() != nil {
			return
		}
		for _, name := range it.ClosingOperators() {
			if !yield(name, nil) {
				return
			}
		}
	}
}

// streamIsEmpty reports whether s yields zero operators, ignoring any
// closing operators that an iterator might contribute.
func streamIsEmpty(s Stream) bool {
	it := s.NewIter()
	for range it.All() {
		return false
	}
	if it.Err() != nil {
		return false
	}
	return len(it.ClosingOperators()) == 0
}

// Operators represents a PDF content stream as a slice of operators.
// It implements both [Stream] and [Iter].
type Operators []Operator

// NewIter returns the Operators value itself as an [Iter].
// Operators has no mutable state, so it can serve as its own iterator.
func (s Operators) NewIter() Iter { return s }

// All returns an iterator over the operators.
func (s Operators) All() iter.Seq2[OpName, []pdf.Object] {
	return func(yield func(OpName, []pdf.Object) bool) {
		for _, op := range s {
			if !yield(op.Name, op.Args) {
				return
			}
		}
	}
}

// Err always returns nil for Operators.
func (s Operators) Err() error {
	return nil
}

// ClosingOperators always returns nil for Operators.
func (s Operators) ClosingOperators() []OpName {
	return nil
}

// Equal determines whether two content streams contain the same sequence of
// operators.
func (s Operators) Equal(other Operators) bool {
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
// or BX/EX), ReadStream appends the missing closing operators.
//
// Operators that are invalid in the current graphics object context are
// either fixed up (text operators get BT auto-inserted) or skipped
// (path operators outside path context).
func ReadStream(open func() (io.ReadCloser, error), v pdf.Version, ct Type, res *Resources) (Operators, error) {
	s := NewScanner(open, v, ct, res)
	it := s.NewIter()
	var ops Operators
	for name, args := range it.All() {
		ops = append(ops, Operator{Name: name, Args: slices.Clone(args)})
	}
	if err := it.Err(); err != nil {
		return nil, err
	}
	for _, name := range it.ClosingOperators() {
		ops = append(ops, Operator{Name: name})
	}
	return ops, nil
}

// streamFactory is the immutable [Stream] implementation that creates independent
// iterators via [streamFactory.NewIter].
type streamFactory struct {
	open    func() (io.ReadCloser, error)
	version pdf.Version
	ct      Type
	res     *Resources
}

// NewScanner returns a [Stream] that lazily scans a content stream.
// The open function is called each time [Stream.NewIter] creates a new
// iterator, so the returned Stream supports multiple independent iterations.
//
// The args yielded by All are transient: they share the scanner's internal
// buffer and are only valid for the current iteration step. Callers that
// need to retain args must clone them.
func NewScanner(open func() (io.ReadCloser, error), v pdf.Version, ct Type, res *Resources) Stream {
	return &streamFactory{
		open:    open,
		version: v,
		ct:      ct,
		res:     res,
	}
}

// NewIter creates a new iterator over the content stream.
func (sc *streamFactory) NewIter() Iter {
	return &scannerIter{
		open:    sc.open,
		version: sc.version,
		ct:      sc.ct,
		res:     sc.res,
	}
}

// scannerIter is a single-use [Iter] that scans operators from a reader.
type scannerIter struct {
	s       *scanner
	state   *State
	version pdf.Version
	ct      Type
	res     *Resources
	err     error
	open    func() (io.ReadCloser, error)
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
		si.s = &scanner{
			buf: make([]byte, 512),
			src: r,
		}
		si.state = NewState(si.ct, si.res)
		si.scanLoop(yield)
	}
}

// Err returns any IO error encountered during iteration.
func (si *scannerIter) Err() error {
	return si.err
}

// ClosingOperators returns the operator names needed to close any open
// contexts after iteration has completed.
func (si *scannerIter) ClosingOperators() []OpName {
	if si.state == nil {
		return nil
	}
	return si.state.ClosingOperators()
}

// scanLoop scans operators from si.s, applying filtering and yielding through
// yield. Returns true if the reader was exhausted normally, false if yield
// returned false or an IO error occurred.
func (si *scannerIter) scanLoop(yield func(OpName, []pdf.Object) bool) bool {
	// save any trailing args from previous reader (for multi-stream pages)
	savedArgs := slices.Clone(si.s.args)
	hasSavedArgs := len(savedArgs) > 0

	for {
		op, err := si.s.Scan()
		switch {
		case err == nil:
			opName := op.Name

			// prepend trailing args from previous reader to first real operator
			if hasSavedArgs && opName != OpRawContent {
				op.Args = append(savedArgs, op.Args...)
				hasSavedArgs = false
			}

			// filter based on version validation
			validErr := opName.isValidName(si.version)
			if validErr == ErrUnknown || validErr == ErrVersion {
				if si.state.InCompatibilitySection() {
					// inside BX/EX: keep unknown operators
				} else if si.version > pdf.MaxVersion {
					// future PDF version: keep (may be valid)
				} else {
					// known PDF version: drop unknown/version-incompatible
					continue
				}
			}
			// handle deprecated operators by substitution
			if validErr == ErrDeprecated {
				switch opName {
				case OpFillCompat:
					opName = OpFill // F -> f
				default:
					continue // drop other deprecated operators
				}
			}

			// filter Type 3 operators based on content type
			if si.ct != Glyph {
				if opName == OpType3ColoredGlyph || opName == OpType3UncoloredGlyph {
					continue
				}
			}

			// check if operator is allowed in current state and fix up if needed
			if si.state.CheckOperatorAllowed(opName) != nil {
				if fixNames := fixupOperatorName(si.state, opName); fixNames != nil {
					for _, fixName := range fixNames {
						si.state.ApplyStateChanges(fixName, nil)
						if !yield(fixName, nil) {
							return false
						}
					}
					// fall through to process original operator via main path
				} else {
					continue
				}
			}

			// skip operators whose required state is not set
			info := operators[opName]
			if info != nil && info.Requires&^si.state.Usable != 0 {
				continue
			}

			// update state and filter improperly nested operators
			if si.state.ApplyStateChanges(opName, op.Args) != nil {
				continue
			}

			// drop Tf operators that reference fonts not in resources
			if opName == OpTextSetFont {
				name, ok := getName(op.Args, 0)
				if !ok || si.res == nil || si.res.Font[name] == nil {
					continue
				}
			}

			// update state bits for operators that set new state
			if info != nil && info.Sets != 0 {
				si.state.Usable |= info.Sets
				si.state.GState.Set |= info.Sets
			}

			if !yield(opName, op.Args) {
				return false
			}
		case errors.Is(err, io.EOF):
			return true
		case errors.Is(err, parseError{}):
			// scanner-level parse error: non-sticky, so we skip the
			// offending token and keep scanning.  Reset the composite
			// stack so that an aborted mid-composite parse does not
			// leave orphan frames that would silently swallow
			// subsequent operator arguments.  This discards any
			// outer composites we were mid-way through too, which is
			// the correct fail-fast-and-resync behaviour.
			si.s.stack = si.s.stack[:0]
		case pdf.IsMalformed(err):
			// filter-level content error (corrupt flate, invalid ASCII85
			// char, …).  These are sticky — the reader will keep returning
			// the same error — so we treat the reader as exhausted.  For
			// [PageScanner] this lets iteration continue into the next
			// stream segment of a multi-stream page rather than discarding
			// the rest of the page.
			return true
		default:
			// real failure (IO, context cancellation, …): propagate.
			si.err = err
			return false
		}
	}
}

// PageScanner scans page content streams that may span multiple PDF stream
// objects. It carries scanner state (graphics state, arg stack) across
// stream boundaries, so that paths and operators split across streams
// are handled correctly.
type PageScanner struct {
	si *scannerIter
}

// NewPageScanner creates a scanner for page content streams.
func NewPageScanner(v pdf.Version, res *Resources) *PageScanner {
	return &PageScanner{
		si: &scannerIter{
			s: &scanner{
				buf: make([]byte, 512),
				src: eofReader{},
			},
			state:   NewState(Page, res),
			version: v,
			ct:      Page,
			res:     res,
		},
	}
}

// SetInitialArgs sets trailing args from a previous content stream segment.
// This must be called before the first ScanReader call.
func (ps *PageScanner) SetInitialArgs(args []pdf.Object) {
	ps.si.s.args = slices.Clone(args)
}

// ScanReader scans operators from r, calling yield for each filtered operator.
// At EOF of r, it returns without emitting closing operators.
// The scanner state and any trailing args carry over for subsequent calls.
// Returns true if the reader was fully consumed, false if yield returned false
// or an IO error occurred.
func (ps *PageScanner) ScanReader(r io.Reader, yield func(OpName, []pdf.Object) bool) bool {
	ps.si.s = &scanner{
		buf:   make([]byte, 512),
		src:   r,
		args:  ps.si.s.args,
		stack: ps.si.s.stack,
	}
	return ps.si.scanLoop(yield)
}

// ClosingOps returns the operators needed to close any open contexts
// (unbalanced q/Q, BT/ET, BMC/EMC, BX/EX, or open paths).
func (ps *PageScanner) ClosingOps() []OpName {
	return ps.si.state.ClosingOperators()
}

// TrailingArgs returns any operator arguments left on the scanner's stack
// after the most recent ScanReader call. These are args that appeared after
// the last operator in a stream, typically because the operator is in the
// next stream segment.
func (ps *PageScanner) TrailingArgs() []pdf.Object {
	return slices.Clone(ps.si.s.args)
}

// Err returns any IO error encountered during scanning.
func (ps *PageScanner) Err() error {
	return ps.si.err
}

type eofReader struct{}

func (eofReader) Read([]byte) (int, error) { return 0, io.EOF }

// fixupOperatorName returns prefix operator names to insert before opName to
// make its context valid, or nil if it cannot be fixed up and should be
// skipped. The returned names do NOT include opName itself.
func fixupOperatorName(state *State, opName OpName) []OpName {
	info := operators[opName]
	if info == nil {
		return nil
	}

	// text operator outside text context: auto-insert BT if allowed
	if info.Allowed == ObjText && state.CurrentObject != ObjText {
		if state.CurrentObject == ObjPage {
			return []OpName{OpTextBegin}
		}
		return nil
	}

	return nil
}

// scanner is an internal scanner for content streams.
//
// Composite values (arrays and dictionaries) are assembled on an explicit
// data stack (s.stack) rather than via recursive readArray/readDict
// calls.  The more obvious recursive design is rejected because
// PageScanner needs to suspend parsing in the middle of an open
// composite at the end of one content stream segment and resume in the
// next.  Per PDF 32000-1 §7.8.2 a /Contents array is parsed as if its
// streams were concatenated, and conforming writers do split clipping
// paths and image objects across streams, so a dict or array may
// legitimately span a stream boundary.  A recursive parser cannot pause
// across that boundary; an explicit stack can simply be carried over by
// PageScanner.ScanReader.
//
// The cost is one error-recovery rule: when a parseError escapes Scan,
// scanLoop must reset s.stack so an aborted mid-composite parse cannot
// poison the next call by silently swallowing its tokens.
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
