// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package cff

// NotSupportedError indicates that the font file seems valid but uses a
// CFF feature which is not supported by this library.
type NotSupportedError struct {
	Feature string
}

func (err *NotSupportedError) Error() string {
	return "cff: " + err.Feature + " not supported"
}

func notSupported(feature string) error {
	return &NotSupportedError{feature}
}

// InvalidFontError indicates a problem with the font file.
type InvalidFontError struct {
	Reason string
}

func (err *InvalidFontError) Error() string {
	return "cff: " + err.Reason
}

func invalidSince(reason string) error {
	return &InvalidFontError{reason}
}
