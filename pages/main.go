package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"

	"seehuhn.de/go/pdf"
)

// Rect takes the coordinates of two diagonally opposite points
// and returns a PDF rectangle.
func Rect(llx, lly, urx, ury int) pdf.Array {
	return pdf.Array{pdf.Integer(llx), pdf.Integer(lly),
		pdf.Integer(urx), pdf.Integer(ury)}
}

const pageTreeWidth = 12

// PageTree represents a PDF page tree.
type PageTree struct {
	w       *pdf.Writer
	root    *pages
	current *pages
}

// NewPageTree allocates a new PageTree object.
func NewPageTree(w *pdf.Writer) *PageTree {
	root := &pages{
		id: w.Alloc(),
	}
	return &PageTree{
		w:       w,
		root:    root,
		current: root,
	}
}

// Flush flushes all internal /Pages notes to the file and returns
// the root of the page tree.  After .Flush() has been called, the
// page tree cannot be used any more.
func (tree *PageTree) Flush() (pdf.Dict, *pdf.Reference, error) {
	current := tree.current
	for current.parent != nil {
		obj := current.toObject()
		_, err := tree.w.WriteIndirect(obj, current.id)
		if err != nil {
			return nil, nil, err
		}
		current = current.parent
	}
	tree.current = nil
	tree.root = nil
	return current.toObject(), current.id, nil
}

// Ship adds a new page or subtree to the PageTree.
func (tree *PageTree) Ship(page pdf.Dict, ref *pdf.Reference) error {
	if page["Type"] != pdf.Name("Page") && page["Type"] != pdf.Name("Pages") {
		return errors.New("wrong pdf.Dict type, expected /Page or /Pages")
	}

	parent, err := tree.splitIfNeeded(tree.current)
	if err != nil {
		return err
	}
	tree.current = parent
	page["Parent"] = parent.id

	ref, err = tree.w.WriteIndirect(page, ref)
	if err != nil {
		return err
	}

	inc := 1
	if cummulative, ok := page["Count"].(pdf.Integer); ok {
		inc = int(cummulative)
	}
	parent.kids = append(parent.kids, ref)
	for parent != nil {
		parent.count += inc
		parent = parent.parent
	}

	return nil
}

func (tree *PageTree) splitIfNeeded(node *pages) (*pages, error) {
	if len(node.kids) < pageTreeWidth {
		return node, nil
	}

	// Node is full: write it to disk and get a new one.

	// First check that there is a parent.
	parent := node.parent
	if parent == nil {
		// tree is full: add another level at the root
		parent = &pages{
			id:    tree.w.Alloc(),
			kids:  []*pdf.Reference{node.id},
			count: node.count,
		}
		node.parent = parent
		tree.root = parent
	}

	// Turn the node into a PDF object and write this to the file.
	nodeObj := node.toObject()
	_, err := tree.w.WriteIndirect(nodeObj, node.id)
	if err != nil {
		return nil, err
	}

	parent, err = tree.splitIfNeeded(parent)
	if err != nil {
		return nil, err
	}
	node = &pages{
		id:     tree.w.Alloc(),
		parent: parent,
	}
	parent.kids = append(parent.kids, node.id)
	return node, nil
}

type pages struct {
	id     *pdf.Reference
	parent *pages
	kids   []*pdf.Reference
	count  int
}

func (pp *pages) toObject() pdf.Dict {
	var kids pdf.Array
	for _, ref := range pp.kids {
		kids = append(kids, ref)
	}
	nodeDict := pdf.Dict{ // page 76
		"Type":  pdf.Name("Pages"),
		"Kids":  kids,
		"Count": pdf.Integer(pp.count),
	}
	if pp.parent != nil {
		nodeDict["Parent"] = pp.parent.id
	}
	return nodeDict
}

func main() {
	fd, err := os.Create("test.pdf")
	if err != nil {
		log.Fatal(err)
	}
	defer fd.Close()

	w, err := pdf.NewWriter(fd, pdf.V1_5)
	if err != nil {
		log.Fatal(err)
	}
	var catalog, info *pdf.Reference

	info, err = w.WriteIndirect(pdf.Dict{ // page 550
		"Title":  pdf.TextString("PDF Test Document"),
		"Author": pdf.TextString("Jochen Voß"),
	}, nil)
	if err != nil {
		log.Fatal(err)
	}

	pageTree := NewPageTree(w)

	font, err := w.WriteIndirect(pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("Type1"),
		"BaseFont": pdf.Name("Helvetica"),
		"Encoding": pdf.Name("MacRomanEncoding"),
	}, nil)
	if err != nil {
		log.Fatal(err)
	}
	resources := pdf.Dict{
		"Font": pdf.Dict{"F1": font},
	}

	for i := 1; i <= 10; i++ {
		buf := bytes.NewReader([]byte(fmt.Sprintf(`BT
/F1 12 Tf
30 30 Td
(page %d) Tj
ET`, i)))
		contentNode, err := w.WriteIndirect(&pdf.Stream{
			Dict: pdf.Dict{
				"Length": pdf.Integer(buf.Size()),
			},
			R: buf,
		}, nil)
		if err != nil {
			log.Fatal(err)
		}
		page := pdf.Dict{ // page 77
			"Type":     pdf.Name("Page"),
			"Contents": contentNode,
		}
		err = pageTree.Ship(page, nil)
		if err != nil {
			log.Fatal(err)
		}
	}

	pages, pagesRef, err := pageTree.Flush()
	if err != nil {
		log.Fatal(err)
	}
	pages["CropBox"] = Rect(0, 0, 200, 100)
	pages["Resources"] = resources
	_, err = w.WriteIndirect(pages, pagesRef)
	if err != nil {
		log.Fatal(err)
	}

	// page 73
	catalog, err = w.WriteIndirect(pdf.Dict{
		"Type":  pdf.Name("Catalog"),
		"Pages": pagesRef,
	}, nil)
	if err != nil {
		log.Fatal(err)
	}

	err = w.Close(catalog, info)
	if err != nil {
		log.Fatal(err)
	}
}
