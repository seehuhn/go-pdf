// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package sections

import (
	"errors"
	"fmt"
	"math"
	"regexp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/destination"
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
	item     *outline.Item
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
	for _, item := range tree.Items {
		err = findMatches(r, pageNumbers, item, regex, &matches)
		if err != nil {
			return nil, err
		}
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

// findMatches recursively searches the outline items for entries matching the regex
func findMatches(r pdf.Getter, pageNumbers map[pdf.Reference]int, item *outline.Item, regex *regexp.Regexp, matches *[]sectionMatch) error {
	if item == nil {
		return nil
	}

	// check if current item matches
	if regex.MatchString(item.Title) {
		pageNo, yCoord, hasCoord, err := extractDestination(pageNumbers, item)
		if err != nil {
			return err
		}

		*matches = append(*matches, sectionMatch{
			item:     item,
			pageNo:   pageNo,
			yCoord:   yCoord,
			hasCoord: hasCoord,
		})
	}

	// recursively check children
	for _, child := range item.Children {
		err := findMatches(r, pageNumbers, child, regex, matches)
		if err != nil {
			return err
		}
	}

	return nil
}

// extractDestination extracts page number and Y coordinate from an outline entry's destination
func extractDestination(pageNumbers map[pdf.Reference]int, item *outline.Item) (pageNo int, yCoord float64, hasCoord bool, err error) {
	// get destination from either Destination field or GoTo action
	var dest destination.Destination
	if item.Destination != nil {
		dest = item.Destination
	} else if goTo, ok := item.Action.(*action.GoTo); ok {
		dest = goTo.Dest
	} else {
		return -1, 0, false, errors.New("outline entry has no destination")
	}

	if dest == nil {
		return -1, 0, false, errors.New("outline entry has nil destination")
	}

	// extract page reference from the destination
	var pageRef pdf.Reference
	switch d := dest.(type) {
	case *destination.XYZ:
		ref, ok := d.Page.(pdf.Reference)
		if !ok {
			return -1, 0, false, errors.New("destination page is not a reference")
		}
		pageRef = ref
		pageNo, ok = pageNumbers[pageRef]
		if !ok {
			return -1, 0, false, errors.New("page reference not found in document")
		}
		if !math.IsNaN(d.Top) {
			return pageNo, d.Top, true, nil
		}
		return pageNo, 0, false, nil

	case *destination.Fit:
		ref, ok := d.Page.(pdf.Reference)
		if !ok {
			return -1, 0, false, errors.New("destination page is not a reference")
		}
		pageRef = ref

	case *destination.FitH:
		ref, ok := d.Page.(pdf.Reference)
		if !ok {
			return -1, 0, false, errors.New("destination page is not a reference")
		}
		pageRef = ref
		pageNo, ok = pageNumbers[pageRef]
		if !ok {
			return -1, 0, false, errors.New("page reference not found in document")
		}
		if !math.IsNaN(d.Top) {
			return pageNo, d.Top, true, nil
		}
		return pageNo, 0, false, nil

	case *destination.FitV:
		ref, ok := d.Page.(pdf.Reference)
		if !ok {
			return -1, 0, false, errors.New("destination page is not a reference")
		}
		pageRef = ref

	case *destination.FitR:
		ref, ok := d.Page.(pdf.Reference)
		if !ok {
			return -1, 0, false, errors.New("destination page is not a reference")
		}
		pageRef = ref
		pageNo, ok = pageNumbers[pageRef]
		if !ok {
			return -1, 0, false, errors.New("page reference not found in document")
		}
		return pageNo, d.Top, true, nil

	case *destination.FitB:
		ref, ok := d.Page.(pdf.Reference)
		if !ok {
			return -1, 0, false, errors.New("destination page is not a reference")
		}
		pageRef = ref

	case *destination.FitBH:
		ref, ok := d.Page.(pdf.Reference)
		if !ok {
			return -1, 0, false, errors.New("destination page is not a reference")
		}
		pageRef = ref
		pageNo, ok = pageNumbers[pageRef]
		if !ok {
			return -1, 0, false, errors.New("page reference not found in document")
		}
		if !math.IsNaN(d.Top) {
			return pageNo, d.Top, true, nil
		}
		return pageNo, 0, false, nil

	case *destination.FitBV:
		ref, ok := d.Page.(pdf.Reference)
		if !ok {
			return -1, 0, false, errors.New("destination page is not a reference")
		}
		pageRef = ref

	default:
		return -1, 0, false, fmt.Errorf("unsupported destination type: %T", dest)
	}

	pageNo, ok := pageNumbers[pageRef]
	if !ok {
		return -1, 0, false, errors.New("page reference not found in document")
	}
	return pageNo, 0, false, nil
}

// findSectionEnd determines where the matched section ends by finding the next section
func findSectionEnd(r pdf.Getter, pageNumbers map[pdf.Reference]int, root *outline.Outline, match sectionMatch) (endPage int, endY float64, err error) {
	// find the level of the matched section
	level := findLevel(root.Items, match.item, 0)

	// find the next sibling at the same or higher level
	nextSection := findNextSectionAtLevel(root.Items, match.item, level)
	if nextSection == nil {
		// this is the last section, it goes to the end of the document
		totalPages := len(pageNumbers)
		return totalPages - 1, math.Inf(-1), nil
	}

	// get the page and coordinates of the next section
	nextPageNo, nextY, hasNextCoord, err := extractDestination(pageNumbers, nextSection)
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

// findLevel finds the level of the target item in the outline tree
func findLevel(items []*outline.Item, target *outline.Item, level int) int {
	for _, item := range items {
		if item == target {
			return level
		}
		if l := findLevel(item.Children, target, level+1); l >= 0 {
			return l
		}
	}
	return -1
}

// findNextSectionAtLevel finds the next section at the same or higher level than the target
func findNextSectionAtLevel(items []*outline.Item, target *outline.Item, targetLevel int) *outline.Item {
	found := false
	return findNextAtLevel(items, target, targetLevel, 0, &found)
}

// findNextAtLevel recursively searches for the next section at the specified level
func findNextAtLevel(items []*outline.Item, target *outline.Item, targetLevel, currentLevel int, found *bool) *outline.Item {
	for _, item := range items {
		if *found && currentLevel <= targetLevel {
			return item
		}

		if item == target {
			*found = true
		}

		if result := findNextAtLevel(item.Children, target, targetLevel, currentLevel+1, found); result != nil {
			return result
		}
	}
	return nil
}

// ListAll returns a list of all outline entries in the document.
func ListAll(r pdf.Getter) ([]string, error) {
	tree, err := outline.Read(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read outline: %w", err)
	}
	if tree == nil {
		return nil, errors.New("document has no outline")
	}

	var sections []string
	for _, item := range tree.Items {
		collectSections(item, "", &sections)
	}
	return sections, nil
}

// collectSections recursively collects all section titles with indentation
func collectSections(item *outline.Item, indent string, sections *[]string) {
	if item == nil {
		return
	}

	if item.Title != "" {
		*sections = append(*sections, indent+item.Title)
	}

	for _, child := range item.Children {
		collectSections(child, indent+"  ", sections)
	}
}

// FindNext finds the next section after the one matching the given pattern.
// Returns the title of the next section, or an empty string if no next section exists.
func FindNext(r pdf.Getter, pattern string) (string, error) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex pattern: %w", err)
	}

	tree, err := outline.Read(r)
	if err != nil {
		return "", fmt.Errorf("failed to read outline: %w", err)
	}
	if tree == nil {
		return "", errors.New("document has no outline")
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
	for _, item := range tree.Items {
		err = findMatches(r, pageNumbers, item, regex, &matches)
		if err != nil {
			return "", err
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no outline entries match pattern %q", pattern)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("pattern %q matches %d outline entries, expected exactly 1", pattern, len(matches))
	}

	match := matches[0]

	// find the level of the matched section
	level := findLevel(tree.Items, match.item, 0)

	// find the next sibling at the same or higher level
	nextSection := findNextSectionAtLevel(tree.Items, match.item, level)
	if nextSection == nil {
		return "", nil // no next section
	}

	return nextSection.Title, nil
}
