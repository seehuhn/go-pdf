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

package measure

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/mock"
)

func TestNumberFormatExtractEmbed(t *testing.T) {
	tests := []struct {
		name string
		nf   NumberFormat
	}{
		{
			name: "basic format",
			nf: NumberFormat{
				Unit:             pdf.Text("ft"),
				ConversionFactor: 12.0,
				Precision:        1,
				FractionFormat:   FractionDecimal,
				// DecimalSeparator: empty string (shorthand for period)
				// After round-trip, will be "."
			},
		},
		{
			name: "fraction format with custom separators",
			nf: NumberFormat{
				Unit:               pdf.Text("in"),
				ConversionFactor:   1.0,
				Precision:          8,
				FractionFormat:     FractionFraction,
				ForceExactFraction: true,
				ThousandsSeparator: pdf.Text(","),
				// DecimalSeparator, PrefixSpacing, SuffixSpacing left as zero values
				// since they match PDF defaults and would be optimized out
				PrefixLabel: false,
			},
		},
		{
			name: "prefix label format",
			nf: NumberFormat{
				Unit:             pdf.Text("m"),
				ConversionFactor: 1000.0,
				Precision:        100,
				FractionFormat:   FractionDecimal,
				PrefixLabel:      true,
				// PrefixSpacing, SuffixSpacing left as zero values (match PDF defaults)
			},
		},
		{
			name: "no thousands separator",
			nf: NumberFormat{
				Unit:             pdf.Text("mm"),
				ConversionFactor: 10.0,
				Precision:        0,
				FractionFormat:   FractionRound,
			},
		},
		{
			name: "custom spacing",
			nf: NumberFormat{
				Unit:             pdf.Text("km"),
				ConversionFactor: 0.001,
				Precision:        1000,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test PDF writer
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)

			// Embed the NumberFormat
			rm := pdf.NewResourceManager(w)
			embedded, err := rm.Embed(&tt.nf)
			if err != nil {
				t.Fatalf("embed failed: %v", err)
			}

			// Extract the NumberFormat back
			x := pdf.NewExtractor(w)
			extracted, err := pdf.CursorAt(x, nil).Decode(embedded, ExtractNumberFormat)
			if err != nil {
				t.Fatalf("extract failed: %v", err)
			}

			// Normalize expected result for comparison
			expected := tt.nf
			if expected.DecimalSeparator.IsZero() {
				expected.DecimalSeparator = pdf.Text(".") // empty string becomes period after round-trip
			}

			// Compare
			if diff := cmp.Diff(*extracted, expected); diff != "" {
				t.Errorf("round trip failed (-got +want):\n%s", diff)
			}
		})
	}
}

func TestNumberFormatExtractDefaults(t *testing.T) {
	// Test extraction with minimal PDF dictionary (only required fields)
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)

	dict := pdf.Dict{
		"U": pdf.String("mi"),
		"C": pdf.Number(1.0),
		"D": pdf.Integer(100),
	}

	x := pdf.NewExtractor(w)
	extracted, err := pdf.CursorAt(x, nil).Decode(dict, ExtractNumberFormat)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	expected := NumberFormat{
		Unit:               pdf.Text("mi"),
		ConversionFactor:   1.0,
		Precision:          100,
		FractionFormat:     FractionDecimal,
		ForceExactFraction: false,
		DecimalSeparator:   pdf.Text("."), // Always filled in when reading
		PrefixLabel:        false,
		SingleUse:          true, // direct dict, not a reference
	}

	if diff := cmp.Diff(*extracted, expected); diff != "" {
		t.Errorf("default extraction failed (-got +want):\n%s", diff)
	}
}

// TestNumberFormatExtractFallback verifies that malformed values in /C and /D
// are silently replaced with safe defaults on read, so the dictionary can
// be re-embedded without error.
func TestNumberFormatExtractFallback(t *testing.T) {
	tests := []struct {
		name          string
		dict          pdf.Dict
		wantConv      float64
		wantPrecision int
	}{
		{
			name: "zero conversion factor",
			dict: pdf.Dict{
				"U": pdf.String("mi"),
				"C": pdf.Number(0),
			},
			wantConv:      1,
			wantPrecision: 100,
		},
		{
			name: "missing conversion factor",
			dict: pdf.Dict{
				"U": pdf.String("mi"),
			},
			wantConv:      1,
			wantPrecision: 100,
		},
		{
			name: "negative decimal precision",
			dict: pdf.Dict{
				"U": pdf.String("mi"),
				"C": pdf.Number(1),
				"F": pdf.Name("D"),
				"D": pdf.Integer(-5),
			},
			wantConv:      1,
			wantPrecision: 100,
		},
		{
			name: "decimal precision not multiple of ten",
			dict: pdf.Dict{
				"U": pdf.String("mi"),
				"C": pdf.Number(1),
				"F": pdf.Name("D"),
				"D": pdf.Integer(7),
			},
			wantConv:      1,
			wantPrecision: 100,
		},
		{
			name: "decimal precision one is valid",
			dict: pdf.Dict{
				"U": pdf.String("mi"),
				"C": pdf.Number(1),
				"F": pdf.Name("D"),
				"D": pdf.Integer(1),
			},
			wantConv:      1,
			wantPrecision: 1,
		},
		{
			name: "fraction precision negative",
			dict: pdf.Dict{
				"U": pdf.String("in"),
				"C": pdf.Number(12),
				"F": pdf.Name("F"),
				"D": pdf.Integer(-3),
			},
			wantConv:      12,
			wantPrecision: 16,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			x := pdf.NewExtractor(w)
			nf, err := pdf.CursorAt(x, nil).Decode(tt.dict, ExtractNumberFormat)
			if err != nil {
				t.Fatalf("extract failed: %v", err)
			}
			if nf.ConversionFactor != tt.wantConv {
				t.Errorf("ConversionFactor = %v, want %v", nf.ConversionFactor, tt.wantConv)
			}
			if nf.Precision != tt.wantPrecision {
				t.Errorf("Precision = %d, want %d", nf.Precision, tt.wantPrecision)
			}
			// round trip: must re-embed without error
			rm := pdf.NewResourceManager(w)
			if _, err := rm.Embed(nf); err != nil {
				t.Errorf("re-embed failed: %v", err)
			}
		})
	}
}

func TestFractionalValueFormatConstants(t *testing.T) {
	// Test that constants have expected values
	if FractionDecimal != 0 {
		t.Errorf("FractionDecimal should be 0, got %d", FractionDecimal)
	}
	if FractionFraction != 1 {
		t.Errorf("FractionFraction should be 1, got %d", FractionFraction)
	}
	if FractionRound != 2 {
		t.Errorf("FractionRound should be 2, got %d", FractionRound)
	}
	if FractionTruncate != 3 {
		t.Errorf("FractionTruncate should be 3, got %d", FractionTruncate)
	}
}

// TestNumberFormatForceExactFraction checks that a dangling reference, null
// and a value of the wrong type in /FD are read like an absent entry.
func TestNumberFormatForceExactFraction(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)
	decode := func(fd pdf.Object, withKey bool) *NumberFormat {
		t.Helper()
		dict := pdf.Dict{
			"U": pdf.String("mi"),
			"C": pdf.Number(1.0),
			"D": pdf.Integer(100),
		}
		if withKey {
			dict["FD"] = fd
		}
		nf, err := ExtractNumberFormat(c, dict, true)
		if err != nil {
			t.Fatal(err)
		}
		return nf
	}

	absent := decode(nil, false)
	if absent.ForceExactFraction {
		t.Fatal("absent FD read as true")
	}
	if !decode(pdf.Boolean(true), true).ForceExactFraction {
		t.Error("FD true has no effect")
	}

	for name, fd := range map[string]pdf.Object{
		"dangling": pdf.NewReference(99, 0),
		"null":     nil,
		"name":     pdf.Name("x"),
		"integer":  pdf.Integer(1),
	} {
		if diff := cmp.Diff(absent, decode(fd, true)); diff != "" {
			t.Errorf("FD %s differs from absent entry (-absent +got):\n%s", name, diff)
		}
	}
}
