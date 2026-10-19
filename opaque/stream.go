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

package opaque

import (
	"fmt"
	"io"
	"maps"

	"seehuhn.de/go/pdf"
)

// streamMechanicsKeys are dict keys produced by [pdf.Copier] and the
// stream writer.  Callers must not pass them in the extras argument of
// [Stream.WriteAt]; doing so would clobber the writer's own values and
// produce a malformed stream.
var streamMechanicsKeys = []pdf.Name{
	"Length", "Filter", "DecodeParms", "F", "FFilter", "FDecodeParms",
}

// Stream holds a PDF stream read from a file.
type Stream struct {
	src    *pdf.Extractor
	stream *pdf.Stream
}

// ExtractStream wraps a stream read from the source extractor's PDF
// file.  The extractor must remain valid for as long as Stream is
// used.
func ExtractStream(x *pdf.Extractor, stream *pdf.Stream) *Stream {
	return &Stream{src: x, stream: stream}
}

// Reader returns a reader that yields the stream's fully decoded
// bytes, applying the source's filter chain.  The caller must close
// the returned reader.
func (s *Stream) Reader() (io.ReadCloser, error) {
	return pdf.CursorAt(s.src, nil).StreamReader(s.stream)
}

// WriteAt writes the stream to the output of rm, under the reference ref.
// The stream data is copied without decode/re-encode, keeping the
// original filters.
//
// The written stream dictionary consists of the source entries Filter,
// DecodeParms, F, FFilter and FDecodeParms, with indirect references
// translated to the output file, together with the entries in extras.
// All other source entries are dropped, so extras must give every entry
// the stream should carry.  WriteAt returns an error if extras contains
// Length, Filter, DecodeParms, F, FFilter or FDecodeParms.
func (s *Stream) WriteAt(rm *pdf.EmbedHelper, ref pdf.Reference, extras pdf.Dict) error {
	for _, key := range streamMechanicsKeys {
		if _, ok := extras[key]; ok {
			return fmt.Errorf("opaque.Stream.WriteAt: extras must not contain stream-mechanics key %q", key)
		}
	}

	// Copying only the stream-mechanics entries avoids copying objects
	// which are referenced by dropped entries.
	src := *s.stream
	src.Dict = pdf.Dict{}
	for _, key := range streamMechanicsKeys {
		if val, ok := s.stream.Dict[key]; ok {
			src.Dict[key] = val
		}
	}

	cp := rm.CopierFrom(s.src)
	copied, err := cp.Copy(&src)
	if err != nil {
		return err
	}
	out := copied.(*pdf.Stream)
	maps.Copy(out.Dict, extras)
	return rm.Out().Put(ref, out)
}
