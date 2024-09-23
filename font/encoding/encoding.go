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
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/pdfenc"
)

// An Encoding describes the meaning of character codes in a simple font.
// Normally, the character codes are mapped to names.
// For use with non-embedded fonts, a code can also refer to glyphs
// from the built-in encoding of a (potentially not yet loaded) font.
// For use with TrueType fonts, raw codes can alse be used, leaving the
// interpretation of the code to the font.
type Encoding struct {
	data  []cmap.CID
	names []string
	find  map[string]uint16
}

func New() *Encoding {
	find := make(map[string]uint16)
	return &Encoding{
		data: make([]cmap.CID, 256),
		find: find,
	}
}

func (e *Encoding) SetName(code byte, name string) cmap.CID {
	var cid cmap.CID

	if name != ".notdef" {
		idx, ok := e.find[name]
		if !ok {
			idx = uint16(len(e.names))
			e.names = append(e.names, name)
			e.find[name] = idx
		}
		cid = makeCID(cidClassName, idx, code)
	}

	if e.data[code] != 0 && e.data[code] != cid {
		panic("duplicate encoding")
	}

	e.data[code] = cid
	return cid
}

func (e *Encoding) LookupCID(code []byte) cmap.CID {
	if len(code) != 1 {
		return 0
	}
	return e.data[code[0]]
}

func (e *Encoding) LookupNotdefCID(code []byte) cmap.CID {
	return 0
}

// AsPDFType1 returns the /Encoding entry for the font dictionary of a Type 1 font.
// If `builtin` is not nil, it will be used as the builtin encoding of the font.
func (e *Encoding) AsPDFType1(builtin []string) (pdf.Native, error) {
	// Check whether any codes refer to the built-in encoding
	// of the font.
	usesBuiltin := false
	for _, c := range e.data {
		if cidClass(c) == cidClassBuiltin {
			usesBuiltin = true
			break
		}
	}
	_ = usesBuiltin
	panic("not implemented")
}

func ExtractType1(r pdf.Getter, obj pdf.Object, isEmbedded, isSymbolic bool) (*Encoding, error) {
	obj, err := pdf.Resolve(r, obj)
	if err != nil {
		return nil, err
	}

	e := New()

	switch obj := obj.(type) {
	case nil:
		e.fillBuiltInEncoding()

	case pdf.Name:
		err := e.fillNamedEncoding(obj)
		if err != nil {
			return nil, err
		}

	case pdf.Dict:
		// construct the base encoding
		base, err := pdf.GetName(r, obj["BaseEncoding"])
		if err != nil {
			return nil, err
		}
		if base != "" {
			err := e.fillNamedEncoding(base)
			if err != nil {
				return nil, err
			}
		} else if !isEmbedded && !isSymbolic {
			e.fillStandardEncoding()
		} else {
			e.fillBuiltInEncoding()
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
				e.SetName(byte(code), string(x))
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
		for i := range 256 {
			e.data[i] = makeCID(cidClassRaw, 0, byte(i))
		}

	case pdf.Name:
		err := e.fillNamedEncoding(obj)
		if err != nil {
			return nil, err
		}

	case pdf.Dict:
		// construct the base encoding
		base, err := pdf.GetName(r, obj["BaseEncoding"])
		if err != nil {
			return nil, err
		}
		if base != "" {
			err := e.fillNamedEncoding(base)
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
				e.SetName(byte(code), string(x))
				code++
			default:
				return nil, pdf.Errorf("encoding: expected Integer or Name, got %T", x)
			}
		}

		// fill any remaining slots using the standard encoding
		for code := range 256 {
			if e.data[code] != 0 {
				continue
			}
			if name := pdfenc.Standard.Encoding[code]; name != ".notdef" {
				e.SetName(byte(code), name)
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
			e.SetName(byte(code), string(x))
			code++
		default:
			return nil, pdf.Errorf("encoding: expected Integer or Name, got %T", x)
		}
	}

	return e, nil
}

func (e *Encoding) fillBuiltInEncoding() {
	for i := range 256 {
		e.data[i] = makeCID(cidClassBuiltin, 0, byte(i))
	}
}

func (e *Encoding) fillNamedEncoding(name pdf.Name) error {
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
		e.SetName(byte(code), name)
	}

	return nil
}

func (e *Encoding) fillStandardEncoding() {
	for code, name := range pdfenc.Standard.Encoding {
		e.SetName(byte(code), name)
	}
}

func makeCID(class byte, data uint16, code byte) cmap.CID {
	return cmap.CID(class)<<24 | cmap.CID(data)<<8 | cmap.CID(code)
}

func cidClass(c cmap.CID) byte {
	return byte(c >> 24)
}

const (
	cidClassNotDef byte = iota
	cidClassBuiltin
	cidClassName
	cidClassRaw
)
