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

type Encoding struct {
	data  []cmap.CID
	names []string
	find  map[string]uint16
}

func New() *Encoding {
	find := make(map[string]uint16)
	find[".notdef"] = 0
	return &Encoding{
		data: make([]cmap.CID, 256),
		find: find,
	}
}

func ExtractType1(r pdf.Getter, obj pdf.Object, isEmbedded, isSymbolic bool) (*Encoding, error) {
	obj, err := pdf.Resolve(r, obj)
	if err != nil {
		return nil, err
	}

	e := New()

	switch obj := obj.(type) {
	case nil:
		e.fillBuiltIn()

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
			e.fillBuiltIn()
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
				e.data[code] = e.get(string(x), byte(code))
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
				e.data[code] = e.get(string(x), byte(code))
				code++
			default:
				return nil, pdf.Errorf("encoding: expected Integer or Name, got %T", x)
			}
		}

		// fill any remaining slots using the standard encoding
		for i := range 256 {
			if e.data[i] != 0 {
				continue
			}
			if name := pdfenc.Standard.Encoding[i]; name != ".notdef" {
				e.data[i] = e.get(name, byte(code))
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
			e.data[code] = e.get(string(x), byte(code))
			code++
		default:
			return nil, pdf.Errorf("encoding: expected Integer or Name, got %T", x)
		}
	}

	return e, nil
}

func (e *Encoding) get(name string, code byte) cmap.CID {
	idx, ok := e.find[name]
	if !ok {
		idx = uint16(len(e.names))
		e.names = append(e.names, name)
		e.find[name] = idx
	}
	return makeCID(cidClassName, idx, code)
}

func (e *Encoding) fillBuiltIn() {
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
		e.data[code] = e.get(name, byte(code))
	}

	return nil
}

func (e *Encoding) fillStandardEncoding() {
	for code, name := range pdfenc.Standard.Encoding {
		e.data[code] = e.get(name, byte(code))
	}
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

func makeCID(class byte, data uint16, code byte) cmap.CID {
	return cmap.CID(class)<<24 | cmap.CID(data)<<8 | cmap.CID(code)
}

const (
	cidClassNotDef byte = iota
	cidClassBuiltin
	cidClassName
	cidClassRaw
)
