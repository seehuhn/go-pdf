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

package tempfile

import (
	"io"
	"testing"
)

var _ io.Reader = (*MemFile)(nil)
var _ io.Writer = (*MemFile)(nil)
var _ io.Seeker = (*MemFile)(nil)

func TestMemFile_Write(t *testing.T) {
	f := New()
	data := []byte("Hello, World!")

	n, err := f.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Write wrote %d bytes; want %d", n, len(data))
	}
	if string(f.Data) != "Hello, World!" {
		t.Errorf("Write stored %q; want %q", string(f.Data), "Hello, World!")
	}
}

func TestMemFile_Read(t *testing.T) {
	f := New()
	f.Data = []byte("Hello, World!")
	f.Offset = 0

	buf := make([]byte, 5)
	n, err := f.Read(buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n != 5 {
		t.Errorf("Read read %d bytes; want 5", n)
	}
	if string(buf) != "Hello" {
		t.Errorf("Read got %q; want %q", string(buf), "Hello")
	}

	// Read to EOF
	buf, err = io.ReadAll(f)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if string(buf) != ", World!" {
		t.Errorf("Read got %q; want %q", string(buf), ", World!")
	}
}

func TestMemFile_Seek(t *testing.T) {
	f := New()
	f.Data = []byte("Hello, World!")

	testCases := []struct {
		offset    int64
		whence    int
		wantPos   int64
		wantError error
	}{
		{5, io.SeekStart, 5, nil},
		{2, io.SeekCurrent, 7, nil},
		{-1, io.SeekEnd, 12, nil},
		{0, io.SeekStart, 0, nil},
		{100, io.SeekStart, 100, nil},
		{-1, io.SeekStart, 0, errInvalidOffset},
		{-14, io.SeekEnd, 0, errInvalidOffset},
		{0, 99, 0, errInvalidWhence},
	}

	for i, tc := range testCases {
		got, err := f.Seek(tc.offset, tc.whence)
		if err != tc.wantError {
			t.Errorf("case %d: Seek(%d, %d) error = %v; want %v", i, tc.offset, tc.whence, err, tc.wantError)
		}
		if got != tc.wantPos {
			t.Errorf("case %d: Seek(%d, %d) = %d; want %d", i, tc.offset, tc.whence, got, tc.wantPos)
		}
	}
}

func TestMemFile_WriteAtOffset(t *testing.T) {
	f := New()
	f.Write([]byte("Hello, World!"))
	f.Seek(7, io.SeekStart)
	f.Write([]byte("Universe!"))

	if string(f.Data) != "Hello, Universe!" {
		t.Errorf("Write at offset produced %q; want %q",
			string(f.Data), "Hello, Universe!")
	}
}

func TestMemFile_WriteExtend(t *testing.T) {
	f := New()
	f.Seek(5, io.SeekStart)
	f.Write([]byte("Hello"))

	if string(f.Data) != "\x00\x00\x00\x00\x00Hello" {
		t.Errorf("Write extension produced %q; want %q", string(f.Data), "\x00\x00\x00\x00\x00Hello")
	}
}
