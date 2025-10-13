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

package action

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

var targetTestCases = []Target{
	// Simple cases
	&TargetParent{},
	&TargetNamedChild{Name: pdf.String("embedded.pdf")},
	&TargetAnnotationChild{
		Page:       pdf.Integer(0),
		Annotation: pdf.Integer(1),
	},
	&TargetAnnotationChild{
		Page:       pdf.String("chapter1"),
		Annotation: pdf.String("attachment1"),
	},

	// Chained cases
	&TargetParent{
		Next: &TargetNamedChild{Name: pdf.String("child.pdf")},
	},
	&TargetNamedChild{
		Name: pdf.String("level1.pdf"),
		Next: &TargetNamedChild{Name: pdf.String("level2.pdf")},
	},
	&TargetNamedChild{
		Name: pdf.String("embedded.pdf"),
		Next: &TargetAnnotationChild{
			Page:       pdf.Integer(5),
			Annotation: pdf.String("attach"),
		},
	},
}

func testTargetRoundTrip(t *testing.T, target Target) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)

	encoded, err := target.Encode(rm)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	x := pdf.NewExtractor(w)
	decoded, err := DecodeTarget(x, encoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if diff := cmp.Diff(decoded, target); diff != "" {
		t.Errorf("round trip failed (-got +want):\n%s", diff)
	}
}

func TestTargetRoundTrip(t *testing.T) {
	for i, target := range targetTestCases {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			testTargetRoundTrip(t, target)
		})
	}
}

func TestTargetParentEncode(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)

	target := &TargetParent{}

	obj, err := target.Encode(rm)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	dict, ok := obj.(pdf.Dict)
	if !ok {
		t.Fatalf("expected Dict, got %T", obj)
	}

	if dict["R"] != pdf.Name("P") {
		t.Errorf("R = %v, want P", dict["R"])
	}
}

func TestTargetNamedChildEncode(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)

	target := &TargetNamedChild{
		Name: pdf.String("embedded.pdf"),
	}

	obj, err := target.Encode(rm)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	dict, ok := obj.(pdf.Dict)
	if !ok {
		t.Fatalf("expected Dict, got %T", obj)
	}

	if dict["R"] != pdf.Name("C") {
		t.Errorf("R = %v, want C", dict["R"])
	}

	name, ok := dict["N"].(pdf.String)
	if !ok {
		t.Fatalf("N is not a pdf.String, got %T", dict["N"])
	}
	if string(name) != "embedded.pdf" {
		t.Errorf("N = %v, want embedded.pdf", name)
	}
}

func TestTargetNamedChildEmptyName(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)

	target := &TargetNamedChild{
		Name: pdf.String(""),
	}

	_, err := target.Encode(rm)
	if err == nil {
		t.Error("expected error for empty Name, got nil")
	}
}

func TestTargetAnnotationChildEncode(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)

	target := &TargetAnnotationChild{
		Page:       pdf.Integer(5),
		Annotation: pdf.String("attach1"),
	}

	obj, err := target.Encode(rm)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	dict, ok := obj.(pdf.Dict)
	if !ok {
		t.Fatalf("expected Dict, got %T", obj)
	}

	if dict["R"] != pdf.Name("C") {
		t.Errorf("R = %v, want C", dict["R"])
	}

	if dict["P"] != pdf.Integer(5) {
		t.Errorf("P = %v, want 5", dict["P"])
	}

	annot, ok := dict["A"].(pdf.String)
	if !ok {
		t.Fatalf("A is not a pdf.String, got %T", dict["A"])
	}
	if string(annot) != "attach1" {
		t.Errorf("A = %v, want attach1", annot)
	}
}

func TestTargetAnnotationChildMissingFields(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)

	tests := []struct {
		name   string
		target *TargetAnnotationChild
	}{
		{"missing page", &TargetAnnotationChild{Annotation: pdf.Integer(0)}},
		{"missing annotation", &TargetAnnotationChild{Page: pdf.Integer(0)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.target.Encode(rm)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestTargetCycle(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)

	// Create a cycle: t1 -> t2 -> t1
	t1 := &TargetParent{}
	t2 := &TargetNamedChild{Name: pdf.String("embedded")}
	t1.Next = t2
	t2.Next = t1

	_, err := t1.Encode(rm)
	if err != errTargetCycle {
		t.Errorf("expected cycle error, got %v", err)
	}
}

func TestDecodeTargetParent(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()

	dict := pdf.Dict{
		"R": pdf.Name("P"),
	}

	x := pdf.NewExtractor(w)
	target, err := DecodeTarget(x, dict)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	parent, ok := target.(*TargetParent)
	if !ok {
		t.Fatalf("expected *TargetParent, got %T", target)
	}

	if parent.Next != nil {
		t.Error("expected Next to be nil")
	}
}

func TestDecodeTargetNamedChild(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()

	dict := pdf.Dict{
		"R": pdf.Name("C"),
		"N": pdf.String("embedded.pdf"),
	}

	x := pdf.NewExtractor(w)
	target, err := DecodeTarget(x, dict)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	child, ok := target.(*TargetNamedChild)
	if !ok {
		t.Fatalf("expected *TargetNamedChild, got %T", target)
	}

	if string(child.Name) != "embedded.pdf" {
		t.Errorf("Name = %v, want embedded.pdf", child.Name)
	}
}

func TestDecodeTargetAnnotationChild(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()

	dict := pdf.Dict{
		"R": pdf.Name("C"),
		"P": pdf.Integer(5),
		"A": pdf.String("attach"),
	}

	x := pdf.NewExtractor(w)
	target, err := DecodeTarget(x, dict)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	child, ok := target.(*TargetAnnotationChild)
	if !ok {
		t.Fatalf("expected *TargetAnnotationChild, got %T", target)
	}

	if child.Page != pdf.Integer(5) {
		t.Errorf("Page = %v, want 5", child.Page)
	}

	annotStr, ok := child.Annotation.(pdf.String)
	if !ok {
		t.Fatalf("Annotation is not a pdf.String, got %T", child.Annotation)
	}
	if string(annotStr) != "attach" {
		t.Errorf("Annotation = %v, want attach", annotStr)
	}
}
