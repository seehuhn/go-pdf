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

package triggers

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action"
)

// PDF 2.0 sections: 12.6.3

// Annotation represents an annotation's additional-actions dictionary.
// This corresponds to the AA entry in an annotation dictionary.
//
// PDF 1.2, Table 197.
type Annotation struct {
	// Enter is an action performed when the cursor enters the annotation's
	// active area.
	Enter action.Action

	// Exit is an action performed when the cursor exits the annotation's
	// active area.
	Exit action.Action

	// Down is an action performed when the mouse button is pressed inside
	// the annotation's active area.
	Down action.Action

	// Up is an action performed when the mouse button is released inside
	// the annotation's active area.
	Up action.Action

	// Focus is an action performed when the annotation receives the input
	// focus (widget annotations only).
	Focus action.Action

	// Blur is an action performed when the annotation loses the input focus
	// (widget annotations only).
	Blur action.Action

	// PageOpen is an action performed when the page containing the annotation
	// is opened (PDF 1.5).
	PageOpen action.Action

	// PageClose is an action performed when the page containing the annotation
	// is closed (PDF 1.5).
	PageClose action.Action

	// PageVisible is an action performed when the page containing the
	// annotation becomes visible (PDF 1.5).
	PageVisible action.Action

	// PageInvisible is an action performed when the page containing the
	// annotation is no longer visible (PDF 1.5).
	PageInvisible action.Action
}

var _ pdf.Encoder = (*Annotation)(nil)

// Encode converts the Annotation to a PDF dictionary.
func (a *Annotation) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	dict := pdf.Dict{}

	if a.Enter != nil {
		if err := pdf.CheckVersion(rm.Out, "annotation AA E entry", pdf.V1_2); err != nil {
			return nil, err
		}
		obj, err := a.Enter.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["E"] = obj
	}

	if a.Exit != nil {
		if err := pdf.CheckVersion(rm.Out, "annotation AA X entry", pdf.V1_2); err != nil {
			return nil, err
		}
		obj, err := a.Exit.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["X"] = obj
	}

	if a.Down != nil {
		if err := pdf.CheckVersion(rm.Out, "annotation AA D entry", pdf.V1_2); err != nil {
			return nil, err
		}
		obj, err := a.Down.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["D"] = obj
	}

	if a.Up != nil {
		if err := pdf.CheckVersion(rm.Out, "annotation AA U entry", pdf.V1_2); err != nil {
			return nil, err
		}
		obj, err := a.Up.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["U"] = obj
	}

	if a.Focus != nil {
		if err := pdf.CheckVersion(rm.Out, "annotation AA Fo entry", pdf.V1_2); err != nil {
			return nil, err
		}
		obj, err := a.Focus.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["Fo"] = obj
	}

	if a.Blur != nil {
		if err := pdf.CheckVersion(rm.Out, "annotation AA Bl entry", pdf.V1_2); err != nil {
			return nil, err
		}
		obj, err := a.Blur.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["Bl"] = obj
	}

	if a.PageOpen != nil {
		if err := pdf.CheckVersion(rm.Out, "annotation AA PO entry", pdf.V1_5); err != nil {
			return nil, err
		}
		obj, err := a.PageOpen.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["PO"] = obj
	}

	if a.PageClose != nil {
		if err := pdf.CheckVersion(rm.Out, "annotation AA PC entry", pdf.V1_5); err != nil {
			return nil, err
		}
		obj, err := a.PageClose.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["PC"] = obj
	}

	if a.PageVisible != nil {
		if err := pdf.CheckVersion(rm.Out, "annotation AA PV entry", pdf.V1_5); err != nil {
			return nil, err
		}
		obj, err := a.PageVisible.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["PV"] = obj
	}

	if a.PageInvisible != nil {
		if err := pdf.CheckVersion(rm.Out, "annotation AA PI entry", pdf.V1_5); err != nil {
			return nil, err
		}
		obj, err := a.PageInvisible.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["PI"] = obj
	}

	return dict, nil
}

// DecodeAnnotation reads an annotation's additional-actions dictionary from
// a PDF object.
func DecodeAnnotation(x *pdf.Extractor, obj pdf.Object) (*Annotation, error) {
	dict, err := x.GetDict(obj)
	if err != nil {
		return nil, err
	}
	if dict == nil {
		return nil, nil
	}

	a := &Annotation{}

	if act, err := pdf.ExtractorGetOptional(x, dict["E"], action.Decode); err != nil {
		return nil, err
	} else {
		a.Enter = act
	}

	if act, err := pdf.ExtractorGetOptional(x, dict["X"], action.Decode); err != nil {
		return nil, err
	} else {
		a.Exit = act
	}

	if act, err := pdf.ExtractorGetOptional(x, dict["D"], action.Decode); err != nil {
		return nil, err
	} else {
		a.Down = act
	}

	if act, err := pdf.ExtractorGetOptional(x, dict["U"], action.Decode); err != nil {
		return nil, err
	} else {
		a.Up = act
	}

	if act, err := pdf.ExtractorGetOptional(x, dict["Fo"], action.Decode); err != nil {
		return nil, err
	} else {
		a.Focus = act
	}

	if act, err := pdf.ExtractorGetOptional(x, dict["Bl"], action.Decode); err != nil {
		return nil, err
	} else {
		a.Blur = act
	}

	if act, err := pdf.ExtractorGetOptional(x, dict["PO"], action.Decode); err != nil {
		return nil, err
	} else {
		a.PageOpen = act
	}

	if act, err := pdf.ExtractorGetOptional(x, dict["PC"], action.Decode); err != nil {
		return nil, err
	} else {
		a.PageClose = act
	}

	if act, err := pdf.ExtractorGetOptional(x, dict["PV"], action.Decode); err != nil {
		return nil, err
	} else {
		a.PageVisible = act
	}

	if act, err := pdf.ExtractorGetOptional(x, dict["PI"], action.Decode); err != nil {
		return nil, err
	} else {
		a.PageInvisible = act
	}

	return a, nil
}
