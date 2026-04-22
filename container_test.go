// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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
	"bytes"
	"compress/zlib"
	"errors"
	"io"
	"os"
	"testing"
)

func TestGetDictTyped_NilObject(t *testing.T) {
	dict, err := GetDictTyped(mockGetter, nil, "test")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if dict != nil {
		t.Fatalf("expected nil, got %v", dict)
	}
}

func TestGetStreamReaderNull(t *testing.T) {
	r, err := GetStreamReader(mockGetter, nil)
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected os.ErrNotExist, got %v", err)
	}
	if r != nil {
		t.Errorf("expected nil reader, got %v", r)
	}
}

// partialErrReader returns the first len(data) bytes successfully, then
// returns err on every subsequent Read.  It is used to exercise the
// sourceErrChecker / sourceAwareReader pair.
type partialErrReader struct {
	data []byte
	pos  int
	err  error
}

func (r *partialErrReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, r.err
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func TestSourceErrCheckerSticky(t *testing.T) {
	// A non-EOF error surfaces once and stays in srcErr for future queries;
	// io.EOF is not recorded.
	diskErr := errors.New("disk fail")
	src := &sourceErrChecker{r: &partialErrReader{data: []byte("abc"), err: diskErr}}

	// first read returns data, no error
	buf := make([]byte, 3)
	n, err := src.Read(buf)
	if n != 3 || err != nil {
		t.Fatalf("first read: got n=%d err=%v", n, err)
	}
	if src.Err() != nil {
		t.Fatalf("srcErr should still be nil after clean read")
	}

	// second read surfaces the error and records it
	_, err = src.Read(buf)
	if err != diskErr {
		t.Errorf("want %v, got %v", diskErr, err)
	}
	if src.Err() != diskErr {
		t.Errorf("srcErr sticky: want %v, got %v", diskErr, src.Err())
	}

	// EOF is *not* recorded as a source error
	src2 := &sourceErrChecker{r: bytes.NewReader([]byte("xyz"))}
	_, _ = io.ReadAll(src2)
	if src2.Err() != nil {
		t.Errorf("EOF must not be recorded, got %v", src2.Err())
	}
}

func TestSourceAwareReaderPromotesSourceErr(t *testing.T) {
	// When the inner reader reports an error and the source has recorded
	// its own failure, sourceAwareReader surfaces the source error.
	diskErr := errors.New("disk fail")
	src := &sourceErrChecker{srcErr: diskErr} // pre-seeded sticky error
	inner := &partialErrReader{
		data: []byte("data"),
		err:  errors.New("some filter-produced error"),
	}
	r := &sourceAwareReader{
		inner: io.NopCloser(inner),
		src:   src,
	}
	// drain the data successfully
	buf := make([]byte, 4)
	_, err := r.Read(buf)
	if err != nil {
		t.Fatalf("unexpected error draining clean prefix: %v", err)
	}
	// next read surfaces an error; the source's is promoted
	_, err = r.Read(buf)
	if err != diskErr {
		t.Errorf("want source error %v promoted, got %v", diskErr, err)
	}
}

func TestSourceErrCheckerPromote(t *testing.T) {
	// promote returns the recorded source error in preference to its
	// argument, so IO failures observed during filter construction
	// (e.g. zlib reading its 2-byte header) reach the caller unchanged.
	diskErr := errors.New("disk fail")
	filterErr := errors.New("filter gave up")

	src := &sourceErrChecker{srcErr: diskErr}
	if got := src.promote(filterErr); got != diskErr {
		t.Errorf("want %v, got %v", diskErr, got)
	}

	// With no source error recorded, the filter's error is returned.
	clean := &sourceErrChecker{}
	if got := clean.promote(filterErr); got != filterErr {
		t.Errorf("want %v, got %v", filterErr, got)
	}
}

func TestDecodeStreamFlateSourceErrorPromoted(t *testing.T) {
	// End-to-end: a flate-compressed stream whose raw source fails mid-way.
	// The filter layer (zlib) may convert the source error into its own
	// io.ErrUnexpectedEOF, but DecodeStream's sourceAwareReader should
	// restore the original source error.
	var buf bytes.Buffer
	zw, _ := zlib.NewWriterLevel(&buf, zlib.BestCompression)
	_, _ = zw.Write([]byte("hello, world, this is enough data to exercise flate"))
	zw.Close()
	compressed := buf.Bytes()

	// truncate compressed data so the source fails after N bytes
	diskErr := errors.New("disk fail")
	src := &sourceErrChecker{
		r: &partialErrReader{data: compressed[:8], err: diskErr},
	}

	zr, err := zlib.NewReader(src)
	if err != nil {
		// zlib may or may not fail at construction time depending on
		// how much of the header was available; accept either.
		if err != diskErr {
			t.Fatalf("header read: want %v or nil, got %v", diskErr, err)
		}
		return
	}

	ar := &sourceAwareReader{inner: zr, src: src}
	_, err = io.ReadAll(ar)
	if err != diskErr {
		t.Errorf("want %v promoted by sourceAwareReader, got %v", diskErr, err)
	}
}

// We can't use mock.Getter here, because this would lead to a dependency cycle.
// Instead, we add a separate implementation.
var mockGetter Getter = mockGetterType{}

type mockGetterType struct{}

func (r mockGetterType) GetMeta() *MetaInfo {
	m := &MetaInfo{
		Version: V2_0,
	}
	return m
}

func (r mockGetterType) Get(ref Reference, canObjStm bool) (Native, error) {
	return nil, nil
}
