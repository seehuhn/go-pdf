package outline

import (
	"errors"
	"fmt"

	"seehuhn.de/go/pdf"
)

type Tree struct {
	Title    string
	Children []*Tree
	Count    pdf.Integer
}

func Read(r *pdf.Reader) (*Tree, error) {
	root := r.Catalog.Outlines
	if root == nil {
		return nil, nil
	}

	seen := map[*pdf.Reference]bool{}
	tree, _, err := dr1(r, seen, root)
	if err != nil {
		return nil, err
	}
	return tree, nil
}

func dr1(r *pdf.Reader, seen map[*pdf.Reference]bool, node *pdf.Reference) (*Tree, pdf.Dict, error) {
	if seen[node] {
		return nil, nil, fmt.Errorf("outline tree contains a loop")
	}
	seen[node] = true
	if len(seen) > 1000 {
		return nil, nil, errors.New("outline too large")
	}

	dict, err := r.GetDict(node)
	if err != nil {
		return nil, nil, err
	}

	tree := &Tree{}

	titleObj, err := r.Resolve(dict["Title"])
	if err != nil {
		return nil, nil, err
	}
	if title, ok := titleObj.(pdf.String); ok {
		tree.Title = title.AsTextString()
	} else if title != nil {
		return nil, nil, fmt.Errorf("invalid /Title in outline (type %T)", titleObj)
	}

	count, _ := dict["Count"].(pdf.Integer)
	tree.Count = count

	ccPtr, _ := dict["First"].(*pdf.Reference)
	cc, err := dr2(r, seen, ccPtr)
	if err != nil {
		return nil, nil, err
	}
	tree.Children = cc

	return tree, dict, nil
}

func dr2(r *pdf.Reader, seen map[*pdf.Reference]bool, node *pdf.Reference) ([]*Tree, error) {
	var res []*Tree
	for node != nil {
		tree, dict, err := dr1(r, seen, node)
		if err != nil {
			return nil, err
		}

		res = append(res, tree)

		node, _ = dict["Next"].(*pdf.Reference)
	}
	return res, nil
}
