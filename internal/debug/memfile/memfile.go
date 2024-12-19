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

package memfile

import (
	"errors"
	"io"
)

// MemFile is a temporary in-memory file.
//
// This type implements the [io.ReadWriteSeeker] interface.
type MemFile struct {
	// Data are the file contents.
	Data []byte

	// Offset is the current file offset.
	Offset int64
}

// New creates a new MemFile.
func New() *MemFile {
	return &MemFile{}
}

// Write writes data to the file.
// This implements the [io.Writer] interface.
func (f *MemFile) Write(p []byte) (n int, err error) {
	if f.Offset > int64(len(f.Data)) {
		// If the offset is beyond the current data length, extend the slice with zeros
		f.Data = append(f.Data, make([]byte, f.Offset-int64(len(f.Data)))...)
	}

	// If writing at the end, just append
	if f.Offset == int64(len(f.Data)) {
		f.Data = append(f.Data, p...)
		n = len(p)
	} else {
		// Otherwise, overwrite existing data
		n = copy(f.Data[f.Offset:], p)
		if n < len(p) {
			f.Data = append(f.Data, p[n:]...)
			n = len(p)
		}
	}

	f.Offset += int64(n)
	return n, nil
}

// Read reads data from the file.
// This implements the [io.Reader] interface.
func (f *MemFile) Read(p []byte) (n int, err error) {
	if f.Offset >= int64(len(f.Data)) {
		return 0, io.EOF
	}
	n = copy(p, f.Data[f.Offset:])
	f.Offset += int64(n)
	if n < len(p) {
		err = io.EOF
	}
	return
}

// Seek sets the offset in the file.
// This implements the [io.Seeker] interface.
func (f *MemFile) Seek(offset int64, whence int) (int64, error) {
	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = f.Offset + offset
	case io.SeekEnd:
		newOffset = int64(len(f.Data)) + offset
	default:
		return 0, errInvalidWhence
	}

	if newOffset < 0 {
		return 0, errInvalidOffset
	}

	f.Offset = newOffset
	return newOffset, nil // Return the new offset, not the old one
}

var (
	errInvalidWhence = errors.New("invalid whence")
	errInvalidOffset = errors.New("invalid offset")
)
