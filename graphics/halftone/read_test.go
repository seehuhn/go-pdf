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

package halftone

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// testCases holds test cases for all halftone types, indexed by type
var testCases = map[int][]testCase{
	1: {
		{
			name: "basic Type1",
			halftone: &Type1{
				Frequency:    60.0,
				Angle:        45.0,
				SpotFunction: SimpleDot,
			},
		},
		{
			name: "Type1 with all fields",
			halftone: &Type1{
				Frequency:        72.0,
				Angle:            30.0,
				SpotFunction:     Round,
				AccurateScreens:  true,
				TransferFunction: function.Identity,
			},
		},
	},
	5: {
		{
			name: "basic Type5",
			halftone: &Type5{
				Default: &Type1{
					Frequency:        60.0,
					Angle:            45.0,
					SpotFunction:     SimpleDot,
					TransferFunction: function.Identity,
				},
				Colorants: map[pdf.Name]graphics.Halftone{
					"Cyan": &Type1{
						Frequency:    72.0,
						Angle:        15.0,
						SpotFunction: Round,
					},
					"Magenta": &Type1{
						Frequency:    72.0,
						Angle:        75.0,
						SpotFunction: Ellipse,
					},
				},
			},
		},
	},
	6: {
		{
			name: "basic Type6",
			halftone: &Type6{
				Width:         4,
				Height:        4,
				ThresholdData: []byte{0, 128, 64, 192, 255, 127, 191, 63, 32, 160, 96, 224, 223, 95, 159, 31},
			},
		},
		{
			name: "Type6 with all fields",
			halftone: &Type6{
				Width:            2,
				Height:           2,
				ThresholdData:    []byte{0, 255, 127, 128},
				TransferFunction: function.Identity,
			},
		},
		{
			name: "Type6 64x64",
			halftone: &Type6{
				Width:         64,
				Height:        64,
				ThresholdData: makeBytePattern(64 * 64),
			},
		},
	},
	10: {
		{
			name: "basic Type10",
			halftone: &Type10{
				Size1:         3,
				Size2:         2,
				ThresholdData: []byte{0, 128, 64, 192, 255, 127, 191, 63, 32, 160, 96, 224, 31},
			},
		},
		{
			name: "Type10 with all fields",
			halftone: &Type10{
				Size1:            2,
				Size2:            1,
				ThresholdData:    []byte{0, 255, 127, 128, 64},
				TransferFunction: function.Identity,
			},
		},
		{
			name: "Type10 large 32+16",
			halftone: &Type10{
				Size1:         32,
				Size2:         16,
				ThresholdData: makeBytePattern(32*32 + 16*16),
			},
		},
	},
	16: {
		{
			name: "basic Type16",
			halftone: &Type16{
				Width:         2,
				Height:        2,
				ThresholdData: []uint16{0, 65535, 32767, 32768},
			},
		},
		{
			name: "Type16 with second rectangle",
			halftone: &Type16{
				Width:         2,
				Height:        1,
				Width2:        1,
				Height2:       2,
				ThresholdData: []uint16{0, 65535, 32767, 32768},
			},
		},
		{
			name: "Type16 with all fields",
			halftone: &Type16{
				Width:            1,
				Height:           1,
				ThresholdData:    []uint16{32767},
				TransferFunction: function.Identity,
			},
		},
		{
			name: "Type16 large 64x64",
			halftone: &Type16{
				Width:         64,
				Height:        64,
				ThresholdData: makeUint16Pattern(64 * 64),
			},
		},
		{
			name: "Type16 two-rect 32x32 + 16x16",
			halftone: &Type16{
				Width:         32,
				Height:        32,
				Width2:        16,
				Height2:       16,
				ThresholdData: makeUint16Pattern(32*32 + 16*16),
			},
		},
	},
}

func makeBytePattern(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}

func makeUint16Pattern(n int) []uint16 {
	v := make([]uint16, n)
	for i := range v {
		v[i] = uint16(i)
	}
	return v
}

type testCase struct {
	name     string
	halftone graphics.Halftone
}

func TestRoundTrip(t *testing.T) {
	for halftoneType, cases := range testCases {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("Type%d-%s", halftoneType, tc.name), func(t *testing.T) {
				roundTripTest(t, tc.halftone)
			})
		}
	}
}

// roundTripTest performs a round-trip test for any halftone type
func roundTripTest(t *testing.T, originalHalftone graphics.Halftone) {
	buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(buf)

	// Embed the halftone
	embedded, err := rm.Embed(originalHalftone)
	if err != nil {
		t.Fatal(err)
	}

	ref := buf.Alloc()
	err = buf.Put(ref, embedded)
	if err != nil {
		t.Fatal(err)
	}

	err = buf.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Read the halftone back
	x := pdf.NewExtractor(buf)
	readHalftone, err := Extract(pdf.CursorAt(x, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the types match
	if readHalftone.HalftoneType() != originalHalftone.HalftoneType() {
		t.Fatalf("halftone type mismatch: expected %d, got %d",
			originalHalftone.HalftoneType(), readHalftone.HalftoneType())
	}

	// Use cmp.Diff to compare the original and read halftone
	if diff := cmp.Diff(originalHalftone, readHalftone); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

// TestMaliciousInputRejected verifies that malformed halftone PDFs are
// rejected with an error rather than panicking or recursing without bound.
// Each subtest constructs a small PDF that would have triggered a DoS in
// extractType16/extractType10/extractType5 before the fix.
func TestMaliciousInputRejected(t *testing.T) {
	t.Run("Type16HugeDims", func(t *testing.T) {
		w, buf := memfile.NewPDFWriter(pdf.V2_0, nil)
		if err := memfile.AddBlankPage(w); err != nil {
			t.Fatal(err)
		}
		ref := w.Alloc()
		dict := pdf.Dict{
			"Type":         pdf.Name("Halftone"),
			"HalftoneType": pdf.Integer(16),
			"Width":        pdf.Integer(1 << 30),
			"Height":       pdf.Integer(1 << 30),
		}
		stm, err := w.OpenStream(ref, dict)
		if err != nil {
			t.Fatal(err)
		}
		if err := stm.Close(); err != nil {
			t.Fatal(err)
		}
		w.GetMeta().Trailer["Quir:E"] = ref
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		assertExtractError(t, buf.Data)
	})

	t.Run("Type10HugeDims", func(t *testing.T) {
		w, buf := memfile.NewPDFWriter(pdf.V2_0, nil)
		if err := memfile.AddBlankPage(w); err != nil {
			t.Fatal(err)
		}
		ref := w.Alloc()
		dict := pdf.Dict{
			"Type":         pdf.Name("Halftone"),
			"HalftoneType": pdf.Integer(10),
			"Xsquare":      pdf.Integer(1 << 20),
			"Ysquare":      pdf.Integer(1 << 20),
		}
		stm, err := w.OpenStream(ref, dict)
		if err != nil {
			t.Fatal(err)
		}
		if err := stm.Close(); err != nil {
			t.Fatal(err)
		}
		w.GetMeta().Trailer["Quir:E"] = ref
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		assertExtractError(t, buf.Data)
	})

	t.Run("Type5InlineNesting", func(t *testing.T) {
		w, buf := memfile.NewPDFWriter(pdf.V2_0, nil)
		if err := memfile.AddBlankPage(w); err != nil {
			t.Fatal(err)
		}
		inner := pdf.Dict{
			"Type":         pdf.Name("Halftone"),
			"HalftoneType": pdf.Integer(1),
			"Frequency":    pdf.Real(60),
			"Angle":        pdf.Real(0),
			"SpotFunction": pdf.Name("SimpleDot"),
		}
		middle := pdf.Dict{
			"Type":         pdf.Name("Halftone"),
			"HalftoneType": pdf.Integer(5),
			"Default":      inner,
		}
		outer := pdf.Dict{
			"Type":         pdf.Name("Halftone"),
			"HalftoneType": pdf.Integer(5),
			"Default":      middle,
		}
		ref := w.Alloc()
		if err := w.Put(ref, outer); err != nil {
			t.Fatal(err)
		}
		w.GetMeta().Trailer["Quir:E"] = ref
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		assertExtractError(t, buf.Data)
	})

	t.Run("Type5DeepLinearChain", func(t *testing.T) {
		const chainLen = 1000
		w, buf := memfile.NewPDFWriter(pdf.V2_0, nil)
		if err := memfile.AddBlankPage(w); err != nil {
			t.Fatal(err)
		}
		// terminating Type 1 halftone
		leafRef := w.Alloc()
		if err := w.Put(leafRef, pdf.Dict{
			"Type":         pdf.Name("Halftone"),
			"HalftoneType": pdf.Integer(1),
			"Frequency":    pdf.Real(60),
			"Angle":        pdf.Real(0),
			"SpotFunction": pdf.Name("SimpleDot"),
		}); err != nil {
			t.Fatal(err)
		}
		// chain of distinct Type 5 dicts, each Default -> next via indirect ref
		prev := pdf.Object(leafRef)
		for range chainLen {
			ref := w.Alloc()
			if err := w.Put(ref, pdf.Dict{
				"Type":         pdf.Name("Halftone"),
				"HalftoneType": pdf.Integer(5),
				"Default":      prev,
			}); err != nil {
				t.Fatal(err)
			}
			prev = ref
		}
		w.GetMeta().Trailer["Quir:E"] = prev
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		assertExtractError(t, buf.Data)
	})
}

// assertExtractError reads the PDF from the trailer's "Quir:E" key, runs
// halftone.Extract, and reports a failure if no error is returned. The
// test fails the surrounding goroutine on panic.
func assertExtractError(t *testing.T, data []byte) {
	t.Helper()
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)), nil)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	obj := r.GetMeta().Trailer["Quir:E"]
	if obj == nil {
		t.Fatal("missing trailer object")
	}
	x := pdf.NewExtractor(r)
	if _, err := Extract(pdf.CursorAt(x, nil), obj, false); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func FuzzRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	for _, cases := range testCases {
		for _, tc := range cases {
			w, buf := memfile.NewPDFWriter(pdf.V2_0, opt)

			// AddBlankPage creates a minimal valid PDF structure.
			// Without this, the seeds will likely be rejected by pdf.NewReader.
			err := memfile.AddBlankPage(w)
			if err != nil {
				continue
			}

			rm := pdf.NewResourceManager(w)

			embedded, err := rm.Embed(tc.halftone)
			if err != nil {
				continue
			}

			err = rm.Close()
			if err != nil {
				continue
			}

			w.GetMeta().Trailer["Quir:E"] = embedded

			err = w.Close()
			if err != nil {
				continue
			}

			f.Add(buf.Data)
		}
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}
		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("missing PDF object")
		}
		x := pdf.NewExtractor(r)
		halftone, err := Extract(pdf.CursorAt(x, nil), obj, false)
		if err != nil {
			t.Skip("malformed PDF object")
		}

		roundTripTest(t, halftone)
	})
}

// An unreadable colorant entry of a Type 5 halftone is dropped, leaving the
// remaining colorants and the default halftone intact. The dropped colorant
// then uses the default halftone, as the spec prescribes for colorants
// without an entry of their own.
func TestType5UnreadableColorantDropped(t *testing.T) {
	type1 := func(freq float64) pdf.Dict {
		return pdf.Dict{
			"HalftoneType": pdf.Integer(1),
			"Frequency":    pdf.Real(freq),
			"Angle":        pdf.Real(0),
			"SpotFunction": pdf.Name("SimpleDot"),
		}
	}

	// a chain of inline Type 5 dicts, nested via colorant entries
	deepChain := pdf.Dict{"HalftoneType": pdf.Integer(5), "Default": type1(60)}
	for range 1000 {
		deepChain = pdf.Dict{
			"HalftoneType": pdf.Integer(5),
			"Default":      type1(60),
			"Cyan":         deepChain,
		}
	}

	badColorants := map[string]pdf.Object{
		"invalid frequency":   type1(0),
		"unsupported type":    pdf.Dict{"HalftoneType": pdf.Integer(42)},
		"not a halftone":      pdf.Integer(7),
		"nested Type 5":       pdf.Dict{"HalftoneType": pdf.Integer(5), "Default": type1(60)},
		"deep colorant chain": deepChain,
	}

	for name, bad := range badColorants {
		t.Run(name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			c := pdf.CursorAt(pdf.NewExtractor(w), nil)

			dict := pdf.Dict{
				"HalftoneType": pdf.Integer(5),
				"Default":      type1(60),
				"Cyan":         bad,
				"Magenta":      type1(72),
			}

			ht, err := Extract(c, dict, false)
			if err != nil {
				t.Fatalf("extract failed: %v", err)
			}

			h5, ok := ht.(*Type5)
			if !ok {
				t.Fatalf("got %T, want *Type5", ht)
			}
			if h5.Default == nil {
				t.Error("default halftone was lost")
			}
			if _, ok := h5.Colorants["Cyan"]; ok {
				t.Error("unreadable colorant was not dropped")
			}
			if _, ok := h5.Colorants["Magenta"]; !ok {
				t.Error("valid colorant was lost")
			}
		})
	}
}

// A nonprimary colorant obliges the default halftone to carry a transfer
// function. Reading supplies the missing functions, and dropping the only
// nonprimary colorant removes the obligation again, so either result can be
// written back out.
func TestType5NonPrimaryColorantTransferFunction(t *testing.T) {
	type1 := func(freq float64) pdf.Dict {
		return pdf.Dict{
			"HalftoneType": pdf.Integer(1),
			"Frequency":    pdf.Real(freq),
			"Angle":        pdf.Real(0),
			"SpotFunction": pdf.Name("SimpleDot"),
		}
	}

	for _, kept := range []bool{true, false} {
		name := "colorant kept"
		spot := type1(80)
		if !kept {
			name = "colorant dropped"
			spot = type1(0) // invalid frequency
		}

		t.Run(name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			c := pdf.CursorAt(pdf.NewExtractor(w), nil)

			ht, err := Extract(c, pdf.Dict{
				"HalftoneType": pdf.Integer(5),
				"Default":      type1(60),
				"Cyan":         type1(72),
				"PANTONE 123":  spot,
			}, false)
			if err != nil {
				t.Fatalf("extract failed: %v", err)
			}
			h5 := ht.(*Type5)

			spotHalftone, ok := h5.Colorants["PANTONE 123"]
			if ok != kept {
				t.Fatalf("nonprimary colorant present = %t, want %t", ok, kept)
			}
			if kept && spotHalftone.GetTransferFunction() == nil {
				t.Error("nonprimary colorant has no transfer function")
			}
			if got := h5.Default.GetTransferFunction() != nil; got != kept {
				t.Errorf("default transfer function present = %t, want %t", got, kept)
			}

			// whatever we read must be writable again
			rm := pdf.NewResourceManager(w)
			if _, err := rm.Embed(h5); err != nil {
				t.Errorf("embed failed: %v", err)
			}
		})
	}
}

// An unusable /SpotFunction must not make the whole halftone unreadable:
// reading falls back to the default spot function and keeps the remaining
// entries.
func TestType1SpotFunctionUnusable(t *testing.T) {
	spotFunctions := map[string]pdf.Object{
		"wrong shape": pdf.Dict{ // 1 -> 1, but a spot function is 2 -> 1
			"FunctionType": pdf.Integer(2),
			"Domain":       pdf.Array{pdf.Real(0), pdf.Real(1)},
			"C0":           pdf.Array{pdf.Real(0)},
			"C1":           pdf.Array{pdf.Real(1)},
			"N":            pdf.Real(1),
		},
		"unknown function type": pdf.Dict{
			"FunctionType": pdf.Integer(99),
			"Domain":       pdf.Array{pdf.Real(0), pdf.Real(1), pdf.Real(0), pdf.Real(1)},
			"Range":        pdf.Array{pdf.Real(-1), pdf.Real(1)},
		},
		"unknown name": pdf.Name("Quir:NoSuchSpotFunction"),
	}

	for name, spot := range spotFunctions {
		t.Run(name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			c := pdf.CursorAt(pdf.NewExtractor(w), nil)

			dict := pdf.Dict{
				"HalftoneType": pdf.Integer(1),
				"Frequency":    pdf.Real(60),
				"Angle":        pdf.Real(45),
				"SpotFunction": spot,
			}

			ht, err := Extract(c, dict, false)
			if err != nil {
				t.Fatalf("extract failed: %v", err)
			}

			h1, ok := ht.(*Type1)
			if !ok {
				t.Fatalf("got %T, want *Type1", ht)
			}
			if h1.SpotFunction != SimpleDot {
				t.Error("spot function was not replaced by the default")
			}
			if h1.Frequency != 60 || h1.Angle != 45 {
				t.Errorf("frequency/angle = %g/%g, want 60/45", h1.Frequency, h1.Angle)
			}
		})
	}
}
