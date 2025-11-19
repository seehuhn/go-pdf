package operator

import (
	"errors"
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/resource"
)

type State struct {
	// Param contains the current values of all graphics parameters.
	// Only those parameters listed in Out are guaranteed to have valid values.
	Param graphics.Parameters

	// In lists all graphics parameters which need to be set before executing
	// a given sequence of operators.
	In graphics.StateBits

	// Out lists all graphics parameters which have been modified
	// by executing a given sequence of operators.
	Out graphics.StateBits

	// CurrentObject lists the current graphics object being constructed.
	CurrentObject graphics.ObjectType

	// stack is the graphics state stack for q/Q
	stack []savedState
}

// argParser provides a scanner-style API for parsing operator arguments
type argParser struct {
	args []pdf.Native
	err  error
}

func (p *argParser) GetFloat() float64 {
	if p.err != nil || len(p.args) == 0 {
		if p.err == nil {
			p.err = errors.New("not enough arguments")
		}
		return 0
	}
	arg := p.args[0]
	p.args = p.args[1:]

	switch x := arg.(type) {
	case pdf.Real:
		return float64(x)
	case pdf.Integer:
		return float64(x)
	default:
		p.err = fmt.Errorf("expected number, got %T", arg)
		return 0
	}
}

func (p *argParser) GetInt() int {
	if p.err != nil || len(p.args) == 0 {
		if p.err == nil {
			p.err = errors.New("not enough arguments")
		}
		return 0
	}
	arg := p.args[0]
	p.args = p.args[1:]

	i, ok := arg.(pdf.Integer)
	if !ok {
		p.err = fmt.Errorf("expected integer, got %T", arg)
		return 0
	}
	return int(i)
}

func (p *argParser) GetName() pdf.Name {
	if p.err != nil || len(p.args) == 0 {
		if p.err == nil {
			p.err = errors.New("not enough arguments")
		}
		return ""
	}
	arg := p.args[0]
	p.args = p.args[1:]

	name, ok := arg.(pdf.Name)
	if !ok {
		p.err = fmt.Errorf("expected name, got %T", arg)
		return ""
	}
	return name
}

func (p *argParser) GetArray() pdf.Array {
	if p.err != nil || len(p.args) == 0 {
		if p.err == nil {
			p.err = errors.New("not enough arguments")
		}
		return nil
	}
	arg := p.args[0]
	p.args = p.args[1:]

	arr, ok := arg.(pdf.Array)
	if !ok {
		p.err = fmt.Errorf("expected array, got %T", arg)
		return nil
	}
	return arr
}

func (p *argParser) GetDict() pdf.Dict {
	if p.err != nil || len(p.args) == 0 {
		if p.err == nil {
			p.err = errors.New("not enough arguments")
		}
		return nil
	}
	arg := p.args[0]
	p.args = p.args[1:]

	dict, ok := arg.(pdf.Dict)
	if !ok {
		p.err = fmt.Errorf("expected dict, got %T", arg)
		return nil
	}
	return dict
}

func (p *argParser) GetString() pdf.String {
	if p.err != nil || len(p.args) == 0 {
		if p.err == nil {
			p.err = errors.New("not enough arguments")
		}
		return nil
	}
	arg := p.args[0]
	p.args = p.args[1:]

	str, ok := arg.(pdf.String)
	if !ok {
		p.err = fmt.Errorf("expected string, got %T", arg)
		return nil
	}
	return str
}

func (p *argParser) Check() error {
	if len(p.args) > 0 {
		return errors.New("too many arguments")
	}
	return p.err
}

// State tracking helpers

func (s *State) markIn(bits graphics.StateBits) {
	s.In |= (bits &^ s.Out)
}

func (s *State) markOut(bits graphics.StateBits) {
	s.Out |= bits
}

// savedState stores graphics state for q/Q operators
type savedState struct {
	param *graphics.Parameters
	out   graphics.StateBits
}

// ApplyOperator applies a single content stream operator to the state
func ApplyOperator(state *State, op Operator, res *resource.Resource) error {
	handler, ok := handlers[op.Name]
	if !ok {
		return fmt.Errorf("unknown operator: %s", op.Name)
	}
	return handler(state, op.Args, res)
}

// opHandler is the function signature for operator handlers
type opHandler func(*State, []pdf.Native, *resource.Resource) error

// handlers maps operator names to their handler functions
var handlers = map[pdf.Name]opHandler{
	// Graphics state
	"q":  handlePushState,
	"Q":  handlePopState,
	"cm": handleConcatMatrix,
	"w":  handleSetLineWidth,
	"J":  handleSetLineCap,
	"j":  handleSetLineJoin,
	"M":  handleSetMiterLimit,
	"d":  handleSetLineDash,
	"ri": handleSetRenderingIntent,
	"i":  handleSetFlatness,
	"gs": handleSetExtGState,
}

// handlePushState implements the q operator (save graphics state)
func handlePushState(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.stack == nil {
		s.stack = make([]savedState, 0, 4)
	}
	s.stack = append(s.stack, savedState{
		param: s.Param.Clone(),
		out:   s.Out,
	})
	return nil
}
