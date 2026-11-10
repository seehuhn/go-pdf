// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package memfile

import (
	"errors"
	"io"
)

// MemFile is a temporary in-memory file.
//
// This type implements the [io.ReadWriteSeeker] and [io.ReaderAt] interfaces.
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

// ReadAt reads data from the file at the given offset.
// This implements the [io.ReaderAt] interface.
func (f *MemFile) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, errInvalidOffset
	}
	if off >= int64(len(f.Data)) {
		return 0, io.EOF
	}
	n := copy(p, f.Data[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
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
	return newOffset, nil
}

var (
	errInvalidWhence = errors.New("invalid whence")
	errInvalidOffset = errors.New("invalid offset")
)
