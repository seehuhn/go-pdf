package triggers

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action"
)

// Page represents a page object's additional-actions dictionary.
// This corresponds to the AA entry in a page dictionary.
//
// PDF 1.2, Table 198.
type Page struct {
	// PageOpen is an action performed when the page is opened (for example,
	// when the user navigates to it from the next or previous page or by
	// means of a link annotation or outline item).
	PageOpen action.Action

	// PageClose is an action performed when the page is closed (for example,
	// when the user navigates to the next or previous page or follows a link
	// annotation or an outline item).
	PageClose action.Action
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
