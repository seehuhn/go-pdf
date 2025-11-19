# Resource Extract and Embed Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement Extract and Embed functions for Resource struct to read/write PDF resource dictionaries.

**Architecture:** Extract reads resource dictionaries permissively (skipping invalid entries), Embed writes strictly (validating version constraints). SingleUse field preserves direct/indirect structure.

**Tech Stack:** Go 1.24, existing pdf package APIs (Extractor, EmbedHelper)

---

## Task 1: Add SingleUse Field to Resource Struct

**Files:**
- Modify: `resource/resource.go:32-41`

**Step 1: Add SingleUse field**

Add this field to the Resource struct after the Properties field:

```go
type Resource struct {
	ExtGState  map[pdf.Name]graphics.ExtGState
	ColorSpace map[pdf.Name]color.Space
	Pattern    map[pdf.Name]color.Pattern
	Shading    map[pdf.Name]graphics.Shading
	XObject    map[pdf.Name]graphics.XObject
	Font       map[pdf.Name]font.Instance
	ProcSet    ProcSet
	Properties map[pdf.Name]property.List

	// SingleUse determines Embed output format.
	// If true, Embed returns dictionary directly.
	// If false, Embed allocates reference.
	SingleUse bool
}
```

**Step 2: Verify compilation**

Run: `go test ./resource/`
Expected: Package compiles successfully

---

## Task 2: Implement Extract Function - Setup and Validation

**Files:**
- Modify: `resource/resource.go:53-55`

**Step 1: Write basic structure**

Replace the panic with this implementation:

```go
func Extract(x *pdf.Extractor, obj pdf.Object) (*Resource, error) {
	// Check if original object was indirect before resolving
	_, wasIndirect := obj.(pdf.Reference)

	// Resolve object
	obj, err := x.Resolve(obj)
	if err != nil {
		return nil, err
	}

	// Handle nil - return empty resource
	if obj == nil {
		return &Resource{SingleUse: true}, nil
	}

	// Must be a dictionary
	dict, ok := obj.(pdf.Dict)
	if !ok {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("resource must be dictionary, got %T", obj),
		}
	}

	// Create result with SingleUse based on indirectness
	res := &Resource{
		SingleUse: !wasIndirect,
	}

	// TODO: Extract subdictionaries

	return res, nil
}
```

**Step 2: Add fmt import**

Add to imports at top of file:

```go
import (
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/property"
)
```

**Step 3: Verify compilation**

Run: `go test ./resource/`
Expected: Package compiles

---

## Task 3: Extract ExtGState Subdictionary

**Files:**
- Modify: `resource/resource.go` (Extract function, before `return res, nil`)

**Step 1: Add ExtGState extraction**

Add before the `return res, nil` line:

```go
	// Extract ExtGState subdictionary
	if extGStateObj, ok := dict["ExtGState"]; ok {
		if extGStateDict, err := x.GetDict(extGStateObj); err == nil && extGStateDict != nil {
			for name, obj := range extGStateDict {
				extGState, err := pdf.ExtractorGet(x, obj, graphics.ExtractExtGState)
				if err != nil {
					// Skip invalid entries (permissive)
					continue
				}
				if res.ExtGState == nil {
					res.ExtGState = make(map[pdf.Name]graphics.ExtGState)
				}
				res.ExtGState[name] = extGState
			}
		}
	}
```

**Step 2: Verify compilation**

Run: `go test ./resource/`
Expected: Package compiles

---

## Task 4: Extract ColorSpace, Pattern, Shading Subdictionaries

**Files:**
- Modify: `resource/resource.go` (Extract function, after ExtGState extraction)

**Step 1: Add ColorSpace extraction**

Add after ExtGState block:

```go
	// Extract ColorSpace subdictionary
	if colorSpaceObj, ok := dict["ColorSpace"]; ok {
		if colorSpaceDict, err := x.GetDict(colorSpaceObj); err == nil && colorSpaceDict != nil {
			for name, obj := range colorSpaceDict {
				cs, err := pdf.ExtractorGet(x, obj, color.ExtractSpace)
				if err != nil {
					continue
				}
				if res.ColorSpace == nil {
					res.ColorSpace = make(map[pdf.Name]color.Space)
				}
				res.ColorSpace[name] = cs
			}
		}
	}
```

**Step 2: Add Pattern extraction**

Add after ColorSpace block:

```go
	// Extract Pattern subdictionary
	if patternObj, ok := dict["Pattern"]; ok {
		if patternDict, err := x.GetDict(patternObj); err == nil && patternDict != nil {
			for name, obj := range patternDict {
				pattern, err := pdf.ExtractorGet(x, obj, color.ExtractPattern)
				if err != nil {
					continue
				}
				if res.Pattern == nil {
					res.Pattern = make(map[pdf.Name]color.Pattern)
				}
				res.Pattern[name] = pattern
			}
		}
	}
```

**Step 3: Add Shading extraction**

Add after Pattern block:

```go
	// Extract Shading subdictionary
	if shadingObj, ok := dict["Shading"]; ok {
		if shadingDict, err := x.GetDict(shadingObj); err == nil && shadingDict != nil {
			for name, obj := range shadingDict {
				shading, err := pdf.ExtractorGet(x, obj, graphics.ExtractShading)
				if err != nil {
					continue
				}
				if res.Shading == nil {
					res.Shading = make(map[pdf.Name]graphics.Shading)
				}
				res.Shading[name] = shading
			}
		}
	}
```

**Step 4: Verify compilation**

Run: `go test ./resource/`
Expected: Package compiles

---

## Task 5: Extract XObject, Font, Properties Subdictionaries

**Files:**
- Modify: `resource/resource.go` (Extract function, after Shading extraction)
- Need to import: `seehuhn.de/go/pdf/font/dict` for ExtractFont

**Step 1: Add import**

Update imports:

```go
import (
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	fontdict "seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/property"
)
```

**Step 2: Add XObject extraction**

Add after Shading block:

```go
	// Extract XObject subdictionary
	if xObjectObj, ok := dict["XObject"]; ok {
		if xObjectDict, err := x.GetDict(xObjectObj); err == nil && xObjectDict != nil {
			for name, obj := range xObjectDict {
				xobject, err := pdf.ExtractorGet(x, obj, graphics.ExtractXObject)
				if err != nil {
					continue
				}
				if res.XObject == nil {
					res.XObject = make(map[pdf.Name]graphics.XObject)
				}
				res.XObject[name] = xobject
			}
		}
	}
```

**Step 3: Add Font extraction**

Add after XObject block:

```go
	// Extract Font subdictionary
	if fontObj, ok := dict["Font"]; ok {
		if fontDict, err := x.GetDict(fontObj); err == nil && fontDict != nil {
			for name, obj := range fontDict {
				f, err := pdf.ExtractorGet(x, obj, fontdict.ExtractFont)
				if err != nil {
					continue
				}
				if res.Font == nil {
					res.Font = make(map[pdf.Name]font.Instance)
				}
				res.Font[name] = f
			}
		}
	}
```

**Step 4: Add Properties extraction**

Add after Font block:

```go
	// Extract Properties subdictionary
	if propsObj, ok := dict["Properties"]; ok {
		if propsDict, err := x.GetDict(propsObj); err == nil && propsDict != nil {
			for name, obj := range propsDict {
				prop, err := pdf.ExtractorGet(x, obj, property.Extract)
				if err != nil {
					continue
				}
				if res.Properties == nil {
					res.Properties = make(map[pdf.Name]property.List)
				}
				res.Properties[name] = prop
			}
		}
	}
```

**Step 5: Verify compilation**

Run: `go test ./resource/`
Expected: Package compiles

---

## Task 6: Extract ProcSet

**Files:**
- Modify: `resource/resource.go` (Extract function, after Properties extraction)

**Step 1: Add ProcSet extraction**

Add after Properties block:

```go
	// Extract ProcSet (array of names)
	if procSetObj, ok := dict["ProcSet"]; ok {
		if procSetArray, err := x.GetArray(procSetObj); err == nil {
			for _, item := range procSetArray {
				if name, ok := item.(pdf.Name); ok {
					switch name {
					case "PDF":
						res.ProcSet.PDF = true
					case "Text":
						res.ProcSet.Text = true
					case "ImageB":
						res.ProcSet.ImageB = true
					case "ImageC":
						res.ProcSet.ImageC = true
					case "ImageI":
						res.ProcSet.ImageI = true
					// Ignore unknown names (permissive)
					}
				}
			}
		}
	}
```

**Step 2: Verify compilation**

Run: `go test ./resource/`
Expected: Package compiles

---

## Task 7: Implement Embed Function - Version Validation

**Files:**
- Modify: `resource/resource.go:57-59`

**Step 1: Write version validation**

Replace the panic in Embed with:

```go
func (r *Resource) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	// Validate PDF version constraints
	if len(r.Shading) > 0 {
		if err := pdf.CheckVersion(rm.Out(), "Shading resources", pdf.V1_3); err != nil {
			return nil, err
		}
	}
	if len(r.Properties) > 0 {
		if err := pdf.CheckVersion(rm.Out(), "Properties resources", pdf.V1_2); err != nil {
			return nil, err
		}
	}
	if r.ProcSet.PDF || r.ProcSet.Text || r.ProcSet.ImageB || r.ProcSet.ImageC || r.ProcSet.ImageI {
		v := rm.Out().GetMeta().Version
		if v.Compare(pdf.V2_0) >= 0 {
			return nil, fmt.Errorf("ProcSet is deprecated in PDF 2.0")
		}
	}

	// Create result dictionary
	dict := pdf.Dict{}

	// TODO: Embed subdictionaries

	// Return based on SingleUse
	if r.SingleUse {
		return dict, nil
	}
	ref := rm.Alloc()
	err := rm.Out().Put(ref, dict)
	if err != nil {
		return nil, err
	}
	return ref, nil
}
```

**Step 2: Verify compilation**

Run: `go test ./resource/`
Expected: Package compiles

---

## Task 8: Embed ExtGState, ColorSpace, Pattern Subdictionaries

**Files:**
- Modify: `resource/resource.go` (Embed function, after `dict := pdf.Dict{}`)

**Step 1: Add ExtGState embedding**

Add after `dict := pdf.Dict{}`:

```go
	// Embed ExtGState subdictionary
	if len(r.ExtGState) > 0 {
		subDict := pdf.Dict{}
		for name, extGState := range r.ExtGState {
			embedded, err := rm.Embed(extGState)
			if err != nil {
				return nil, fmt.Errorf("embedding ExtGState %s: %w", name, err)
			}
			subDict[name] = embedded
		}
		dict["ExtGState"] = subDict
	}
```

**Step 2: Add ColorSpace embedding**

Add after ExtGState block:

```go
	// Embed ColorSpace subdictionary
	if len(r.ColorSpace) > 0 {
		subDict := pdf.Dict{}
		for name, cs := range r.ColorSpace {
			embedded, err := rm.Embed(cs)
			if err != nil {
				return nil, fmt.Errorf("embedding ColorSpace %s: %w", name, err)
			}
			subDict[name] = embedded
		}
		dict["ColorSpace"] = subDict
	}
```

**Step 3: Add Pattern embedding**

Add after ColorSpace block:

```go
	// Embed Pattern subdictionary
	if len(r.Pattern) > 0 {
		subDict := pdf.Dict{}
		for name, pattern := range r.Pattern {
			embedded, err := rm.Embed(pattern)
			if err != nil {
				return nil, fmt.Errorf("embedding Pattern %s: %w", name, err)
			}
			subDict[name] = embedded
		}
		dict["Pattern"] = subDict
	}
```

**Step 4: Verify compilation**

Run: `go test ./resource/`
Expected: Package compiles

---

## Task 9: Embed Shading, XObject, Font, Properties Subdictionaries

**Files:**
- Modify: `resource/resource.go` (Embed function, after Pattern embedding)

**Step 1: Add Shading embedding**

Add after Pattern block:

```go
	// Embed Shading subdictionary
	if len(r.Shading) > 0 {
		subDict := pdf.Dict{}
		for name, shading := range r.Shading {
			embedded, err := rm.Embed(shading)
			if err != nil {
				return nil, fmt.Errorf("embedding Shading %s: %w", name, err)
			}
			subDict[name] = embedded
		}
		dict["Shading"] = subDict
	}
```

**Step 2: Add XObject embedding**

Add after Shading block:

```go
	// Embed XObject subdictionary
	if len(r.XObject) > 0 {
		subDict := pdf.Dict{}
		for name, xobject := range r.XObject {
			embedded, err := rm.Embed(xobject)
			if err != nil {
				return nil, fmt.Errorf("embedding XObject %s: %w", name, err)
			}
			subDict[name] = embedded
		}
		dict["XObject"] = subDict
	}
```

**Step 3: Add Font embedding**

Add after XObject block:

```go
	// Embed Font subdictionary
	if len(r.Font) > 0 {
		subDict := pdf.Dict{}
		for name, f := range r.Font {
			embedded, err := rm.Embed(f)
			if err != nil {
				return nil, fmt.Errorf("embedding Font %s: %w", name, err)
			}
			subDict[name] = embedded
		}
		dict["Font"] = subDict
	}
```

**Step 4: Add Properties embedding**

Add after Font block:

```go
	// Embed Properties subdictionary
	if len(r.Properties) > 0 {
		subDict := pdf.Dict{}
		for name, prop := range r.Properties {
			embedded, err := rm.Embed(prop)
			if err != nil {
				return nil, fmt.Errorf("embedding Properties %s: %w", name, err)
			}
			subDict[name] = embedded
		}
		dict["Properties"] = subDict
	}
```

**Step 5: Verify compilation**

Run: `go test ./resource/`
Expected: Package compiles

---

## Task 10: Embed ProcSet

**Files:**
- Modify: `resource/resource.go` (Embed function, after Properties embedding)

**Step 1: Add ProcSet embedding**

Add after Properties block, before the SingleUse return logic:

```go
	// Embed ProcSet (convert booleans to array)
	if r.ProcSet.PDF || r.ProcSet.Text || r.ProcSet.ImageB || r.ProcSet.ImageC || r.ProcSet.ImageI {
		var procSetArray pdf.Array
		if r.ProcSet.PDF {
			procSetArray = append(procSetArray, pdf.Name("PDF"))
		}
		if r.ProcSet.Text {
			procSetArray = append(procSetArray, pdf.Name("Text"))
		}
		if r.ProcSet.ImageB {
			procSetArray = append(procSetArray, pdf.Name("ImageB"))
		}
		if r.ProcSet.ImageC {
			procSetArray = append(procSetArray, pdf.Name("ImageC"))
		}
		if r.ProcSet.ImageI {
			procSetArray = append(procSetArray, pdf.Name("ImageI"))
		}
		dict["ProcSet"] = procSetArray
	}
```

**Step 2: Verify compilation**

Run: `go test ./resource/`
Expected: Package compiles

---

## Task 11: Write Round-Trip Test Infrastructure

**Files:**
- Create: `resource/resource_test.go`

**Step 1: Create test file with imports and helper**

Create file with:

```go
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

package resource

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// roundTripTest performs a round-trip test on a Resource
func roundTripTest(t *testing.T, original *Resource) {
	t.Helper()

	// Embed the resource
	buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(buf)

	embedded, err := rm.Embed(original)
	if err != nil {
		t.Fatalf("failed to embed resource: %v", err)
	}

	// Write to PDF
	ref := buf.Alloc()
	err = buf.Put(ref, embedded)
	if err != nil {
		t.Fatalf("failed to put resource: %v", err)
	}

	err = buf.Close()
	if err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	// Extract the resource back
	x := pdf.NewExtractor(buf)
	readResource, err := Extract(x, ref)
	if err != nil {
		t.Fatalf("failed to extract resource: %v", err)
	}

	// Compare
	opts := []cmp.Option{
		cmpopts.IgnoreUnexported(Resource{}),
	}
	if diff := cmp.Diff(original, readResource, opts...); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}
```

**Step 2: Verify compilation**

Run: `go test ./resource/`
Expected: Package compiles (no tests run yet)

---

## Task 12: Write Round-Trip Tests for Each Resource Type

**Files:**
- Modify: `resource/resource_test.go` (add test cases)

**Step 1: Add TestRoundTrip with table tests**

Add after roundTripTest function:

```go
func TestRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		resource *Resource
	}{
		{
			name: "empty resource",
			resource: &Resource{
				SingleUse: true,
			},
		},
		{
			name: "only ProcSet",
			resource: &Resource{
				ProcSet: ProcSet{
					PDF:    true,
					Text:   true,
					ImageB: true,
				},
				SingleUse: true,
			},
		},
		{
			name: "SingleUse false (indirect)",
			resource: &Resource{
				ProcSet: ProcSet{
					PDF: true,
				},
				SingleUse: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roundTripTest(t, tt.resource)
		})
	}
}
```

**Step 2: Run tests**

Run: `go test ./resource/ -v`
Expected: All tests pass

---

## Task 13: Write Version Validation Tests

**Files:**
- Modify: `resource/resource_test.go`

**Step 1: Add version validation tests**

Add after TestRoundTrip:

```go
func TestVersionValidation(t *testing.T) {
	tests := []struct {
		name     string
		version  pdf.Version
		resource *Resource
		wantErr  bool
	}{
		{
			name:    "Shading with PDF 1.2 fails",
			version: pdf.V1_2,
			resource: &Resource{
				Shading: map[pdf.Name]graphics.Shading{
					"S1": nil, // non-nil map is enough to trigger validation
				},
				SingleUse: true,
			},
			wantErr: true,
		},
		{
			name:    "Shading with PDF 1.3 succeeds",
			version: pdf.V1_3,
			resource: &Resource{
				Shading:   map[pdf.Name]graphics.Shading{},
				SingleUse: true,
			},
			wantErr: false,
		},
		{
			name:    "Properties with PDF 1.1 fails",
			version: pdf.V1_1,
			resource: &Resource{
				Properties: map[pdf.Name]property.List{
					"P1": nil,
				},
				SingleUse: true,
			},
			wantErr: true,
		},
		{
			name:    "Properties with PDF 1.2 succeeds",
			version: pdf.V1_2,
			resource: &Resource{
				Properties: map[pdf.Name]property.List{},
				SingleUse:  true,
			},
			wantErr: false,
		},
		{
			name:    "ProcSet with PDF 2.0 fails",
			version: pdf.V2_0,
			resource: &Resource{
				ProcSet: ProcSet{
					PDF: true,
				},
				SingleUse: true,
			},
			wantErr: true,
		},
		{
			name:    "ProcSet with PDF 1.7 succeeds",
			version: pdf.V1_7,
			resource: &Resource{
				ProcSet: ProcSet{
					PDF: true,
				},
				SingleUse: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf, _ := memfile.NewPDFWriter(tt.version, nil)
			rm := pdf.NewResourceManager(buf)

			_, err := rm.Embed(tt.resource)
			if (err != nil) != tt.wantErr {
				t.Errorf("Embed() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
```

**Step 2: Run tests**

Run: `go test ./resource/ -v -run TestVersionValidation`
Expected: All tests pass

---

## Task 14: Write ProcSet Conversion Tests

**Files:**
- Modify: `resource/resource_test.go`

**Step 1: Add ProcSet conversion tests**

Add after TestVersionValidation:

```go
func TestProcSetConversion(t *testing.T) {
	tests := []struct {
		name    string
		procSet ProcSet
	}{
		{
			name:    "all false",
			procSet: ProcSet{},
		},
		{
			name: "PDF only",
			procSet: ProcSet{
				PDF: true,
			},
		},
		{
			name: "all true",
			procSet: ProcSet{
				PDF:    true,
				Text:   true,
				ImageB: true,
				ImageC: true,
				ImageI: true,
			},
		},
		{
			name: "mixed",
			procSet: ProcSet{
				PDF:    true,
				ImageC: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := &Resource{
				ProcSet:   tt.procSet,
				SingleUse: true,
			}
			roundTripTest(t, original)
		})
	}
}
```

**Step 2: Run tests**

Run: `go test ./resource/ -v -run TestProcSetConversion`
Expected: All tests pass

---

## Task 15: Add Fuzz Test

**Files:**
- Modify: `resource/resource_test.go`

**Step 1: Add fuzz test**

Add at end of file:

```go
func FuzzRoundTrip(f *testing.F) {
	// Seed corpus with simple test cases
	f.Add([]byte("<< >>"))
	f.Add([]byte("<< /ProcSet [/PDF /Text] >>"))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Try to parse as PDF object
		buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)

		// Skip if can't create reader
		r, err := pdf.NewReader(nil, nil)
		if err != nil {
			t.Skip("invalid PDF structure")
		}

		// Try to extract resource
		x := pdf.NewExtractor(r)
		resource, err := Extract(x, pdf.Dict{})
		if err != nil {
			t.Skip("extraction failed (permissive)")
		}

		// Try round-trip
		roundTripTest(t, resource)
	})
}
```

**Step 2: Run fuzz test briefly**

Run: `go test ./resource/ -fuzz=FuzzRoundTrip -fuzztime=5s`
Expected: No crashes

---

## Task 16: Run Full Test Suite and Verify

**Files:**
- Test: `resource/resource_test.go`

**Step 1: Run all tests**

Run: `go test ./resource/ -v`
Expected: All tests pass

**Step 2: Check coverage**

Run: `go test ./resource/ -cover`
Expected: Reasonable coverage (>80%)

**Step 3: Run tests in parent directory to ensure no breakage**

Run: `go test ./...`
Expected: All tests pass

---

## Completion Checklist

- [ ] All 16 tasks completed
- [ ] All tests pass
- [ ] No compilation errors
- [ ] Code follows project conventions
