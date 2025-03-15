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

// Simple represents the encoding of a simple font.
// This is a map which gives the glyph name for each code point.
// The empty string indicates unused codes.
// The special value [UseBuiltin] indicates that the corresponding glyph from
// the built-in encoding should be used.
type Simple func(code byte) string

const UseBuiltin = "@"

var (
	Builtin Simple = func(code byte) string {
		return UseBuiltin
	}
	WinAnsi Simple = func(code byte) string {
		return pdfenc.WinAnsi.Encoding[code]
	}
	MacRoman Simple = func(code byte) string {
		return pdfenc.MacRoman.Encoding[code]
	}
	MacExpert Simple = func(code byte) string {
		return pdfenc.MacExpert.Encoding[code]
	}
	Standard Simple = func(code byte) string {
		return pdfenc.Standard.Encoding[code]
	}
)

// ExtractType1 extracts the encoding from the /Encoding entry of a Type1
// font dictionary.
//
// If the argument nonSymbolicExt is true, the function assumes that the font
// has the non-symbolic flag set in the font descriptor and that the font is
// not embedded in the PDF file.
//
// If /Encoding is malformed, the font's built-in encoding is used as a
// fallback.
func ExtractType1(r pdf.Getter, obj pdf.Object, nonSymbolicExt bool) (Simple, error) {
	obj, err := pdf.Resolve(r, obj)
	if err != nil {
		return nil, err
	}

	if name, ok := obj.(pdf.Name); ok {
		switch name {
		case "WinAnsiEncoding":
			return WinAnsi, nil
		case "MacRomanEncoding":
			return MacRoman, nil
		case "MacExpertEncoding":
			return MacExpert, nil
		}
	}

	dict, _ := obj.(pdf.Dict)
	if dict == nil {
		return Builtin, nil
	}
	if err := pdf.CheckDictType(r, dict, "Encoding"); err != nil {
		return Builtin, err
	}

	// If we reach this point, we have found an encoding dictionary.

	var baseEnc Simple
	baseEncName, _ := pdf.GetName(r, dict["BaseEncoding"])
	switch baseEncName {
	case "WinAnsiEncoding":
		baseEnc = WinAnsi
	case "MacRomanEncoding":
		baseEnc = MacRoman
	case "MacExpertEncoding":
		baseEnc = MacExpert
	default:
		if nonSymbolicExt { // non-symbolic and not embedded
			baseEnc = Standard
		} else { // symbolic or embedded
			baseEnc = Builtin
		}
	}

	differences := make(map[byte]string)
	if diffArray, _ := pdf.GetArray(r, dict["Differences"]); diffArray != nil {
		currentCode := pdf.Integer(-1)
		for _, item := range diffArray {
			item, err = pdf.Resolve(r, item)
			if err != nil {
				return nil, err
			}

			switch item := item.(type) {
			case pdf.Integer:
				currentCode = item

			case pdf.Name:
				if currentCode >= 0 && currentCode < 256 {
					differences[byte(currentCode)] = string(item)
					currentCode++
				}
			}
		}
	}
	if len(differences) == 0 {
		return baseEnc, nil
	}

	return func(code byte) string {
		if glyphName, ok := differences[code]; ok {
			return glyphName
		}
		return baseEnc(code)
	}, nil
}

// AsPDFType1 returns the /Encoding entry for Type1 font dictionary.
//
// If the argument baseIsStd is true, Differences arrays record changes from
// the standard encoding. Otherwise, Differences arrays record changes from the
// built-in encoding. The flag should be set if the font is non-symbolic and is
// not be embedded in the PDF file. If the flag is set, the built-in encoding
// must either be used for all mapped codes, or not at all.
//
// The resulting PDF object describes an encoding which maps all characters
// mapped by e to the given glyph name, but it may also imply glyph names for
// the unmapped codes.
func (e Simple) AsPDFType1(baseIsStd bool, opt pdf.OutputOptions) (pdf.Object, error) {
	type candInfo struct {
		encName     pdf.Native
		enc         []string
		differences pdf.Array
	}

	// First check whether we can use the built-in encoding.
	canUseBuiltin := true
	for code := range 256 {
		if e(byte(code)) != "" && e(byte(code)) != UseBuiltin {
			canUseBuiltin = false
			break
		}
	}
	if canUseBuiltin {
		return nil, nil
	}

	// Next, if no codes are mapped to the built-in encoding, we may be able to
	// use a named encoding.
	noBuiltin := true
	for code := range 256 {
		if e(byte(code)) == UseBuiltin {
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
				if glyphName := e(byte(code)); glyphName != "" && glyphName != cand.enc[code] {
					// we got a conflict, try the next candidate
					continue candidateLoop
				}
			}
			return cand.encName, nil
		}
	}

	// If we reach this point, we need an encoding dictionary. We choose the
	// base encoding which leads to the smallest Differences array.

	var candidates []*candInfo
	if noBuiltin {
		candidates = []*candInfo{
			{encName: pdf.Name("WinAnsiEncoding"), enc: pdfenc.WinAnsi.Encoding[:]},
			{encName: pdf.Name("MacRomanEncoding"), enc: pdfenc.MacRoman.Encoding[:]},
			{encName: pdf.Name("MacExpertEncoding"), enc: pdfenc.MacExpert.Encoding[:]},
		}
		if baseIsStd {
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
				glyphName := e(byte(code))
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
		if baseIsStd {
			// If the font is marked as non-symbolic in the font descriptor and
			// the font is not embedded, a missing `BaseEncoding` field
			// represents the standard encoding. In this case, there is no way
			// to refer to the built-in encoding.
			return nil, errInvalidEncoding
		}

		var diff pdf.Array
		lastDiff := 999
		for code := range 256 {
			glyphName := e(byte(code))
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
		if len(cand.differences) == 0 {
			// Adobe Reader compatibility:
			//
			// Adobe Reader seems to require for there to be a non-empty
			// /Differences array.  If we don't have any differences (because
			// we are using the standard encoding), we just list one of the
			// codes as a difference.
			//
			// TODO(voss): find out what other libraries are doing.
			cand.differences = pdf.Array{
				pdf.Integer(32),
				pdf.Name(cand.enc[32]),
			}
		}

		if L := len(cand.differences); L < bestDiffLength {
			bestDiffLength = L
			bestDict = pdf.Dict{}
			if cand.encName != nil {
				bestDict["BaseEncoding"] = cand.encName
			}
			if L > 0 {
				bestDict["Differences"] = cand.differences
			}
		}
	}
	if opt.HasAny(pdf.OptDictTypes) {
		bestDict["Type"] = pdf.Name("Encoding")
	}

	return bestDict, nil
}

// ExtractType3 extracts the encoding from the /Encoding entry of a Type3
// font dictionary.
func ExtractType3(r pdf.Getter, obj pdf.Object) (Simple, error) {
	dict, err := pdf.GetDictTyped(r, obj, "Encoding")
	if err != nil {
		return nil, err
	}

	diffArray, err := pdf.GetArray(r, dict["Differences"])
	if err != nil {
		return nil, err
	}

	differences := make(map[byte]string)

	currentCode := pdf.Integer(-1)
	for _, item := range diffArray {
		item, err = pdf.Resolve(r, item)
		if err != nil {
			return nil, err
		}

		switch item := item.(type) {
		case pdf.Integer:
			currentCode = item

		case pdf.Name:
			if currentCode >= 0 && currentCode < 256 {
				differences[byte(currentCode)] = string(item)
				currentCode++
			}
		}
	}

	if len(differences) == 0 {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("missing /Differences array"),
		}
	}

	return func(code byte) string {
		return differences[code]
	}, nil
}

// AsPDFType3 returns the /Encoding entry for Type3 font dictionary.
func (e Simple) AsPDFType3(opt pdf.OutputOptions) (pdf.Object, error) {
	var differences pdf.Array

	lastDiff := 999
	for code := range 256 {
		glyphName := e(byte(code))
		if glyphName == "" {
			continue
		}

		if code != lastDiff+1 {
			differences = append(differences, pdf.Integer(code))
		}
		differences = append(differences, pdf.Name(glyphName))
		lastDiff = code
	}

	dict := pdf.Dict{
		"Differences": differences,
	}
	if opt.HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Encoding")
	}

	return dict, nil
}

var errInvalidEncoding = errors.New("invalid encoding")
