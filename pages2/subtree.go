package pages2

import "seehuhn.de/go/pdf"

const maxDegree = 16

type nodeInfo struct {
	dict pdf.Dict // a \Page or \Pages object
	ref  *pdf.Reference
}

type subtree struct {
	// The slice levels contains all the \Page and \Pages objects which still
	// need to be written to the tree.  levels[0] contains the leaf nodes
	// (subtrees of depth 0), levels[1] contains sub-trees of depth 1, etc.
	// Nodes with higher depth come before nodes with lower depth in the
	// document page order.
	levels [][]*nodeInfo
}

func (s *subtree) append(depth int, node *nodeInfo) {
	for len(s.levels) <= depth {
		s.levels = append(s.levels, []*nodeInfo{})
	}
	s.levels[depth] = append(s.levels[depth], node)
}

func (s *subtree) prepend(w *pdf.Writer, t *subtree) error {
	level := len(s.levels) - 1
	if level < 0 {
		s.levels = t.levels
		return nil
	}

	err := t.clearToLevel(w, level)
	if err != nil {
		return err
	}

	s.levels[level] = append(t.levels[level], s.levels[level]...)
	s.levels = append(s.levels, t.levels[level+1:]...)
	return s.tryCollapse(w)
}

func (s *subtree) clearToLevel(w *pdf.Writer, minDepth int) error {
	depth := 0
	for depth < len(s.levels) && depth < minDepth {
		for len(s.levels[depth]) > 0 {
			err := s.collapse(w, depth, maxDegree)
			if err != nil {
				return err
			}
		}
		depth++
	}
	return nil
}

// tryCollapse tries to collapse nodes into \Pages objects at all levels.
// After the call has returned, the tree is in a state where every level
// has at most maxDegree-1 nodes.
func (s *subtree) tryCollapse(w *pdf.Writer) error {
	depth := 0
	for depth < len(s.levels) {
		for len(s.levels[depth]) >= maxDegree {
			err := s.collapse(w, depth, maxDegree)
			if err != nil {
				return err
			}
		}
		depth++
	}
	return nil
}

// collapse collapses the first [num] nodes at level depth into a
// \Pages object at level depth+1.
func (s *subtree) collapse(w *pdf.Writer, depth, num int) error {
	if num > len(s.levels[depth]) {
		num = len(s.levels[depth])
	}

	if num == 1 {
		// Shortcut: avoid internal nodes with only one child.
		// Note that this makes some branches of the tree less deep than
		// they normally would be.
		node := s.levels[depth][0]
		s.append(depth+1, node)
		s.levels[depth] = append(s.levels[depth][:0], s.levels[depth][1:]...)
		return nil
	}

	childNodes := s.levels[depth][:num]

	childRefs := make([]*pdf.Reference, len(childNodes))
	childDicts := make([]pdf.Object, len(childNodes))
	parentRef := w.Alloc()
	var total pdf.Integer
	for i, node := range childNodes {
		childRef := node.ref
		if childRef == nil {
			childRef = w.Alloc()
		}
		childRefs[i] = childRef

		childDict := node.dict
		childDict["Parent"] = parentRef
		childDicts[i] = childDict

		if childDict["Type"] == pdf.Name("Pages") {
			total += childDict["Count"].(pdf.Integer)
		} else {
			total++
		}
	}
	_, err := w.WriteCompressed(childRefs, childDicts...)
	if err != nil {
		return err
	}

	kids := make(pdf.Array, len(childRefs))
	for i, ref := range childRefs {
		kids[i] = ref
	}
	parentNode := &nodeInfo{
		dict: pdf.Dict{
			"Type":  pdf.Name("Pages"),
			"Kids":  kids,
			"Count": total,
		},
		ref: parentRef,
	}
	s.append(depth+1, parentNode)

	s.levels[depth] = append(s.levels[depth][:0], s.levels[depth][num:]...)
	return nil
}
