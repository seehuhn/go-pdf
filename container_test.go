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

	"seehuhn.de/go/pdf/internal/filter/asciihex"
	"seehuhn.de/go/pdf/internal/limits"
)

func TestGetDictTyped_NilObject(t *testing.T) {
	dict, err := NewCursor(mockGetter).DictTyped(nil, "test")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if dict != nil {
		t.Fatalf("expected nil, got %v", dict)
	}
}

func TestStreamReaderNull(t *testing.T) {
	r, err := NewCursor(mockGetter).StreamReader(nil)
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

// TestCheckDictType covers the three ways a "Type" entry can stand towards the
// expected type: a matching name, a name naming something else, and an entry
// carrying no type information at all.  The last case includes a "Type" which
// is not a name; a reader that rejected those would drop dictionaries over a
// producer's syntax slip, when the context they were reached through already
// says what they are.
func TestCheckDictType(t *testing.T) {
	cases := []struct {
		name    string
		typeObj Object
		wantErr bool
	}{
		{name: "matching name", typeObj: Name("Wanted")},
		{name: "different name", typeObj: Name("Other"), wantErr: true},
		{name: "missing", typeObj: nil},
		{name: "empty name", typeObj: Name("")},
		{name: "string instead of a name", typeObj: String("Wanted")},
		{name: "number instead of a name", typeObj: Integer(3)},
	}

	c := NewCursor(mockGetter)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dict := Dict{}
			if tc.typeObj != nil {
				dict["Type"] = tc.typeObj
			}
			err := c.CheckDictType(dict, "Wanted")
			if tc.wantErr && err == nil {
				t.Error("no error for a dictionary of the wrong type")
			} else if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestGetFiltersDecodeParmsShape verifies that a wrongly typed
// /DecodeParms is dropped, or used where its intended filter is plain,
// instead of making the stream unreadable.
func TestGetFiltersDecodeParmsShape(t *testing.T) {
	withPred := Dict{"Predictor": Integer(12), "Columns": Integer(4)}
	cases := []struct {
		name      string
		filter    Object
		parms     Object
		predictor FlatePredictor
	}{
		{"name+dict", Name("FlateDecode"), withPred, FlatePredictorPNGUp},
		{"name+array1", Name("FlateDecode"), Array{withPred}, FlatePredictorPNGUp},
		{"name+array2", Name("FlateDecode"), Array{withPred, withPred}, FlatePredictorNone},
		{"name+int", Name("FlateDecode"), Integer(3), FlatePredictorNone},
		{"array1+dict", Array{Name("FlateDecode")}, withPred, FlatePredictorPNGUp},
		{"array2+dict", Array{Name("ASCIIHexDecode"), Name("FlateDecode")}, withPred, FlatePredictorNone},
		{"array+array", Array{Name("FlateDecode")}, Array{withPred}, FlatePredictorPNGUp},
		{"array+badelem", Array{Name("FlateDecode")}, Array{Integer(3)}, FlatePredictorNone},
		{"array+int", Array{Name("FlateDecode")}, Integer(3), FlatePredictorNone},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			filters, err := GetFilters(nil, nil, Dict{"Filter": c.filter, "DecodeParms": c.parms})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(filters) == 0 {
				t.Fatal("no filters")
			}
			ff, ok := filters[len(filters)-1].(FilterFlate)
			if !ok {
				t.Fatalf("last filter is %T, want FilterFlate", filters[len(filters)-1])
			}
			if ff.Predictor != c.predictor {
				t.Errorf("predictor = %v, want %v", ff.Predictor, c.predictor)
			}
		})
	}
}

// TestAlignDecodeParmsBadFilter verifies that a /Filter value which is
// neither a name nor an array yields no parms entries, so that a chain
// built on top of it stays aligned.
func TestAlignDecodeParmsBadFilter(t *testing.T) {
	for _, filter := range []Object{nil, Integer(3), Dict{}} {
		if got := alignDecodeParms(filter, Dict{"Predictor": Integer(12)}); len(got) != 0 {
			t.Errorf("filter %v: got %d parms entries, want 0", filter, len(got))
		}
	}
}

func TestDecodeStreamChainExhaustsBudget(t *testing.T) {
	// a tiny stream gets the base budget; this many readers exceed it
	n := limits.StreamBudgetBase/asciihex.ReaderBytes + 2
	chain := make(Array, n)
	for i := range chain {
		chain[i] = Name("ASCIIHexDecode")
	}
	stm := NewStream(Dict{"Filter": chain}, []byte("4142>"))

	filters, err := GetFilters(mockGetter, nil, stm.Dict)
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != n {
		t.Fatalf("got %d filters, want %d", len(filters), n)
	}

	r, err := DecodeStream(mockGetter, nil, stm)
	if err == nil {
		_, err = io.ReadAll(r)
	}
	if !IsMalformed(err) {
		t.Errorf("expected malformed error, got %v", err)
	}
}
