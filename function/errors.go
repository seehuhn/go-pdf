// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
