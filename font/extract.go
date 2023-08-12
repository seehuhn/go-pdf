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

package font

import (
	"fmt"

	"seehuhn.de/go/pdf"
)

type Dicts struct {
	FontDict       pdf.Dict
	CIDFontDict    pdf.Dict
	FontDescriptor pdf.Dict
	FontProgram    pdf.Reference
	FontProgramKey pdf.Name
}

func ExtractDicts(r pdf.Getter, ref pdf.Reference) (*Dicts, error) {
	res := &Dicts{}

	fontDict, err := pdf.GetDict(r, ref)
	if err != nil {
		return nil, err
	}
	err = pdf.CheckDictType(r, fontDict, "Font")
	if err != nil {
		return nil, err
	}
	res.FontDict = fontDict

	subtype, err := pdf.GetName(r, fontDict["Subtype"])
	if err != nil {
		return nil, err
	}

	if subtype == "Type0" {
		descendantFonts, err := pdf.GetArray(r, fontDict["DescendantFonts"])
		if err != nil {
			return nil, err
		} else if len(descendantFonts) != 1 {
			return nil, fmt.Errorf("invalid descendant fonts: %v", descendantFonts)
		}

		cidFontDict, err := pdf.GetDict(r, descendantFonts[0])
		if err != nil {
			return nil, err
		}
		err = pdf.CheckDictType(r, cidFontDict, "Font")
		if err != nil {
			return nil, err
		}
		res.CIDFontDict = cidFontDict

		fontDict = cidFontDict
	}

	fontDescriptor, err := pdf.GetDict(r, fontDict["FontDescriptor"])
	if err != nil {
		return nil, err
	}
	if fontDescriptor == nil {
		return res, nil
	}
	res.FontDescriptor = fontDescriptor

	for _, key := range []pdf.Name{"FontFile", "FontFile2", "FontFile3"} {
		if ref, _ := fontDescriptor[key].(pdf.Reference); ref != 0 {
			res.FontProgram = ref
			res.FontProgramKey = key
			break
		}
	}

	return res, nil
}
