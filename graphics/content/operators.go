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

// Operators is an in-memory [Stream] backed by an [Operator] slice.
// Two pages embedding the same *Operators value via [pdf.ResourceManager]
// share a single PDF stream object (pointer-identity deduplication).
//
// Wrapped operators are treated as pre-validated builder output;
// [Operators.NewIter] yields them verbatim, without applying any fix-ups.
type Operators struct {
	Ops []Operator
}

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

// NewIter returns an iterator over the wrapped operators.
func (m *Operators) NewIter() Iter {
	return &operatorsIter{ops: m.Ops}
}

// Embed writes the wrapped operators to a new PDF stream object and
// returns a reference to it.  Repeated embeds of the same *Operators
// value via [pdf.ResourceManager] dedup to a single object.
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
	if err := m.writeOperators(stm); err != nil {
		stm.Close()
		return nil, err
	}
	if err := stm.Close(); err != nil {
		return nil, err
	}
	return ref, nil
}

// RawBytes serialises the wrapped operators into an in-memory buffer
// and returns a reader over it.  It is used by [seehuhn.de/go/pdf/page.Page.NewIter]
// to thread in-memory segments through the unified content-stream scanner.
func (m *Operators) RawBytes() (io.ReadCloser, error) {
	var buf bytes.Buffer
	if err := m.writeOperators(&buf); err != nil {
		return nil, err
	}
	return io.NopCloser(&buf), nil
}

func (m *Operators) writeOperators(w io.Writer) error {
	for _, op := range m.Ops {
		if err := op.Format(w); err != nil {
			return err
		}
	}
	return nil
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
