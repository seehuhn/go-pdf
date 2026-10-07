// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package function

import "fmt"

// InvalidFunctionError is returned when the fields of a function object
// do not satisfy the constraints for its function type, for example
// when a Domain array has the wrong length or Bounds are out of order.
type InvalidFunctionError struct {
	FunctionType int
	Field        string
	Message      string
}

func (e *InvalidFunctionError) Error() string {
	return fmt.Sprintf("Type %d function invalid %s: %s", e.FunctionType, e.Field, e.Message)
}

func (e *InvalidFunctionError) Is(target error) bool {
	_, ok := target.(*InvalidFunctionError)
	return ok
}

// newInvalidFunctionError creates a new InvalidFunctionError.
func newInvalidFunctionError(functionType int, field, format string, args ...any) *InvalidFunctionError {
	return &InvalidFunctionError{
		FunctionType: functionType,
		Field:        field,
		Message:      fmt.Sprintf(format, args...),
	}
}
