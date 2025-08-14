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
)

func TestNumberFormatExtractEmbed(t *testing.T) {
	tests := []struct {
		name string
		nf   NumberFormat
	}{
		{
			name: "basic format",
			nf: NumberFormat{
				Unit:             "ft",
				ConversionFactor: 12.0,
				Precision:        1,
				FractionFormat:   FractionDecimal,
			},
		},
		{
			name: "fraction format with custom separators",
			nf: NumberFormat{
				Unit:               "in",
				ConversionFactor:   1.0,
				Precision:          8,
				FractionFormat:     FractionFraction,
				ForceExactFraction: true,
				ThousandsSeparator: ",",
				DecimalSeparator:   ".",
				PrefixSpacing:      " ",
				SuffixSpacing:      " ",
				PrefixLabel:        false,
			},
		},
		{
			name: "prefix label format",
			nf: NumberFormat{
				Unit:             "m",
				ConversionFactor: 1000.0,
				Precision:        100,
				FractionFormat:   FractionDecimal,
				PrefixLabel:      true,
				PrefixSpacing:    " ",
				SuffixSpacing:    " ",
			},
		},
		{
			name: "no thousands separator",
			nf: NumberFormat{
				Unit:               "mm",
				ConversionFactor:   10.0,
				Precision:          1,
				FractionFormat:     FractionRound,
				ThousandsSeparator: "",
			},
		},
		{
			name: "custom spacing",
			nf: NumberFormat{
				Unit:             "km",
				ConversionFactor: 0.001,
				Precision:        1000,
				PrefixSpacing:    "",
				SuffixSpacing:    "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test PDF writer
			w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

			// Embed the NumberFormat
			rm := pdf.NewResourceManager(w)
			embedded, _, err := tt.nf.Embed(rm)
			if err != nil {
				t.Fatalf("embed failed: %v", err)
			}

			// Extract the NumberFormat back
			extracted, err := ExtractNumberFormat(w, embedded)
			if err != nil {
				t.Fatalf("extract failed: %v", err)
			}

			// Compare
			if diff := cmp.Diff(*extracted, tt.nf); diff != "" {
				t.Errorf("round trip failed (-got +want):\n%s", diff)
			}
		})
	}
}

func TestNumberFormatExtractDefaults(t *testing.T) {
	// Test extraction with minimal PDF dictionary (only required fields)
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	dict := pdf.Dict{
		"U": pdf.String("mi"),
		"C": pdf.Number(1.0),
		"D": pdf.Integer(100),
	}

	extracted, err := ExtractNumberFormat(w, dict)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	expected := NumberFormat{
		Unit:               "mi",
		ConversionFactor:   1.0,
		Precision:          100,
		FractionFormat:     FractionDecimal,
		ForceExactFraction: false,
		ThousandsSeparator: "",
		DecimalSeparator:   "",
		PrefixSpacing:      "",
		SuffixSpacing:      "",
		PrefixLabel:        false,
	}

	if diff := cmp.Diff(*extracted, expected); diff != "" {
		t.Errorf("default extraction failed (-got +want):\n%s", diff)
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

func TestNumberFormatHelperMethods(t *testing.T) {
	nf := NumberFormat{
		DecimalSeparator: "",
		PrefixSpacing:    "",
		SuffixSpacing:    "",
	}

	if got := nf.getDecimalSeparator(); got != "." {
		t.Errorf("getDecimalSeparator() with empty = %q, want %q", got, ".")
	}

	if got := nf.getPrefixSpacing(); got != " " {
		t.Errorf("getPrefixSpacing() with empty = %q, want %q", got, " ")
	}

	if got := nf.getSuffixSpacing(); got != " " {
		t.Errorf("getSuffixSpacing() with empty = %q, want %q", got, " ")
	}

	nf.DecimalSeparator = ","
	nf.PrefixSpacing = "["
	nf.SuffixSpacing = "]"

	if got := nf.getDecimalSeparator(); got != "," {
		t.Errorf("getDecimalSeparator() with custom = %q, want %q", got, ",")
	}

	if got := nf.getPrefixSpacing(); got != "[" {
		t.Errorf("getPrefixSpacing() with custom = %q, want %q", got, "[")
	}

	if got := nf.getSuffixSpacing(); got != "]" {
		t.Errorf("getSuffixSpacing() with custom = %q, want %q", got, "]")
	}
}
