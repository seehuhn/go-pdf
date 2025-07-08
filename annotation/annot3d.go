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

package annotation

import (
	"fmt"

	"seehuhn.de/go/pdf"
)

// Annot3D represents a 3D annotation (PDF 1.6+).
// 3D annotations are a way to include 3D artwork in PDF documents.
type Annot3D struct {
	Common

	// D (required) is a 3D stream or reference dictionary.
	D pdf.Object

	// V (optional) is the default initial view specification.
	V pdf.Object

	// A (optional) is an activation dictionary.
	A *ThreeDActivation

	// I (optional) is the interactive flag.
	I bool

	// B (optional) is a 3D view box rectangle.
	B *pdf.Rectangle

	// U (optional, PDF 2.0) is a units dictionary.
	U pdf.Reference

	// GEO (optional, PDF 2.0) is geospatial information.
	GEO pdf.Reference
}

var _ pdf.Annotation = (*Annot3D)(nil)

// ThreeDActivation represents a 3D activation dictionary.
type ThreeDActivation struct {
	// A (optional) - activation circumstances
	A pdf.Name

	// AIS (optional) - artwork instance state upon activation
	AIS pdf.Name

	// D (optional) - deactivation circumstances
	D pdf.Name

	// DIS (optional) - artwork instance state upon deactivation
	DIS pdf.Name

	// TB (optional, PDF 1.7) - toolbar display flag
	TB bool

	// NP (optional, PDF 1.7) - navigation panel display flag
	NP bool

	// Style (optional, PDF 2.0) - display style
	Style pdf.Name

	// Window (optional, PDF 2.0) - window dictionary
	Window pdf.Reference

	// Transparent (optional, PDF 2.0) - transparency flag
	Transparent bool
}

// AnnotationType returns the annotation type.
func (a *Annot3D) AnnotationType() pdf.Name {
	return "3D"
}

// Embed writes the annotation to a PDF file.
func (a *Annot3D) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	if err := pdf.CheckVersion(rm.Out, "3D annotation", pdf.V1_6); err != nil {
		return nil, zero, err
	}

	dict := pdf.Dict{
		"Type":    pdf.Name("Annot"),
		"Subtype": pdf.Name("3D"),
	}

	// Add common annotation fields
	if err := a.Common.fillDict(rm, dict); err != nil {
		return nil, zero, err
	}

	// 3DD (required)
	if a.D == nil {
		return nil, zero, fmt.Errorf("3DD field is required")
	}
	dict["3DD"] = a.D

	// 3DV (optional)
	if a.V != nil {
		dict["3DV"] = a.V
	}

	// 3DA (optional)
	if a.A != nil {
		activationDict := pdf.Dict{}

		// A (optional) - default XA
		if a.A.A != "" {
			activationDict["A"] = a.A.A
		}

		// AIS (optional) - default L
		if a.A.AIS != "" {
			activationDict["AIS"] = a.A.AIS
		}

		// D (optional) - default PI
		if a.A.D != "" {
			activationDict["D"] = a.A.D
		}

		// DIS (optional) - default U
		if a.A.DIS != "" {
			activationDict["DIS"] = a.A.DIS
		}

		// TB (optional, PDF 1.7) - default true
		if !a.A.TB {
			if err := pdf.CheckVersion(rm.Out, "3D annotation TB entry", pdf.V1_7); err != nil {
				return nil, zero, err
			}
			activationDict["TB"] = pdf.Boolean(false)
		}

		// NP (optional, PDF 1.7) - default false
		if a.A.NP {
			if err := pdf.CheckVersion(rm.Out, "3D annotation NP entry", pdf.V1_7); err != nil {
				return nil, zero, err
			}
			activationDict["NP"] = pdf.Boolean(true)
		}

		// Style (optional, PDF 2.0) - default Embedded
		if a.A.Style != "" && a.A.Style != "Embedded" {
			if err := pdf.CheckVersion(rm.Out, "3D annotation Style entry", pdf.V2_0); err != nil {
				return nil, zero, err
			}
			activationDict["Style"] = a.A.Style
		}

		// Window (optional, PDF 2.0)
		if a.A.Window != 0 {
			if err := pdf.CheckVersion(rm.Out, "3D annotation Window entry", pdf.V2_0); err != nil {
				return nil, zero, err
			}
			activationDict["Window"] = a.A.Window
		}

		// Transparent (optional, PDF 2.0) - default false
		if a.A.Transparent {
			if err := pdf.CheckVersion(rm.Out, "3D annotation Transparent entry", pdf.V2_0); err != nil {
				return nil, zero, err
			}
			activationDict["Transparent"] = pdf.Boolean(true)
		}

		if len(activationDict) > 0 {
			dict["3DA"] = activationDict
		}
	}

	// 3DI (optional) - default true
	if !a.I {
		dict["3DI"] = pdf.Boolean(false)
	}

	// 3DB (optional)
	if a.B != nil {
		dict["3DB"] = pdf.Array{
			pdf.Real(a.B.LLx),
			pdf.Real(a.B.LLy),
			pdf.Real(a.B.URx),
			pdf.Real(a.B.URy),
		}
	}

	// 3DU (optional, PDF 2.0)
	if a.U != 0 {
		if err := pdf.CheckVersion(rm.Out, "3D annotation 3DU entry", pdf.V2_0); err != nil {
			return nil, zero, err
		}
		dict["3DU"] = a.U
	}

	// GEO (optional, PDF 2.0)
	if a.GEO != 0 {
		if err := pdf.CheckVersion(rm.Out, "3D annotation GEO entry", pdf.V2_0); err != nil {
			return nil, zero, err
		}
		dict["GEO"] = a.GEO
	}

	return dict, zero, nil
}

func extractAnnot3D(r pdf.Getter, dict pdf.Dict) (*Annot3D, error) {
	annot3D := &Annot3D{}

	// Extract common annotation fields
	if err := extractCommon(r, dict, &annot3D.Common); err != nil {
		return nil, err
	}

	// 3DD (required)
	if threeDD := dict["3DD"]; threeDD != nil {
		annot3D.D = threeDD
	} else {
		return nil, fmt.Errorf("3DD field is required")
	}

	// 3DV (optional)
	if threeDV := dict["3DV"]; threeDV != nil {
		annot3D.V = threeDV
	}

	// 3DA (optional)
	if dict["3DA"] != nil {
		if threeDa, err := pdf.GetDict(r, dict["3DA"]); err == nil {
			activation := &ThreeDActivation{
				TB: true, // default value
			}

			// A (optional) - default XA
			if a, err := pdf.GetName(r, threeDa["A"]); err == nil {
				activation.A = a
			}

			// AIS (optional) - default L
			if ais, err := pdf.GetName(r, threeDa["AIS"]); err == nil {
				activation.AIS = ais
			}

			// D (optional) - default PI
			if d, err := pdf.GetName(r, threeDa["D"]); err == nil {
				activation.D = d
			}

			// DIS (optional) - default U
			if dis, err := pdf.GetName(r, threeDa["DIS"]); err == nil {
				activation.DIS = dis
			}

			// TB (optional, PDF 1.7) - default true
			if tb, err := pdf.GetBoolean(r, threeDa["TB"]); err == nil {
				activation.TB = bool(tb)
			}

			// NP (optional, PDF 1.7) - default false
			if np, err := pdf.GetBoolean(r, threeDa["NP"]); err == nil {
				activation.NP = bool(np)
			}

			// Style (optional, PDF 2.0) - default Embedded
			if style, err := pdf.GetName(r, threeDa["Style"]); err == nil {
				activation.Style = style
			}

			// Window (optional, PDF 2.0)
			if window, ok := threeDa["Window"].(pdf.Reference); ok {
				activation.Window = window
			}

			// Transparent (optional, PDF 2.0) - default false
			if transparent, err := pdf.GetBoolean(r, threeDa["Transparent"]); err == nil {
				activation.Transparent = bool(transparent)
			}

			annot3D.A = activation
		}
	}

	// 3DI (optional) - default true
	annot3D.I = true // default value
	if dict["3DI"] != nil {
		if threeDI, err := pdf.GetBoolean(r, dict["3DI"]); err == nil {
			annot3D.I = bool(threeDI)
		}
	}

	// 3DB (optional)
	if threeDB, err := pdf.GetArray(r, dict["3DB"]); err == nil && len(threeDB) == 4 {
		llx, err1 := pdf.GetReal(r, threeDB[0])
		lly, err2 := pdf.GetReal(r, threeDB[1])
		urx, err3 := pdf.GetReal(r, threeDB[2])
		ury, err4 := pdf.GetReal(r, threeDB[3])
		if err1 == nil && err2 == nil && err3 == nil && err4 == nil {
			annot3D.B = &pdf.Rectangle{
				LLx: float64(llx),
				LLy: float64(lly),
				URx: float64(urx),
				URy: float64(ury),
			}
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
