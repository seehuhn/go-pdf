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
)

// headerMarker starts the PDF file header; byte offsets within the file
// are counted from its first byte.
const headerMarker = "%PDF-"

// ReadHeader locates the PDF file header in data and returns the byte
// offset of its "%PDF-" marker together with the version number which
// follows the marker.  The argument size is the length of data in bytes.
// Any number of bytes may precede the header; the whole file is searched.
//
// The header is normally the first marker in the file.  If that marker is
// not followed by a well-formed version number, the first marker with a
// well-formed version number starting less than 1024 bytes after it is used
// instead.  If there is no such marker, or no marker at all, an error is
// returned; a file with a malformed header cannot be read.
//
// A well-formed version number has the form "d.d", where the first digit
// is 1 to 9, and is not followed by a further digit or dot.  Versions
// newer than the ones this library writes are returned as given.
func ReadHeader(data io.ReaderAt, size int64) (offset int64, version Version, err error) {
	const chunkSize = 64 * 1024
	marker := []byte(headerMarker)

	// Consecutive chunks overlap by the length of a marker and its version
	// number, so that every marker starting within a chunk can be
	// examined there.  Markers starting in the overlap are left to the
	// next chunk.
	buf := make([]byte, chunkSize+len(marker)+maxVersionLen)
	offset = -1
	for pos := int64(0); pos < size; pos += chunkSize {
		if offset >= 0 && pos >= offset+headerWindow {
			break
		}
		n, err := data.ReadAt(buf[:min(int64(len(buf)), size-pos)], pos)
		if err != nil && err != io.EOF {
			return 0, 0, err
		}
		chunk := buf[:n]
		for start := 0; start < min(n, chunkSize); {
			idx := bytes.Index(chunk[start:], marker)
			if idx < 0 || start+idx >= chunkSize {
				break
			}
			idx += start
			markerPos := pos + int64(idx)
			if offset < 0 {
				offset = markerPos
			} else if markerPos >= offset+headerWindow {
				return 0, 0, malformedHeader()
			}
			if ver := markerVersion(chunk[idx+len(marker):]); ver != 0 {
				return markerPos, ver, nil
			}
			start = idx + 1
		}
	}
	if offset < 0 {
		return 0, 0, &MalformedFileError{Err: errNoPDF}
	}
	return 0, 0, malformedHeader()
}

// malformedHeader returns the error for a file which has a "%PDF-" marker
// but no well-formed version number.  A fresh value is returned each time,
// since [Wrap] appends locations to a MalformedFileError in place.
func malformedHeader() error {
	return &MalformedFileError{
		Err: errVersion,
		Loc: []string{"PDF header"},
	}
}

// headerWindow is how far past the first marker a marker with a
// well-formed version number may start and still be taken as the header.
// The limit keeps a "%PDF-" quoted later in the file, for example in an
// embedded PDF file, from being taken as the header of a file whose own
// header is malformed.
const headerWindow = 1024

// maxVersionLen is the number of bytes after a marker which markerVersion
// examines: a version number "d.d" and one byte to check that it ends there.
const maxVersionLen = 4

// markerVersion returns the version number at the start of buf, or zero if
// the version number is malformed.
func markerVersion(buf []byte) Version {
	// collect the whole run of digits and dots, so that "1.50" is
	// rejected rather than read as "1.5"
	buf = buf[:min(len(buf), maxVersionLen)]
	end := 0
	for end < len(buf) && (buf[end] >= '0' && buf[end] <= '9' || buf[end] == '.') {
		end++
	}
	ver, err := ParseVersion(string(buf[:end]))
	if err != nil {
		return 0
	}
	return ver
}
