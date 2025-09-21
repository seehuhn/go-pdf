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

package oc

import "seehuhn.de/go/pdf"

// PDF 2.0 sections: 8.11.2

// VisibilityExpression represents a PDF visibility expression for optional content.
// This must be one of:
//   - [VisibilityExpressionGroup]
//   - [VisibilityExpressionAnd]
//   - [VisibilityExpressionOr]
//   - [VisibilityExpressionNot]
type VisibilityExpression interface {
	isVisible(map[*Group]bool) bool
	pdf.Embedder[pdf.Unused]
}

var (
	_ VisibilityExpression = (*VisibilityExpressionGroup)(nil)
	_ VisibilityExpression = (*VisibilityExpressionAnd)(nil)
	_ VisibilityExpression = (*VisibilityExpressionOr)(nil)
	_ VisibilityExpression = (*VisibilityExpressionNot)(nil)
)

func ExtractVisibilityExpression(x *pdf.Extractor, obj pdf.Object) (VisibilityExpression, error) {
	obj, err := pdf.Resolve(x.R, obj)
	if err != nil {
		return nil, err
	}

	switch v := obj.(type) {
	case pdf.Array:
		if len(v) == 0 {
			return nil, pdf.Error("invalid visibility expression: empty array")
		}

		var args []VisibilityExpression
		for _, elem := range v[1:] {
			arg, err := pdf.ExtractorGet(x, elem, ExtractVisibilityExpression)
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
		}

		op, _ := pdf.GetName(x.R, v[0])
		switch op {
		case "And":
			if len(args) == 0 {
				return nil, pdf.Error("invalid visibility expression: missing operands for And")
			}
			return &VisibilityExpressionAnd{Args: args}, nil
		case "Or":
			if len(args) == 0 {
				return nil, pdf.Error("invalid visibility expression: missing operands for Or")
			}
			return &VisibilityExpressionOr{Args: args}, nil
		case "Not":
			if len(args) != 1 {
				return nil, pdf.Error("invalid visibility expression: Not requires exactly one operand")
			}
			return &VisibilityExpressionNot{Args: args[0]}, nil
		default:
			return nil, pdf.Errorf("invalid visibility expression: unknown operator %q", op)
		}
	case pdf.Dict:
		g, err := pdf.ExtractorGet(x, v, ExtractGroup)
		if err != nil {
			return nil, err
		}
		return &VisibilityExpressionGroup{Group: g}, nil
	default:
		return nil, pdf.Errorf("invalid visibility expression: unexpected %T object", obj)
	}
}

// VisibilityExpressionGroup represents a visibility expression that references a single group.
type VisibilityExpressionGroup struct {
	Group *Group
}

// isVisible returns the state of the referenced group.
func (g *VisibilityExpressionGroup) isVisible(states map[*Group]bool) bool {
	return states[g.Group]
}

// Embed converts the group reference to a PDF object reference.
func (g *VisibilityExpressionGroup) Embed(rm *pdf.EmbedHelper) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused
	if g.Group == nil {
		return nil, zero, pdf.Error("VisibilityExpressionGroup.Group is nil")
	}

	// embed the group using ResourceManager
	groupRef, _, err := pdf.EmbedHelperEmbed(rm, g.Group)
	if err != nil {
		return nil, zero, err
	}
	return groupRef, zero, nil
}

// VisibilityExpressionAnd represents a logical AND of multiple visibility expressions.
type VisibilityExpressionAnd struct {
	Args []VisibilityExpression
}

// isVisible returns true if all operands are active.
func (a *VisibilityExpressionAnd) isVisible(groupStates map[*Group]bool) bool {
	for _, operand := range a.Args {
		if !operand.isVisible(groupStates) {
			return false
		}
	}
	return true
}

// Embed converts the AND expression to a PDF array.
func (a *VisibilityExpressionAnd) Embed(rm *pdf.EmbedHelper) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused
	if len(a.Args) == 0 {
		return nil, zero, pdf.Error("VisibilityExpressionAnd requires at least one operand")
	}

	// create array starting with operator
	arr := make(pdf.Array, 1+len(a.Args))
	arr[0] = pdf.Name("And")

	// embed each operand
	for i, operand := range a.Args {
		obj, _, err := pdf.EmbedHelperEmbed(rm, operand)
		if err != nil {
			return nil, zero, err
		}
		arr[i+1] = obj
	}

	return arr, zero, nil
}

// VisibilityExpressionOr represents a logical OR of multiple visibility expressions.
type VisibilityExpressionOr struct {
	Args []VisibilityExpression
}

// isVisible returns true if any operand is active.
func (o *VisibilityExpressionOr) isVisible(groupStates map[*Group]bool) bool {
	for _, operand := range o.Args {
		if operand.isVisible(groupStates) {
			return true
		}
	}
	return false // empty OR is false
}

// Embed converts the OR expression to a PDF array.
func (o *VisibilityExpressionOr) Embed(rm *pdf.EmbedHelper) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused
	if len(o.Args) == 0 {
		return nil, zero, pdf.Error("VisibilityExpressionOr requires at least one operand")
	}

	// create array starting with operator
	arr := make(pdf.Array, 1+len(o.Args))
	arr[0] = pdf.Name("Or")

	// embed each operand
	for i, operand := range o.Args {
		obj, _, err := pdf.EmbedHelperEmbed(rm, operand)
		if err != nil {
			return nil, zero, err
		}
		arr[i+1] = obj
	}

	return arr, zero, nil
}

// VisibilityExpressionNot represents a logical NOT of a single visibility expression.
type VisibilityExpressionNot struct {
	Args VisibilityExpression
}

// isVisible returns the negation of the operand's state.
func (n *VisibilityExpressionNot) isVisible(groupStates map[*Group]bool) bool {
	return !n.Args.isVisible(groupStates)
}

// Embed converts the NOT expression to a PDF array.
func (n *VisibilityExpressionNot) Embed(rm *pdf.EmbedHelper) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused
	if n.Args == nil {
		return nil, zero, pdf.Error("VisibilityExpressionNot requires exactly one operand")
	}

	// embed the operand
	obj, _, err := pdf.EmbedHelperEmbed(rm, n.Args)
	if err != nil {
		return nil, zero, err
	}

	// create array with operator and operand
	arr := pdf.Array{pdf.Name("Not"), obj}
	return arr, zero, nil
}
