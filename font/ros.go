// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/postscript/cid"
)

// ExtractCIDSystemInfo extracts a CIDSystemInfo object from a PDF file.
func ExtractCIDSystemInfo(r pdf.Getter, obj pdf.Object) (*cid.SystemInfo, error) {
	dict, err := pdf.GetDict(r, obj)
	if err != nil {
		return nil, err
	}

	var registry, ordering string
	s, e1 := pdf.GetString(r, dict["Registry"])
	registry = string(s)
	s, e2 := pdf.GetString(r, dict["Ordering"])
	ordering = string(s)

	supplement, e3 := pdf.GetInteger(r, dict["Supplement"])
	if supplement < 0 || supplement > math.MaxInt32 {
		supplement = 0
	}

	// only report errors if absolutely necessary
	if registry == "" && ordering == "" && supplement == 0 {
		if e1 != nil {
			return nil, e1
		}
		if e2 != nil {
			return nil, e2
		}
		if e3 != nil {
			return nil, e3
		}
	}

	return &cid.SystemInfo{
		Registry:   registry,
		Ordering:   ordering,
		Supplement: int32(supplement),
	}, nil
}

func WriteCIDSystemInfo(rm *pdf.ResourceManager, ROS *cid.SystemInfo) (pdf.Object, error) {
	if ROS == nil {
		return nil, nil
	}
	obj := pdf.Dict{
		"Registry":   pdf.String(ROS.Registry),
		"Ordering":   pdf.String(ROS.Ordering),
		"Supplement": pdf.Integer(ROS.Supplement),
	}

	ref := rm.Out.Alloc()
	err := rm.Out.Put(ref, obj)
	if err != nil {
		return nil, err
	}

	return ref, nil
}
