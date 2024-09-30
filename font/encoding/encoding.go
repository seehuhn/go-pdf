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
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/pdfenc"
)

// An Encoding describes a mapping between one-byte character codes and CIDs.
//
// CID values can represent either glyph names, or entries in the built-in
// encoding of a font.  The interpretation of CID values is specific to the
// encoder instance which was used to allocate them.  CID 0 is reserved for
// unmapped codes.
type Encoding struct {
	enc        [256]cmap.CID
	glyphNames []string
}

// New allocates a new Encoding object.
func New() *Encoding {
	return &Encoding{}
}

// Allocate allocates a new CID for a named glyph.
//
// If a CID has already been allocated for the glyph name, the same CID is
// returned.  Otherwise, a new CID is allocated and returned.
func (e *Encoding) Allocate(glyphName string) cmap.CID {
	if glyphName == "" {
		panic("encoding: missing glyph name")
	}

	for i, prevName := range e.glyphNames {
		if prevName == glyphName {
			return cmap.CID(1 + 256 + i)
		}
	}

	cid := cmap.CID(1 + 256 + len(e.glyphNames))
	e.glyphNames = append(e.glyphNames, glyphName)
	return cid
}

func (e *Encoding) UseBuiltinEncoding(code byte) cmap.CID {
	cid := 1 + cmap.CID(code)
	e.enc[code] = cid
	return cid
}

// GlyphName returns the glyph name associated with a CID.
//
// For unmapped codes (CID 0) and codes mapped via the built-in encoding, the
// empty string is returned.
func (e *Encoding) GlyphName(cid cmap.CID) string {
	base := cmap.CID(1 + 256)
	if cid < base {
		return ""
	}
	idx := int(cid - base)
	if idx >= len(e.glyphNames) {
		return ""
	}

	return e.glyphNames[idx]
}

// Decode returns the CID associated with a character code.
// If the code is not mapped, 0 is returned.
func (e *Encoding) Decode(code byte) cmap.CID {
	return e.enc[code]
}

// AsPDFType1 returns the /Encoding entry for Type1 font dictionary.
//
// If `builtin` is not nil, it will be used as the builtin encoding of the
// font. If the argument nonSymbolicExt is true, the function assumes that the
// font has the non-symbolic flag set in the font descriptor and that the font
// will not be embedded into the PDF file.
//
// The resulting PDF object describes an encoding which maps all characters
// mapped by e in the specified way, but it may also map additional codes.
func (e *Encoding) AsPDFType1(builtin []string, nonSymbolicExt bool, opt pdf.OutputOptions) (pdf.Native, error) {
	type candInfo struct {
		encName     pdf.Native
		enc         []string
		differences pdf.Array
		impossible  bool
	}

	// First try whether we can match the encoding without using an encoding
	// dictionary.
	var candidates []*candInfo
	candidates = append(candidates,
		&candInfo{encName: nil, enc: builtin},
		&candInfo{encName: pdf.Name("WinAnsiEncoding"), enc: pdfenc.WinAnsi.Encoding[:]},
		&candInfo{encName: pdf.Name("MacRomanEncoding"), enc: pdfenc.MacRoman.Encoding[:]},
		&candInfo{encName: pdf.Name("MacExpertEncoding"), enc: pdfenc.MacExpert.Encoding[:]},
	)
candidateLoop:
	for _, cand := range candidates {
		for code := range 256 {
			cid := e.enc[code]
			if cid == 0 {
				// If we don't use a code, this code can't conflict.
				continue
			}

			glyphName := e.GlyphName(cid)
			if glyphName == "" {
				if cand.encName == nil {
					// If we can, just use the built-in encoding.
					continue
				} else if code < len(builtin) {
					// Otherwise, if we know the glyph name in the built-in
					// encoding, we can try to find this glyph name in the
					// named encodings.
					glyphName = builtin[code]
				} else {
					// If we don't know the glyph name, none of the named
					// encodings can be used.
					//
					// Note: this assumes that the built-in encoding is tried
					// first.
					break candidateLoop
				}
			}

			if code < len(cand.enc) && cand.enc[code] == glyphName {
				continue
			}

			// If we got a conflict, try the next candidate.
			continue candidateLoop
		}
		return cand.encName, nil
	}

	// If we need an encoding dictionary, use the base encoding which leads to
	// the smallest Differences array.
	if nonSymbolicExt {
		// If a font has the non-symbolic flag set in the font descriptor and
		// the font is not embedded, a missing `BaseEncoding` field represents
		// the standard encoding.  In all other cases, a missing `BaseEncoding`
		// field represents the font's built-in encoding.
		candidates[0] = &candInfo{encName: nil, enc: pdfenc.Standard.Encoding[:]}
	}
candidateLoop2:
	for _, cand := range candidates {
		lastDiff := 999
		for code := range 256 {
			cid := e.enc[code]
			if cid == 0 {
				// If we don't use a code, this code can't conflict.
				continue
			}

			glyphName := e.GlyphName(cid)
			if glyphName == "" {
				if cand.encName == nil && !nonSymbolicExt {
					// If we can, just use the built-in encoding.
					continue
				} else if code < len(builtin) {
					// Otherwise, if we know the glyph name in the built-in
					// encoding, we can use this glyph name.
					glyphName = builtin[code]
				} else {
					// If we don't know the glyph name, named encodings cannot
					// be used.
					cand.impossible = true
					continue candidateLoop2
				}
			}

			if code < len(cand.enc) && cand.enc[code] == glyphName {
				continue
			}

			if code != lastDiff+1 {
				cand.differences = append(cand.differences, pdf.Integer(code))
			}
			cand.differences = append(cand.differences, pdf.Name(glyphName))
			lastDiff = code
		}
	}

	var bestDict pdf.Dict
	bestDiffLength := 999
	for _, cand := range candidates {
		if cand.impossible {
			continue
		}
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
	if bestDict == nil {
		return nil, errors.New("the built-in encoding must be specified for this encoding")
	}
	return bestDict, nil
}

// AsPDFTrueType returns the /Encoding entry in a TrueType font dictionary.
//
// If `builtin` is not nil, it will be used as the builtin encoding of the
// font. The function assumes that the non-symbolic flag in the font descriptor
// is set, and on success it always returns either a name or a dictionary.  The
// output never refers to to the built-in encoding of the font.
//
// The resulting PDF object describes an encoding which maps all characters
// mapped by e in the specified way, but it may also map additional codes.
//
// The glyph names for all mapped codes must be known (either via the encoding,
// or via the builtin encoding).  Otherwise an error is returned.
func (e *Encoding) AsPDFTrueType(builtin []string, opt pdf.OutputOptions) (pdf.Native, error) {
	// First check that all glyph names are known.
	for code := range 256 {
		cid := e.enc[code]
		if cid == 0 {
			continue
		}
		if e.GlyphName(cid) != "" {
			continue
		}
		if code < len(builtin) && builtin[code] != "" {
			continue
		}
		return nil, fmt.Errorf("encoding: missing glyph name for code %d", code)
	}

	type candInfo struct {
		encName     pdf.Native
		enc         []string
		differences pdf.Array
	}

	// Next, try whether we can match the encoding without using an encoding
	// dictionary.
	var candidates []*candInfo
	candidates = append(candidates,
		&candInfo{encName: pdf.Name("WinAnsiEncoding"), enc: pdfenc.WinAnsi.Encoding[:]},
		&candInfo{encName: pdf.Name("MacRomanEncoding"), enc: pdfenc.MacRoman.Encoding[:]},
	)
candidateLoop:
	for _, cand := range candidates {
		for code := range 256 {
			cid := e.enc[code]
			if cid == 0 {
				continue
			}

			glyphName := e.GlyphName(cid)
			if glyphName == "" {
				glyphName = builtin[code]
			}

			if cand.enc[code] != glyphName {
				// If we got a conflict, try the next candidate.
				continue candidateLoop
			}
		}
		return cand.encName, nil
	}

	// If we need an encoding dictionary, use the base encoding which leads to
	// the smaller Differences array.
	for _, cand := range candidates {
		lastDiff := 999
		for code := range 256 {
			cid := e.enc[code]
			if cid == 0 {
				continue
			}

			glyphName := e.GlyphName(cid)
			if glyphName == "" {
				glyphName = builtin[code]
			}
			if cand.enc[code] != glyphName {
				if code != lastDiff+1 {
					cand.differences = append(cand.differences, pdf.Integer(code))
				}
				cand.differences = append(cand.differences, pdf.Name(glyphName))
				lastDiff = code
			}
		}
	}

	cand := candidates[0]
	if len(candidates[1].differences) < len(cand.differences) {
		cand = candidates[1]
	}

	dict := pdf.Dict{}
	if opt.HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Encoding")
	}
	if cand.encName != nil {
		dict["BaseEncoding"] = cand.encName
	}
	if len(cand.differences) > 0 {
		dict["Differences"] = cand.differences
	}
	return dict, nil
}

// AsPDFType3 returns the /Encoding entry for the font dictionary of a Type 3
// font.
//
// On success, the function returns a [pdf.Dict] object.
func (e *Encoding) AsPDFType3(opt pdf.OutputOptions) (pdf.Native, error) {
	dict := pdf.Dict{}
	if opt.HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Encoding")
	}

	var differences pdf.Array
	lastDiff := 999
	for code := range 256 {
		cid := e.enc[code]
		if cid == 0 {
			continue
		}
		name := pdf.Name(e.GlyphName(cid))
		if name == "" {
			return nil, fmt.Errorf("encoding: missing glyph name for code %d", code)
		}

		if code != lastDiff+1 {
			differences = append(differences, pdf.Integer(code))
		}
		differences = append(differences, name)
		lastDiff = code
	}
	dict["Differences"] = differences

	return dict, nil
}

func ExtractType1(r pdf.Getter, obj pdf.Object, nonSymbolicExt bool) (*Encoding, error) {
	obj, err := pdf.Resolve(r, obj)
	if err != nil {
		return nil, err
	}

	e := New()

	switch obj := obj.(type) {
	case nil:
		e.initBuiltInEncoding()

	case pdf.Name:
		err := e.initNamedEncoding(obj)
		if err != nil {
			return nil, err
		}

	case pdf.Dict:
		if err := pdf.CheckDictType(r, obj, "Encoding"); err != nil {
			return nil, err
		}

		// construct the base encoding
		base, err := pdf.GetName(r, obj["BaseEncoding"])
		if err != nil {
			return nil, err
		}
		if base != "" {
			err := e.initNamedEncoding(base)
			if err != nil {
				return nil, err
			}
		} else if nonSymbolicExt {
			e.initStandardEncoding()
		} else {
			e.initBuiltInEncoding()
		}

		// apply the differences
		a, err := pdf.GetArray(r, obj["Differences"])
		if err != nil {
			return nil, err
		}
		code := -1
		for _, x := range a {
			switch x := x.(type) {
			case pdf.Integer:
				if x < 0 || x >= 256 {
					return nil, pdf.Errorf("encoding: invalid code %d", x)
				}
				code = int(x)
			case pdf.Name:
				if code < 0 || code >= 256 {
					return nil, pdf.Errorf("encoding: invalid code %d", code)
				}
				e.enc[code] = e.Allocate(string(x))
				code++
			default:
				return nil, pdf.Errorf("encoding: expected Integer or Name, got %T", x)
			}
		}

	default:
		return nil, pdf.Errorf("encoding: expected Name or Dict, got %T", obj)
	}

	return e, nil
}

func ExtractTrueType(r pdf.Getter, obj pdf.Object) (*Encoding, error) {
	obj, err := pdf.Resolve(r, obj)
	if err != nil {
		return nil, err
	}

	e := New()

	switch obj := obj.(type) {
	case nil:
		e.initBuiltInEncoding()

	case pdf.Name:
		err := e.initNamedEncoding(obj)
		if err != nil {
			return nil, err
		}

	case pdf.Dict:
		if err := pdf.CheckDictType(r, obj, "Encoding"); err != nil {
			return nil, err
		}

		// construct the base encoding
		base, err := pdf.GetName(r, obj["BaseEncoding"])
		if err != nil {
			return nil, err
		}
		if base != "" {
			err := e.initNamedEncoding(base)
			if err != nil {
				return nil, err
			}
		}

		// apply the differences
		a, err := pdf.GetArray(r, obj["Differences"])
		if err != nil {
			return nil, err
		}
		code := -1
		for _, x := range a {
			switch x := x.(type) {
			case pdf.Integer:
				if x < 0 || x >= 256 {
					return nil, pdf.Errorf("encoding: invalid code %d", x)
				}
				code = int(x)
			case pdf.Name:
				if code < 0 || code >= 256 {
					return nil, pdf.Errorf("encoding: invalid code %d", code)
				}
				e.enc[code] = e.Allocate(string(x))
				code++
			default:
				return nil, pdf.Errorf("encoding: expected Integer or Name, got %T", x)
			}
		}

		// fill any remaining slots using the standard encoding
		for code := range 256 {
			if e.enc[code] != 0 {
				continue
			}
			if name := pdfenc.Standard.Encoding[code]; name != ".notdef" {
				e.enc[code] = e.Allocate(name)
			}
		}

	default:
		return nil, pdf.Errorf("encoding: expected Name or Dict, got %T", obj)
	}

	return e, nil
}

func ExtractType3(r pdf.Getter, obj pdf.Object) (*Encoding, error) {
	dict, err := pdf.GetDictTyped(r, obj, "Encoding")
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("encoding: missing Encoding dictionary")
	}

	e := New()

	// apply the differences
	a, err := pdf.GetArray(r, dict["Differences"])
	if err != nil {
		return nil, err
	}
	code := -1
	for _, x := range a {
		switch x := x.(type) {
		case pdf.Integer:
			if x < 0 || x >= 256 {
				return nil, pdf.Errorf("encoding: invalid code %d", x)
			}
			code = int(x)
		case pdf.Name:
			if code < 0 || code >= 256 {
				return nil, pdf.Errorf("encoding: invalid code %d", code)
			}
			e.enc[code] = e.Allocate(string(x))
			code++
		default:
			return nil, pdf.Errorf("encoding: expected Integer or Name, got %T", x)
		}
	}

	return e, nil
}

func (e *Encoding) initBuiltInEncoding() {
	for code := range 256 {
		e.UseBuiltinEncoding(byte(code))
	}
}

func (e *Encoding) initNamedEncoding(name pdf.Name) error {
	var enc []string
	switch name {
	case "WinAnsiEncoding":
		enc = pdfenc.WinAnsi.Encoding[:]
	case "MacRomanEncoding":
		enc = pdfenc.MacRoman.Encoding[:]
	case "MacExpertEncoding":
		enc = pdfenc.MacExpert.Encoding[:]
	default:
		return pdf.Errorf("encoding: unknown named encoding %s", name)
	}

	for code, name := range enc {
		if name == ".notdef" {
			continue
		}
		e.enc[code] = e.Allocate(name)
	}

	return nil
}

func (e *Encoding) initStandardEncoding() {
	for code, name := range pdfenc.Standard.Encoding {
		if name == ".notdef" {
			continue
		}
		e.enc[code] = e.Allocate(name)
	}
}
