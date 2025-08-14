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
	// The length must be 2*m, where m is the number of input variables.
	Domain []float64

	// Range gives clipping ranges for the output variables as [min0, max0, min1, max1, ...].
	// The length must be 2*n, where n is the number of output variables.
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

// GetDomain returns the function's input domain.
func (f *Type4) GetDomain() []float64 {
	return f.Domain
}

// extractType4 reads a Type 4 function from a PDF stream object.
func extractType4(r pdf.Getter, stream *pdf.Stream) (*Type4, error) {
	d := stream.Dict

	var domain []float64
	if domainObj, ok := d["Domain"]; ok {
		var err error
		domain, err = pdf.GetFloatArray(r, domainObj)
		if err != nil {
			return nil, fmt.Errorf("failed to read Domain: %w", err)
		}
	}

	var rangeArray []float64
	if rangeObj, ok := d["Range"]; ok {
		var err error
		rangeArray, err = pdf.GetFloatArray(r, rangeObj)
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
	var filters []pdf.Filter
	if !rm.Out.GetOptions().HasAny(pdf.OptPretty) {
		filters = append(filters, &pdf.FilterCompress{})
	}
	stm, err := rm.Out.OpenStream(ref, dict, filters...)
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

// Apply applies the function to the given input values and returns the output values.
func (f *Type4) Apply(inputs ...float64) []float64 {
	m, n := f.Shape()
	if len(inputs) != m {
		panic(fmt.Sprintf("expected %d inputs, got %d", m, len(inputs)))
	}

	// Clip inputs to domain
	clippedInputs := make([]float64, m)
	for i := range m {
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

	// Clip outputs to range
	for i := range n {
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
	type4Dict := f.makeType4SystemDict()
	intp.DictStack = []postscript.Dict{
		type4Dict,
		{}, // userdict
	}
	intp.SystemDict = type4Dict

	for _, input := range inputs {
		intp.Stack = append(intp.Stack, postscript.Real(input))
	}

	err := intp.ExecuteString(f.Program)
	if err != nil {
		return nil, err
	}

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

	_, n := f.Shape()
	if len(outputs) > n {
		outputs = outputs[len(outputs)-n:] // take last n outputs
	} else {
		for len(outputs) < n {
			outputs = append(outputs, 0.0) // pad with zeros if not enough outputs
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
		"true":  postscript.Boolean(true),
		"false": postscript.Boolean(false),
	}
	for _, name := range allowedOps {
		if impl, exists := systemDict[postscript.Name(name)]; exists {
			type4Dict[postscript.Name(name)] = impl
		}
	}

	return type4Dict
}

// list of operators allowed in Type 4 functions per PDF specification Table 42
var allowedOps = []string{
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
