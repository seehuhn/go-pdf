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

package decode

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
)

func decodeAnnot3D(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict) (*annotation.Annot3D, error) {
	annot3D := &annotation.Annot3D{}

	// Extract common annotation fields
	if err := decodeCommon(x, path, &annot3D.Common, dict); err != nil {
		return nil, err
	}

	// 3DD (required)
	if threeDD := dict["3DD"]; threeDD != nil {
		annot3D.D = threeDD
	} else {
		return nil, pdf.Error("3DD field is required")
	}

	// 3DV (optional)
	if threeDV := dict["3DV"]; threeDV != nil {
		annot3D.V = threeDV
	}

	// 3DA (optional)
	if dict["3DA"] != nil {
		if threeDa, err := x.GetDict(path, dict["3DA"]); err == nil {
			activation := &annotation.Annot3DActivation{
				TB: true, // default value
			}

			// A (optional) - default XA
			if a, err := x.GetName(path, threeDa["A"]); err == nil {
				activation.A = a
			}

			// AIS (optional) - default L
			if ais, err := x.GetName(path, threeDa["AIS"]); err == nil {
				activation.AIS = ais
			}

			// D (optional) - default PI
			if d, err := x.GetName(path, threeDa["D"]); err == nil {
				activation.D = d
			}

			// DIS (optional) - default U
			if dis, err := x.GetName(path, threeDa["DIS"]); err == nil {
				activation.DIS = dis
			}

			// TB (optional, PDF 1.7) - default true
			if tb, err := x.GetBoolean(path, threeDa["TB"]); err == nil {
				activation.TB = bool(tb)
			}

			// NP (optional, PDF 1.7) - default false
			if np, err := x.GetBoolean(path, threeDa["NP"]); err == nil {
				activation.NP = bool(np)
			}

			// Style (optional, PDF 2.0) - default Embedded
			if style, err := x.GetName(path, threeDa["Style"]); err == nil {
				activation.Style = style
			}

			// Window (optional, PDF 2.0)
			if window, ok := threeDa["Window"].(pdf.Reference); ok {
				activation.Window = window
			}

			// Transparent (optional, PDF 2.0) - default false
			if transparent, err := x.GetBoolean(path, threeDa["Transparent"]); err == nil {
				activation.Transparent = bool(transparent)
			}

			annot3D.A = activation
		}
	}

	// 3DI (optional) - default true
	annot3D.I = true // default value
	if dict["3DI"] != nil {
		if threeDI, err := x.GetBoolean(path, dict["3DI"]); err == nil {
			annot3D.I = bool(threeDI)
		}
	}

	// 3DB (optional)
	if threeDB, err := pdf.GetFloatArray(x.R, dict["3DB"]); err == nil && len(threeDB) == 4 {
		annot3D.B = &pdf.Rectangle{
			LLx: threeDB[0],
			LLy: threeDB[1],
			URx: threeDB[2],
			URy: threeDB[3],
		}
	}

	// 3DU (optional, PDF 2.0)
	if threeDU, ok := dict["3DU"].(pdf.Reference); ok {
		annot3D.U = threeDU
	}

	// GEO (optional, PDF 2.0)
	if geo, ok := dict["GEO"].(pdf.Reference); ok {
		annot3D.GEO = geo
	}

	return annot3D, nil
}
