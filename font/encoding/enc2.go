// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package encoding

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/pdfenc"
)

// Type1 gives the glyph name for each code point.
// The empty string is used for unmapped glyphs.
// The special value [UseBuiltin] indicates that the corresponding glyph from
// the build-in encoding should be used.
//
// TODO(voss): use `func(code byte) string` instead?
type Type1 [256]string

const UseBuiltin = "@"

// ExtractType1New extracts the encoding from the /Encoding entry of a Type1 font
// dictionary.
func ExtractType1New(r pdf.Getter, obj pdf.Object) (*Type1, error) {
	panic("not implemented")
}

// AsPDF returns the /Encoding entry for Type1 font dictionary.
//
// If the argument nonSymbolicExt is true, the function assumes that the font
// has the non-symbolic flag set in the font descriptor and that the font is
// not be embedded in the PDF file.  In this case, the built-in encoding must
// either be used for all mapped codes, or not at all.
//
// The resulting PDF object describes an encoding which maps all characters
// mapped by e to the given glyph name, but it may also imply glyph names for
// the unmapped codes.
func (e *Type1) AsPDF(nonSymbolicExt bool, opt pdf.OutputOptions) (pdf.Object, error) {
	type candInfo struct {
		encName     pdf.Native
		enc         []string
		differences pdf.Array
	}

	// First check whether we can use the built-in encoding.
	canUseBuiltIn := true
	for code := range 256 {
		if e[code] != "" && e[code] != UseBuiltin {
			canUseBuiltIn = false
			break
		}
	}
	if canUseBuiltIn {
		return nil, nil
	}

	// Next, if no codes are mapped to the built-in encoding, we may be able to
	// use a named encoding.
	noBuiltin := true
	for code := range 256 {
		if e[code] == UseBuiltin {
			noBuiltin = false
			break
		}
	}
	if noBuiltin {
		candidates := []*candInfo{
			{encName: pdf.Name("WinAnsiEncoding"), enc: pdfenc.WinAnsi.Encoding[:]},
			{encName: pdf.Name("MacRomanEncoding"), enc: pdfenc.MacRoman.Encoding[:]},
			{encName: pdf.Name("MacExpertEncoding"), enc: pdfenc.MacExpert.Encoding[:]},
		}
	candidateLoop:
		for _, cand := range candidates {
			for code := range 256 {
				if glyphName := e[code]; glyphName != "" && glyphName != cand.enc[code] {
					// we got a conflict, try the next candidate
					continue candidateLoop
				}
			}
			return cand.encName, nil
		}
	}

	// If we reach this point, we need an encoding dictionary. We use the base
	// encoding which leads to the smallest Differences array.

	var candidates []*candInfo
	if noBuiltin {
		candidates = []*candInfo{
			{encName: pdf.Name("WinAnsiEncoding"), enc: pdfenc.WinAnsi.Encoding[:]},
			{encName: pdf.Name("MacRomanEncoding"), enc: pdfenc.MacRoman.Encoding[:]},
			{encName: pdf.Name("MacExpertEncoding"), enc: pdfenc.MacExpert.Encoding[:]},
		}
		if nonSymbolicExt {
			// If a font is marked as non-symbolic in the font descriptor and
			// the font is not embedded, a missing `BaseEncoding` field
			// represents the standard encoding.
			candidates = append(candidates,
				&candInfo{encName: nil, enc: pdfenc.Standard.Encoding[:]},
			)
		}
		for _, cand := range candidates {
			lastDiff := 999
			for code := range 256 {
				glyphName := e[code]
				if glyphName == "" || glyphName == cand.enc[code] {
					continue
				}

				if code != lastDiff+1 {
					cand.differences = append(cand.differences, pdf.Integer(code))
				}
				cand.differences = append(cand.differences, pdf.Name(glyphName))
				lastDiff = code
			}
		}
	} else {
		if nonSymbolicExt {
			// If a font is marked as symbolic in the font descriptor or the
			// font is embedded, a missing `BaseEncoding` field represents the
			// font's built-in encoding. Since one these conditions is
			// violated, there is no way to refer to the built-in encoding.
			return nil, errors.New("invalid encoding")
		}

		var diff pdf.Array
		lastDiff := 999
		for code := range 256 {
			glyphName := e[code]
			if glyphName == "" || glyphName == UseBuiltin {
				continue
			}

			if code != lastDiff+1 {
				diff = append(diff, pdf.Integer(code))
			}
			diff = append(diff, pdf.Name(glyphName))
			lastDiff = code
		}

		candidates = append(candidates, &candInfo{
			encName:     nil,
			differences: diff,
		})
	}

	// candidates is non-empty at this point

	var bestDict pdf.Dict
	bestDiffLength := 999
	for _, cand := range candidates {
		if L := len(cand.differences); L < bestDiffLength {
			bestDiffLength = L
			bestDict = pdf.Dict{}
			if opt.HasAny(pdf.OptDictTypes) {
				bestDict["Type"] = pdf.Name("Encoding")
			}
			if cand.encName != nil {
				bestDict["BaseEncoding"] = cand.encName
			}
			if L > 0 {
				bestDict["Differences"] = cand.differences
			}
		}
	}
	return bestDict, nil
}
