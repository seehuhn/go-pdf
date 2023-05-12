// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package ascii85

import (
	"bytes"
	"encoding/ascii85"
	"fmt"
	"io"
	"testing"
)

func FuzzReader(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("1234"))
	f.Add([]byte("12345678"))
	f.Add([]byte("z"))
	f.Add([]byte("ABCDE"))

	f.Fuzz(func(t *testing.T, data []byte) {
		for _, c := range data {
			if c <= ' ' && !isSpace(c) {
				return
			}
		}

		in := bytes.NewReader(data)
		enc1 := ascii85.NewDecoder(in)
		out1, err1 := io.ReadAll(enc1)

		data2 := make([]byte, len(data), len(data)+2)
		copy(data2, data)
		data2 = append(data2, '~', '>')
		in = bytes.NewReader(data2)
		enc2, err := (*Info)(nil).Decode(in)
		if err != nil {
			t.Fatal(err)
		}
		out2, err2 := io.ReadAll(enc2)

		if err2 != nil && err1 == nil {
			t.Errorf("err2=%v, err1=nil", err2)
		}
		if err1 == nil && !bytes.Equal(out1, out2) {
			t.Errorf("out1=%q, out2=%q", out1, out2)
		}
	})
}

func FuzzWriter(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("Hello world!"))
	f.Add([]byte("\000"))

	f.Fuzz(func(t *testing.T, in []byte) {
		buf := &bytes.Buffer{}
		enc, err := Filter.Encode(withDummyClose{buf})
		if err != nil {
			t.Fatal(err)
		}
		_, err = enc.Write(in)
		if err != nil {
			t.Fatal(err)
		}
		err = enc.Close()
		if err != nil {
			t.Fatal(err)
		}

		dec, err := Filter.Decode(bytes.NewReader(buf.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		out, err := io.ReadAll(dec)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(in, out) {
			fmt.Println(buf.String())
			t.Errorf("in=%q, out=%q", in, out)
		}
	})
}

type funnyReader struct {
	pos int
}

func (r *funnyReader) Read(p []byte) (n int, err error) {
	for i := range p {
		p[i] = byte(r.pos%85) + '!'
		r.pos++
	}
	return len(p), nil
}

func BenchmarkReader(b *testing.B) {
	r, err := Filter.Decode(&funnyReader{})
	if err != nil {
		b.Fatal(err)
	}

	blockSize := 1019
	buf := make([]byte, blockSize)

	b.ResetTimer()
	b.SetBytes(int64(blockSize))
	for i := 0; i < b.N; i++ {
		io.ReadFull(r, buf)
	}
}

func BenchmarkReaderStdLib(b *testing.B) {
	r := ascii85.NewDecoder(&funnyReader{})

	blockSize := 1019
	buf := make([]byte, blockSize)

	b.ResetTimer()
	b.SetBytes(int64(blockSize))
	for i := 0; i < b.N; i++ {
		io.ReadFull(r, buf)
	}
}

func BenchmarkWriter(b *testing.B) {
	w, err := Filter.Encode(withDummyClose{io.Discard})
	if err != nil {
		b.Fatal(err)
	}

	blockSize := 1019
	buf := make([]byte, blockSize)
	for i := range buf {
		buf[i] = byte(7 * i)
	}

	b.ResetTimer()
	b.SetBytes(int64(blockSize))
	for i := 0; i < b.N; i++ {
		w.Write(buf)
	}
}

func BenchmarkWriterStdLib(b *testing.B) {
	w := ascii85.NewEncoder(io.Discard)

	blockSize := 1019
	buf := make([]byte, blockSize)
	for i := range buf {
		buf[i] = byte(7 * i)
	}

	b.ResetTimer()
	b.SetBytes(int64(blockSize))
	for i := 0; i < b.N; i++ {
		w.Write(buf)
	}
}

// withDummyClose turns and io.Writer into an io.WriteCloser.
type withDummyClose struct {
	io.Writer
}

func (w withDummyClose) Close() error {
	return nil
}
