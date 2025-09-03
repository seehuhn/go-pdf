package oc

import "seehuhn.de/go/pdf"

// PDF 2.0 sections: 8.11.2

type VisibilityExpression interface {
	IsActive(map[*Group]bool) bool
	pdf.Embedder[pdf.Unused]
}

var (
	_ VisibilityExpression = (*VisibilityExpressionGroup)(nil)
	_ VisibilityExpression = (*VisibilityExpressionAnd)(nil)
	_ VisibilityExpression = (*VisibilityExpressionOr)(nil)
	_ VisibilityExpression = (*VisibilityExpressionNot)(nil)
)

// VisibilityExpressionGroup represents a visibility expression that references a single group.
type VisibilityExpressionGroup struct {
	Group *Group
}

// IsActive returns the state of the referenced group.
func (g *VisibilityExpressionGroup) IsActive(groupStates map[*Group]bool) bool {
	return groupStates[g.Group]
}

// Embed converts the group reference to a PDF object reference.
func (g *VisibilityExpressionGroup) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused
	if g.Group == nil {
		return nil, zero, pdf.Error("VisibilityExpressionGroup.Group is nil")
	}

	// embed the group using ResourceManager
	groupRef, _, err := pdf.ResourceManagerEmbed(rm, g.Group)
	if err != nil {
		return nil, zero, err
	}
	return groupRef, zero, nil
}

// VisibilityExpressionAnd represents a logical AND of multiple visibility expressions.
type VisibilityExpressionAnd struct {
	Operands []VisibilityExpression
}

// IsActive returns true if all operands are active.
func (a *VisibilityExpressionAnd) IsActive(groupStates map[*Group]bool) bool {
	for _, operand := range a.Operands {
		if !operand.IsActive(groupStates) {
			return false
		}
	}
	return true
}

// Embed converts the AND expression to a PDF array.
func (a *VisibilityExpressionAnd) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused
	if len(a.Operands) == 0 {
		return nil, zero, pdf.Error("VisibilityExpressionAnd requires at least one operand")
	}

	// create array starting with operator
	arr := make(pdf.Array, 1+len(a.Operands))
	arr[0] = pdf.Name("And")

	// embed each operand
	for i, operand := range a.Operands {
		obj, _, err := pdf.ResourceManagerEmbed(rm, operand)
		if err != nil {
			return nil, zero, err
		}
		arr[i+1] = obj
	}

	return arr, zero, nil
}

// VisibilityExpressionOr represents a logical OR of multiple visibility expressions.
type VisibilityExpressionOr struct {
	Operands []VisibilityExpression
}

// IsActive returns true if any operand is active.
func (o *VisibilityExpressionOr) IsActive(groupStates map[*Group]bool) bool {
	for _, operand := range o.Operands {
		if operand.IsActive(groupStates) {
			return true
		}
	}
	return false // empty OR is false
}

// Embed converts the OR expression to a PDF array.
func (o *VisibilityExpressionOr) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused
	if len(o.Operands) == 0 {
		return nil, zero, pdf.Error("VisibilityExpressionOr requires at least one operand")
	}

	// create array starting with operator
	arr := make(pdf.Array, 1+len(o.Operands))
	arr[0] = pdf.Name("Or")

	// embed each operand
	for i, operand := range o.Operands {
		obj, _, err := pdf.ResourceManagerEmbed(rm, operand)
		if err != nil {
			return nil, zero, err
		}
		arr[i+1] = obj
	}

	return arr, zero, nil
}

// VisibilityExpressionNot represents a logical NOT of a single visibility expression.
type VisibilityExpressionNot struct {
	Operand VisibilityExpression
}

// IsActive returns the negation of the operand's state.
func (n *VisibilityExpressionNot) IsActive(groupStates map[*Group]bool) bool {
	return !n.Operand.IsActive(groupStates)
}

// Embed converts the NOT expression to a PDF array.
func (n *VisibilityExpressionNot) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused
	if n.Operand == nil {
		return nil, zero, pdf.Error("VisibilityExpressionNot requires exactly one operand")
	}

	// embed the operand
	obj, _, err := pdf.ResourceManagerEmbed(rm, n.Operand)
	if err != nil {
		return nil, zero, err
	}

	// create array with operator and operand
	arr := pdf.Array{pdf.Name("Not"), obj}
	return arr, zero, nil
}
