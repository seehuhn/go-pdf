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

package font

// InvalidFontError indicates a problem with font data.
type InvalidFontError struct {
	SubSystem string
	Reason    string
}

func (err *InvalidFontError) Error() string {
	return err.SubSystem + ": " + err.Reason
}

// NotSupportedError indicates that a font file seems valid but uses a
// CFF feature which is not supported by this library.
type NotSupportedError struct {
	SubSystem string
	Feature   string
}

func (err *NotSupportedError) Error() string {
	return err.SubSystem + ": " + err.Feature + " not supported"
}

// IsUnsupported returns true if the error is a NotSupportedError.
func IsUnsupported(err error) bool {
	_, ok := err.(*NotSupportedError)
	return ok
}
