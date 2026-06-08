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

package annotation

import "seehuhn.de/go/pdf"

// shared array read/write helpers for the signature lock and seed value
// dictionaries. each writer omits the key for an empty slice; each reader skips
// malformed elements.

func writeNameArray(dict pdf.Dict, key pdf.Name, vals []pdf.Name) {
	if len(vals) == 0 {
		return
	}
	arr := make(pdf.Array, len(vals))
	for i, v := range vals {
		arr[i] = v
	}
	dict[key] = arr
}

func readNameArray(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object) ([]pdf.Name, error) {
	arr, err := pdf.Optional(x.GetArray(path, obj))
	if err != nil || len(arr) == 0 {
		return nil, err
	}
	out := make([]pdf.Name, 0, len(arr))
	for _, el := range arr {
		if name, err := pdf.Optional(x.GetName(path, el)); err != nil {
			return nil, err
		} else if name != "" {
			out = append(out, name)
		}
	}
	return out, nil
}

func writeTextStringArray(dict pdf.Dict, key pdf.Name, vals []string) {
	if len(vals) == 0 {
		return
	}
	arr := make(pdf.Array, len(vals))
	for i, s := range vals {
		arr[i] = pdf.TextString(s)
	}
	dict[key] = arr
}

func readTextStringArray(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object) ([]string, error) {
	arr, err := pdf.Optional(x.GetArray(path, obj))
	if err != nil || len(arr) == 0 {
		return nil, err
	}
	out := make([]string, 0, len(arr))
	for _, el := range arr {
		if s, err := pdf.Optional(pdf.GetTextString(x.R, el)); err != nil {
			return nil, err
		} else {
			out = append(out, string(s))
		}
	}
	return out, nil
}

func writeASCIIStringArray(dict pdf.Dict, key pdf.Name, vals []string) {
	if len(vals) == 0 {
		return
	}
	arr := make(pdf.Array, len(vals))
	for i, s := range vals {
		arr[i] = pdf.String(s)
	}
	dict[key] = arr
}

func readASCIIStringArray(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object) ([]string, error) {
	arr, err := pdf.Optional(x.GetArray(path, obj))
	if err != nil || len(arr) == 0 {
		return nil, err
	}
	out := make([]string, 0, len(arr))
	for _, el := range arr {
		if s, err := pdf.Optional(x.GetString(path, el)); err != nil {
			return nil, err
		} else {
			out = append(out, string(s))
		}
	}
	return out, nil
}

func writeByteStringArray(dict pdf.Dict, key pdf.Name, vals [][]byte) {
	if len(vals) == 0 {
		return
	}
	arr := make(pdf.Array, len(vals))
	for i, b := range vals {
		arr[i] = pdf.String(b)
	}
	dict[key] = arr
}

func readByteStringArray(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object) ([][]byte, error) {
	arr, err := pdf.Optional(x.GetArray(path, obj))
	if err != nil || len(arr) == 0 {
		return nil, err
	}
	out := make([][]byte, 0, len(arr))
	for _, el := range arr {
		if s, err := pdf.Optional(x.GetString(path, el)); err != nil {
			return nil, err
		} else {
			out = append(out, []byte(s))
		}
	}
	return out, nil
}
