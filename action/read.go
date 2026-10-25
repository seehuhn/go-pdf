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

package action

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/opaque"
)

// decodeNext reads the Next entry of an action.  A malformed entry is
// dropped, but a cycle or an over-deep chain fails the action, since
// dropping the entry would make the decoded (and cached) value depend on
// where decoding started.
//
// For a cycle, propagating the error makes the outcome independent of the
// order of reads: every action from which the cycle can be reached fails.
// This does not hold for the depth limit.  The decode cache is consulted
// before the depth of the chain is checked, so a chain longer than the limit
// fails when read from its head, but decodes if an action in its middle was
// decoded and cached first.
//
// TODO(voss): make the depth limit independent of decode order, e.g. by
// recording the height of the decoded subtree with each cache entry and
// failing a cache hit at depth d when d plus that height exceeds the limit.
func decodeNext(c pdf.Cursor, obj pdf.Object) (ActionList, error) {
	next, err := c.Decode(obj, DecodeActionList)
	if pdf.IsMalformed(err) && !isLoop(err) {
		return nil, nil
	}
	return next, err
}

// isLoop reports whether err comes from a cycle of references or from a
// chain of references nested too deeply.
func isLoop(err error) bool {
	return errors.Is(err, pdf.ErrCycle) || errors.Is(err, pdf.ErrDepth)
}

// decodePresent decodes an optional entry whose value, when present, must be
// usable: an absent or null entry gives the zero value, a malformed one
// fails the read.  This suits entries where dropping a broken value would
// change the meaning of what remains, rather than leave it as though the
// entry had never been given.
func decodePresent[T any](c pdf.Cursor, obj pdf.Object, decode func(pdf.Cursor, pdf.Object, bool) (T, error)) (T, error) {
	var zero T
	resolved, err := c.Resolve(obj)
	if err != nil || resolved == nil {
		return zero, err
	}
	return c.Decode(obj, decode)
}

// readStringOrStream resolves obj.  The result is nil if obj is absent, null,
// or neither a string nor a stream.
func readStringOrStream(c pdf.Cursor, obj pdf.Object) (pdf.Native, error) {
	resolved, err := pdf.Optional(c.Resolve(obj))
	if err != nil {
		return nil, err
	}
	switch resolved.(type) {
	case pdf.String, *pdf.Stream:
		return resolved, nil
	}
	return nil, nil
}

// readOpaque wraps the value of an entry whose content the library does not
// interpret.  The result is nil if obj is absent, null, or a reference which
// cannot be resolved.
func readOpaque(c pdf.Cursor, obj pdf.Object) (*opaque.Object, error) {
	resolved, err := pdf.Optional(c.Resolve(obj))
	if err != nil || resolved == nil {
		return nil, err
	}
	return opaque.Extract(c.Extractor(), obj), nil
}
