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

package pdf

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type testReaderAt int64

func (r testReaderAt) ReadAt(p []byte, off int64) (n int, err error) {
	a := off
	b := off + int64(len(p))
	if a >= int64(r) {
		return 0, io.EOF
	}

	if b > int64(r) {
		return int(int64(r) - a), io.EOF
	}

	return int(b - a), nil
}

func TestGetSize(t *testing.T) {
	testDir := t.TempDir()
	for _, sz := range []int64{0, 1, 13, 1234, 5678} {
		buf := make([]byte, sz)

		testFileName := filepath.Join(testDir, "testfile")
		testFile, err := os.Create(testFileName)
		if err != nil {
			t.Fatal(err)
		}
		_, err = testFile.Write(buf)
		if err != nil {
			t.Fatal(err)
		}

		r1 := testReaderAt(sz)
		r2 := bytes.NewReader(buf)
		var r3 io.ReaderAt = testFile

		for i, r := range []io.ReaderAt{r1, r2, r3} {
			sz2, err := getSize(r)
			if err != nil {
				t.Error(err)
			}
			if sz2 != sz {
				t.Errorf("r%d, expected %d, got %d", i+1, sz, sz2)
			}
		}

		err = testFile.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}
