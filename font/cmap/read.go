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

package cmap

import (
	"fmt"
	"io"

	"seehuhn.de/go/postscript"
	pscid "seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
)

// Extract reads a CMap from a PDF file.
func Extract(r pdf.Getter, obj pdf.Object) (*InfoOld, error) {
	obj, err := pdf.Resolve(r, obj)
	if err != nil {
		return nil, err
	}
	switch obj := obj.(type) {
	case pdf.Name:
		r, err := openPredefined(string(obj))
		if err != nil {
			return nil, err
		}
		defer r.Close()
		return Read(r, nil)
	case *pdf.Stream:
		if _, ok := obj.Dict["UseCMap"].(pdf.Name); ok {
			panic("not implemented: UseCMap") // TODO(voss): implement this
		}

		r, err := pdf.DecodeStream(r, obj, 0)
		if err != nil {
			return nil, err
		}
		return Read(r, nil)
	default:
		return nil, fmt.Errorf("invalid CMap object: %v", obj)
	}
}

func Read(r io.Reader, other map[string]*InfoOld) (*InfoOld, error) {
	raw, err := postscript.ReadCMap(r)
	if err != nil {
		return nil, err
	}

	res := &InfoOld{
		ROS:            &CIDSystemInfo{},
		CodeSpaceRange: nil,
		WMode:          0,
	}

	if tp, _ := raw["CMapType"].(postscript.Integer); !(tp == 0 || tp == 1) {
		return nil, fmt.Errorf("invalid CMapType: %v", tp)
	}
	if name, ok := raw["CMapName"].(postscript.Name); ok {
		res.Name = string(name)
	} else {
		return nil, fmt.Errorf("invalid CMapName: %v", raw["CMapName"])
	}
	if wmode, ok := raw["WMode"].(postscript.Integer); ok {
		res.WMode = int(wmode)
	}
	if ROS, ok := raw["CIDSystemInfo"].(postscript.Dict); !ok {
		return nil, fmt.Errorf("invalid CIDSystemInfo: %v", raw["CIDSystemInfo"])
	} else {
		ros := &CIDSystemInfo{}
		if registry, ok := ROS["Registry"].(postscript.String); !ok {
			return nil, fmt.Errorf("invalid Registry: %v", ROS["Registry"])
		} else {
			ros.Registry = string(registry)
		}
		if ordering, ok := ROS["Ordering"].(postscript.String); !ok {
			return nil, fmt.Errorf("invalid Ordering: %v", ROS["Ordering"])
		} else {
			ros.Ordering = string(ordering)
		}
		if supplement, ok := ROS["Supplement"].(postscript.Integer); !ok {
			return nil, fmt.Errorf("invalid Supplement: %v", ROS["Supplement"])
		} else {
			ros.Supplement = pdf.Integer(supplement)
		}
		res.ROS = ros
	}

	codeMap, ok := raw["CodeMap"].(*postscript.CMapInfo)
	if !ok {
		return nil, fmt.Errorf("unsupported CMap format")
	}

	if codeMap.UseCMap != "" {
		res.UseCMap = string(codeMap.UseCMap)
	}

	var rr []charcode.Range
	if codeMap.UseCMap != "" {
		if other == nil {
			other = make(map[string]*InfoOld)
		}
		if other, ok := other[string(codeMap.UseCMap)]; ok {
			rr = append(rr, other.CodeSpaceRange...)
		} else if other, ok := builtinCS[string(codeMap.UseCMap)]; ok {
			rr = append(rr, other...)
		} else {
			return nil, fmt.Errorf("unknown CMap %q", codeMap.UseCMap)
		}
	}
	var rrFile []charcode.Range
	for _, r := range codeMap.CodeSpaceRanges {
		rrFile = append(rrFile, charcode.Range{Low: r.Low, High: r.High})
	}
	// TODO(voss): do we need to sort the ranges?
	res.CodeSpaceRange = charcode.CodeSpaceRange(append(rr, rrFile...))
	res.CSFile = charcode.CodeSpaceRange(rrFile)

	for _, m := range codeMap.CidChars {
		code, k := res.CodeSpaceRange.Decode(m.Src)
		if k != len(m.Src) || code < 0 {
			return nil, fmt.Errorf("invalid code <%02x>", m.Src)
		}
		if cid, ok := m.Dst.(postscript.Integer); !ok {
			return nil, fmt.Errorf("invalid CID %v", m.Dst)
		} else {
			res.Singles = append(res.Singles, SingleOld{
				Code:  code,
				Value: pscid.CID(cid),
			})
		}
	}

	for _, m := range codeMap.CidRanges {
		low, k := res.CodeSpaceRange.Decode(m.Low)
		if k != len(m.Low) || low < 0 {
			return nil, fmt.Errorf("invalid code <%02x>", m.Low)
		}
		high, k := res.CodeSpaceRange.Decode(m.High)
		if k != len(m.High) || high < 0 {
			return nil, fmt.Errorf("invalid code <%02x>", m.High)
		}
		if cid, ok := m.Dst.(postscript.Integer); !ok {
			return nil, fmt.Errorf("invalid CID %v", m.Dst)
		} else {
			res.Ranges = append(res.Ranges, RangeOld{
				First: low,
				Last:  high,
				Value: pscid.CID(cid),
			})
		}
	}

	return res, nil
}
