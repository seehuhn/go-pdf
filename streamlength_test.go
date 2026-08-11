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
	"bytes"
	"io"
	"regexp"
	"testing"
)

const streamLengthBody = "hello world, some stream content"

// lengthPat matches the /Length entry of the probe stream written by
// writeStreamLengthFile.  The stream is the only one carrying /Quir:T, and
// dictionary keys are written in sorted order, so /Length precedes it.
var lengthPat = regexp.MustCompile(`/Length \d+(\n/Quir:T /probe)`)

// writeStreamLengthFile builds a one-stream PDF file and returns its bytes.
func writeStreamLengthFile(t *testing.T) []byte {
	t.Helper()

	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, V1_7, &WriterOptions{HumanReadable: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := addPage(w); err != nil {
		t.Fatal(err)
	}

	ref := w.Alloc()
	stm, err := w.OpenStream(ref, Dict{"Quir:T": Name("probe")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stm.Write([]byte(streamLengthBody)); err != nil {
		t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}
	w.GetMeta().Trailer["Quir:E"] = ref
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	data := buf.Bytes()
	if !lengthPat.Match(data) {
		t.Fatalf("probe stream not found in generated file:\n%s", data)
	}
	return data
}

// TestStreamLengthRoundTrip checks that a stream survives a read-write-read
// cycle however its /Length is written.  A file may declare a length which is
// wrong or missing entirely; reading repairs this silently, and the repaired
// stream must then be writable, with the second read matching the first.
//
// The replacements below all keep the byte count of the file unchanged, so
// that the cross-reference table stays valid and only the declared length
// varies.
func TestStreamLengthRoundTrip(t *testing.T) {
	good := writeStreamLengthFile(t)

	for _, tc := range []struct {
		name        string
		replacement string
	}{
		{"correct", ""},
		{"too small", "/Length 07$1"},
		{"too large", "/Length 99$1"},
		{"missing", "/Quir:Z 32$1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := good
			if tc.replacement != "" {
				data = lengthPat.ReplaceAll(good, []byte(tc.replacement))
				if len(data) != len(good) {
					t.Fatalf("mutation changed the file size, %d != %d",
						len(data), len(good))
				}
			}

			first := readProbeStream(t, data)
			if got := string(first.body); got != streamLengthBody {
				t.Errorf("wrong stream data: got %q, want %q", got, streamLengthBody)
			}

			second := readProbeStream(t, writeProbeStream(t, first))
			if first.length != second.length {
				t.Errorf("stream length changed: %d then %d",
					first.length, second.length)
			}
			if !bytes.Equal(first.body, second.body) {
				t.Error("stream data changed")
			}
			if !Equal(first.dict, second.dict) {
				t.Errorf("stream dictionary changed:\n%v\n%v",
					first.dict, second.dict)
			}
		})
	}
}

// TestStreamLengthMatchesReader checks the contract [Stream.Length] states:
// the value is the number of bytes [Stream.NewReader] yields, whatever the
// stream's provenance.  The encrypted cases matter most — there the bytes on
// disk are the encrypted form, and its length differs from the plaintext.
func TestStreamLengthMatchesReader(t *testing.T) {
	check := func(t *testing.T, stm *Stream) {
		t.Helper()
		data, err := io.ReadAll(stm.NewReader())
		if err != nil {
			t.Fatal(err)
		}
		if got := stm.Length(); got != int64(len(data)) {
			t.Errorf("Length is %d, but NewReader yields %d bytes", got, len(data))
		}
	}

	t.Run("in memory", func(t *testing.T) {
		check(t, NewStream(Dict{"Length": Integer(999)}, []byte(streamLengthBody)))
	})

	t.Run("read from a file", func(t *testing.T) {
		p := readProbeStream(t, writeStreamLengthFile(t))
		check(t, p.stream)
	})

	for _, opt := range []*WriterOptions{
		nil,
		{UserPassword: "user", OwnerPassword: "owner"},
	} {
		name := "plain"
		if opt != nil {
			name = "encrypted"
		}
		t.Run(name, func(t *testing.T) {
			r, ref := writeStreamFileWithOptions(t, opt)
			stm, err := NewCursor(r).Stream(ref)
			if err != nil {
				t.Fatal(err)
			}
			check(t, stm)

			// a copy of an encrypted stream holds the decrypted bytes, and
			// the length has to follow them
			buf := &bytes.Buffer{}
			w, err := NewWriter(buf, V1_7, nil)
			if err != nil {
				t.Fatal(err)
			}
			copied, err := NewCopier(w, r).Copy(stm)
			if err != nil {
				t.Fatal(err)
			}
			check(t, copied.(*Stream))

			// The encrypted case is only worth testing while the two forms
			// have different lengths; AES adds an initialisation vector and
			// padding.  A cipher which preserved the length would make this
			// sub-test say nothing.
			if opt != nil && stm.Length() == copied.(*Stream).Length() {
				t.Errorf("encrypted and decrypted lengths agree (%d): "+
					"this no longer tests anything", stm.Length())
			}
		})
	}
}

// writeStreamFileWithOptions writes a PDF file containing one stream and
// returns a reader for it together with the stream's reference.
func writeStreamFileWithOptions(t *testing.T, opt *WriterOptions) (*Reader, Reference) {
	t.Helper()

	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, V2_0, opt)
	if err != nil {
		t.Fatal(err)
	}
	ref := w.Alloc()
	stm, err := w.OpenStream(ref, nil, FilterCompress{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stm.Write([]byte(streamLengthBody)); err != nil {
		t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := addPage(w, Name("Contents"), ref); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	var readerOpt *ReaderOptions
	if opt != nil {
		readerOpt = &ReaderOptions{Password: opt.UserPassword}
	}
	r, err := NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()), readerOpt)
	if err != nil {
		t.Fatal(err)
	}
	return r, ref
}

type probeStream struct {
	dict   Dict
	length int64
	body   []byte
	reader *Reader
	stream *Stream
}

func readProbeStream(t *testing.T, data []byte) probeStream {
	t.Helper()

	r, err := NewReader(bytes.NewReader(data), int64(len(data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	stm, err := NewCursor(r).Stream(r.GetMeta().Trailer["Quir:E"])
	if err != nil {
		t.Fatal(err)
	}
	if _, present := stm.Dict["Length"]; present {
		t.Error("stream dictionary carries a /Length entry")
	}
	body, err := io.ReadAll(stm.NewReader())
	if err != nil {
		t.Fatal(err)
	}
	return probeStream{
		dict:   stm.Dict,
		length: stm.Length(),
		body:   body,
		reader: r,
		stream: stm,
	}
}

// writeProbeStream copies the stream into a fresh file and returns its bytes.
func writeProbeStream(t *testing.T, p probeStream) []byte {
	t.Helper()

	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, V1_7, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := addPage(w); err != nil {
		t.Fatal(err)
	}

	ref := w.Alloc()
	copied, err := NewCopier(w, p.reader).Copy(p.stream)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Put(ref, copied); err != nil {
		t.Fatal(err)
	}
	w.GetMeta().Trailer["Quir:E"] = ref
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
