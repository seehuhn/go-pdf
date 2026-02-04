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
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

var membershipTestCases = []struct {
	name       string
	version    pdf.Version
	membership *Membership
}{
	{
		name:    "simple with single OCG and default policy",
		version: pdf.V1_7,
		membership: &Membership{
			OCGs: []*Group{
				{Name: "Layer1"},
			},
			Policy: PolicyAnyOn,
		},
	},
	{
		name:    "multiple OCGs with AllOn policy",
		version: pdf.V1_7,
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
		name:    "AllOff policy",
		version: pdf.V1_7,
		membership: &Membership{
			OCGs: []*Group{
				{Name: "HiddenLayer"},
			},
			Policy: PolicyAllOff,
		},
	},
	{
		name:    "AnyOff policy",
		version: pdf.V1_7,
		membership: &Membership{
			OCGs: []*Group{
				{Name: "Layer1"},
				{Name: "Layer2"},
			},
			Policy: PolicyAnyOff,
		},
	},
	{
		name:    "with visibility expression - simple And",
		version: pdf.V2_0,
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
		name:    "with visibility expression - complex nested",
		version: pdf.V2_0,
		membership: &Membership{
			VE: &VisibilityExpressionOr{
				Args: []VisibilityExpression{
					&VisibilityExpressionGroup{Group: &Group{Name: "Layer1"}},
					&VisibilityExpressionNot{
						Arg: &VisibilityExpressionGroup{Group: &Group{Name: "Layer2"}},
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
		name:    "VE with compatibility OCGs",
		version: pdf.V2_0,
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

func TestMembershipRoundTrip(t *testing.T) {
	for _, tc := range membershipTestCases {
		t.Run(tc.name, func(t *testing.T) {
			// test with SingleUse = false (indirect reference)
			tc.membership.SingleUse = false
			testMembershipRoundTrip(t, tc.version, tc.membership)

			// test with SingleUse = true (direct dictionary)
			tc.membership.SingleUse = true
			testMembershipRoundTrip(t, tc.version, tc.membership)
		})
	}
}

func testMembershipRoundTrip(t *testing.T, version pdf.Version, original *Membership) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(w)

	// embed the membership dictionary
	obj, err := rm.Embed(original)
	if err != nil {
		if pdf.IsWrongVersion(err) {
			t.Skip("version not supported")
		}
		t.Fatalf("embed: %v", err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("rm.Close: %v", err)
	}

	err = w.Close()
	if err != nil {
		t.Fatalf("w.Close: %v", err)
	}

	// extract the membership dictionary
	extractor := pdf.NewExtractor(w)
	extracted, err := pdf.ExtractorGet(extractor, obj, ExtractMembership)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	// normalize for comparison
	normalizeMembership(original)
	normalizeMembership(extracted)

	if diff := cmp.Diff(extracted, original, cmp.AllowUnexported(Membership{})); diff != "" {
		t.Errorf("round trip failed (-got +want):\n%s", diff)
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
		normalizeVisibilityExpression(expr.Arg)
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

	// create a minimal OCG
	ocgDict := pdf.Dict{
		"Type": pdf.Name("OCG"),
		"Name": pdf.TextString("Test"),
	}
	ocgRef := rm.Out.Alloc()
	err := rm.Out.Put(ocgRef, ocgDict)
	if err != nil {
		t.Fatalf("put ocg: %v", err)
	}

	// create membership with invalid policy that should be ignored during extraction
	dict := pdf.Dict{
		"Type": pdf.Name("OCMD"),
		"OCGs": ocgRef,
		"P":    pdf.Name("InvalidPolicy"), // invalid policy should be ignored
	}

	ref := rm.Out.Alloc()
	err = rm.Out.Put(ref, dict)
	if err != nil {
		t.Fatalf("put dict: %v", err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("close: %v", err)
	}

	extractor := pdf.NewExtractor(buf)
	membership, err := pdf.ExtractorGet(extractor, ref, ExtractMembership)
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

	extracted, err := pdf.ExtractorGet(extractor, obj, ExtractMembership)
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
	membership, err := pdf.ExtractorGet(extractor, ref, ExtractMembership)
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

func FuzzMembershipRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}

	// build seed corpus from test cases
	for _, tc := range membershipTestCases {
		for _, singleUse := range []bool{false, true} {
			tc.membership.SingleUse = singleUse

			w, buf := memfile.NewPDFWriter(tc.version, opt)

			err := memfile.AddBlankPage(w)
			if err != nil {
				continue
			}

			rm := pdf.NewResourceManager(w)

			obj, err := rm.Embed(tc.membership)
			if err != nil {
				continue
			}

			err = rm.Close()
			if err != nil {
				continue
			}

			w.GetMeta().Trailer["Quir:E"] = obj
			err = w.Close()
			if err != nil {
				continue
			}

			f.Add(buf.Data)
		}
	}

	// fuzz function: read-write-read cycle
	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}

		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("missing test object")
		}

		x := pdf.NewExtractor(r)
		data, err := pdf.ExtractorGet(x, obj, ExtractMembership)
		if err != nil {
			t.Skip("malformed object")
		}

		testMembershipRoundTrip(t, pdf.GetVersion(r), data)
	})
}
