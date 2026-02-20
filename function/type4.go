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

import (
	"errors"
	"fmt"
	"io"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 7.10.5

// Type4 represents a Type 4 PostScript calculator function that uses a subset
// of PostScript language to define arbitrary calculations.
//
// A Type4 function must not be used concurrently.
type Type4 struct {
	// Domain defines the valid input ranges as [min0, max0, min1, max1, ...].
	// The length must be 2*m, where m is the number of input variables.
	Domain []float64

	// Range gives clipping ranges for the output variables as [min0, max0, min1, max1, ...].
	// The length must be 2*n, where n is the number of output variables.
	Range []float64

	// Program contains the PostScript code (without enclosing braces).
	Program string

	compiled   []instruction
	compileErr error
	stack      []value
}

// FunctionType returns 4 for Type 4 functions.
func (f *Type4) FunctionType() int {
	return 4
}

// Shape returns the number of input and output values of the function.
func (f *Type4) Shape() (int, int) {
	m := len(f.Domain) / 2
	n := len(f.Range) / 2
	return m, n
}

// GetDomain returns the function's input domain.
func (f *Type4) GetDomain() []float64 {
	return f.Domain
}

// extractType4 reads a Type 4 function from a PDF stream object.
func extractType4(x *pdf.Extractor, stream *pdf.Stream) (*Type4, error) {
	d := stream.Dict

	domain, err := pdf.Optional(getFloatArray(x, d["Domain"]))
	if err != nil {
		return nil, err
	}

	rangeArray, err := pdf.Optional(getFloatArray(x, d["Range"]))
	if err != nil {
		return nil, err
	}

	f := &Type4{
		Domain: domain,
		Range:  rangeArray,
	}

	// read PostScript program from stream
	stmReader, err := pdf.DecodeStream(x.R, stream, 0)
	if err != nil {
		return nil, err
	}
	defer stmReader.Close()

	const maxProgramSize = 16 * 1024
	programBytes, err := io.ReadAll(io.LimitReader(stmReader, maxProgramSize+1))
	if err != nil {
		return nil, err
	}
	if len(programBytes) > maxProgramSize {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("Type 4 function program too large"),
		}
	}

	program := string(programBytes)
	// Remove surrounding braces if present
	if len(program) >= 2 && program[0] == '{' && program[len(program)-1] == '}' {
		program = program[1 : len(program)-1]
	}
	f.Program = program

	f.repair()
	if err := f.validate(); err != nil {
		return nil, err
	}

	return f, nil
}

// repair sets default values and tries to fix mal-formed function dicts.
func (f *Type4) repair() {
	if len(f.Domain)%2 == 1 {
		f.Domain = f.Domain[:len(f.Domain)-1]
	}
	if len(f.Domain) == 0 {
		f.Domain = []float64{0.0, 1.0}
	}
	if len(f.Range)%2 == 1 {
		f.Range = f.Range[:len(f.Range)-1]
	}
	if len(f.Range) == 0 {
		f.Range = []float64{0, 1}
	}
}

// validate checks if the Type4 function is properly configured.
func (f *Type4) validate() error {
	m, n := f.Shape()

	if len(f.Domain) != 2*m {
		return newInvalidFunctionError(4, "domain", "length must be 2*m (%d), got %d", 2*m, len(f.Domain))
	}
	if len(f.Range) != 2*n {
		return newInvalidFunctionError(4, "range", "length must be 2*n (%d), got %d", 2*n, len(f.Range))
	}
	if len(f.Range) == 0 {
		return newInvalidFunctionError(4, "range", "must not be empty")
	}

	if f.Program == "" {
		return newInvalidFunctionError(4, "program", "cannot be empty")
	}

	// Basic syntax check - ensure braces are balanced
	braceCount := 0
	for _, char := range f.Program {
		switch char {
		case '{':
			braceCount++
		case '}':
			braceCount--
		}
		if braceCount < 0 {
			return newInvalidFunctionError(4, "program", "unbalanced braces")
		} else if braceCount > 255 {
			return newInvalidFunctionError(4, "program", "too many nested braces (max 255)")
		}
	}
	if braceCount != 0 {
		return newInvalidFunctionError(4, "program", "unbalanced braces (count: %d)", braceCount)
	}

	return nil
}

// Embed embeds the function into a PDF file.
func (f *Type4) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out(), "Type 4 functions", pdf.V1_3); err != nil {
		return nil, err
	} else if err := f.validate(); err != nil {
		return nil, err
	}

	// Build the function dictionary
	dict := pdf.Dict{
		"FunctionType": pdf.Integer(4),
		"Domain":       arrayFromFloats(f.Domain),
		"Range":        arrayFromFloats(f.Range),
	}

	// Wrap program in braces
	program := "{" + f.Program + "}"

	// Create stream with PostScript program
	ref := rm.Alloc()
	var filters []pdf.Filter
	if !rm.Out().GetOptions().HasAny(pdf.OptPretty) {
		filters = append(filters, &pdf.FilterCompress{})
	}
	stm, err := rm.Out().OpenStream(ref, dict, filters...)
	if err != nil {
		return nil, err
	}

	_, err = stm.Write([]byte(program))
	if err != nil {
		return nil, err
	}

	err = stm.Close()
	if err != nil {
		return nil, err
	}

	return ref, nil
}

// Reset clears the compiled bytecode cache.
// This must be called if Program is modified after the first Apply call.
func (f *Type4) Reset() {
	f.compiled = nil
	f.compileErr = nil
	f.stack = nil
}

// Apply applies the function to the given input values
// and writes the output values into out.
func (f *Type4) Apply(out []float64, inputs ...float64) {
	m, n := f.Shape()
	if len(inputs) != m {
		panic(fmt.Sprintf("expected %d inputs, got %d", m, len(inputs)))
	}
	if len(out) != n {
		panic(fmt.Sprintf("expected %d outputs, got %d", n, len(out)))
	}

	// compile on first use
	if f.compiled == nil && f.compileErr == nil {
		f.compiled, f.compileErr = compile(f.Program)
	}

	clear(out)

	if f.compileErr == nil {
		// reset stack and push clipped inputs
		f.stack = f.stack[:0]
		for i := range m {
			min := f.Domain[2*i]
			max := f.Domain[2*i+1]
			f.stack = append(f.stack, realVal(clip(inputs[i], min, max)))
		}

		var err error
		f.stack, err = execute(f.compiled, f.stack)
		if err == nil {
			// extract outputs from stack (last n values)
			start := 0
			if len(f.stack) > n {
				start = len(f.stack) - n
			}
			for i := range n {
				si := start + i
				if si >= len(f.stack) {
					break
				}
				v := f.stack[si]
				switch v.tag {
				case tagInt:
					out[i] = float64(v.ival)
				case tagReal:
					out[i] = v.fval
				case tagBool:
					if v.ival != 0 {
						out[i] = 1
					}
				}
			}
		}
	}

	// clip outputs to range
	for i := range n {
		min := f.Range[2*i]
		max := f.Range[2*i+1]
		out[i] = clip(out[i], min, max)
	}
}

// Equal reports whether f and other represent the same Type4 function.
func (f *Type4) Equal(other *Type4) bool {
	if f == nil || other == nil {
		return f == other
	}

	if !floatSlicesEqual(f.Domain, other.Domain, floatEpsilon) {
		return false
	}

	if !floatSlicesEqual(f.Range, other.Range, floatEpsilon) {
		return false
	}

	// compare program (exact string comparison)
	if f.Program != other.Program {
		return false
	}

	return true
}
