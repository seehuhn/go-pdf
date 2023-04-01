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
	"io"
)

// A SequentialScanner reads a PDF file sequentially, extracting information
// about the file structure and the location of indirect objects.
// This can be used to attempt to read damaged PDF files, in particular
// in cases where the cross-reference table is missing or corrupt.
type SequentialScanner struct {
	s *scanner
}

// NewSequentialScanner creates a new SequentialScanner that reads from r.
func NewSequentialScanner(r io.ReadSeeker) (*SequentialScanner, error) {
	_, err := r.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}

	ss := &SequentialScanner{
		s: newScanner(r, nil, nil),
	}
	return ss, nil
}

func getSize(r io.Seeker) (int64, error) {
	cur, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}

	size, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}

	_, err = r.Seek(cur, io.SeekStart)
	if err != nil {
		return 0, err
	}

	return size, nil
}
