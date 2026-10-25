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

func decodeAnnot3D(c pdf.Cursor, dict pdf.Dict) (*annotation.Annot3D, error) {
	annot3D := &annotation.Annot3D{}

	if err := decodeCommon(c, &annot3D.Common, dict); err != nil {
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
	if threeDa, err := pdf.Optional(c.Dict(dict["3DA"])); err != nil {
		return nil, err
	} else if threeDa != nil {
		activation := &annotation.Annot3DActivation{
			TB: true, // default value
		}

		// A (optional) - default XA
		if a, err := c.Name(threeDa["A"]); err == nil {
			activation.A = a
		} else if pdf.IsReadError(err) {
			return nil, err
		}

		// AIS (optional) - default L
		if ais, err := c.Name(threeDa["AIS"]); err == nil {
			activation.AIS = ais
		} else if pdf.IsReadError(err) {
			return nil, err
		}

		// D (optional) - default PI
		if d, err := c.Name(threeDa["D"]); err == nil {
			activation.D = d
		} else if pdf.IsReadError(err) {
			return nil, err
		}

		// DIS (optional) - default U
		if dis, err := c.Name(threeDa["DIS"]); err == nil {
			activation.DIS = dis
		} else if pdf.IsReadError(err) {
			return nil, err
		}

		// TB (optional, PDF 1.7) - default true
		if tb, ok, err := c.OptionalBoolean(threeDa["TB"]); err != nil {
			return nil, err
		} else if ok {
			activation.TB = tb
		}

		// NP (optional, PDF 1.7) - default false
		np, _, err := c.OptionalBoolean(threeDa["NP"])
		if err != nil {
			return nil, err
		}
		activation.NP = np

		// Style (optional, PDF 2.0) - default Embedded
		if style, err := c.Name(threeDa["Style"]); err == nil {
			activation.Style = style
		} else if pdf.IsReadError(err) {
			return nil, err
		}

		// Window (optional, PDF 2.0)
		if window, ok := threeDa["Window"].(pdf.Reference); ok {
			activation.Window = window
		}

		// Transparent (optional, PDF 2.0) - default false
		transparent, _, err := c.OptionalBoolean(threeDa["Transparent"])
		if err != nil {
			return nil, err
		}
		activation.Transparent = transparent

		annot3D.A = activation
	}

	// 3DI (optional) - default true
	annot3D.I = true // default value
	if threeDI, ok, err := c.OptionalBoolean(dict["3DI"]); err != nil {
		return nil, err
	} else if ok {
		annot3D.I = threeDI
	}

	// 3DB (optional)
	if threeDB, err := c.FloatArray(dict["3DB"]); err == nil && len(threeDB) == 4 {
		annot3D.B = &pdf.Rectangle{
			LLx: threeDB[0],
			LLy: threeDB[1],
			URx: threeDB[2],
			URy: threeDB[3],
		}
	} else if pdf.IsReadError(err) {
		return nil, err
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
