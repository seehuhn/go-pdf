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
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Format converts a measurement value into formatted text using
// the provided number format array.
func Format(value float64, formats []*NumberFormat) (string, error) {
	if len(formats) == 0 {
		return "", fmt.Errorf("no number formats provided")
	}

	var result strings.Builder
	currentValue := value

	for i, format := range formats {
		// Step b: Multiply by conversion factor
		valueInUnits := currentValue * format.ConversionFactor

		// Extract integer and fractional parts
		integerPart := int64(valueInUnits)
		fractionalPart := valueInUnits - float64(integerPart)

		// Step c: If no fractional portion, format and complete
		if fractionalPart == 0 {
			if integerPart != 0 || (i == len(formats)-1 && value == 0) {
				if integerPart == 0 && value == 0 {
					// Special case for zero - always format with unit
					formatted := "0" + format.getSuffixSpacing() + format.Unit
					if result.Len() > 0 {
						result.WriteString(" ")
					}
					result.WriteString(formatted)
				} else {
					formatted := format.formatInteger(integerPart)
					if formatted != "" {
						result.WriteString(formatted)
					}
				}
			}
			break
		}

		// Step d: If fractional portion and last format, format fraction and complete
		if i == len(formats)-1 {
			if len(formats) == 1 && format.FractionFormat == FractionDecimal {
				// Special case: single format with decimal - format as single decimal value
				formatted := format.formatDecimalComplete(valueInUnits)
				if result.Len() > 0 {
					result.WriteString(" ")
				}
				result.WriteString(formatted)

			} else if len(formats) == 1 && (format.FractionFormat == FractionRound || format.FractionFormat == FractionTruncate) {
				// Special case: single format with rounding/truncation - apply to complete value
				var rounded int64
				if format.FractionFormat == FractionRound {
					rounded = int64(valueInUnits + 0.5)
				} else {
					rounded = int64(valueInUnits)
				}
				formatted := strconv.FormatInt(rounded, 10) + format.getSuffixSpacing() + format.Unit
				if result.Len() > 0 {
					result.WriteString(" ")
				}
				result.WriteString(formatted)

			} else if integerPart != 0 && fractionalPart != 0 {
				// Multiple formats or non-decimal: separate integer and fractional parts
				intFormatted := format.formatIntegerOnly(integerPart)
				fracFormatted, err := format.formatFractionOnly(fractionalPart)
				if err != nil {
					return "", err
				}

				if result.Len() > 0 {
					result.WriteString(" ")
				}
				result.WriteString(intFormatted + " " + fracFormatted + format.getSuffixSpacing() + format.Unit)

			} else if integerPart != 0 {
				// Only integer part
				intFormatted := format.formatInteger(integerPart)
				if intFormatted != "" {
					if result.Len() > 0 {
						result.WriteString(" ")
					}
					result.WriteString(intFormatted)
				}
			} else if fractionalPart != 0 {
				// Only fractional part
				fracFormatted, err := format.formatFraction(fractionalPart)
				if err != nil {
					return "", err
				}
				if fracFormatted != "" {
					if result.Len() > 0 {
						result.WriteString(" ")
					}
					result.WriteString(fracFormatted)
				}
			}
			break
		}

		// Step e: If fractional portion and more formats, continue
		if integerPart != 0 {
			formatted := format.formatInteger(integerPart)
			if formatted != "" {
				if result.Len() > 0 {
					result.WriteString(" ")
				}
				result.WriteString(formatted)
			}
		}

		// Continue with fractional part for next format
		currentValue = fractionalPart
	}

	return result.String(), nil
}

// formatInteger formats an integer value with thousands separator and unit label.
func (nf *NumberFormat) formatInteger(value int64) string {
	if value == 0 {
		return ""
	}

	// Convert to string
	numStr := strconv.FormatInt(value, 10)

	// Apply thousands separator if present
	if nf.ThousandsSeparator != "" {
		numStr = addThousandsSeparator(numStr, nf.ThousandsSeparator)
	}

	// Add unit label with appropriate spacing
	spacing := nf.getSuffixSpacing()
	if nf.PrefixLabel {
		spacing = nf.getPrefixSpacing()
		return spacing + nf.Unit + spacing + numStr
	} else {
		return numStr + spacing + nf.Unit
	}
}

// formatFraction formats a fractional value according to the FractionFormat.
func (nf *NumberFormat) formatFraction(fractional float64) (string, error) {
	if fractional == 0 {
		return "", nil
	}

	switch nf.FractionFormat {
	case FractionDecimal:
		return nf.formatDecimal(fractional)
	case FractionFraction:
		return nf.formatFractionValue(fractional)
	case FractionRound:
		rounded := int64(fractional + 0.5)
		if rounded == 0 {
			return "", nil
		}
		return nf.formatInteger(rounded), nil
	case FractionTruncate:
		truncated := int64(fractional)
		if truncated == 0 {
			return "", nil
		}
		return nf.formatInteger(truncated), nil
	default:
		return nf.formatDecimal(fractional)
	}
}

// formatDecimal formats a fractional value as a decimal.
func (nf *NumberFormat) formatDecimal(fractional float64) (string, error) {
	// Calculate number of decimal places from precision
	decimalPlaces := 0
	precision := nf.Precision
	for precision >= 10 {
		precision /= 10
		decimalPlaces++
	}

	// Format with calculated decimal places
	formatted := fmt.Sprintf("%."+strconv.Itoa(decimalPlaces)+"f", fractional)

	// Remove leading zero
	if strings.HasPrefix(formatted, "0.") {
		formatted = formatted[1:]
	}

	// Replace decimal separator if needed
	decimalSep := nf.getDecimalSeparator()
	if decimalSep != "." {
		formatted = strings.Replace(formatted, ".", decimalSep, 1)
	}

	// Add unit label with appropriate spacing
	spacing := nf.getSuffixSpacing()
	if nf.PrefixLabel {
		spacing = nf.getPrefixSpacing()
		return spacing + nf.Unit + spacing + formatted, nil
	} else {
		return formatted + spacing + nf.Unit, nil
	}
}

// formatFractionValue formats a fractional value as a fraction.
func (nf *NumberFormat) formatFractionValue(fractional float64) (string, error) {
	denominator := nf.Precision
	numerator := int64(fractional*float64(denominator) + 0.5)

	if numerator == 0 {
		return "", nil
	}

	// Reduce fraction if not forced to keep exact
	if !nf.ForceExactFraction {
		gcd := greatestCommonDivisor(numerator, int64(denominator))
		numerator /= gcd
		denominator = int(int64(denominator) / gcd)
	}

	// Format as fraction
	fractionStr := fmt.Sprintf("%d/%d", numerator, denominator)

	// Add unit label with appropriate spacing
	spacing := nf.getSuffixSpacing()
	if nf.PrefixLabel {
		spacing = nf.getPrefixSpacing()
		return spacing + nf.Unit + spacing + fractionStr, nil
	} else {
		return fractionStr + spacing + nf.Unit, nil
	}
}

// addThousandsSeparator adds thousands separators to a number string.
func addThousandsSeparator(numStr, separator string) string {
	if len(numStr) <= 3 {
		return numStr
	}

	var result strings.Builder
	for i, char := range numStr {
		if i > 0 && (len(numStr)-i)%3 == 0 {
			result.WriteString(separator)
		}
		result.WriteRune(char)
	}
	return result.String()
}

// formatIntegerOnly formats an integer value with thousands separator but no unit label.
func (nf *NumberFormat) formatIntegerOnly(value int64) string {
	numStr := strconv.FormatInt(value, 10)

	// Apply thousands separator if present
	if nf.ThousandsSeparator != "" {
		numStr = addThousandsSeparator(numStr, nf.ThousandsSeparator)
	}

	return numStr
}

// formatFractionOnly formats a fractional value without unit label.
func (nf *NumberFormat) formatFractionOnly(fractional float64) (string, error) {
	if fractional == 0 {
		return "", nil
	}

	switch nf.FractionFormat {
	case FractionDecimal:
		return nf.formatDecimalOnly(fractional)
	case FractionFraction:
		return nf.formatFractionValueOnly(fractional)
	case FractionRound:
		rounded := int64(fractional + 0.5)
		return strconv.FormatInt(rounded, 10), nil
	case FractionTruncate:
		truncated := int64(fractional)
		return strconv.FormatInt(truncated, 10), nil
	default:
		return nf.formatDecimalOnly(fractional)
	}
}

// formatDecimalOnly formats a fractional value as a decimal without unit.
func (nf *NumberFormat) formatDecimalOnly(fractional float64) (string, error) {
	// Calculate number of decimal places from precision
	decimalPlaces := 0
	precision := nf.Precision
	for precision >= 10 {
		precision /= 10
		decimalPlaces++
	}

	// Format with calculated decimal places
	formatted := fmt.Sprintf("%."+strconv.Itoa(decimalPlaces)+"f", fractional)

	// Remove leading zero
	if strings.HasPrefix(formatted, "0.") {
		formatted = formatted[1:]
	}

	// Replace decimal separator if needed
	decimalSep := nf.getDecimalSeparator()
	if decimalSep != "." {
		formatted = strings.Replace(formatted, ".", decimalSep, 1)
	}

	return formatted, nil
}

// formatFractionValueOnly formats a fractional value as a fraction without unit.
func (nf *NumberFormat) formatFractionValueOnly(fractional float64) (string, error) {
	denominator := nf.Precision
	numerator := int64(fractional*float64(denominator) + 0.5)

	if numerator == 0 {
		return "", nil
	}

	// Reduce fraction if not forced to keep exact
	if !nf.ForceExactFraction {
		gcd := greatestCommonDivisor(numerator, int64(denominator))
		numerator /= gcd
		denominator = int(int64(denominator) / gcd)
	}

	return fmt.Sprintf("%d/%d", numerator, denominator), nil
}

// formatDecimalComplete formats a complete decimal value with unit label.
func (nf *NumberFormat) formatDecimalComplete(value float64) string {
	// Calculate number of decimal places from precision
	decimalPlaces := 0
	precision := nf.Precision
	for precision >= 10 {
		precision /= 10
		decimalPlaces++
	}

	// Format with calculated decimal places
	formatted := fmt.Sprintf("%."+strconv.Itoa(decimalPlaces)+"f", value)

	// Apply thousands separator to integer part if present
	if nf.ThousandsSeparator != "" {
		parts := strings.Split(formatted, ".")
		if len(parts) == 2 {
			parts[0] = addThousandsSeparator(parts[0], nf.ThousandsSeparator)
			formatted = parts[0] + "." + parts[1]
		}
	}

	// Replace decimal separator if needed
	decimalSep := nf.getDecimalSeparator()
	if decimalSep != "." {
		formatted = strings.Replace(formatted, ".", decimalSep, 1)
	}

	// Add unit label with appropriate spacing
	spacing := nf.getSuffixSpacing()
	if nf.PrefixLabel {
		spacing = nf.getPrefixSpacing()
		return spacing + nf.Unit + spacing + formatted
	} else {
		return formatted + spacing + nf.Unit
	}
}

// greatestCommonDivisor calculates the GCD of two integers.
func greatestCommonDivisor(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	return int64(math.Abs(float64(a)))
}
