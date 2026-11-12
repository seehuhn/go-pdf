// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package content

import (
	"bytes"
	"io"
	"iter"

	"seehuhn.de/go/pdf"
)

// Operators is a sequence of PDF content stream operators.
// This implements both [Stream] and [seehuhn.de/go/pdf/page.Segment].
type Operators struct {
	Ops []Operator
}

var _ Stream = (*Operators)(nil)

// Equal reports whether two streams contain the same operator sequence.
// Two nil values compare equal; nil vs non-nil are unequal.
func (m *Operators) Equal(other *Operators) bool {
	if m == nil && other == nil {
		return true
	}
	if m == nil || other == nil {
		return false
	}
	if len(m.Ops) != len(other.Ops) {
		return false
	}
	for i := range m.Ops {
		if !m.Ops[i].Equal(other.Ops[i]) {
			return false
		}
	}
	return true
}

// NewIter returns an iterator over the individual operators.
//
// This is used to implement the [Stream] interface.
func (m *Operators) NewIter() Iter {
	return &operatorsIter{ops: m.Ops}
}

// operatorsIter is the single-use [Iter] returned by [Operators.NewIter].
type operatorsIter struct {
	ops []Operator
}

func (mi *operatorsIter) All() iter.Seq2[OpName, []pdf.Object] {
	return func(yield func(OpName, []pdf.Object) bool) {
		for _, op := range mi.ops {
			if !yield(op.Name, op.Args) {
				return
			}
		}
	}
}

func (mi *operatorsIter) Err() error { return nil }

// RawBytes returns a reader over the serialized operator sequence, in PDF 1.2
// and later syntax.  Use [WriteStream] to write the operators for a specific
// output file.
//
// The reader serialises operators lazily: peak transient memory is
// bounded by the largest single operator's serialised size.
//
// This is used to implement both the [Stream]
// and the [seehuhn.de/go/pdf/page.Segment] interfaces.
func (m *Operators) RawBytes() (io.ReadCloser, error) {
	return &operatorsReader{ops: m.Ops}, nil
}

// operatorsReader is the lazy [io.ReadCloser] returned by [Operators.RawBytes].
// It formats one operator at a time into pending and drains pending into the
// caller's buffer, so the serialised byte stream never has to be materialised
// in full.
type operatorsReader struct {
	ops     []Operator
	idx     int
	pending bytes.Buffer
}

func (r *operatorsReader) Read(p []byte) (n int, err error) {
	for n < len(p) {
		if r.pending.Len() > 0 {
			k, _ := r.pending.Read(p[n:])
			n += k
			continue
		}
		if r.idx >= len(r.ops) {
			if n > 0 {
				return n, nil
			}
			return 0, io.EOF
		}
		if err := r.ops[r.idx].Format(&r.pending, 0); err != nil {
			return n, err
		}
		r.idx++
	}
	return n, nil
}

func (*operatorsReader) Close() error { return nil }

// Embed writes the wrapped operators to a new PDF stream object and
// returns a reference to it.
//
// This is used to implement the [seehuhn.de/go/pdf/page.Segment] interface.
func (m *Operators) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	ref := e.Out().Alloc()

	var filters []pdf.Filter
	if !e.Out().GetOptions().HasAny(pdf.OptPretty) {
		filters = append(filters, pdf.FilterCompress{})
	}

	stm, err := e.Out().OpenStream(ref, nil, filters...)
	if err != nil {
		return nil, err
	}
	if err := WriteStream(stm, m, e.Out().GetOptions()); err != nil {
		stm.Close()
		return nil, err
	}
	if err := stm.Close(); err != nil {
		return nil, err
	}
	return ref, nil
}

// WriteStream writes the bytes of the content stream s to w.
//
// An [Operators] stream and a stream rebuilt by [RepairGlyph] are formatted
// for the output options opt, which should be those of the PDF file the
// content is written to (see [Operator.Format]).  Any other stream is copied
// unchanged from [Stream.RawBytes].  For a stream read from a PDF file, this
// keeps the bytes as found, including comments, but nothing checks that they
// are valid for the version of the output file: names keep any #xx escapes,
// and operators from later PDF versions are kept.  A stream composed of
// several parts, such as a page with several content segments, is copied
// unchanged as well, so any [Operators] part in it uses PDF 1.2 syntax.
func WriteStream(w io.Writer, s Stream, opt pdf.OutputOptions) error {
	switch s.(type) {
	case *Operators, *repairedGlyph:
		it := s.NewIter()
		for name, args := range it.All() {
			if err := (Operator{Name: name, Args: args}).Format(w, opt); err != nil {
				return err
			}
		}
		return it.Err()
	}

	rc, err := s.RawBytes()
	if err != nil {
		return err
	}
	_, err = io.Copy(w, rc)
	// Decoders repeat on Close the decoding errors which the permissive
	// reader has already turned into a clean end of data.
	rc.Close()
	return err
}
