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
	"errors"
	"strings"
	"testing"
)

func TestReadHeader(t *testing.T) {
	tests := []struct {
		name    string
		prefix  int
		in      string
		offset  int64
		version Version
	}{
		{"no preamble", 0, "%PDF-1.7\nrest of file", 0, V1_7},
		{"short preamble", 4, "%PDF-1.5\nrest of file", 4, V1_5},
		{"preamble beyond 1024", 5000, "%PDF-1.4\n", 5000, V1_4},
		{"marker straddles chunk boundary", 64*1024 - 2, "%PDF-2.0\n", 64*1024 - 2, V2_0},
		{"marker in later chunk", 3*64*1024 + 17, "%PDF-1.6\n", 3*64*1024 + 17, V1_6},
		{"version beyond MaxVersion", 0, "%PDF-2.3\n", 0, Version(203)},
		{"stray marker before header", 0, "%PDF-\n%PDF-1.3\n", 6, V1_3},
		{"valid marker straddles chunk boundary", 64*1024 - 7, "%PDF-x%PDF-1.2\n", 64*1024 - 1, V1_2},
		{"version at end of chunk", 64*1024 - 5, "%PDF-1.4", 64*1024 - 5, V1_4},
		{"valid marker at end of window", 0, "%PDF-x" + strings.Repeat("y", 1017) + "%PDF-1.4\n", 1023, V1_4},
		{"window crosses chunk boundary", 64*1024 - 100, "%PDF-x" + strings.Repeat("y", 500) + "%PDF-1.4\n", 64*1024 + 406, V1_4},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := append(bytes.Repeat([]byte{0xff}, tc.prefix), tc.in...)
			off, ver, err := ReadHeader(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Fatal(err)
			}
			if off != tc.offset {
				t.Errorf("wrong offset: got %d, want %d", off, tc.offset)
			}
			if ver != tc.version {
				t.Errorf("wrong version: got %d, want %d", ver, tc.version)
			}
		})
	}
}

func TestReadHeaderMalformed(t *testing.T) {
	tests := []struct {
		name   string
		prefix int
		in     string
	}{
		{"empty", 0, ""},
		{"no marker", 0, "this is not a PDF file"},
		{"many near misses", 0, strings.Repeat("%PDF%PDF %PD\n", 20000)},
		{"truncated version", 0, "%PDF-1"},
		{"version too long", 0, "%PDF-1.50\n"},
		{"non-numeric version", 0, "%PDF-x.y\n"},
		{"marker at end of file", 3, "%PDF-"},
		{"malformed markers only", 2, "%PDF-x\n%PDF-1.50\n"},
		{"marker in final overlap", 64*1024 + 1, "%PDF-"},
		{"truncated version in final overlap", 64*1024 + 1, "%PDF-1."},
		{"valid marker beyond window", 0, "%PDF-x" + strings.Repeat("y", 1018) + "%PDF-1.4\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := append(bytes.Repeat([]byte{0xff}, tc.prefix), tc.in...)
			_, _, err := ReadHeader(bytes.NewReader(data), int64(len(data)))
			if err == nil {
				t.Error("expected error for malformed header")
			}
		})
	}
}

type failingReaderAt struct{}

func (failingReaderAt) ReadAt([]byte, int64) (int, error) {
	return 0, errors.New("read failed")
}

func TestReadHeaderReadError(t *testing.T) {
	_, _, err := ReadHeader(failingReaderAt{}, 100)
	if err == nil {
		t.Error("expected error from failing reader")
	}
}

// TestReadHeaderManyMarkers checks that a file consisting of malformed
// markers is rejected after reading only the header window.
func TestReadHeaderManyMarkers(t *testing.T) {
	data := bytes.Repeat([]byte("%PDF-"), 200_000)
	r := &countingReaderAt{r: bytes.NewReader(data)}
	_, _, err := ReadHeader(r, int64(len(data)))
	if err == nil {
		t.Error("expected error for malformed header")
	}
	if r.n > 2*64*1024 {
		t.Errorf("read %d bytes from a %d byte file", r.n, len(data))
	}
}

type countingReaderAt struct {
	r *bytes.Reader
	n int64
}

func (c *countingReaderAt) ReadAt(p []byte, off int64) (int, error) {
	n, err := c.r.ReadAt(p, off)
	c.n += int64(n)
	return n, err
}
