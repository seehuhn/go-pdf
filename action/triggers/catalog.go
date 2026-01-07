package triggers

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action"
)

// Catalog represents a document catalog's additional-actions dictionary.
// This corresponds to the AA entry in the document catalog dictionary.
//
// PDF 1.4, Table 200.
type Catalog struct {
	// WillClose is an ECMAScript action performed before closing a document.
	WillClose *action.JavaScript

	// WillSave is an ECMAScript action performed before saving a document.
	WillSave *action.JavaScript

	// DidSave is an ECMAScript action performed after saving a document.
	DidSave *action.JavaScript

	// WillPrint is an ECMAScript action performed before printing a document.
	WillPrint *action.JavaScript

	// DidPrint is an ECMAScript action performed after printing a document.
	DidPrint *action.JavaScript
}

var _ pdf.Encoder = (*Catalog)(nil)

// Encode converts the Catalog to a PDF dictionary.
func (c *Catalog) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	dict := pdf.Dict{}

	if c.WillClose != nil {
		if err := pdf.CheckVersion(rm.Out, "catalog AA WC entry", pdf.V1_4); err != nil {
			return nil, err
		}
		obj, err := c.WillClose.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["WC"] = obj
	}

	if c.WillSave != nil {
		if err := pdf.CheckVersion(rm.Out, "catalog AA WS entry", pdf.V1_4); err != nil {
			return nil, err
		}
		obj, err := c.WillSave.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["WS"] = obj
	}

	if c.DidSave != nil {
		if err := pdf.CheckVersion(rm.Out, "catalog AA DS entry", pdf.V1_4); err != nil {
			return nil, err
		}
		obj, err := c.DidSave.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["DS"] = obj
	}

	if c.WillPrint != nil {
		if err := pdf.CheckVersion(rm.Out, "catalog AA WP entry", pdf.V1_4); err != nil {
			return nil, err
		}
		obj, err := c.WillPrint.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["WP"] = obj
	}

	if c.DidPrint != nil {
		if err := pdf.CheckVersion(rm.Out, "catalog AA DP entry", pdf.V1_4); err != nil {
			return nil, err
		}
		obj, err := c.DidPrint.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["DP"] = obj
	}

	return dict, nil
}

// DecodeCatalog reads a document catalog's additional-actions dictionary from
// a PDF object.
func DecodeCatalog(x *pdf.Extractor, obj pdf.Object) (*Catalog, error) {
	dict, err := x.GetDict(obj)
	if err != nil {
		return nil, err
	}
	if dict == nil {
		return nil, nil
	}

	c := &Catalog{}

	if act, err := pdf.ExtractorGetOptional(x, dict["WC"], action.Decode); err != nil {
		return nil, err
	} else if js, ok := act.(*action.JavaScript); ok {
		c.WillClose = js
	}

	if act, err := pdf.ExtractorGetOptional(x, dict["WS"], action.Decode); err != nil {
		return nil, err
	} else if js, ok := act.(*action.JavaScript); ok {
		c.WillSave = js
	}

	if act, err := pdf.ExtractorGetOptional(x, dict["DS"], action.Decode); err != nil {
		return nil, err
	} else if js, ok := act.(*action.JavaScript); ok {
		c.DidSave = js
	}

	if act, err := pdf.ExtractorGetOptional(x, dict["WP"], action.Decode); err != nil {
		return nil, err
	} else if js, ok := act.(*action.JavaScript); ok {
		c.WillPrint = js
	}

	if act, err := pdf.ExtractorGetOptional(x, dict["DP"], action.Decode); err != nil {
		return nil, err
	} else if js, ok := act.(*action.JavaScript); ok {
		c.DidPrint = js
	}

	return c, nil
}
