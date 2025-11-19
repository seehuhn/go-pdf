package operator

import "seehuhn.de/go/pdf/graphics"

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
}

func ApplyOperator(state *State, op Operator) error {
	panic("not implemented")
}
