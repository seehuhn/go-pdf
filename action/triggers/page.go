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

// Page represents a page object's additional-actions dictionary.
// This corresponds to the AA entry in a page dictionary.
//
// PDF 1.2, Table 198.
type Page struct {
	// PageOpen is an action performed when the page is opened (for example,
	// when the user navigates to it from the next or previous page or by
	// means of a link annotation or outline item).
	PageOpen pdf.Action

	// PageClose is an action performed when the page is closed (for example,
	// when the user navigates to the next or previous page or follows a link
	// annotation or an outline item).
	PageClose pdf.Action
}

var _ pdf.Encoder = (*Page)(nil)

// Encode converts the Page to a PDF dictionary.
func (p *Page) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	dict := pdf.Dict{}

	if p.PageOpen != nil {
		if err := pdf.CheckVersion(rm.Out, "page AA O entry", pdf.V1_2); err != nil {
			return nil, err
		}
		obj, err := p.PageOpen.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["O"] = obj
	}

	if p.PageClose != nil {
		if err := pdf.CheckVersion(rm.Out, "page AA C entry", pdf.V1_2); err != nil {
			return nil, err
		}
		obj, err := p.PageClose.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["C"] = obj
	}

	return dict, nil
}

// DecodePage reads a page object's additional-actions dictionary from
// a PDF object.
func DecodePage(x *pdf.Extractor, obj pdf.Object) (*Page, error) {
	dict, err := x.GetDict(obj)
	if err != nil {
		return nil, err
	}
	if dict == nil {
		return nil, nil
	}

	p := &Page{}

	if act, err := pdf.ExtractorGetOptional(x, dict["O"], action.Decode); err != nil {
		return nil, err
	} else {
		p.PageOpen = act
	}

	if act, err := pdf.ExtractorGetOptional(x, dict["C"], action.Decode); err != nil {
		return nil, err
	} else {
		p.PageClose = act
	}

	return p, nil
}
