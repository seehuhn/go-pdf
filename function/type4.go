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
	"fmt"
	"io"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/postscript"
)

// Type4 represents a Type 4 PostScript calculator function that uses a subset
// of PostScript language to define arbitrary calculations.
type Type4 struct {
	// Domain defines the valid input ranges as [min0, max0, min1, max1, ...].
	Domain []float64

	// Range defines the valid output ranges as [min0, max0, min1, max1, ...].
	Range []float64

	// Program contains the PostScript code (without enclosing braces).
	Program string
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

// Apply applies the function to the given input values and returns the output values.
func (f *Type4) Apply(inputs ...float64) []float64 {
	m, n := f.Shape()
	if len(inputs) != m {
		panic(fmt.Sprintf("expected %d inputs, got %d", m, len(inputs)))
	}

	// Clip inputs to domain
	clippedInputs := make([]float64, m)
	for i := 0; i < m; i++ {
		min := f.Domain[2*i]
		max := f.Domain[2*i+1]
		clippedInputs[i] = clip(inputs[i], min, max)
	}

	// Execute PostScript program
	outputs, err := f.executePostScript(clippedInputs)
	if err != nil {
		// In case of error, return zero values
		outputs = make([]float64, n)
	}

	// Ensure we have the right number of outputs
	if len(outputs) < n {
		// Pad with zeros if not enough outputs
		padded := make([]float64, n)
		copy(padded, outputs)
		outputs = padded
	} else if len(outputs) > n {
		// Truncate if too many outputs
		outputs = outputs[:n]
	}

	// Clip outputs to range
	for i := 0; i < n; i++ {
		min := f.Range[2*i]
		max := f.Range[2*i+1]
		outputs[i] = clip(outputs[i], min, max)
	}

	return outputs
}

// executePostScript executes the PostScript program with the given inputs using
// a restricted PostScript interpreter that only supports PDF Type 4 operators.
func (f *Type4) executePostScript(inputs []float64) ([]float64, error) {
	// create a PostScript interpreter with Type 4 restricted operators
	intp := postscript.NewInterpreter()

	// create a custom system dictionary with only allowed operators
	type4Dict := f.makeType4SystemDict()

	// replace the system dictionary with our restricted one
	intp.DictStack = []postscript.Dict{
		type4Dict,
		{}, // userdict
	}
	intp.SystemDict = type4Dict

	// push input values onto the stack
	for _, input := range inputs {
		intp.Stack = append(intp.Stack, postscript.Real(input))
	}

	// execute program without wrapping - the PostScript interpreter expects the program without outer braces
	err := intp.ExecuteString(f.Program)
	if err != nil {
		return nil, fmt.Errorf("PostScript execution error: %w", err)
	}

	// convert stack values back to float64 slice
	outputs := make([]float64, len(intp.Stack))
	for i, obj := range intp.Stack {
		switch v := obj.(type) {
		case postscript.Integer:
			outputs[i] = float64(v)
		case postscript.Real:
			outputs[i] = float64(v)
		case postscript.Boolean:
			if v {
				outputs[i] = 1.0
			} else {
				outputs[i] = 0.0
			}
		default:
			return nil, fmt.Errorf("invalid result type on stack: %T", obj)
		}
	}

	return outputs, nil
}

// makeType4SystemDict creates a restricted system dictionary for Type 4 functions
// containing only the operators allowed by the PDF specification.
func (f *Type4) makeType4SystemDict() postscript.Dict {
	// get a full system dictionary to copy implementations from
	tempIntp := postscript.NewInterpreter()
	systemDict := tempIntp.SystemDict

	// Create Type 4 dictionary with only allowed operators from Table 42
	// of the PDF 2.0 specification.
	type4Dict := postscript.Dict{
		// add constants
		"true":  postscript.Boolean(true),
		"false": postscript.Boolean(false),
	}

	// list of operators allowed in Type 4 functions per PDF specification Table 42
	allowedOps := []string{
		// Arithmetic operators
		"abs", "add", "atan", "ceiling", "cos", "cvi", "cvr", "div", "exp",
		"floor", "idiv", "ln", "log", "mod", "mul", "neg", "round", "sin",
		"sqrt", "sub", "truncate",

		// Relational, boolean, and bitwise operators
		"and", "bitshift", "eq", "ge", "gt", "le", "lt", "ne", "not", "or", "xor",

		// Conditional operators
		"if", "ifelse",

		// Stack operators
		"copy", "dup", "exch", "index", "pop", "roll",
	}

	// copy the actual implementations for allowed operators
	for _, name := range allowedOps {
		if impl, exists := systemDict[postscript.Name(name)]; exists {
			type4Dict[postscript.Name(name)] = impl
		}
	}

	return type4Dict
}

// Embed embeds the function into a PDF file.
func (f *Type4) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	if err := pdf.CheckVersion(rm.Out, "Type 4 functions", pdf.V1_3); err != nil {
		return nil, zero, err
	} else if err := f.validate(); err != nil {
		return nil, zero, err
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
	ref := rm.Out.Alloc()
	stm, err := rm.Out.OpenStream(ref, dict, pdf.FilterCompress{})
	if err != nil {
		return nil, zero, err
	}

	_, err = stm.Write([]byte(program))
	if err != nil {
		return nil, zero, err
	}

	err = stm.Close()
	if err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
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
	}
	if braceCount != 0 {
		return newInvalidFunctionError(4, "program", "unbalanced braces (count: %d)", braceCount)
	}

	return nil
}

// extractType4 reads a Type 4 function from a PDF stream object.
func extractType4(r pdf.Getter, stream *pdf.Stream) (*Type4, error) {
	d := stream.Dict

	var domain []float64
	if domainObj, ok := d["Domain"]; ok {
		var err error
		domain, err = readFloats(r, domainObj)
		if err != nil {
			return nil, fmt.Errorf("failed to read Domain: %w", err)
		}
	}

	var rangeArray []float64
	if rangeObj, ok := d["Range"]; ok {
		var err error
		rangeArray, err = readFloats(r, rangeObj)
		if err != nil {
			return nil, fmt.Errorf("failed to read Range: %w", err)
		}
	}

	f := &Type4{
		Domain: domain,
		Range:  rangeArray,
	}

	// Read PostScript program from stream
	stmReader, err := pdf.DecodeStream(r, stream, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to decode stream: %w", err)
	}
	defer stmReader.Close()

	programBytes, err := io.ReadAll(stmReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read program: %w", err)
	}

	program := string(programBytes)
	// Remove surrounding braces if present
	if len(program) >= 2 && program[0] == '{' && program[len(program)-1] == '}' {
		program = program[1 : len(program)-1]
	}
	f.Program = program

	return f, nil
}
