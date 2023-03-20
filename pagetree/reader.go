package pagetree

import (
	"fmt"

	"seehuhn.de/go/pdf"
)

type Reader struct {
	r    *pdf.Reader
	root pdf.Dict
}

func NewReader(r *pdf.Reader) (*Reader, error) {
	root, err := r.GetDict(r.Catalog.Pages)
	if err != nil {
		return nil, err
	}

	res := &Reader{
		r:    r,
		root: root,
	}
	return res, nil
}

func (r *Reader) NumPages() (pdf.Integer, error) {
	return r.r.GetInt(r.root["Count"])
}

func (r *Reader) Get(pageNo pdf.Integer) (pdf.Dict, error) {
	var pos pdf.Integer
	dict := r.root
treeLoop:
	for dict["Type"] != pdf.Name("Page") {
		children, err := r.r.GetArray(dict["Kids"])
		if err != nil {
			return nil, err
		}
		for _, kid := range children {
			childDict, err := r.r.GetDict(kid)
			if err != nil {
				return nil, err
			}
			var count pdf.Integer
			switch childDict["Type"] {
			case pdf.Name("Pages"):
				count, err = r.r.GetInt(childDict["Count"])
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
