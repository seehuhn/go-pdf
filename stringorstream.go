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

package pdf

import (
	"seehuhn.de/go/pdf/internal/limits"
)

// StringOrStream represents a PDF value that may be stored either as a text
// string or as a stream.  Several PDF dictionary entries use this dual form
// (for example form field values, rich text contents, and embedded scripts) so
// that large values can be compressed in a stream while small values stay
// inline.  Both forms carry the same logical text.
type StringOrStream struct {
	// Value is the logical text content, decoded from whichever form the file
	// used.
	Value string

	// IsStream reports whether the value is held as a stream rather than an
	// inline text string.  It is preserved across a read so that a value stored
	// as a stream is written back as a stream.
	IsStream bool
}

// Embed returns the PDF representation of s for storage in a containing object.
// The inline form yields a direct text string; the stream form writes a stream
// object and returns a reference to it. Identical stream-form values share a
// single stream object, since [StringOrStream] is embedded through the
// resource manager's deduplication cache.
func (s StringOrStream) Embed(e *EmbedHelper) (Native, error) {
	// the stream form is a text stream, so both forms share the text-string
	// encoding (PDF 32000-2 §7.9.3)
	encoded := TextString(s.Value).AsPDF(e.Out().GetOptions()).(String)
	if !s.IsStream {
		return encoded, nil
	}

	// the stream form is a text stream, introduced in PDF 1.5
	w := e.Out()
	if err := CheckVersion(w, "text stream value", V1_5); err != nil {
		return nil, err
	}
	ref := e.Alloc()
	body, err := w.OpenStream(ref, nil, FilterCompress{})
	if err != nil {
		return nil, err
	}
	if _, err := body.Write([]byte(encoded)); err != nil {
		return nil, err
	}
	if err := body.Close(); err != nil {
		return nil, err
	}
	return ref, nil
}

var _ Embedder = StringOrStream{}

// StringOrStream resolves obj to a [StringOrStream].  The object may be a
// string, a stream, or an indirect reference to either.  A nil object, or an
// object that is neither a string nor a stream, yields the zero value.
func (c Cursor) StringOrStream(obj Object) (StringOrStream, error) {
	resolved, err := c.resolve(obj)
	if err != nil {
		return StringOrStream{}, err
	}
	switch v := resolved.(type) {
	case String:
		return StringOrStream{Value: string(v.AsTextString())}, nil
	case *Stream:
		data, err := ReadAll(c.x.R, c.path, v, limits.MaxStringOrStreamBytes)
		if err != nil {
			return StringOrStream{}, err
		}
		return StringOrStream{Value: string(String(data).AsTextString()), IsStream: true}, nil
	default:
		return StringOrStream{}, nil
	}
}
