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
	"strings"
)

// A SequentialScanner reads a PDF file sequentially, extracting information
// about the file structure and the location of indirect objects.
// This can be used to attempt to read damaged PDF files, in particular
// in cases where the cross-reference table is missing or corrupt.
type SequentialScanner struct {
	s *scanner
}

// NewSequentialScanner creates a new SequentialScanner that reads from r.
func NewSequentialScanner(r io.Reader) (*SequentialScanner, error) {
	ss := &SequentialScanner{
		s: newScanner(r, nil, nil),
	}
	return ss, nil
}

func getSize(r io.ReaderAt) (int64, error) {
	if f, ok := r.(*os.File); ok {
		fi, err := f.Stat()
		if err != nil {
			return 0, err
		}
		return fi.Size(), nil
	}
	if b, ok := r.(*bytes.Reader); ok {
		return int64(b.Size()), nil
	}
	if s, ok := r.(*strings.Reader); ok {
		return int64(s.Size()), nil
	}

	buf := make([]byte, 1024)
	n, err := r.ReadAt(buf, 0)
	if err == io.EOF {
		return int64(n), nil
	} else if err != nil {
		return 0, err
	}

	lowerBound := int64(n) // all bytes before lowerBound are known to be present
	var upperBound int64   // at least one byte before upperBound is known to be missing
	for {
		test := 2 * lowerBound
		_, err := r.ReadAt(buf[:1], test-1)
		if err == io.EOF {
			upperBound = test
			break
		} else if err != nil {
			return 0, err
		}
		lowerBound = test
	}

	for lowerBound+1 < upperBound {
		test := (lowerBound + upperBound + 1) / 2
		_, err := r.ReadAt(buf[:1], test-1)
		if err == io.EOF {
			upperBound = test
		} else if err != nil {
			return 0, err
		} else {
			lowerBound = test
		}
	}
	return lowerBound, nil
}
