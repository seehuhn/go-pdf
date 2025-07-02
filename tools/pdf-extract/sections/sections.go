package sections

import (
	"errors"
	"fmt"
	"math"
	"regexp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/outline"
	"seehuhn.de/go/pdf/pagetree"
)

// PageRange represents the page range and Y coordinates for a section
type PageRange struct {
	FirstPage int     // first page number (0-based)
	LastPage  int     // last page number (0-based)
	YMax      float64 // Y coordinate at top of section on first page
	YMin      float64 // Y coordinate at bottom of section on last page
}

// sectionMatch holds information about a matched outline entry
type sectionMatch struct {
	tree     *outline.Tree
	pageNo   int
	yCoord   float64
	hasCoord bool
}

// Pages finds a section matching the given pattern and returns its page range.
// The pattern must match exactly one outline entry, otherwise an error is returned.
func Pages(r pdf.Getter, pattern string) (*PageRange, error) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	tree, err := outline.Read(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read outline: %w", err)
	}
	if tree == nil {
		return nil, errors.New("document has no outline")
	}

	// build page number mapping
	pageNumbers := make(map[pdf.Reference]int)
	pageNo := 0
	for ref := range pagetree.NewIterator(r).All() {
		pageNumbers[ref] = pageNo
		pageNo++
	}

	// find all matching sections
	var matches []sectionMatch
	err = findMatches(r, pageNumbers, tree, regex, &matches)
	if err != nil {
		return nil, err
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no outline entries match pattern %q", pattern)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("pattern %q matches %d outline entries, expected exactly 1", pattern, len(matches))
	}

	match := matches[0]

	// find the end of this section by looking for the next section at same or higher level
	endPageNo, endY, err := findSectionEnd(r, pageNumbers, tree, match)
	if err != nil {
		return nil, err
	}

	result := &PageRange{
		FirstPage: match.pageNo,
		LastPage:  endPageNo,
		YMin:      math.Inf(-1), // default to unbounded
		YMax:      math.Inf(+1), // default to unbounded
	}

	if match.hasCoord {
		result.YMax = match.yCoord
	}

	if !math.IsInf(endY, -1) {
		result.YMin = endY
	}

	return result, nil
}

// findMatches recursively searches the outline tree for entries matching the regex
func findMatches(r pdf.Getter, pageNumbers map[pdf.Reference]int, node *outline.Tree, regex *regexp.Regexp, matches *[]sectionMatch) error {
	if node == nil {
		return nil
	}

	// check if current node matches
	if regex.MatchString(node.Title) {
		pageNo, yCoord, hasCoord, err := extractDestination(r, pageNumbers, node)
		if err != nil {
			return err
		}

		*matches = append(*matches, sectionMatch{
			tree:     node,
			pageNo:   pageNo,
			yCoord:   yCoord,
			hasCoord: hasCoord,
		})
	}

	// recursively check children
	for _, child := range node.Children {
		err := findMatches(r, pageNumbers, child, regex, matches)
		if err != nil {
			return err
		}
	}

	return nil
}

// extractDestination extracts page number and Y coordinate from an outline entry's destination
func extractDestination(r pdf.Getter, pageNumbers map[pdf.Reference]int, node *outline.Tree) (pageNo int, yCoord float64, hasCoord bool, err error) {
	if node.Action == nil {
		return -1, 0, false, errors.New("outline entry has no action")
	}

	actionType, err := pdf.GetName(r, node.Action["S"])
	if err != nil {
		return -1, 0, false, fmt.Errorf("failed to get action type: %w", err)
	}

	if actionType != "GoTo" {
		return -1, 0, false, fmt.Errorf("unsupported action type: %s", actionType)
	}

	dest, err := pdf.Resolve(r, node.Action["D"])
	if err != nil {
		return -1, 0, false, fmt.Errorf("failed to resolve destination: %w", err)
	}

	destArray, ok := dest.(pdf.Array)
	if !ok || len(destArray) == 0 {
		return -1, 0, false, errors.New("invalid destination format")
	}

	// extract page reference
	pageRef, ok := destArray[0].(pdf.Reference)
	if !ok {
		return -1, 0, false, errors.New("destination does not contain page reference")
	}

	pageNo, ok = pageNumbers[pageRef]
	if !ok {
		return -1, 0, false, errors.New("page reference not found in document")
	}

	// extract Y coordinate if this is an XYZ destination
	if len(destArray) >= 4 {
		fitType, ok := destArray[1].(pdf.Name)
		if ok && fitType == "XYZ" {
			if yVal := destArray[3]; yVal != nil {
				switch y := yVal.(type) {
				case pdf.Number:
					return pageNo, float64(y), true, nil
				case pdf.Integer:
					return pageNo, float64(y), true, nil
				}
			}
		}
	}

	return pageNo, 0, false, nil
}

// findSectionEnd determines where the matched section ends by finding the next section
func findSectionEnd(r pdf.Getter, pageNumbers map[pdf.Reference]int, root *outline.Tree, match sectionMatch) (endPage int, endY float64, err error) {
	// find the parent and level of the matched section
	parent, level := findParentAndLevel(root, match.tree, nil, 0)
	if parent == nil {
		// this is a top-level section, look for next top-level section
		parent = root
	}

	// find the next sibling at the same or higher level
	nextSection := findNextSectionAtLevel(root, match.tree, level)
	if nextSection == nil {
		// this is the last section, it goes to the end of the document
		totalPages := len(pageNumbers)
		return totalPages - 1, math.Inf(-1), nil
	}

	// get the page and coordinates of the next section
	nextPageNo, nextY, hasNextCoord, err := extractDestination(r, pageNumbers, nextSection)
	if err != nil {
		return -1, 0, err
	}

	if nextPageNo == match.pageNo {
		// next section is on the same page, use its Y coordinate
		if hasNextCoord {
			return match.pageNo, nextY, nil
		}
		return match.pageNo, math.Inf(-1), nil
	}

	// next section is on a different page
	if nextPageNo > match.pageNo {
		// our section ends where the next section starts
		endPage = nextPageNo
		if hasNextCoord {
			endY = nextY
		} else {
			endY = math.Inf(-1) // no coordinate info, unbounded
		}
	} else {
		// this shouldn't happen in a well-formed outline, but handle it
		endPage = nextPageNo
		if hasNextCoord {
			endY = nextY
		}
	}

	return endPage, endY, nil
}

// findParentAndLevel finds the parent node and level of the target node
func findParentAndLevel(node, target, parent *outline.Tree, level int) (*outline.Tree, int) {
	if node == target {
		return parent, level
	}

	for _, child := range node.Children {
		if p, l := findParentAndLevel(child, target, node, level+1); p != nil {
			return p, l
		}
	}

	return nil, -1
}

// findNextSectionAtLevel finds the next section at the same or higher level than the target
func findNextSectionAtLevel(root, target *outline.Tree, targetLevel int) *outline.Tree {
	found := false
	return findNextAtLevel(root, target, targetLevel, 0, &found)
}

// findNextAtLevel recursively searches for the next section at the specified level
func findNextAtLevel(node, target *outline.Tree, targetLevel, currentLevel int, found *bool) *outline.Tree {
	if *found && currentLevel <= targetLevel {
		return node
	}

	if node == target {
		*found = true
	}

	for _, child := range node.Children {
		if result := findNextAtLevel(child, target, targetLevel, currentLevel+1, found); result != nil {
			return result
		}
	}

	return nil
}
