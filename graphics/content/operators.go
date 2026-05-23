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

// RawBytes returns a reader over the serialized operator sequence.
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
		if err := r.ops[r.idx].Format(&r.pending); err != nil {
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
	for _, op := range m.Ops {
		if err := op.Format(stm); err != nil {
			stm.Close()
			return nil, err
		}
	}
	if err := stm.Close(); err != nil {
		return nil, err
	}
	return ref, nil
}
