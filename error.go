// seehuhn.de/go/pdf - support for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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
	"errors"
	"strconv"
)

var (
	// ErrNoAuth indicates that authentication failed because the
	// correct password was not supplied.
	ErrNoAuth = errors.New("authentication failed")

	errVersion   = errors.New("unsupported PDF version")
	errCorrupted = errors.New("corrupted ciphertext")
	errNoDate    = errors.New("not a valid date string")
)

// MalformedFileError indicates that the PDF file could not be parsed.
type MalformedFileError struct {
	Err error
	Pos int64
}

func (err *MalformedFileError) Error() string {
	middle := ""
	if err.Err != nil {
		middle = ": " + err.Err.Error()
	}
	tail := ""
	if err.Pos > 0 {
		tail = " (at byte " + strconv.FormatInt(err.Pos, 10) + ")"
	}
	return "not a valid PDF file" + middle + tail
}

func (err *MalformedFileError) Unwrap() error {
	return err.Err
}

// VersionError is returned when trying to use a feature in a PDF file which is
// not supported by the PDF version used.
type VersionError struct {
	Earliest  Version
	Operation string
}

func (err *VersionError) Error() string {
	return (err.Operation + " requires PDF version " +
		err.Earliest.String() + " or newer")
}
