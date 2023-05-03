// seehuhn.de/go/pdf - a library for reading and writing PDF files
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
	"fmt"
	"strings"
)

var (
	errNoPDF           = errors.New("no header")
	errVersion         = errors.New("unsupported PDF version")
	errCorrupted       = errors.New("corrupted ciphertext")
	errNoDate          = errors.New("not a valid date string")
	errNoRectangle     = errors.New("not a valid PDF rectangle")
	errDuplicateRef    = errors.New("object already written")
	errShortID         = errors.New("PDF file identifier too short")
	errInvalidPassword = errors.New("invalid password")
)

// AuthenticationError indicates that authentication failed because the correct
// password has not been supplied.
type AuthenticationError struct {
	ID []byte
}

func (err *AuthenticationError) Error() string {
	if err.ID == nil {
		return "authentication failed"
	}
	return fmt.Sprintf("authentication failed for document ID %x", err.ID)
}

// MalformedFileError indicates that a PDF file could not be parsed.
type MalformedFileError struct {
	Err error
	Loc []string
}

func (err *MalformedFileError) Error() string {
	parts := make([]string, 0, len(err.Loc)+2)
	parts = append(parts, "invalid PDF: ")
	for i := len(err.Loc) - 1; i >= 0; i-- {
		parts = append(parts, err.Loc[i]+": ")
	}
	parts = append(parts, err.Err.Error())
	return strings.Join(parts, "")
}

func (err *MalformedFileError) Unwrap() error {
	return err.Err
}

func wrap(err error, loc string) error {
	if e, ok := err.(*MalformedFileError); ok {
		e.Loc = append(e.Loc, loc)
		return e
	}
	return fmt.Errorf("%s: %w", loc, err)
}

// VersionError is returned when trying to use a feature in a PDF file which is
// not supported by the PDF version used.  Use [Writer.CheckVersion] to create
// VersionError objects.
type VersionError struct {
	Operation string
	Earliest  Version
}

func (err *VersionError) Error() string {
	return (err.Operation + " requires PDF version " +
		err.Earliest.String() + " or later")
}
