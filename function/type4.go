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
	"math"
	"strconv"
	"strings"

	"seehuhn.de/go/pdf"
)

// Type4 represents a Type 4 PostScript calculator function that uses a subset
// of PostScript language to define arbitrary calculations.
type Type4 struct {
	// Domain defines the valid input ranges as [min0, max0, min1, max1, ...]
	Domain []float64

	// Range defines the valid output ranges as [min0, max0, min1, max1, ...]
	Range []float64

	// Program contains the PostScript code (without enclosing braces)
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
		clippedInputs[i] = clipValue(inputs[i], min, max)
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
		outputs[i] = clipValue(outputs[i], min, max)
	}

	return outputs
}

// postScriptStack represents a PostScript operand stack
type postScriptStack struct {
	values []float64
}

func (s *postScriptStack) push(val float64) {
	s.values = append(s.values, val)
}

func (s *postScriptStack) pop() (float64, error) {
	if len(s.values) == 0 {
		return 0, errors.New("stack underflow")
	}
	val := s.values[len(s.values)-1]
	s.values = s.values[:len(s.values)-1]
	return val, nil
}

func (s *postScriptStack) peek() (float64, error) {
	if len(s.values) == 0 {
		return 0, errors.New("stack underflow")
	}
	return s.values[len(s.values)-1], nil
}

// executePostScript executes the PostScript program with the given inputs
func (f *Type4) executePostScript(inputs []float64) ([]float64, error) {
	// Initialize stack with input values
	stack := &postScriptStack{}
	for _, input := range inputs {
		stack.push(input)
	}

	// Tokenize and execute the program
	tokens := f.tokenize(f.Program)
	err := f.executeTokens(tokens, stack, 0)
	if err != nil {
		return nil, err
	}

	// Return remaining stack values as outputs
	return stack.values, nil
}

// tokenize splits the PostScript program into tokens
func (f *Type4) tokenize(program string) []string {
	// Simple tokenization - split by whitespace and handle braces
	program = strings.TrimSpace(program)
	tokens := strings.Fields(program)

	// Expand tokens that contain braces
	var result []string
	for _, token := range tokens {
		if strings.Contains(token, "{") || strings.Contains(token, "}") {
			// Split tokens containing braces
			for i, char := range token {
				if char == '{' || char == '}' {
					if i > 0 {
						result = append(result, token[:i])
					}
					result = append(result, string(char))
					if i < len(token)-1 {
						result = append(result, token[i+1:])
					}
					break
				}
			}
		} else {
			result = append(result, token)
		}
	}

	return result
}

// executeTokens executes a sequence of PostScript tokens
func (f *Type4) executeTokens(tokens []string, stack *postScriptStack, nestingLevel int) error {
	if nestingLevel > 255 {
		return errors.New("maximum nesting depth exceeded")
	}

	for i := 0; i < len(tokens); i++ {
		token := tokens[i]

		if token == "{" {
			// Find matching closing brace
			braceCount := 1
			start := i + 1
			end := start
			for end < len(tokens) && braceCount > 0 {
				switch tokens[end] {
				case "{":
					braceCount++
				case "}":
					braceCount--
				}
				end++
			}
			if braceCount > 0 {
				return errors.New("unmatched opening brace")
			}

			// Execute the block
			blockTokens := tokens[start : end-1]
			err := f.executeTokens(blockTokens, stack, nestingLevel+1)
			if err != nil {
				return err
			}

			i = end - 1 // Skip to after the closing brace
			continue
		}

		if token == "}" {
			return errors.New("unmatched closing brace")
		}

		// Try to parse as number
		if val, err := strconv.ParseFloat(token, 64); err == nil {
			stack.push(val)
			continue
		}

		// Execute operator
		err := f.executeOperator(token, stack)
		if err != nil {
			return err
		}
	}

	return nil
}

// executeOperator executes a PostScript operator
func (f *Type4) executeOperator(op string, stack *postScriptStack) error {
	switch op {
	// Arithmetic operators
	case "add":
		b, err := stack.pop()
		if err != nil {
			return err
		}
		a, err := stack.pop()
		if err != nil {
			return err
		}
		stack.push(a + b)

	case "sub":
		b, err := stack.pop()
		if err != nil {
			return err
		}
		a, err := stack.pop()
		if err != nil {
			return err
		}
		stack.push(a - b)

	case "mul":
		b, err := stack.pop()
		if err != nil {
			return err
		}
		a, err := stack.pop()
		if err != nil {
			return err
		}
		stack.push(a * b)

	case "div":
		b, err := stack.pop()
		if err != nil {
			return err
		}
		a, err := stack.pop()
		if err != nil {
			return err
		}
		if b == 0 {
			return errors.New("division by zero")
		}
		stack.push(a / b)

	case "idiv":
		b, err := stack.pop()
		if err != nil {
			return err
		}
		a, err := stack.pop()
		if err != nil {
			return err
		}
		if b == 0 {
			return errors.New("division by zero")
		}
		stack.push(math.Floor(a / b))

	case "mod":
		b, err := stack.pop()
		if err != nil {
			return err
		}
		a, err := stack.pop()
		if err != nil {
			return err
		}
		if b == 0 {
			return errors.New("modulo by zero")
		}
		stack.push(math.Mod(a, b))

	case "abs":
		a, err := stack.pop()
		if err != nil {
			return err
		}
		stack.push(math.Abs(a))

	case "neg":
		a, err := stack.pop()
		if err != nil {
			return err
		}
		stack.push(-a)

	case "ceiling":
		a, err := stack.pop()
		if err != nil {
			return err
		}
		stack.push(math.Ceil(a))

	case "floor":
		a, err := stack.pop()
		if err != nil {
			return err
		}
		stack.push(math.Floor(a))

	case "round":
		a, err := stack.pop()
		if err != nil {
			return err
		}
		stack.push(math.Round(a))

	case "truncate":
		a, err := stack.pop()
		if err != nil {
			return err
		}
		stack.push(math.Trunc(a))

	case "sqrt":
		a, err := stack.pop()
		if err != nil {
			return err
		}
		stack.push(math.Sqrt(a))

	case "sin":
		a, err := stack.pop()
		if err != nil {
			return err
		}
		stack.push(math.Sin(a))

	case "cos":
		a, err := stack.pop()
		if err != nil {
			return err
		}
		stack.push(math.Cos(a))

	case "atan":
		a, err := stack.pop()
		if err != nil {
			return err
		}
		stack.push(math.Atan(a))

	case "exp":
		a, err := stack.pop()
		if err != nil {
			return err
		}
		stack.push(math.Exp(a))

	case "ln":
		a, err := stack.pop()
		if err != nil {
			return err
		}
		stack.push(math.Log(a))

	case "log":
		a, err := stack.pop()
		if err != nil {
			return err
		}
		stack.push(math.Log10(a))

	case "cvi":
		a, err := stack.pop()
		if err != nil {
			return err
		}
		stack.push(math.Trunc(a))

	case "cvr":
		a, err := stack.pop()
		if err != nil {
			return err
		}
		stack.push(a) // Already a real number

	// Relational and Boolean operators
	case "eq":
		b, err := stack.pop()
		if err != nil {
			return err
		}
		a, err := stack.pop()
		if err != nil {
			return err
		}
		if a == b {
			stack.push(1)
		} else {
			stack.push(0)
		}

	case "ne":
		b, err := stack.pop()
		if err != nil {
			return err
		}
		a, err := stack.pop()
		if err != nil {
			return err
		}
		if a != b {
			stack.push(1)
		} else {
			stack.push(0)
		}

	case "gt":
		b, err := stack.pop()
		if err != nil {
			return err
		}
		a, err := stack.pop()
		if err != nil {
			return err
		}
		if a > b {
			stack.push(1)
		} else {
			stack.push(0)
		}

	case "ge":
		b, err := stack.pop()
		if err != nil {
			return err
		}
		a, err := stack.pop()
		if err != nil {
			return err
		}
		if a >= b {
			stack.push(1)
		} else {
			stack.push(0)
		}

	case "lt":
		b, err := stack.pop()
		if err != nil {
			return err
		}
		a, err := stack.pop()
		if err != nil {
			return err
		}
		if a < b {
			stack.push(1)
		} else {
			stack.push(0)
		}

	case "le":
		b, err := stack.pop()
		if err != nil {
			return err
		}
		a, err := stack.pop()
		if err != nil {
			return err
		}
		if a <= b {
			stack.push(1)
		} else {
			stack.push(0)
		}

	case "and":
		b, err := stack.pop()
		if err != nil {
			return err
		}
		a, err := stack.pop()
		if err != nil {
			return err
		}
		if a != 0 && b != 0 {
			stack.push(1)
		} else {
			stack.push(0)
		}

	case "or":
		b, err := stack.pop()
		if err != nil {
			return err
		}
		a, err := stack.pop()
		if err != nil {
			return err
		}
		if a != 0 || b != 0 {
			stack.push(1)
		} else {
			stack.push(0)
		}

	case "xor":
		b, err := stack.pop()
		if err != nil {
			return err
		}
		a, err := stack.pop()
		if err != nil {
			return err
		}
		if (a != 0) != (b != 0) {
			stack.push(1)
		} else {
			stack.push(0)
		}

	case "not":
		a, err := stack.pop()
		if err != nil {
			return err
		}
		if a == 0 {
			stack.push(1)
		} else {
			stack.push(0)
		}

	case "true":
		stack.push(1)

	case "false":
		stack.push(0)

	// Stack operators
	case "pop":
		_, err := stack.pop()
		if err != nil {
			return err
		}

	case "exch":
		b, err := stack.pop()
		if err != nil {
			return err
		}
		a, err := stack.pop()
		if err != nil {
			return err
		}
		stack.push(b)
		stack.push(a)

	case "dup":
		a, err := stack.peek()
		if err != nil {
			return err
		}
		stack.push(a)

	// Note: if and ifelse are handled during token execution, not here
	// They require special parsing of the code blocks

	default:
		return fmt.Errorf("unknown operator: %s", op)
	}

	return nil
}

// Embed embeds the function into a PDF file.
func (f *Type4) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	if err := pdf.CheckVersion(rm.Out, "Type 4 functions", pdf.V1_3); err != nil {
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
		return errors.New("domain length must be 2*m")
	}
	if len(f.Range) != 2*n {
		return errors.New("range length must be 2*n")
	}

	if f.Program == "" {
		return errors.New("program cannot be empty")
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
		return errors.New("unbalanced braces in program")
	}

	return nil
}

func readType4(r pdf.Getter, stream *pdf.Stream) (*Type4, error) {
	d := stream.Dict
	domain, err := floatsFromPDF(r, d["Domain"])
	if err != nil {
		return nil, fmt.Errorf("failed to read Domain: %w", err)
	}

	rangeArray, err := floatsFromPDF(r, d["Range"])
	if err != nil {
		return nil, fmt.Errorf("failed to read Range: %w", err)
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
