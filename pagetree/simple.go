package pagetree

import (
	"errors"

	"seehuhn.de/go/pdf"
)

func GetPage(r pdf.Getter, pageNo int) (pdf.Dict, error) {
	if pageNo < 0 {
		return nil, errors.New("invalid page number")
	}

	catalog := r.GetMeta().Catalog
	skip := pdf.Integer(pageNo)
	pageTreeNode, err := pdf.GetDict(r, catalog.Pages)
	if err != nil {
		return nil, err
	}

	// TODO(voss): track inherited attributes

levelLoop:
	for {
		tp, err := pdf.GetName(r, pageTreeNode["Type"])
		if err != nil {
			return nil, err
		}
		if tp != "Pages" {
			return pageTreeNode, nil
		}

		kids, err := pdf.GetArray(r, pageTreeNode["Kids"])
		if err != nil {
			return nil, err
		}

		for _, ref := range kids {
			kid, err := pdf.GetDict(r, ref)
			if err != nil {
				return nil, err
			}

			count, err := pdf.GetInt(r, pageTreeNode["Count"])
			if err != nil {
				return nil, err
			}

			if count < 0 {
				return nil, errors.New("page tree corrupted")
			} else if skip < count {
				pageTreeNode = kid
				continue levelLoop
			} else {
				skip -= count
			}
		}
		return nil, errors.New("page not found")
	}
}
