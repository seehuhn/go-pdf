// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package opaque

import (
	"fmt"
	"io"
	"maps"

	"seehuhn.de/go/pdf"
)

// streamMechanicsKeys are the dict keys which describe how the stream
// data is stored.  They are copied from the source stream;
// [pdf.Copier] replaces the abbreviation /DP by /DecodeParms.  Callers
// must not pass them in the extras argument of [Stream.WriteAt]; doing so
// would clobber the copied values and produce a malformed stream.
var streamMechanicsKeys = []pdf.Name{
	"Length", "Filter", "DecodeParms", "DP", "F", "FFilter", "FDecodeParms",
}

// Stream holds a PDF stream read from a file.
type Stream struct {
	src    *pdf.Extractor
	stream *pdf.Stream

	// filters is the decoded filter chain of stream.  It is used to check
	// that the filters are available in the PDF version of the output file.
	filters []pdf.Filter
}

// ExtractStream wraps a stream read from the source extractor's PDF
// file.  The extractor must remain valid for as long as Stream is
// used.
//
// An error is returned if the stream's filter chain is malformed, since
// the stream data could then be neither decoded nor written out as a
// valid stream.
func ExtractStream(x *pdf.Extractor, stream *pdf.Stream) (*Stream, error) {
	filters, err := pdf.GetFilters(x.R, nil, stream.Dict)
	if err != nil {
		return nil, err
	}
	return &Stream{src: x, stream: stream, filters: filters}, nil
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
// Length, Filter, DecodeParms, DP, F, FFilter or FDecodeParms, and a
// [pdf.VersionError] if a filter of the stream is not available in the
// PDF version of the output file.
func (s *Stream) WriteAt(rm *pdf.EmbedHelper, ref pdf.Reference, extras pdf.Dict) error {
	for _, key := range streamMechanicsKeys {
		if _, ok := extras[key]; ok {
			return fmt.Errorf("opaque.Stream.WriteAt: extras must not contain stream-mechanics key %q", key)
		}
	}

	// The data is copied without re-encoding, so the filters are not
	// passed to OpenStream, which would check them.  Crypt filters are
	// exempt: the copier keeps a crypt filter only if the output file
	// uses crypt filters, which requires PDF 1.5 or later.
	v := pdf.GetVersion(rm.Out())
	for _, f := range s.filters {
		if _, ok := f.(pdf.CryptFilter); ok {
			continue
		}
		if _, _, err := f.Info(v); err != nil {
			return err
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
