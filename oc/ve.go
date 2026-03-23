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

// VisibilityExpression represents a PDF visibility expression for optional content.
// This must be one of:
//   - [VisibilityExpressionGroup]
//   - [VisibilityExpressionAnd]
//   - [VisibilityExpressionOr]
//   - [VisibilityExpressionNot]
type VisibilityExpression interface {
	isVisible(*GroupStates) (bool, bool)
	pdf.Embedder
}

var (
	_ VisibilityExpression = (*VisibilityExpressionGroup)(nil)
	_ VisibilityExpression = (*VisibilityExpressionAnd)(nil)
	_ VisibilityExpression = (*VisibilityExpressionOr)(nil)
	_ VisibilityExpression = (*VisibilityExpressionNot)(nil)
)

// ExtractVisibilityExpression reads a visibility expression from a PDF object.
// The object can be either an array (for And/Or/Not expressions) or a dictionary
// (for a single optional content group reference).
func ExtractVisibilityExpression(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, _ bool) (VisibilityExpression, error) {
	obj, err := x.Resolve(path, obj)
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
			arg, err := pdf.ExtractorGet(x, path, elem, ExtractVisibilityExpression)
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
		}

		op, err := pdf.Optional(x.GetName(path, v[0]))
		if err != nil {
			return nil, err
		}
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
			return &VisibilityExpressionNot{Arg: args[0]}, nil
		default:
			return nil, pdf.Errorf("invalid visibility expression: unknown operator %q", op)
		}
	case pdf.Dict:
		// recover the original reference so the extractor cache is
		// consulted, preserving pointer identity with groups extracted
		// elsewhere (e.g. from OCProperties)
		var groupObj pdf.Object = v
		groupPath := path
		if path != nil {
			groupObj = path.Ref
			groupPath = path.Parent
		}
		g, err := pdf.ExtractorGet(x, groupPath, groupObj, ExtractGroup)
		if err != nil {
			return nil, err
		}
		return &VisibilityExpressionGroup{Group: g}, nil
	default:
		return nil, pdf.Errorf("invalid visibility expression: unexpected %T object", obj)
	}
}

// PDF 2.0 sections: 8.11.2

// VisibilityExpressionGroup represents a visibility expression that references a single group.
type VisibilityExpressionGroup struct {
	Group *Group
}

// isVisible returns the state of the referenced group.
// Non-participating groups return (false, false) to indicate no opinion.
func (g *VisibilityExpressionGroup) isVisible(s *GroupStates) (bool, bool) {
	if !s.Participates(g.Group) {
		return false, false
	}
	return s.IsOn(g.Group), true
}

// Embed converts the group reference to a PDF object reference.
func (g *VisibilityExpressionGroup) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "visibility expressions", pdf.V1_6); err != nil {
		return nil, err
	}
	if g.Group == nil {
		return nil, pdf.Error("VisibilityExpressionGroup.Group is nil")
	}

	groupRef, err := e.Embed(g.Group)
	if err != nil {
		return nil, err
	}
	return groupRef, nil
}

// PDF 2.0 sections: 8.11.2

// VisibilityExpressionAnd represents a logical AND of multiple visibility expressions.
type VisibilityExpressionAnd struct {
	Args []VisibilityExpression
}

// isVisible returns true if all operands with an opinion are active.
// If no operands have an opinion, it returns (false, false).
func (a *VisibilityExpressionAnd) isVisible(s *GroupStates) (bool, bool) {
	anyOpinion := false
	for _, operand := range a.Args {
		val, ok := operand.isVisible(s)
		if !ok {
			continue
		}
		anyOpinion = true
		if !val {
			return false, true
		}
	}
	if !anyOpinion {
		return false, false
	}
	return true, true
}

// Embed converts the AND expression to a PDF array.
func (a *VisibilityExpressionAnd) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "visibility expressions", pdf.V1_6); err != nil {
		return nil, err
	}
	if len(a.Args) == 0 {
		return nil, pdf.Error("VisibilityExpressionAnd requires at least one operand")
	}

	arr := make(pdf.Array, 1+len(a.Args))
	arr[0] = pdf.Name("And")

	for i, operand := range a.Args {
		obj, err := e.Embed(operand)
		if err != nil {
			return nil, err
		}
		arr[i+1] = obj
	}

	return arr, nil
}

// PDF 2.0 sections: 8.11.2

// VisibilityExpressionOr represents a logical OR of multiple visibility expressions.
type VisibilityExpressionOr struct {
	Args []VisibilityExpression
}

// isVisible returns true if any operand with an opinion is active.
// If no operands have an opinion, it returns (false, false).
func (o *VisibilityExpressionOr) isVisible(s *GroupStates) (bool, bool) {
	anyOpinion := false
	for _, operand := range o.Args {
		val, ok := operand.isVisible(s)
		if !ok {
			continue
		}
		anyOpinion = true
		if val {
			return true, true
		}
	}
	if !anyOpinion {
		return false, false
	}
	return false, true
}

// Embed converts the OR expression to a PDF array.
func (o *VisibilityExpressionOr) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "visibility expressions", pdf.V1_6); err != nil {
		return nil, err
	}
	if len(o.Args) == 0 {
		return nil, pdf.Error("VisibilityExpressionOr requires at least one operand")
	}

	arr := make(pdf.Array, 1+len(o.Args))
	arr[0] = pdf.Name("Or")

	for i, operand := range o.Args {
		obj, err := e.Embed(operand)
		if err != nil {
			return nil, err
		}
		arr[i+1] = obj
	}

	return arr, nil
}

// PDF 2.0 sections: 8.11.2

// VisibilityExpressionNot represents a logical NOT of a single visibility expression.
type VisibilityExpressionNot struct {
	Arg VisibilityExpression
}

// isVisible returns the negation of the operand's state.
// If the operand has no opinion, it returns (false, false).
func (n *VisibilityExpressionNot) isVisible(s *GroupStates) (bool, bool) {
	val, ok := n.Arg.isVisible(s)
	if !ok {
		return false, false
	}
	return !val, true
}

// Embed converts the NOT expression to a PDF array.
func (n *VisibilityExpressionNot) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "visibility expressions", pdf.V1_6); err != nil {
		return nil, err
	}
	if n.Arg == nil {
		return nil, pdf.Error("VisibilityExpressionNot requires exactly one operand")
	}

	// embed the operand
	obj, err := e.Embed(n.Arg)
	if err != nil {
		return nil, err
	}

	// create array with operator and operand
	arr := pdf.Array{pdf.Name("Not"), obj}
	return arr, nil
}

// veEqual reports whether two visibility expressions are semantically equal.
func veEqual(a, b VisibilityExpression) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	switch a := a.(type) {
	case *VisibilityExpressionGroup:
		b, ok := b.(*VisibilityExpressionGroup)
		return ok && a.Group.Equal(b.Group)
	case *VisibilityExpressionNot:
		b, ok := b.(*VisibilityExpressionNot)
		return ok && veEqual(a.Arg, b.Arg)
	case *VisibilityExpressionAnd:
		b, ok := b.(*VisibilityExpressionAnd)
		if !ok || len(a.Args) != len(b.Args) {
			return false
		}
		for i := range a.Args {
			if !veEqual(a.Args[i], b.Args[i]) {
				return false
			}
		}
		return true
	case *VisibilityExpressionOr:
		b, ok := b.(*VisibilityExpressionOr)
		if !ok || len(a.Args) != len(b.Args) {
			return false
		}
		for i := range a.Args {
			if !veEqual(a.Args[i], b.Args[i]) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
