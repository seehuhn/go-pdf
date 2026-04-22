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

// Error classification
//
// Errors returned by this package fall into two categories:
//
//   - Malformed-PDF errors, which wrap [*MalformedFileError]. They indicate
//     recoverable content violations such as spec-incompatible data, unknown
//     filters, or corrupt filter bodies. Permissive readers treat these as
//     recoverable; strict callers can check with [IsMalformed].
//   - Everything else is treated as a real failure (IO, context cancellation,
//     programmer error) and is always propagated.
//
// Use [Optional] to apply the permissive-reader policy at a call site: it
// returns the zero value on malformed errors and propagates everything else.

var (
	errCorrupted       = errors.New("corrupted ciphertext")
	errDuplicateRef    = errors.New("object already written")
	errInvalidID       = errors.New("invalid PDF file identifier")
	errInvalidPassword = errors.New("invalid password")
	errInvalidXref     = errors.New("invalid cross-reference table")
	errNoDate          = Error("not a valid date string")
	errNoPDF           = errors.New("no header")
	errNoRectangle     = errors.New("not a valid PDF rectangle")
	errVersion         = errors.New("unsupported PDF version")
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
//
// TODO(voss): introduce a distinction between an invalid PDF container,
// and invalid PDF content within a valid container?
type MalformedFileError struct {
	Err error
	Loc []string
}

// Error creates a new [MalformedFileError] with the given message.
// This is only for read errors, but not for write errors.
func Error(msg string) error {
	return &MalformedFileError{Err: errors.New(msg)}
}

// Errorf creates a new [MalformedFileError] with the given message.
func Errorf(format string, args ...any) error {
	return &MalformedFileError{Err: fmt.Errorf(format, args...)}
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

// Optional zeros out any [MalformedFileError] but returns all other errors.
// The check uses [errors.As] so wrapped malformed errors are recognised,
// consistent with [IsMalformed].
func Optional[T any](value T, err error) (T, error) {
	var zero T
	if IsMalformed(err) {
		return zero, nil
	} else if err != nil {
		return zero, err
	}
	return value, nil
}

// Wrap wraps an error with a location.
// If the error wraps a [MalformedFileError], the location is appended to the
// list of locations on that inner error (and the outer wrapping is preserved).
// Otherwise, the error is wrapped using [fmt.Errorf].
//
// This should only be used for read errors, not for write errors.
func Wrap(err error, loc string) error {
	if err == nil {
		return nil
	}
	var e *MalformedFileError
	if errors.As(err, &e) {
		e.Loc = append(e.Loc, loc)
		return err
	}
	return fmt.Errorf("%s: %w", loc, err)
}

// IsMalformed returns true if the error or any of its wrapped errors is a
// [MalformedFileError].
func IsMalformed(err error) bool {
	var malformed *MalformedFileError
	return errors.As(err, &malformed)
}

// IsReadError returns true if the error is non-nil and not a
// [MalformedFileError].  It is the inverse of the malformed-content
// predicate: a real failure (IO, context cancellation, programmer error)
// that the caller should propagate.
func IsReadError(err error) bool {
	return err != nil && !IsMalformed(err)
}

// VersionError is returned when trying to use a feature in a PDF file which is
// not supported by the PDF version used.  Use [CheckVersion] to create
// VersionError objects.
type VersionError struct {
	Operation string
	Earliest  Version
}

// CheckVersion checks whether the PDF file being written has version
// minVersion or later.  If the version is new enough, nil is returned.
// Otherwise a [VersionError] for the given operation is returned.
func CheckVersion(pdf *Writer, operation string, minVersion Version) error {
	if pdf.GetMeta().Version >= minVersion {
		return nil
	}
	return &VersionError{
		Earliest:  minVersion,
		Operation: operation,
	}
}

func (err *VersionError) Error() string {
	return (err.Operation + " requires PDF version " +
		err.Earliest.String() + " or later")
}

// IsWrongVersion returns true if the error or any of its wrapped errors is a
// [VersionError].
func IsWrongVersion(err error) bool {
	var versionError *VersionError
	return errors.As(err, &versionError)
}
