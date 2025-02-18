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
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/pdfenc"
)

// An EncodingOld describes a mapping between one-byte character codes and CIDs.
//
// CID values can represent either glyph names, or entries in the built-in
// encoding of a font.  The interpretation of CID values is specific to the
// encoder instance.  CID 0 is reserved for unmapped codes.
//
// TODO(voss): remove
//
// Deprecated: Use one of the other implementations instead.
type EncodingOld struct {
	enc        [256]cmap.CID
	glyphNames []string
}

// New allocates a new Encoding object.
func New() *EncodingOld {
	return &EncodingOld{}
}

// Allocate allocates a new CID for a named glyph.
//
// If a CID has already been allocated for the glyph name, the same CID is
// returned.  Otherwise, a new CID is allocated and returned.
func (e *EncodingOld) Allocate(glyphName string) cmap.CID {
	if glyphName == "" {
		panic("encoding: missing glyph name")
	}
	if glyphName == ".notdef" {
		return 0
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

// UseBuiltinEncoding maps a character code to the corresponding glyph
// of the built-in encoding.
func (e *EncodingOld) UseBuiltinEncoding(code byte) cmap.CID {
	cid := 1 + cmap.CID(code)
	e.enc[code] = cid
	return cid
}

// GlyphName returns the glyph name associated with a CID.
//
// For codes mapped via the built-in encoding, the empty string is returned.
func (e *EncodingOld) GlyphName(cid cmap.CID) string {
	if cid == 0 {
		return ".notdef"
	}

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
func (e *EncodingOld) Decode(code byte) cmap.CID {
	return e.enc[code]
}

// ExtractType1Old extracts an encoding from a Type1 font dictionary.
//
// Deprecated: Use [ExtractType1] instead.
//
// TODO(voss): remove
func ExtractType1Old(r pdf.Getter, dicts *font.Dicts) (*EncodingOld, error) {
	obj, err := pdf.Resolve(r, dicts.FontDict["Encoding"])
	if err != nil {
		return nil, err
	}

	e := New()

	switch obj := obj.(type) {
	case nil:
		e.initBuiltinEncoding()

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
		} else if dicts.IsNonSymbolic() && dicts.IsExternal() {
			e.initStandardEncoding()
		} else {
			e.initBuiltinEncoding()
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

func ExtractTrueType(r pdf.Getter, obj pdf.Object) (*EncodingOld, error) {
	obj, err := pdf.Resolve(r, obj)
	if err != nil {
		return nil, err
	}

	e := New()

	switch obj := obj.(type) {
	case nil:
		e.initBuiltinEncoding()

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

func ExtractType3Old(r pdf.Getter, obj pdf.Object) (*EncodingOld, error) {
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

func (e *EncodingOld) initBuiltinEncoding() {
	for code := range 256 {
		e.UseBuiltinEncoding(byte(code))
	}
}

func (e *EncodingOld) initNamedEncoding(name pdf.Name) error {
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

func (e *EncodingOld) initStandardEncoding() {
	for code, name := range pdfenc.Standard.Encoding {
		if name == ".notdef" {
			continue
		}
		e.enc[code] = e.Allocate(name)
	}
}
