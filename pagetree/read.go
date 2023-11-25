package pagetree

import "seehuhn.de/go/pdf"

func FindPages(r pdf.Getter) ([]pdf.Reference, error) {
	meta := r.GetMeta()
	catalog := meta.Catalog
	if catalog.Pages == 0 {
		return nil, errInvalidPageTree
	}

	var res []pdf.Reference
	todo := []pdf.Reference{catalog.Pages}
	seen := map[pdf.Reference]bool{
		catalog.Pages: true,
	}
	for len(todo) > 0 {
		k := len(todo) - 1
		ref := todo[k]
		todo = todo[:k]

		node, err := pdf.GetDict(r, ref)
		if err != nil {
			return nil, err
		}
		tp, err := pdf.GetName(r, node["Type"])
		if err != nil {
			return nil, err
		}
		switch tp {
		case "Page":
			res = append(res, ref)
		case "Pages":
			kids, err := pdf.GetArray(r, node["Kids"])
			if err != nil {
				return nil, err
			}
			for i := len(kids) - 1; i >= 0; i-- {
				kid := kids[i]
				if kidRef, ok := kid.(pdf.Reference); ok && !seen[kidRef] {
					todo = append(todo, kidRef)
					seen[kidRef] = true
				} else {
					res = append(res, 0)
				}
			}
		}
	}

	return res, nil
}
