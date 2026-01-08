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

// PDF 2.0 sections: 12.9

// NumberFormat represents a unit of measurement with formatting information.
type NumberFormat struct {
	// Unit specifies the label for displaying this unit.
	Unit string

	// ConversionFactor is the multiplier used to convert from the previous unit
	// or initial measurement value to this unit.
	ConversionFactor float64

	// Precision specifies the precision for decimal display or denominator for
	// fractions.
	Precision int

	// FractionFormat specifies how to display fractional values.
	FractionFormat FractionalFormat

	// ForceExactFraction prevents reduction of fractions and truncation of
	// zeros.
	ForceExactFraction bool

	// ThousandsSeparator (optional) specifies text used between thousands in
	// numerical display.
	ThousandsSeparator string

	// DecimalSeparator (optional) specifies text used as the decimal position.
	//
	// When writing, an empty string can be used as a shorthand for a period.
	DecimalSeparator string

	// PrefixSpacing specifies text concatenated before the unit label.
	PrefixSpacing string

	// SuffixSpacing specifies text concatenated after the unit label.
	SuffixSpacing string

	// PrefixLabel determines unit label position. False positions the label
	// after the value, true positions it before.
	PrefixLabel bool

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// ExtractNumberFormat extracts a NumberFormat from a PDF object.
func ExtractNumberFormat(x *pdf.Extractor, obj pdf.Object) (*NumberFormat, error) {
	dict, err := pdf.GetDict(x.R, obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing number format dictionary")
	}

	nf := &NumberFormat{}

	// Extract required fields
	unit, err := pdf.GetString(x.R, dict["U"])
	if err != nil {
		return nil, err
	}
	if len(unit) == 0 {
		return nil, pdf.Error("missing required Unit")
	}
	nf.Unit = string(unit)

	conversion, err := pdf.GetNumber(x.R, dict["C"])
	if err != nil {
		return nil, err
	}
	nf.ConversionFactor = float64(conversion)

	// Extract optional fields with defaults
	if f, err := pdf.Optional(pdf.GetName(x.R, dict["F"])); err != nil {
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

	// Extract Precision - conditional based on FractionFormat
	precisionMeaningful := nf.FractionFormat == FractionDecimal || nf.FractionFormat == FractionFraction
	if precisionMeaningful {
		if precision, err := pdf.Optional(pdf.GetInteger(x.R, dict["D"])); err != nil {
			return nil, err
		} else if precision != 0 {
			nf.Precision = int(precision)
		} else {
			// Use default values when not present
			if nf.FractionFormat == FractionDecimal {
				nf.Precision = 100 // default for decimal
			} else {
				nf.Precision = 16 // default for fraction
			}
		}
	} else {
		nf.Precision = 0 // not meaningful for round/truncate
	}

	if fd, err := pdf.Optional(pdf.GetBoolean(x.R, dict["FD"])); err != nil {
		return nil, err
	} else {
		nf.ForceExactFraction = bool(fd)
	}

	if rt, err := pdf.Optional(pdf.GetString(x.R, dict["RT"])); err != nil {
		return nil, err
	} else {
		// RT present: use the value (even if empty string)
		nf.ThousandsSeparator = string(rt)
	} // If RT not present, PDF uses comma default, but we leave empty in Go

	if rd, err := pdf.Optional(pdf.GetString(x.R, dict["RD"])); err != nil {
		return nil, err
	} else if rd != nil && string(rd) != "" {
		// RD present and non-empty: use the value
		nf.DecimalSeparator = string(rd)
	} else {
		// RD not present or empty: use period
		nf.DecimalSeparator = "."
	}

	if ps, err := pdf.Optional(pdf.GetString(x.R, dict["PS"])); err != nil {
		return nil, err
	} else {
		// PS: store the value (empty if not present)
		nf.PrefixSpacing = string(ps)
	}

	if ss, err := pdf.Optional(pdf.GetString(x.R, dict["SS"])); err != nil {
		return nil, err
	} else {
		// SS: store the value (empty if not present)
		nf.SuffixSpacing = string(ss)
	}

	if o, err := pdf.Optional(pdf.GetName(x.R, dict["O"])); err != nil {
		return nil, err
	} else {
		nf.PrefixLabel = (o == "P")
	}

	return nf, nil
}

// Embed converts the NumberFormat into a PDF dictionary.
func (nf *NumberFormat) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	// Validate required fields
	if nf.Unit == "" {
		return nil, pdf.Errorf("missing required Unit")
	}
	if nf.ConversionFactor == 0 {
		return nil, pdf.Errorf("ConversionFactor cannot be zero")
	}

	// Validate Precision based on FractionFormat
	precisionMeaningful := nf.FractionFormat == FractionDecimal || nf.FractionFormat == FractionFraction
	if precisionMeaningful {
		if nf.Precision <= 0 {
			return nil, pdf.Errorf("Precision must be positive when FractionFormat is decimal or fraction")
		}
		if nf.FractionFormat == FractionDecimal && nf.Precision%10 != 0 && nf.Precision != 1 {
			return nil, pdf.Errorf("Precision must be 1 or a multiple of 10 for decimal format")
		}
	} else {
		if nf.Precision != 0 {
			return nil, pdf.Errorf("Precision must be 0 when FractionFormat is round or truncate")
		}
	}

	dict := pdf.Dict{
		"U": pdf.String(nf.Unit),
		"C": pdf.Number(nf.ConversionFactor),
	}

	// Optional Type field
	if rm.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("NumberFormat")
	}

	// Precision - only include when meaningful and different from defaults
	if precisionMeaningful {
		defaultPrecision := 100 // default for decimal
		if nf.FractionFormat == FractionFraction {
			defaultPrecision = 16 // default for fraction
		}
		if nf.Precision != defaultPrecision {
			dict["D"] = pdf.Integer(nf.Precision)
		}
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

	// Separators and spacing - optimize by using PDF defaults
	// RT: default is comma, so always write it (even if empty string)
	dict["RT"] = pdf.String(nf.ThousandsSeparator)

	// RD: empty string is shorthand for period, only write if different from period
	decimalSep := nf.DecimalSeparator
	if decimalSep == "" {
		decimalSep = "."
	}
	if decimalSep != "." {
		dict["RD"] = pdf.String(decimalSep)
	}

	// PS: default is single space, only write if different from default
	if nf.PrefixSpacing != " " {
		dict["PS"] = pdf.String(nf.PrefixSpacing)
	}

	// SS: default is single space, only write if different from default
	if nf.SuffixSpacing != " " {
		dict["SS"] = pdf.String(nf.SuffixSpacing)
	}

	// Label position
	if nf.PrefixLabel {
		dict["O"] = pdf.Name("P")
	} else {
		dict["O"] = pdf.Name("S")
	}

	if nf.SingleUse {
		return dict, nil
	}

	ref := rm.Alloc()
	err := rm.Out().Put(ref, dict)
	if err != nil {
		return nil, err
	}

	return ref, nil
}
