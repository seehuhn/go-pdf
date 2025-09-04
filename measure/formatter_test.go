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
)

func TestFormatMeasurement(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		formats  []*NumberFormat
		expected string
	}{
		{
			name:  "spec example - 1.4505 miles",
			value: 1.4505,
			formats: []*NumberFormat{
				{Unit: "mi", ConversionFactor: 1.0, Precision: 100000, FractionFormat: FractionDecimal, ThousandsSeparator: ",", SuffixSpacing: " "},
				{Unit: "ft", ConversionFactor: 5280, Precision: 1, FractionFormat: FractionDecimal, ThousandsSeparator: ",", SuffixSpacing: " "},
				{Unit: "in", ConversionFactor: 12, Precision: 8, FractionFormat: FractionFraction, ThousandsSeparator: ",", SuffixSpacing: " "},
			},
			expected: "1 mi 2,378 ft 7 5/8 in",
		},
		{
			name:  "simple single format",
			value: 5.25,
			formats: []*NumberFormat{
				{Unit: "ft", ConversionFactor: 1.0, Precision: 100, FractionFormat: FractionDecimal, SuffixSpacing: " "},
			},
			expected: "5.25 ft",
		},
		{
			name:  "whole number only",
			value: 3.0,
			formats: []*NumberFormat{
				{Unit: "m", ConversionFactor: 1.0, Precision: 100, FractionFormat: FractionDecimal, SuffixSpacing: " "},
			},
			expected: "3 m",
		},
		{
			name:  "zero value",
			value: 0.0,
			formats: []*NumberFormat{
				{Unit: "cm", ConversionFactor: 1.0, Precision: 10, FractionFormat: FractionDecimal, SuffixSpacing: " "},
			},
			expected: "0 cm",
		},
		{
			name:  "fraction rounding",
			value: 2.6,
			formats: []*NumberFormat{
				{Unit: "units", ConversionFactor: 1.0, Precision: 0, FractionFormat: FractionRound, SuffixSpacing: " "},
			},
			expected: "3 units",
		},
		{
			name:  "fraction truncation",
			value: 2.9,
			formats: []*NumberFormat{
				{Unit: "units", ConversionFactor: 1.0, Precision: 0, FractionFormat: FractionTruncate, SuffixSpacing: " "},
			},
			expected: "2 units",
		},
		{
			name:  "prefix label",
			value: 1.5,
			formats: []*NumberFormat{
				{Unit: "kg", ConversionFactor: 1.0, Precision: 10, FractionFormat: FractionDecimal, PrefixLabel: true, PrefixSpacing: " ", SuffixSpacing: " "},
			},
			expected: " kg 1.5",
		},
		{
			name:  "no thousands separator",
			value: 12345.0,
			formats: []*NumberFormat{
				{Unit: "units", ConversionFactor: 1.0, Precision: 1, FractionFormat: FractionDecimal, ThousandsSeparator: "", SuffixSpacing: " "},
			},
			expected: "12345 units",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Format(tt.value, tt.formats)
			if err != nil {
				t.Fatalf("FormatMeasurement failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("FormatMeasurement() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatMeasurementErrors(t *testing.T) {
	_, err := Format(1.0, []*NumberFormat{})
	if err == nil {
		t.Error("FormatMeasurement with empty formats should return error")
	}
}

func TestFormatInteger(t *testing.T) {
	tests := []struct {
		name     string
		nf       NumberFormat
		value    int64
		expected string
	}{
		{
			name:     "basic suffix",
			nf:       NumberFormat{Unit: "ft", ThousandsSeparator: "", PrefixSpacing: " ", SuffixSpacing: " "},
			value:    123,
			expected: "123 ft",
		},
		{
			name:     "with thousands separator",
			nf:       NumberFormat{Unit: "m", ThousandsSeparator: ",", PrefixSpacing: " ", SuffixSpacing: " "},
			value:    12345,
			expected: "12,345 m",
		},
		{
			name:     "prefix label",
			nf:       NumberFormat{Unit: "kg", PrefixLabel: true, PrefixSpacing: " ", SuffixSpacing: " "},
			value:    42,
			expected: " kg 42",
		},
		{
			name:     "zero value",
			nf:       NumberFormat{Unit: "cm", PrefixSpacing: " ", SuffixSpacing: " "},
			value:    0,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.nf.formatInteger(tt.value)
			if result != tt.expected {
				t.Errorf("formatInteger() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatFraction(t *testing.T) {
	tests := []struct {
		name       string
		nf         NumberFormat
		fractional float64
		expected   string
	}{
		{
			name:       "decimal fraction",
			nf:         NumberFormat{Unit: "in", Precision: 100, FractionFormat: FractionDecimal, DecimalSeparator: ".", PrefixSpacing: " ", SuffixSpacing: " "},
			fractional: 0.75,
			expected:   ".75 in",
		},
		{
			name:       "simple fraction",
			nf:         NumberFormat{Unit: "in", Precision: 8, FractionFormat: FractionFraction, PrefixSpacing: " ", SuffixSpacing: " "},
			fractional: 0.625,
			expected:   "5/8 in",
		},
		{
			name:       "force exact fraction",
			nf:         NumberFormat{Unit: "in", Precision: 8, FractionFormat: FractionFraction, ForceExactFraction: true, PrefixSpacing: " ", SuffixSpacing: " "},
			fractional: 0.25,
			expected:   "2/8 in",
		},
		{
			name:       "rounded fraction",
			nf:         NumberFormat{Unit: "units", Precision: 1, FractionFormat: FractionRound, PrefixSpacing: " ", SuffixSpacing: " "},
			fractional: 0.6,
			expected:   "1 units",
		},
		{
			name:       "truncated fraction",
			nf:         NumberFormat{Unit: "units", Precision: 1, FractionFormat: FractionTruncate, PrefixSpacing: " ", SuffixSpacing: " "},
			fractional: 0.9,
			expected:   "",
		},
		{
			name:       "zero fraction",
			nf:         NumberFormat{Unit: "in", Precision: 8, FractionFormat: FractionFraction, PrefixSpacing: " ", SuffixSpacing: " "},
			fractional: 0.0,
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.nf.formatFraction(tt.fractional)
			if err != nil {
				t.Fatalf("formatFraction failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("formatFraction() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestAddThousandsSeparator(t *testing.T) {
	tests := []struct {
		input     string
		separator string
		expected  string
	}{
		{"123", ",", "123"},
		{"1234", ",", "1,234"},
		{"12345", ",", "12,345"},
		{"1234567", ",", "1,234,567"},
		{"1234567890", ".", "1.234.567.890"},
		{"42", " ", "42"},
	}

	for _, tt := range tests {
		result := addThousandsSeparator(tt.input, tt.separator)
		if result != tt.expected {
			t.Errorf("addThousandsSeparator(%q, %q) = %q, want %q", tt.input, tt.separator, result, tt.expected)
		}
	}
}

func TestGreatestCommonDivisor(t *testing.T) {
	tests := []struct {
		a, b     int64
		expected int64
	}{
		{8, 12, 4},
		{5, 8, 1},
		{48, 18, 6},
		{0, 5, 5},
		{7, 0, 7},
	}

	for _, tt := range tests {
		result := greatestCommonDivisor(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("greatestCommonDivisor(%d, %d) = %d, want %d", tt.a, tt.b, result, tt.expected)
		}
	}
}
