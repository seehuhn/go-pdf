// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package main

import (
	"maps"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// fpFile is an in-memory PDF file for fingerprint tests.
type fpFile struct {
	t *testing.T
	w *pdf.Writer
}

func newFPFile(t *testing.T) *fpFile {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	// shift the object numbers, so that equal objects in different files
	// have different references
	for range 3 {
		w.Alloc()
	}
	return &fpFile{t: t, w: w}
}

func (f *fpFile) put(obj pdf.Object) pdf.Reference {
	ref := f.w.Alloc()
	if err := f.w.Put(ref, obj); err != nil {
		f.t.Fatal(err)
	}
	return ref
}

func (f *fpFile) stream(data string, filters ...pdf.Filter) pdf.Reference {
	ref := f.w.Alloc()
	s, err := f.w.OpenStream(ref, pdf.Dict{"Type": pdf.Name("Test")}, filters...)
	if err != nil {
		f.t.Fatal(err)
	}
	if _, err := s.Write([]byte(data)); err != nil {
		f.t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		f.t.Fatal(err)
	}
	return ref
}

// fingerprint computes the fingerprint of obj, as resource /Test /R.
func (f *fpFile) fingerprint(obj pdf.Object, budget int64) (resourceSum, bool, error) {
	dr := map[pdf.Name]pdf.Dict{"Test": {"R": obj}}
	return newFingerprinter(f.w, dr, budget).sum("Test", "R")
}

func (f *fpFile) sum(obj pdf.Object) resourceSum {
	sum, ok, err := f.fingerprint(obj, 1<<20)
	if err != nil || !ok {
		f.t.Fatalf("fingerprint: %v, %v", ok, err)
	}
	return sum
}

func TestFingerprint(t *testing.T) {
	type build func(f *fpFile) pdf.Object
	font := func(base string) build {
		return func(f *fpFile) pdf.Object {
			desc := f.put(pdf.Dict{"FontName": pdf.Name(base), "Flags": pdf.Integer(32)})
			return f.put(pdf.Dict{
				"Type":           pdf.Name("Font"),
				"BaseFont":       pdf.Name(base),
				"FontDescriptor": desc,
			})
		}
	}
	cycle := func(f *fpFile) pdf.Object {
		a, b := f.w.Alloc(), f.w.Alloc()
		if err := f.w.Put(a, pdf.Dict{"Next": b}); err != nil {
			f.t.Fatal(err)
		}
		if err := f.w.Put(b, pdf.Dict{"Next": a}); err != nil {
			f.t.Fatal(err)
		}
		return a
	}
	cases := []struct {
		name  string
		a, b  build
		equal bool
	}{
		{"same font", font("Helvetica"), font("Helvetica"), true},
		{"different font", font("Helvetica"), font("Courier"), false},
		{
			"direct and indirect",
			func(f *fpFile) pdf.Object { return pdf.Dict{"A": pdf.Integer(1)} },
			func(f *fpFile) pdf.Object { return f.put(pdf.Dict{"A": pdf.Integer(1)}) },
			false,
		},
		{
			"integer and real",
			func(f *fpFile) pdf.Object { return pdf.Array{pdf.Integer(1)} },
			func(f *fpFile) pdf.Object { return pdf.Array{pdf.Real(1)} },
			false,
		},
		{
			"name and string",
			func(f *fpFile) pdf.Object { return pdf.Array{pdf.Name("x")} },
			func(f *fpFile) pdf.Object { return pdf.Array{pdf.String("x")} },
			false,
		},
		{
			"booleans",
			func(f *fpFile) pdf.Object { return pdf.Array{pdf.Boolean(true)} },
			func(f *fpFile) pdf.Object { return pdf.Array{pdf.Boolean(false)} },
			false,
		},
		{
			"null element",
			func(f *fpFile) pdf.Object { return pdf.Array{nil} },
			func(f *fpFile) pdf.Object { return pdf.Array{} },
			false,
		},
		{
			"nil entry ignored",
			func(f *fpFile) pdf.Object { return pdf.Dict{"A": pdf.Integer(1), "B": nil} },
			func(f *fpFile) pdf.Object { return pdf.Dict{"A": pdf.Integer(1)} },
			true,
		},
		{
			"stream encoding ignored",
			func(f *fpFile) pdf.Object { return f.stream("hello") },
			func(f *fpFile) pdf.Object { return f.stream("hello", pdf.FilterFlate{}) },
			true,
		},
		{
			"stream data compared",
			func(f *fpFile) pdf.Object { return f.stream("hello") },
			func(f *fpFile) pdf.Object { return f.stream("world") },
			false,
		},
		{
			"shared and duplicated",
			func(f *fpFile) pdf.Object {
				x := f.put(pdf.Integer(1))
				return pdf.Array{x, x}
			},
			func(f *fpFile) pdf.Object {
				return pdf.Array{f.put(pdf.Integer(1)), f.put(pdf.Integer(1))}
			},
			false,
		},
		{"cycle", cycle, cycle, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fa, fb := newFPFile(t), newFPFile(t)
			fb.w.Alloc()
			sa := fa.sum(c.a(fa))
			sb := fb.sum(c.b(fb))
			if (sa == sb) != c.equal {
				t.Errorf("fingerprints equal = %v, want %v", sa == sb, c.equal)
			}
		})
	}
}

// TestFingerprintMalformed checks that an object with an undecodable
// stream has no fingerprint.
func TestFingerprintMalformed(t *testing.T) {
	f := newFPFile(t)
	ref := f.w.Alloc()
	s, err := f.w.OpenStream(ref, pdf.Dict{"Filter": pdf.Name("FlateDecode")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Write([]byte("not flate data")); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	_, ok, err := f.fingerprint(pdf.Dict{"S": ref}, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("malformed stream has a fingerprint")
	}
}

// TestFingerprintBudget checks that a resource has no fingerprint once the
// budget is used up, and that the budget is shared by all resources of an
// input.
func TestFingerprintBudget(t *testing.T) {
	f := newFPFile(t)
	big := f.stream(string(make([]byte, 1000)), pdf.FilterFlate{})
	small := pdf.Dict{"A": pdf.Integer(1)}

	if _, ok, err := f.fingerprint(big, 500); err != nil || ok {
		t.Errorf("stream over budget: ok = %v, err = %v", ok, err)
	}
	if _, ok, err := f.fingerprint(big, 2000); err != nil || !ok {
		t.Errorf("stream within budget: ok = %v, err = %v", ok, err)
	}

	dr := map[pdf.Name]pdf.Dict{"Test": {"Big": big, "Small": small}}
	fp := newFingerprinter(f.w, dr, 1100)
	if _, ok, err := fp.sum("Test", "Big"); err != nil || !ok {
		t.Fatalf("first resource: ok = %v, err = %v", ok, err)
	}
	if _, ok, err := fp.sum("Test", "Big"); err != nil || ok {
		t.Errorf("budget not shared: ok = %v, err = %v", ok, err)
	}
	if _, ok, err := fp.sum("Test", "Small"); err != nil || ok {
		t.Errorf("exhausted budget: ok = %v, err = %v", ok, err)
	}
}

// TestFingerprintDA checks that a /DA string enters a fingerprint through
// the resources it names, not through their names.
func TestFingerprintDA(t *testing.T) {
	// The resource /Properties /P is an annotation whose /DA uses the given
	// font name for a Type 1 font with the given base font.  extra adds
	// further fonts to /DR.
	sum := func(da string, fontName, baseFont pdf.Name, extra pdf.Dict) resourceSum {
		f := newFPFile(t)
		font := f.put(pdf.Dict{"Type": pdf.Name("Font"), "BaseFont": baseFont})
		fonts := pdf.Dict{fontName: font}
		maps.Copy(fonts, extra)
		annot := f.put(pdf.Dict{"Subtype": pdf.Name("FreeText"), "DA": pdf.String(da)})
		dr := map[pdf.Name]pdf.Dict{
			"Font":       fonts,
			"Properties": {"P": pdf.Dict{"Annot": annot}},
		}
		s, ok, err := newFingerprinter(f.w, dr, 1<<20).sum("Properties", "P")
		if err != nil || !ok {
			t.Fatalf("fingerprint: %v, %v", ok, err)
		}
		return s
	}

	base := sum("/Helv 12 Tf 0 g", "Helv", "Helvetica", nil)
	cases := []struct {
		name  string
		got   resourceSum
		equal bool
	}{
		{"same", sum("/Helv 12 Tf 0 g", "Helv", "Helvetica", nil), true},
		{"other name", sum("/F1 12 Tf 0 g", "F1", "Helvetica", nil), true},
		{"white space and comments", sum("/Helv  12 Tf %x\n0 g", "Helv", "Helvetica", nil), true},
		{"other font", sum("/Helv 12 Tf 0 g", "Helv", "Courier", nil), false},
		{"other size", sum("/Helv 10 Tf 0 g", "Helv", "Helvetica", nil), false},
		{"missing font", sum("/Cour 12 Tf 0 g", "Helv", "Helvetica", nil), false},
		{
			"unused font ignored",
			sum("/Helv 12 Tf 0 g", "Helv", "Helvetica", pdf.Dict{"Cour": pdf.Dict{}}),
			true,
		},
	}
	for _, c := range cases {
		if (c.got == base) != c.equal {
			t.Errorf("%s: fingerprints equal = %v, want %v", c.name, c.got == base, c.equal)
		}
	}

	// missing resources are compared by name
	if sum("/A 12 Tf", "Helv", "Helvetica", nil) == sum("/B 12 Tf", "Helv", "Helvetica", nil) {
		t.Error("different missing fonts have equal fingerprints")
	}
}

// TestFingerprintDACycle checks that a resource whose /DA names the resource
// itself has a fingerprint.
func TestFingerprintDACycle(t *testing.T) {
	sum := func(name pdf.Name) resourceSum {
		f := newFPFile(t)
		dr := map[pdf.Name]pdf.Dict{
			"Properties": {name: pdf.Dict{"DA": pdf.String("/Tag /" + string(name) + " BDC")}},
		}
		s, ok, err := newFingerprinter(f.w, dr, 1<<20).sum("Properties", name)
		if err != nil || !ok {
			t.Fatalf("fingerprint: %v, %v", ok, err)
		}
		return s
	}
	if sum("P") != sum("Q") {
		t.Error("self-referencing resources under different names differ")
	}
}
