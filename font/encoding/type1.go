// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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
	"bytes"
	"errors"
	"fmt"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/pdfenc"
)

// DescribeEncodingType1 returns the /Encoding entry for the font dictionary
// of a Type 1 font.  The arguments are the encoding used by the client,
// and the font's builtin encoding.
//
// See section 9.6.1 and 9.6.5 of ISO 32000-2:2020.
func DescribeEncodingType1(encoding, builtin []string) pdf.Object {
	type cand struct {
		name pdf.Object
		enc  []string
	}
	candidates := []cand{
		{nil, builtin},
		{pdf.Name("WinAnsiEncoding"), pdfenc.WinAnsiEncoding[:]},
		{pdf.Name("MacRomanEncoding"), pdfenc.MacRomanEncoding[:]},
		{pdf.Name("MacExpertEncoding"), pdfenc.MacExpertEncoding[:]},
	}

	type D struct {
		code    int
		newName pdf.Name
	}
	var diff []D
	var desc pdf.Dict
	descLen := math.MaxInt
	for _, cand := range candidates {
		diff = diff[:0]
		for code, name := range encoding {
			if name != ".notdef" && name != cand.enc[code] {
				diff = append(diff, D{code, pdf.Name(name)})
			}
		}
		if len(diff) == 0 {
			return cand.name
		}

		newDesc := pdf.Dict{}
		if cand.name != nil {
			newDesc["BaseEncoding"] = cand.name
		}
		var a pdf.Array
		prev := 256
		for _, d := range diff {
			if d.code != prev+1 {
				a = append(a, pdf.Integer(d.code))
			}
			a = append(a, d.newName)
			prev = d.code
		}
		newDesc["Differences"] = a

		b := &bytes.Buffer{}
		newDesc.PDF(b)
		if b.Len() < descLen {
			desc = newDesc
			descLen = b.Len()
		}
	}

	return desc
}

// UndescribeEncodingType1 returns the encoding used by the client, given
// the /Encoding entry for the font dictionary of a Type 1 font and the
// font's builtin encoding.
//
// This function is nearly the inverse of [DescribeEncodingType1]: if
// the name assigned to a code is not `.notdef`, then [DescribeEncodingType1]
// followed by [UndescribeEncodingType1] will return the same name.
func UndescribeEncodingType1(r pdf.Getter, desc pdf.Object, builtin []string) ([]string, error) {
	desc, err := pdf.Resolve(r, desc)
	if err != nil {
		return nil, err
	}

	switch desc := desc.(type) {
	case nil:
		return builtin, nil
	case pdf.Name:
		return getNamedEncoding(desc)
	case pdf.Dict:
		base, err := pdf.GetName(r, desc["BaseEncoding"])
		if err != nil {
			return nil, err
		}
		res := make([]string, 256)
		baseEnc := builtin
		if base != "" {
			baseEnc, err = getNamedEncoding(base)
			if err != nil {
				return nil, err
			}
		}
		if baseEnc == nil {
			return nil, errors.New("encoding: invalid base encoding")
		}
		copy(res, baseEnc)

		a, err := pdf.GetArray(r, desc["Differences"])
		if err != nil {
			return nil, err
		}
		code := -1
		for _, x := range a {
			switch x := x.(type) {
			case pdf.Integer:
				code = int(x)
			case pdf.Name:
				if code < 0 || code >= 256 {
					return nil, fmt.Errorf("encoding: invalid code %d", code)
				}
				res[code] = string(x)
				code++
			default:
				return nil, fmt.Errorf("encoding: expected Integer or Name, got %T", x)
			}
		}

		return res, nil
	default:
		return nil, fmt.Errorf("encoding: expected Name or Dict, got %T", desc)
	}
}

func getNamedEncoding(name pdf.Name) ([]string, error) {
	switch name {
	case "WinAnsiEncoding":
		return pdfenc.WinAnsiEncoding[:], nil
	case "MacRomanEncoding":
		return pdfenc.MacRomanEncoding[:], nil
	case "MacExpertEncoding":
		return pdfenc.MacExpertEncoding[:], nil
	default:
		return nil, fmt.Errorf("unknown encoding %q", name)
	}
}
