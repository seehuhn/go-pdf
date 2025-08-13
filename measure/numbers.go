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
	"seehuhn.de/go/pdf"
)

// FractionalFormat specifies how fractional values are displayed.
type FractionalFormat byte

const (
	FractionDecimal  FractionalFormat = 0 // show as decimal
	FractionFraction FractionalFormat = 1 // show as fraction
	FractionRound    FractionalFormat = 2 // round to whole unit
	FractionTruncate FractionalFormat = 3 // truncate to whole unit
)

// NumberFormat represents a unit of measurement with formatting information.
type NumberFormat struct {
	// Unit specifies the label for displaying this unit.
	Unit string

	// ConversionFactor is the multiplier used to convert from the previous unit
	// or initial measurement value to this unit.
	ConversionFactor float64

	// Precision specifies the precision for decimal display or denominator for fractions.
	Precision int

	// FractionFormat specifies how to display fractional values.
	FractionFormat FractionalFormat

	// ForceExactFraction prevents reduction of fractions and truncation of zeros.
	ForceExactFraction bool

	// ThousandsSeparator specifies text used between thousands in numerical display.
	ThousandsSeparator string

	// DecimalSeparator specifies text used as the decimal position.
	// An empty string uses the standard period separator.
	DecimalSeparator string

	// PrefixSpacing specifies text concatenated before the unit label.
	// An empty string uses a single space.
	PrefixSpacing string

	// SuffixSpacing specifies text concatenated after the unit label.
	// An empty string uses a single space.
	SuffixSpacing string

	// PrefixLabel determines unit label position.
	// false positions the label after the value, true positions it before.
	PrefixLabel bool

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// ExtractNumberFormat extracts a NumberFormat from a PDF object.
func ExtractNumberFormat(r pdf.Getter, obj pdf.Object) (*NumberFormat, error) {
	dict, err := pdf.GetDict(r, obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing number format dictionary")
	}

	nf := &NumberFormat{}

	// Extract required fields
	unit, err := pdf.GetString(r, dict["U"])
	if err != nil {
		return nil, err
	}
	nf.Unit = string(unit)

	conversion, err := pdf.GetNumber(r, dict["C"])
	if err != nil {
		return nil, err
	}
	nf.ConversionFactor = float64(conversion)

	precision, err := pdf.GetInteger(r, dict["D"])
	if err != nil {
		return nil, err
	}
	nf.Precision = int(precision)

	// Extract optional fields with defaults
	if f, err := pdf.Optional(pdf.GetName(r, dict["F"])); err != nil {
		return nil, err
	} else {
		switch f {
		case "D":
			nf.FractionFormat = FractionDecimal
		case "F":
			nf.FractionFormat = FractionFraction
		case "R":
			nf.FractionFormat = FractionRound
		case "T":
			nf.FractionFormat = FractionTruncate
		default:
			nf.FractionFormat = FractionDecimal
		}
	}

	if fd, err := pdf.Optional(pdf.GetBoolean(r, dict["FD"])); err != nil {
		return nil, err
	} else {
		nf.ForceExactFraction = bool(fd)
	}

	if rt, err := pdf.Optional(pdf.GetString(r, dict["RT"])); err != nil {
		return nil, err
	} else {
		nf.ThousandsSeparator = string(rt)
	}

	if rd, err := pdf.Optional(pdf.GetString(r, dict["RD"])); err != nil {
		return nil, err
	} else {
		nf.DecimalSeparator = string(rd)
	}

	if ps, err := pdf.Optional(pdf.GetString(r, dict["PS"])); err != nil {
		return nil, err
	} else {
		nf.PrefixSpacing = string(ps)
	}

	if ss, err := pdf.Optional(pdf.GetString(r, dict["SS"])); err != nil {
		return nil, err
	} else {
		nf.SuffixSpacing = string(ss)
	}

	if o, err := pdf.Optional(pdf.GetName(r, dict["O"])); err != nil {
		return nil, err
	} else {
		nf.PrefixLabel = (o == "P")
	}

	return nf, nil
}

// Embed converts the NumberFormat into a PDF dictionary.
func (nf *NumberFormat) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	dict := pdf.Dict{
		"U": pdf.String(nf.Unit),
		"C": pdf.Number(nf.ConversionFactor),
		"D": pdf.Integer(nf.Precision),
	}

	// Optional Type field
	if rm.Out.GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("NumberFormat")
	}

	// Fraction format
	switch nf.FractionFormat {
	case FractionDecimal:
		dict["F"] = pdf.Name("D")
	case FractionFraction:
		dict["F"] = pdf.Name("F")
	case FractionRound:
		dict["F"] = pdf.Name("R")
	case FractionTruncate:
		dict["F"] = pdf.Name("T")
	default:
		dict["F"] = pdf.Name("D")
	}

	// Force exact fraction
	if nf.ForceExactFraction {
		dict["FD"] = pdf.Boolean(true)
	}

	// Separators and spacing - only encode if different from Go zero values
	dict["RT"] = pdf.String(nf.ThousandsSeparator)

	if nf.DecimalSeparator != "" {
		dict["RD"] = pdf.String(nf.DecimalSeparator)
	}

	if nf.PrefixSpacing != "" {
		dict["PS"] = pdf.String(nf.PrefixSpacing)
	}

	if nf.SuffixSpacing != "" {
		dict["SS"] = pdf.String(nf.SuffixSpacing)
	}

	// Label position
	if nf.PrefixLabel {
		dict["O"] = pdf.Name("P")
	} else {
		dict["O"] = pdf.Name("S")
	}

	if nf.SingleUse {
		return dict, zero, nil
	}

	ref := rm.Out.Alloc()
	err := rm.Out.Put(ref, dict)
	if err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
}

// Helper methods for getting separator/spacing values with defaults

func (nf *NumberFormat) getDecimalSeparator() string {
	if nf.DecimalSeparator == "" {
		return "."
	}
	return nf.DecimalSeparator
}

func (nf *NumberFormat) getPrefixSpacing() string {
	if nf.PrefixSpacing == "" {
		return " "
	}
	return nf.PrefixSpacing
}

func (nf *NumberFormat) getSuffixSpacing() string {
	if nf.SuffixSpacing == "" {
		return " "
	}
	return nf.SuffixSpacing
}
