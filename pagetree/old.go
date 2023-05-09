package pagetree

import (
	"fmt"

	"seehuhn.de/go/pdf"
)

type OldReader struct {
	r    *pdf.Reader
	root pdf.Dict
}

func NewOldReader(r *pdf.Reader) (*OldReader, error) {
	root, err := pdf.GetDict(r, r.Catalog.Pages)
	if err != nil {
		return nil, err
	}

	res := &OldReader{
		r:    r,
		root: root,
	}
	return res, nil
}

func (r *OldReader) NumPages() (pdf.Integer, error) {
	return pdf.GetInt(r.r, r.root["Count"])
}

func (r *OldReader) Get(pageNo pdf.Integer) (pdf.Dict, error) {
	var pos pdf.Integer
	dict := r.root
treeLoop:
	for dict["Type"] != pdf.Name("Page") {
		children, err := pdf.GetArray(r.r, dict["Kids"])
		if err != nil {
			return nil, err
		}
		for _, kid := range children {
			childDict, err := pdf.GetDict(r.r, kid)
			if err != nil {
				return nil, err
			}
			var count pdf.Integer
			switch childDict["Type"] {
			case pdf.Name("Pages"):
				count, err = pdf.GetInt(r.r, childDict["Count"])
				if err != nil {
					return nil, err
				}
			case pdf.Name("Page"):
				count = 1
			default:
				return nil, fmt.Errorf("unexpected page type %v", childDict["Type"])
			}

			if pageNo < pos+count {
				dict = childDict
				continue treeLoop
			}
			pos += count
		}
		return nil, fmt.Errorf("page %d not found", pageNo)
	}
	if pageNo != pos {
		return nil, fmt.Errorf("page %d not found", pageNo)
	}
	return dict, nil
}
