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

package cmap

import (
	"strconv"

	"seehuhn.de/go/pdf"
)

// CIDSystemInfo describes a character collection covered by a font.
// A character collection implies an encoding which maps Character IDs to glyphs.
//
// See section 5.11.2 of the PLRM and section 9.7.3 of PDF 32000-1:2008.
type CIDSystemInfo struct {
	Registry   string
	Ordering   string
	Supplement pdf.Integer
}

// ExtractCIDSystemInfo extracts a CIDSystemInfo object from a PDF file.
func ExtractCIDSystemInfo(r pdf.Getter, obj pdf.Object) (*CIDSystemInfo, error) {
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

	return &CIDSystemInfo{
		Registry:   registry,
		Ordering:   ordering,
		Supplement: supplement,
	}, nil
}

func (ROS *CIDSystemInfo) String() string {
	return ROS.Registry + "-" + ROS.Ordering + "-" + strconv.Itoa(int(ROS.Supplement))
}

// Embed converts the CIDSystemInfo object into a PDF object.
// This implements the [pdf.Embedder] interface.
func (ROS *CIDSystemInfo) Embed(rm *pdf.ResourceManager) (pdf.Object, pdf.Unused, error) {
	var zero pdf.Unused

	dict := pdf.Dict{
		"Registry":   pdf.String(ROS.Registry),
		"Ordering":   pdf.String(ROS.Ordering),
		"Supplement": ROS.Supplement,
	}

	// TODO(voss): embed as an indirect object?

	return dict, zero, nil
}
