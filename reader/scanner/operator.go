package scanner

import "seehuhn.de/go/pdf"

// Operator represents a PDF operator together with its arguments.
type Operator struct {
	Name     string
	Args     []pdf.Object
	HasError bool
}

// OK returns true if all arguments have been consumed without error.
func (op Operator) OK() bool {
	return !op.HasError && len(op.Args) == 0
}

// GetInteger returns the next argument as an integer.
// In case of an error, HasError is set.
func (op *Operator) GetInteger() pdf.Integer {
	if op.HasError || len(op.Args) == 0 {
		op.HasError = true
		return 0
	}
	arg := op.Args[0]
	op.Args = op.Args[1:]

	if i, ok := arg.(pdf.Integer); ok {
		return i
	}
	op.HasError = true
	return 0
}

// GetNumber returns the next argument as a number.
// In case of an error, HasError is set.
func (op *Operator) GetNumber() float64 {
	if op.HasError || len(op.Args) == 0 {
		op.HasError = true
		return 0
	}
	arg := op.Args[0]
	op.Args = op.Args[1:]

	switch x := arg.(type) {
	case pdf.Real:
		return float64(x)
	case pdf.Integer:
		return float64(x)
	case pdf.Number:
		return float64(x)
	default:
		op.HasError = true
		return 0
	}
}

// GetName returns the next argument as a PDF name.
// In case of an error, HasError is set.
func (op *Operator) GetName() pdf.Name {
	if op.HasError || len(op.Args) == 0 {
		op.HasError = true
		return ""
	}
	arg := op.Args[0]
	op.Args = op.Args[1:]

	if n, ok := arg.(pdf.Name); ok {
		return n
	}
	op.HasError = true
	return ""
}

// GetString returns the next argument as a PDF string.
// In case of an error, HasError is set.
func (op *Operator) GetString() pdf.String {
	if op.HasError || len(op.Args) == 0 {
		op.HasError = true
		return nil
	}
	arg := op.Args[0]
	op.Args = op.Args[1:]

	if s, ok := arg.(pdf.String); ok {
		return s
	}
	op.HasError = true
	return nil
}

// GetArray returns the next argument as a PDF array.
// In case of an error, HasError is set.
func (op *Operator) GetArray() pdf.Array {
	if op.HasError || len(op.Args) == 0 {
		op.HasError = true
		return nil
	}
	arg := op.Args[0]
	op.Args = op.Args[1:]

	if a, ok := arg.(pdf.Array); ok {
		return a
	}
	op.HasError = true
	return nil
}
