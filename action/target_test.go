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
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/mock"
	"seehuhn.de/go/pdf/internal/limits"
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

	w, _ := memfile.NewPDFWriter(t, version, nil)
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
	decoded, err := DecodeTarget(pdf.CursorAt(x, nil), encoded, false)
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
		w, buf := memfile.NewPDFWriter(f, pdf.V1_7, opt)

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
		r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}

		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("missing test object")
		}

		x := pdf.NewExtractor(r)
		target, err := DecodeTarget(pdf.CursorAt(x, nil), obj, false)
		if err != nil {
			t.Skip("malformed target")
		}

		// reading accepts versions the writer cannot reproduce
		version := pdf.GetVersion(r)
		if !version.IsSupported() {
			t.Skip("version cannot be written")
		}

		testTargetRoundTrip(t, version, target)
	})
}

func TestTargetNamedChildEmptyName(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
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
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
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
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)

	// Create a cycle: t1 -> t2 -> t1
	t1 := &TargetParent{}
	t2 := &TargetNamedChild{Name: pdf.String("embedded")}
	t1.Next = t2
	t2.Next = t1

	_, err := t1.Encode(rm)
	if !errors.Is(err, pdf.ErrCycle) {
		t.Errorf("expected cycle error, got %v", err)
	}
}

// TestDecodeTargetCycleSelf checks that a target dictionary which references
// itself via /T fails with a cycle error.
func TestDecodeTargetCycleSelf(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)

	ref := w.Alloc()
	err := w.Put(ref, pdf.Dict{
		"R": pdf.Name("P"),
		"T": ref,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	if _, err := pdf.CursorAt(x, nil).Decode(ref, DecodeTarget); !errors.Is(err, pdf.ErrCycle) {
		t.Errorf("got %v, want cycle error", err)
	}
}

// TestDecodeTargetCycleMutual checks that two target dictionaries which
// reference each other via /T fail with a cycle error.
func TestDecodeTargetCycleMutual(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)

	refA := w.Alloc()
	refB := w.Alloc()
	if err := w.Put(refA, pdf.Dict{"R": pdf.Name("P"), "T": refB}); err != nil {
		t.Fatal(err)
	}
	if err := w.Put(refB, pdf.Dict{"R": pdf.Name("P"), "T": refA}); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	if _, err := pdf.CursorAt(x, nil).Decode(refA, DecodeTarget); !errors.Is(err, pdf.ErrCycle) {
		t.Errorf("got %v, want cycle error", err)
	}
}

// TestDecodeTargetDeepChainBounded guards against a stack-overflow DoS: a /T
// target chain of distinct dictionaries is acyclic, so the cycle guard never
// trips, yet recursing one frame per level would exhaust the Go stack. The
// Decode depth cap must fail the target rather than crash.
func TestDecodeTargetDeepChainBounded(t *testing.T) {
	depth := limits.MaxExtractDepth + 10
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)

	refs := make([]pdf.Reference, depth)
	for i := range refs {
		refs[i] = w.Alloc()
	}
	for i, ref := range refs {
		d := pdf.Dict{"R": pdf.Name("P")}
		if i+1 < depth {
			d["T"] = refs[i+1]
		}
		if err := w.Put(ref, d); err != nil {
			t.Fatal(err)
		}
	}

	x := pdf.NewExtractor(w)
	if _, err := pdf.CursorAt(x, nil).Decode(refs[0], DecodeTarget); !errors.Is(err, pdf.ErrDepth) {
		t.Errorf("got %v, want depth error", err)
	}
}

// TestDecodeTargetInvalidNestedT checks that a nested /T entry which is
// present but broken fails the whole target, while a reference to a
// nonexistent object reads like an absent entry.
func TestDecodeTargetInvalidNestedT(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)
	dict := pdf.Dict{"R": pdf.Name("C"), "N": pdf.String("a.pdf"), "T": pdf.NewReference(99, 0)}
	got, err := DecodeTarget(c, dict, true)
	if err != nil {
		t.Fatal(err)
	}
	want := Target(&TargetNamedChild{Name: pdf.String("a.pdf")})
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("dangling T read differently from absent entry (-want +got):\n%s", diff)
	}

	broken := []pdf.Object{
		pdf.Name("x"),
		pdf.Integer(5),
		pdf.Dict{"R": pdf.Name("bogus")},
		pdf.Dict{"R": pdf.Name("C")},
	}
	for _, bad := range broken {
		dict := pdf.Dict{"R": pdf.Name("C"), "N": pdf.String("a.pdf"), "T": bad}
		if _, err := DecodeTarget(c, dict, true); !pdf.IsMalformed(err) {
			t.Errorf("T = %v: got %v, want malformed error", bad, err)
		}
	}
}

// TestDecodeTargetReadError checks that a genuine read error is returned
// when reading the N entry, instead of falling through to P and A.
func TestDecodeTargetReadError(t *testing.T) {
	c := pdf.NewCursor(failGetter{})
	dict := pdf.Dict{"R": pdf.Name("C"), "N": pdf.NewReference(1, 0)}
	if _, err := DecodeTarget(c, dict, true); !errors.Is(err, errTestRead) {
		t.Errorf("err = %v, want read error", err)
	}
}

// TestDanglingTargetEntry checks that a reference to a nonexistent object
// does not satisfy the P and A entries of an annotation-based child target.
func TestDanglingTargetEntry(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)
	dangling := pdf.NewReference(99, 0)
	for _, key := range []pdf.Name{"P", "A"} {
		t.Run(string(key), func(t *testing.T) {
			dict := pdf.Dict{
				"R": pdf.Name("C"),
				"P": pdf.Integer(0),
				"A": pdf.Integer(1),
			}
			dict[key] = dangling
			if _, err := DecodeTarget(c, dict, true); err == nil {
				t.Error("missing error")
			}
		})
	}
}

// TestDecodeTargetEmptyName checks that an empty /N is ignored, so that the
// target is decoded from /P and /A instead.
func TestDecodeTargetEmptyName(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)
	dict := pdf.Dict{
		"R": pdf.Name("C"),
		"N": pdf.String(""),
		"P": pdf.Integer(0),
		"A": pdf.Integer(1),
	}
	got, err := DecodeTarget(c, dict, true)
	if err != nil {
		t.Fatal(err)
	}
	want := &TargetAnnotationChild{Page: pdf.Integer(0), Annotation: pdf.Integer(1)}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("wrong target (-want +got):\n%s", diff)
	}
}

// TestDecodeTargetPageAnnotation checks that P and A of an annotation-based
// child target are read as a non-negative integer or a string.
func TestDecodeTargetPageAnnotation(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	intRef := w.Alloc()
	if err := w.Put(intRef, pdf.Integer(3)); err != nil {
		t.Fatal(err)
	}
	strRef := w.Alloc()
	if err := w.Put(strRef, pdf.String("name")); err != nil {
		t.Fatal(err)
	}
	c := pdf.NewCursor(w)

	valid := map[string]struct{ in, want pdf.Object }{
		"integer":           {pdf.Integer(0), pdf.Integer(0)},
		"real":              {pdf.Real(2.0), pdf.Integer(2)},
		"string":            {pdf.String("name"), pdf.String("name")},
		"integer reference": {intRef, pdf.Integer(3)},
		"string reference":  {strRef, pdf.String("name")},
	}
	for _, key := range []pdf.Name{"P", "A"} {
		for name, tc := range valid {
			t.Run(string(key)+" "+name, func(t *testing.T) {
				dict := pdf.Dict{"R": pdf.Name("C"), "P": pdf.Integer(1), "A": pdf.Integer(2)}
				dict[key] = tc.in
				got, err := DecodeTarget(c, dict, true)
				if err != nil {
					t.Fatal(err)
				}
				child := got.(*TargetAnnotationChild)
				value := child.Page
				if key == "A" {
					value = child.Annotation
				}
				if diff := cmp.Diff(tc.want, value); diff != "" {
					t.Errorf("wrong value (-want +got):\n%s", diff)
				}

				// read-write-read
				encoded, err := got.Encode(pdf.NewResourceManager(w))
				if err != nil {
					t.Fatal(err)
				}
				again, err := DecodeTarget(c, encoded, true)
				if err != nil {
					t.Fatal(err)
				}
				if diff := cmp.Diff(got, again); diff != "" {
					t.Errorf("round trip failed (-first +second):\n%s", diff)
				}
			})
		}
	}

	invalid := map[string]pdf.Object{
		"negative": pdf.Integer(-1),
		"name":     pdf.Name("x"),
		"boolean":  pdf.Boolean(true),
		"array":    pdf.Array{pdf.Integer(1)},
		"dict":     pdf.Dict{},
		"null":     nil,
	}
	for _, key := range []pdf.Name{"P", "A"} {
		for name, value := range invalid {
			t.Run(string(key)+" invalid "+name, func(t *testing.T) {
				dict := pdf.Dict{"R": pdf.Name("C"), "P": pdf.Integer(1), "A": pdf.Integer(2)}
				dict[key] = value
				if _, err := DecodeTarget(c, dict, true); !pdf.IsMalformed(err) {
					t.Errorf("got %v, want malformed error", err)
				}
			})
		}
	}
}

// TestTargetAnnotationChildEncodeInvalid checks that a Page or Annotation of
// the wrong type is rejected on write.
func TestTargetAnnotationChildEncodeInvalid(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	for name, value := range map[string]pdf.Object{
		"negative":  pdf.Integer(-1),
		"name":      pdf.Name("x"),
		"real":      pdf.Real(1.5),
		"array":     pdf.Array{pdf.Integer(1)},
		"reference": pdf.NewReference(1, 0),
	} {
		for _, bad := range []*TargetAnnotationChild{
			{Page: value, Annotation: pdf.Integer(0)},
			{Page: pdf.Integer(0), Annotation: value},
		} {
			if _, err := bad.Encode(rm); err == nil {
				t.Errorf("%s: encode succeeded", name)
			}
		}
	}
}

// TestDecodeTargetInvalidN checks that an N entry which is not a string
// reads like an absent entry.
func TestDecodeTargetInvalidN(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)
	for name, bad := range map[string]pdf.Object{
		"name":     pdf.Name("x"),
		"integer":  pdf.Integer(5),
		"array":    pdf.Array{pdf.String("a.pdf")},
		"dangling": pdf.NewReference(99, 0),
	} {
		t.Run(name, func(t *testing.T) {
			dict := pdf.Dict{"R": pdf.Name("C"), "N": bad, "P": pdf.Integer(0), "A": pdf.Integer(1)}
			got, err := DecodeTarget(c, dict, true)
			if err != nil {
				t.Fatal(err)
			}
			want := &TargetAnnotationChild{Page: pdf.Integer(0), Annotation: pdf.Integer(1)}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("wrong target (-want +got):\n%s", diff)
			}

			// without P and A nothing is left to locate the child
			delete(dict, "P")
			delete(dict, "A")
			if _, err := DecodeTarget(c, dict, true); !pdf.IsMalformed(err) {
				t.Errorf("got %v, want malformed error", err)
			}
		})
	}
}
