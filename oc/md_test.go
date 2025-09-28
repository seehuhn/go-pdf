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

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestMembershipRoundTrip(t *testing.T) {
	tests := []struct {
		name       string
		membership *Membership
	}{
		{
			name: "simple with single OCG and default policy",
			membership: &Membership{
				OCGs: []*Group{
					{Name: "Layer1"},
				},
				Policy: PolicyAnyOn, // default policy
			},
		},
		{
			name: "multiple OCGs with AllOn policy",
			membership: &Membership{
				OCGs: []*Group{
					{Name: "Layer1"},
					{Name: "Layer2"},
					{Name: "Layer3"},
				},
				Policy: PolicyAllOn,
			},
		},
		{
			name: "AllOff policy",
			membership: &Membership{
				OCGs: []*Group{
					{Name: "HiddenLayer"},
				},
				Policy: PolicyAllOff,
			},
		},
		{
			name: "AnyOff policy",
			membership: &Membership{
				OCGs: []*Group{
					{Name: "Layer1"},
					{Name: "Layer2"},
				},
				Policy: PolicyAnyOff,
			},
		},
		{
			name: "with visibility expression - simple And",
			membership: &Membership{
				VE: &VisibilityExpressionAnd{
					Args: []VisibilityExpression{
						&VisibilityExpressionGroup{Group: &Group{Name: "Layer1"}},
						&VisibilityExpressionGroup{Group: &Group{Name: "Layer2"}},
					},
				},
			},
		},
		{
			name: "with visibility expression - complex nested",
			membership: &Membership{
				VE: &VisibilityExpressionOr{
					Args: []VisibilityExpression{
						&VisibilityExpressionGroup{Group: &Group{Name: "Layer1"}},
						&VisibilityExpressionNot{
							Args: &VisibilityExpressionGroup{Group: &Group{Name: "Layer2"}},
						},
						&VisibilityExpressionAnd{
							Args: []VisibilityExpression{
								&VisibilityExpressionGroup{Group: &Group{Name: "Layer3"}},
								&VisibilityExpressionGroup{Group: &Group{Name: "Layer4"}},
							},
						},
					},
				},
			},
		},
		{
			name: "VE with compatibility OCGs",
			membership: &Membership{
				OCGs: []*Group{
					{Name: "Layer1"},
					{Name: "Layer2"},
				},
				Policy: PolicyAllOn,
				VE: &VisibilityExpressionAnd{
					Args: []VisibilityExpression{
						&VisibilityExpressionGroup{Group: &Group{Name: "Layer1"}},
						&VisibilityExpressionGroup{Group: &Group{Name: "Layer2"}},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// test with SingleUse = false (indirect reference)
			tt.membership.SingleUse = false
			testMembershipRoundTrip(t, tt.membership, "indirect")

			// test with SingleUse = true (direct dictionary)
			tt.membership.SingleUse = true
			testMembershipRoundTrip(t, tt.membership, "direct")
		})
	}
}

func testMembershipRoundTrip(t *testing.T, original *Membership, mode string) {
	// Use PDF 2.0 for visibility expressions, 1.7 for basic features
	version := pdf.V1_7
	if original.VE != nil {
		version = pdf.V2_0
	}
	buf, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(buf)

	// embed the membership dictionary
	obj, err := rm.Embed(original)
	if err != nil {
		t.Fatalf("%s: embed: %v", mode, err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("%s: close writer: %v", mode, err)
	}

	// extract the membership dictionary
	extractor := pdf.NewExtractor(buf)
	extracted, err := ExtractMembership(extractor, obj)
	if err != nil {
		t.Fatalf("%s: extract: %v", mode, err)
	}

	// normalize for comparison
	normalizeMembership(original)
	normalizeMembership(extracted)

	if diff := cmp.Diff(extracted, original, cmp.AllowUnexported(Membership{})); diff != "" {
		t.Errorf("%s: round trip failed (-got +want):\n%s", mode, diff)
	}
}

func normalizeMembership(m *Membership) {
	if m == nil {
		return
	}

	// normalize Policy - empty means AnyOn
	if m.Policy == "" {
		m.Policy = PolicyAnyOn
	}

	// normalize Groups
	for _, group := range m.OCGs {
		normalizeGroup(group)
	}

	// normalize VisibilityExpression
	normalizeVisibilityExpression(m.VE)
}

func normalizeVisibilityExpression(ve VisibilityExpression) {
	if ve == nil {
		return
	}

	switch expr := ve.(type) {
	case *VisibilityExpressionGroup:
		normalizeGroup(expr.Group)
	case *VisibilityExpressionAnd:
		for _, operand := range expr.Args {
			normalizeVisibilityExpression(operand)
		}
	case *VisibilityExpressionOr:
		for _, operand := range expr.Args {
			normalizeVisibilityExpression(operand)
		}
	case *VisibilityExpressionNot:
		normalizeVisibilityExpression(expr.Args)
	}
}

func TestMembershipValidation(t *testing.T) {
	buf, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(buf)

	// test empty membership (should fail)
	empty := &Membership{}
	_, err := rm.Embed(empty)
	if err == nil {
		t.Error("expected error for empty membership, got nil")
	}

	// test invalid policy (should fail)
	invalidPolicy := &Membership{
		OCGs: []*Group{
			{Name: "Test"},
		},
		Policy: Policy("Invalid"),
	}
	_, err = rm.Embed(invalidPolicy)
	if err == nil {
		t.Error("expected error for invalid policy, got nil")
	}

	rm.Close()
}

func TestMembershipExtractPermissive(t *testing.T) {
	// test that extraction handles malformed data gracefully
	buf, _ := memfile.NewPDFWriter(pdf.V1_0, nil)
	rm := pdf.NewResourceManager(buf)

	// create membership with invalid policy that should be ignored during extraction
	dict := pdf.Dict{
		"Type": pdf.Name("OCMD"),
		"P":    pdf.Name("InvalidPolicy"), // invalid policy should be ignored
	}

	ref := rm.Out.Alloc()
	err := rm.Out.Put(ref, dict)
	if err != nil {
		t.Fatalf("put dict: %v", err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("close: %v", err)
	}

	extractor := pdf.NewExtractor(buf)
	membership, err := ExtractMembership(extractor, ref)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	// should have default policy due to invalid value being ignored
	if membership.Policy != PolicyAnyOn {
		t.Errorf("expected default policy %s, got %s", PolicyAnyOn, membership.Policy)
	}
}

func TestMembershipPolicyConstants(t *testing.T) {
	policies := []Policy{PolicyAllOn, PolicyAnyOn, PolicyAnyOff, PolicyAllOff}
	expectedValues := []string{"AllOn", "AnyOn", "AnyOff", "AllOff"}

	for i, policy := range policies {
		if string(policy) != expectedValues[i] {
			t.Errorf("policy constant %d: expected %s, got %s", i, expectedValues[i], string(policy))
		}
	}
}

func TestMembershipSingleOCG(t *testing.T) {
	buf, _ := memfile.NewPDFWriter(pdf.V1_0, nil)
	rm := pdf.NewResourceManager(buf)

	// create membership with single OCG
	membership := &Membership{
		OCGs: []*Group{
			{Name: "SingleLayer"},
		},
		Policy:    PolicyAnyOn,
		SingleUse: false,
	}

	obj, err := rm.Embed(membership)
	if err != nil {
		t.Fatalf("embed: %v", err)
	}

	rm.Close()

	extractor := pdf.NewExtractor(buf)

	extracted, err := ExtractMembership(extractor, obj)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if len(extracted.OCGs) != 1 {
		t.Errorf("expected 1 OCG, got %d", len(extracted.OCGs))
	}
	if extracted.OCGs[0].Name != "SingleLayer" {
		t.Errorf("expected OCG name 'SingleLayer', got %s", extracted.OCGs[0].Name)
	}
}

func TestMembershipWithNullOCGs(t *testing.T) {
	// test that null values in OCGs array are ignored per PDF spec
	buf, _ := memfile.NewPDFWriter(pdf.V1_0, nil)
	rm := pdf.NewResourceManager(buf)

	// create a valid group first
	group := &Group{Name: "ValidGroup"}
	groupRef, err := rm.Embed(group)
	if err != nil {
		t.Fatalf("embed group: %v", err)
	}

	// create OCGs array with null values
	ocgsArray := pdf.Array{
		nil,      // null value - should be ignored
		groupRef, // valid group
		nil,      // another null - should be ignored
	}

	dict := pdf.Dict{
		"Type": pdf.Name("OCMD"),
		"OCGs": ocgsArray,
		"P":    pdf.Name("AnyOn"),
	}

	ref := rm.Out.Alloc()
	err = rm.Out.Put(ref, dict)
	if err != nil {
		t.Fatalf("put dict: %v", err)
	}

	rm.Close()

	extractor := pdf.NewExtractor(buf)
	membership, err := ExtractMembership(extractor, ref)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	// should only have one valid group
	if len(membership.OCGs) != 1 {
		t.Errorf("expected 1 OCG after filtering nulls, got %d", len(membership.OCGs))
	}
	if membership.OCGs[0].Name != "ValidGroup" {
		t.Errorf("expected OCG name 'ValidGroup', got %s", membership.OCGs[0].Name)
	}
}
