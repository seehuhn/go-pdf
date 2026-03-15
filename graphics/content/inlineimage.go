// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package content

import (
	"bytes"
	"fmt"
	"io"

	"seehuhn.de/go/pdf"
)

// inlineFilterAbbreviations maps abbreviated inline image filter names
// (PDF 2.0, Table 90) to their full names.
var inlineFilterAbbreviations = map[pdf.Name]pdf.Name{
	"AHx": "ASCIIHexDecode",
	"A85": "ASCII85Decode",
	"LZW": "LZWDecode",
	"Fl":  "FlateDecode",
	"RL":  "RunLengthDecode",
	"CCF": "CCITTFaxDecode",
	"DCT": "DCTDecode",
}

// DecodeInlineImage decompresses the image data from an inline image operator.
// The operator must have name [OpInlineImage] with two arguments:
// a [pdf.Dict] containing image parameters and a [pdf.String] holding the
// raw image data.
func DecodeInlineImage(op Operator) ([]byte, error) {
	if op.Name != OpInlineImage {
		return nil, fmt.Errorf("expected %s operator, got %s", OpInlineImage, op.Name)
	}
	if len(op.Args) < 2 {
		return nil, fmt.Errorf("inline image: expected 2 arguments, got %d", len(op.Args))
	}

	dict, ok := op.Args[0].(pdf.Dict)
	if !ok {
		return nil, fmt.Errorf("inline image: expected Dict, got %T", op.Args[0])
	}
	rawData, ok := op.Args[1].(pdf.String)
	if !ok {
		return nil, fmt.Errorf("inline image: expected String, got %T", op.Args[1])
	}

	data := []byte(rawData)

	// extract filter name(s)
	filterObj, _ := dict["F"]
	if filterObj == nil {
		filterObj, _ = dict["Filter"]
	}
	if filterObj == nil {
		return data, nil
	}

	// extract decode parameters
	parmsObj, _ := dict["DP"]
	if parmsObj == nil {
		parmsObj, _ = dict["DecodeParms"]
	}

	type filterSpec struct {
		name  pdf.Name
		parms pdf.Dict
	}

	var filters []filterSpec
	switch f := filterObj.(type) {
	case pdf.Name:
		if full, ok := inlineFilterAbbreviations[f]; ok {
			f = full
		}
		var pDict pdf.Dict
		if d, ok := parmsObj.(pdf.Dict); ok {
			pDict = d
		}
		filters = append(filters, filterSpec{f, pDict})
	case pdf.Array:
		parmsArr, _ := parmsObj.(pdf.Array)
		for i, elem := range f {
			name, ok := elem.(pdf.Name)
			if !ok {
				return nil, fmt.Errorf("inline image: filter element %d: expected Name, got %T", i, elem)
			}
			if full, ok := inlineFilterAbbreviations[name]; ok {
				name = full
			}
			var pDict pdf.Dict
			if i < len(parmsArr) {
				if d, ok := parmsArr[i].(pdf.Dict); ok {
					pDict = d
				}
			}
			filters = append(filters, filterSpec{name, pDict})
		}
	default:
		return nil, fmt.Errorf("inline image: unexpected filter type %T", filterObj)
	}

	// chain filters
	var r io.Reader = bytes.NewReader(data)
	var closers []io.Closer
	for _, fs := range filters {
		f := pdf.MakeFilter(fs.name, fs.parms)
		rc, err := f.Decode(pdf.V2_0, r)
		if err != nil {
			return nil, err
		}
		closers = append(closers, rc)
		r = rc
	}

	result, err := io.ReadAll(r)
	for i := len(closers) - 1; i >= 0; i-- {
		if cerr := closers[i].Close(); err == nil {
			err = cerr
		}
	}
	if err != nil {
		return nil, fmt.Errorf("inline image: reading decompressed data: %w", err)
	}
	return result, nil
}
