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
	"bytes"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

var targetTestCases = []Target{
	// TargetParent
	&TargetParent{},
	&TargetParent{
		Next: &TargetNamedChild{Name: pdf.String("child.pdf")},
	},

	// TargetNamedChild
	&TargetNamedChild{Name: pdf.String("embedded.pdf")},
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

	// TargetAnnotationChild
	&TargetAnnotationChild{
		Page:       pdf.Integer(0),
		Annotation: pdf.Integer(1),
	},
	&TargetAnnotationChild{
		Page:       pdf.String("chapter1"),
		Annotation: pdf.String("attachment1"),
	},
}

func targetTypeName(t Target) string {
	switch t.(type) {
	case *TargetParent:
		return "TargetParent"
	case *TargetNamedChild:
		return "TargetNamedChild"
	case *TargetAnnotationChild:
		return "TargetAnnotationChild"
	default:
		return fmt.Sprintf("%T", t)
	}
}

func testTargetRoundTrip(t *testing.T, version pdf.Version, target Target) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(w)

	encoded, err := target.Encode(rm)
	if err != nil {
		if pdf.IsWrongVersion(err) {
			t.Skip("version not supported")
		}
		t.Fatalf("encode error: %v", err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("rm.Close error: %v", err)
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
	versions := []pdf.Version{pdf.V1_7, pdf.V2_0}

	for _, version := range versions {
		t.Run(version.String(), func(t *testing.T) {
			for _, target := range targetTestCases {
				t.Run(targetTypeName(target), func(t *testing.T) {
					testTargetRoundTrip(t, version, target)
				})
			}
		})
	}
}

func FuzzTargetRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}

	for _, target := range targetTestCases {
		w, buf := memfile.NewPDFWriter(pdf.V1_7, opt)

		err := memfile.AddBlankPage(w)
		if err != nil {
			continue
		}

		rm := pdf.NewResourceManager(w)
		obj, err := target.Encode(rm)
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
		target, err := DecodeTarget(x, obj)
		if err != nil {
			t.Skip("malformed target")
		}

		testTargetRoundTrip(t, pdf.GetVersion(r), target)
	})
}

func TestTargetNamedChildEmptyName(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)

	target := &TargetNamedChild{
		Name: pdf.String(""),
	}

	_, err := target.Encode(rm)
	if err == nil {
		t.Error("expected error for empty Name, got nil")
	}
}

func TestTargetAnnotationChildMissingFields(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
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
