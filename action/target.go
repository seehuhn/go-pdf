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
	"seehuhn.de/go/pdf"
)

var errTargetCycle = pdf.Error("target dictionary contains cycle")

// Target represents a step in navigating through a hierarchy of embedded PDF files.
// Target dictionaries can be chained via the Next field to specify a path through
// multiple levels of embedding.
// This must be one of [*TargetParent], [*TargetNamedChild], or [*TargetAnnotationChild].
type Target interface {
	pdf.Encoder
	encodeTargetSafe(rm *pdf.ResourceManager, visited map[Target]bool) (pdf.Native, error)
}

// PDF 2.0 sections: 12.6.4.4

// TargetParent navigates up to the parent document.
type TargetParent struct {
	// Next is the next step in the target path, or nil if the parent is the final target.
	Next Target
}

func (t *TargetParent) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	visited := make(map[Target]bool)
	return t.encodeTargetSafe(rm, visited)
}

func (t *TargetParent) encodeTargetSafe(rm *pdf.ResourceManager, visited map[Target]bool) (pdf.Native, error) {
	if visited[t] {
		return nil, errTargetCycle
	}
	visited[t] = true

	dict := pdf.Dict{
		"R": pdf.Name("P"),
	}

	if t.Next != nil {
		nextDict, err := t.Next.encodeTargetSafe(rm, visited)
		if err != nil {
			return nil, err
		}
		dict["T"] = nextDict
	}

	return dict, nil
}

// TargetNamedChild navigates down to a child document by name in the EmbeddedFiles tree.
type TargetNamedChild struct {
	// Name is the name of the file in the EmbeddedFiles name tree.
	Name pdf.String

	// Next is the next step in the target path, or nil if this child is the final target.
	Next Target
}

func (t *TargetNamedChild) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	visited := make(map[Target]bool)
	return t.encodeTargetSafe(rm, visited)
}

func (t *TargetNamedChild) encodeTargetSafe(rm *pdf.ResourceManager, visited map[Target]bool) (pdf.Native, error) {
	if visited[t] {
		return nil, errTargetCycle
	}
	visited[t] = true

	if len(t.Name) == 0 {
		return nil, pdf.Error("TargetNamedChild must have a non-empty Name")
	}

	dict := pdf.Dict{
		"R": pdf.Name("C"),
		"N": t.Name,
	}

	if t.Next != nil {
		nextDict, err := t.Next.encodeTargetSafe(rm, visited)
		if err != nil {
			return nil, err
		}
		dict["T"] = nextDict
	}

	return dict, nil
}

// TargetAnnotationChild navigates down to a child document via a file attachment annotation.
type TargetAnnotationChild struct {
	// Page specifies the page containing the annotation.
	// Can be pdf.Integer (zero-based page number) or pdf.String (named destination).
	Page pdf.Object

	// Annotation specifies which annotation on the page.
	// Can be pdf.Integer (zero-based index in Annots array) or pdf.String (NM field).
	Annotation pdf.Object

	// Next is the next step in the target path, or nil if this child is the final target.
	Next Target
}

func (t *TargetAnnotationChild) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	visited := make(map[Target]bool)
	return t.encodeTargetSafe(rm, visited)
}

func (t *TargetAnnotationChild) encodeTargetSafe(rm *pdf.ResourceManager, visited map[Target]bool) (pdf.Native, error) {
	if visited[t] {
		return nil, errTargetCycle
	}
	visited[t] = true

	if t.Page == nil {
		return nil, pdf.Error("TargetAnnotationChild must have a Page")
	}
	if t.Annotation == nil {
		return nil, pdf.Error("TargetAnnotationChild must have an Annotation")
	}

	dict := pdf.Dict{
		"R": pdf.Name("C"),
		"P": t.Page,
		"A": t.Annotation,
	}

	if t.Next != nil {
		nextDict, err := t.Next.encodeTargetSafe(rm, visited)
		if err != nil {
			return nil, err
		}
		dict["T"] = nextDict
	}

	return dict, nil
}

// DecodeTarget reads a target dictionary from a PDF object.
func DecodeTarget(x *pdf.Extractor, obj pdf.Object) (Target, error) {
	if obj == nil {
		return nil, nil
	}

	dict, err := x.GetDict(obj)
	if err != nil {
		return nil, err
	}

	relationship, err := x.GetName(dict["R"])
	if err != nil {
		return nil, err
	}

	// recursively decode nested target
	var next Target
	if dict["T"] != nil {
		next, err = DecodeTarget(x, dict["T"])
		if err != nil {
			return nil, err
		}
	}

	switch relationship {
	case "P":
		return &TargetParent{
			Next: next,
		}, nil

	case "C":
		// check if it's named or annotation-based
		if name, _ := x.GetString(dict["N"]); name != nil {
			return &TargetNamedChild{
				Name: name,
				Next: next,
			}, nil
		}

		// must be annotation-based
		page := dict["P"]
		annotation := dict["A"]
		if page == nil || annotation == nil {
			return nil, pdf.Error("child target must have either N or both P and A")
		}

		return &TargetAnnotationChild{
			Page:       page,
			Annotation: annotation,
			Next:       next,
		}, nil

	default:
		return nil, pdf.Error("invalid target relationship: " + string(relationship))
	}
}
