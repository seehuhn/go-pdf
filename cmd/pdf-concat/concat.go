// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package main

import (
	"fmt"
	"strings"
	"time"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/destination"
	"seehuhn.de/go/pdf/outline"
	"seehuhn.de/go/pdf/pagetree"
)

// Concat represents a PDF file made by concatenating other PDF files.
type Concat struct {
	v  pdf.Version
	w  *pdf.Writer
	rm *pdf.ResourceManager

	pages    *pagetree.Writer
	numPages int

	children []*Child

	// merged interactive form
	formFields          []pdf.Reference   // root fields of every input's form, in order
	usedNames           map[string]bool   // root field partial names already taken
	usedFontNames       map[pdf.Name]bool // /DR /Font resource names already taken
	drFonts             pdf.Dict          // union of the inputs' /DR /Font resources
	formDA              pdf.Object        // document-wide default appearance (first seen)
	formQ               pdf.Object        // document-wide quadding (first seen)
	formNeedAppearances bool              // OR of the inputs' NeedAppearances
	formSigFlags        pdf.Integer       // OR of the inputs' SigFlags
}

// Child represents a child document in the concatenated file.
type Child struct {
	Title     string
	FirstPage pdf.Reference
	Outline   []*outline.Item
}

// NewConcat creates a new Concat object.
func NewConcat(out string, v pdf.Version) (*Concat, error) {
	w, err := pdf.Create(out, v, nil)
	if err != nil {
		return nil, err
	}

	rm := pdf.NewResourceManager(w)
	pages := pagetree.NewWriter(w, rm)

	c := &Concat{
		v:     v,
		w:     w,
		rm:    rm,
		pages: pages,
	}

	return c, nil
}

// Close closes the output file.
func (c *Concat) Close() error {
	pagesRef, err := c.pages.Close()
	if err != nil {
		return err
	}

	meta := c.w.GetMeta()
	now := time.Now()
	meta.Info = &pdf.Info{
		Producer:     "seehuhn.de/go/pdf/cmd/pdf-concat",
		CreationDate: pdf.Date(now),
		ModDate:      pdf.Date(now),
	}
	meta.Catalog.Pages = pagesRef

	outlineTree := &outline.Outline{}
	for _, child := range c.children {
		entry := outlineTree.AddItem(child.Title)
		entry.Destination = &destination.Fit{Page: child.FirstPage}
		entry.Children = child.Outline
	}
	outlineRef, err := c.rm.Store(outlineTree)
	if err != nil {
		return err
	}
	c.rm.Out.GetMeta().Catalog.Outlines = outlineRef

	// merged interactive form
	if len(c.formFields) > 0 {
		fields := make(pdf.Array, len(c.formFields))
		for i, ref := range c.formFields {
			fields[i] = ref
		}
		acro := pdf.Dict{"Fields": fields}
		if len(c.drFonts) > 0 {
			acro["DR"] = pdf.Dict{"Font": c.drFonts}
		}
		if c.formDA != nil {
			acro["DA"] = c.formDA
		}
		if c.formQ != nil {
			acro["Q"] = c.formQ
		}
		if c.formNeedAppearances {
			acro["NeedAppearances"] = pdf.Boolean(true)
		}
		if c.formSigFlags != 0 {
			acro["SigFlags"] = c.formSigFlags
		}
		// /CO (calculation order) and /XFA are intentionally dropped: both
		// reference per-input state that cannot be merged across documents
		acroRef := c.w.Alloc()
		if err := c.w.Put(acroRef, acro); err != nil {
			return err
		}
		meta.Catalog.AcroForm = acroRef
	}

	err = c.rm.Close()
	if err != nil {
		return err
	}

	return c.w.Close()
}

// Append appends a PDF file to the output.
func (c *Concat) Append(fname string) error {
	r, err := pdf.Open(fname, nil)
	if err != nil {
		return err
	}
	defer r.Close()

	copy := pdf.NewCopier(c.w, r)

	// Set up the form's root fields before copying pages, so that widgets copied
	// as part of a page have their /Parent redirected to the renamed field.
	prepared, fontRename, err := c.prepareForm(r, copy)
	if err != nil {
		return err
	}

	meta := r.GetMeta()
	outlineTree, _ := pdf.DecodeOptional(pdf.NewCursor(r), r.GetMeta().Catalog.Outlines, outline.Decode)

	var title string
	if meta.Info != nil && meta.Info.Title != "" {
		title = string(meta.Info.Title)
	} else {
		title = fname
	}

	child := &Child{
		Title: title,
	}

	var copyError error
	for oldRef, dict := range pagetree.NewIterator(r).All() {
		newRef := c.w.Alloc()

		// Since we rebuild the page tree, we can't use `copy` to copy the page
		// dictionary.  Instead, we manually install a redirect from the newly
		// constructed page dict to the old one.
		copy.Redirect(oldRef, newRef)

		newDict, err := copy.CopyDict(dict)
		if err != nil {
			copyError = err
			break
		}
		err = c.pages.AppendPageDict(newRef, newDict)
		if err != nil {
			copyError = err
			break
		}

		if child.FirstPage == 0 {
			child.FirstPage = newRef
		}

		c.numPages++
	}
	if copyError != nil {
		return copyError
	}

	// write the (renamed) root field dictionaries now that their widgets,
	// copied with the pages, exist and reference the redirected field objects
	if err := c.finishForm(copy, prepared, fontRename); err != nil {
		return err
	}

	if outlineTree != nil {
		items, err := c.CopyOutlineItems(copy, outlineTree.Items)
		if err != nil {
			return err
		}
		child.Outline = items
	}

	c.children = append(c.children, child)

	return nil
}

// preparedField records a source root field that has been redirected to a fresh
// reference in the output (so widgets copied with the pages point at it) and is
// to be (re)written, possibly with a new partial name, by finishForm.
type preparedField struct {
	newRef  pdf.Reference
	srcDict pdf.Dict
	rename  string // new partial name on a collision, "" to keep the original
}

// prepareForm reads one input's interactive form. It merges the form's default
// resources, default appearance, and document-level flags, and for each root
// field allocates an output reference, redirects the source reference to it (so
// the field's widgets, copied with the pages, point back to it), and resolves
// any partial-name collision. It returns the prepared root fields and a map of
// the font resources renamed to avoid a collision, to be applied to the fields'
// /DA strings. Pages must be copied after this, and finishForm called afterwards.
func (c *Concat) prepareForm(r *pdf.Reader, cp *pdf.Copier) ([]preparedField, map[pdf.Name]pdf.Name, error) {
	cur := pdf.NewCursor(r)
	acro, _ := cur.Dict(r.GetMeta().Catalog.AcroForm)
	if acro == nil {
		return nil, nil, nil
	}

	// merge /DR /Font, renaming on a collision so no input's font is lost
	var fontRename map[pdf.Name]pdf.Name
	if dr, _ := cur.Dict(acro["DR"]); dr != nil {
		if fonts, _ := cur.Dict(dr["Font"]); fonts != nil {
			if c.drFonts == nil {
				c.drFonts = pdf.Dict{}
			}
			if c.usedFontNames == nil {
				c.usedFontNames = map[pdf.Name]bool{}
			}
			for name, fontObj := range fonts {
				fontRef, ok := fontObj.(pdf.Reference)
				if !ok {
					continue // form fonts are indirect; skip an inline oddity
				}
				copied, err := cp.CopyReference(fontRef)
				if err != nil {
					return nil, nil, err
				}
				outName := name
				if c.usedFontNames[name] {
					outName = c.freshFontName(name)
					if fontRename == nil {
						fontRename = map[pdf.Name]pdf.Name{}
					}
					fontRename[name] = outName
				}
				c.usedFontNames[outName] = true
				c.drFonts[outName] = copied
			}
		}
	}
	if c.formDA == nil {
		if da, _ := cur.String(acro["DA"]); da != nil {
			c.formDA = da // a self-contained string needs no copying
		}
	}
	if c.formQ == nil {
		if _, ok := acro["Q"]; ok {
			if q, err := cur.Integer(acro["Q"]); err == nil {
				c.formQ = q
			}
		}
	}
	if na, _ := cur.Boolean(acro["NeedAppearances"]); na {
		c.formNeedAppearances = true
	}
	if sf, _ := cur.Integer(acro["SigFlags"]); sf != 0 {
		c.formSigFlags |= sf
	}

	fields, _ := cur.Array(acro["Fields"])
	if c.usedNames == nil {
		c.usedNames = map[string]bool{}
	}

	var prepared []preparedField
	for _, el := range fields {
		ref, ok := el.(pdf.Reference)
		if !ok {
			continue
		}
		fieldDict, _ := cur.Dict(ref)
		if fieldDict == nil {
			continue
		}
		name, _ := cur.TextString(fieldDict["T"])

		newRef := c.w.Alloc()
		cp.Redirect(ref, newRef)
		prepared = append(prepared, preparedField{
			newRef:  newRef,
			srcDict: fieldDict,
			rename:  c.uniqueName(string(name)),
		})
	}
	return prepared, fontRename, nil
}

// finishForm writes the prepared root field dictionaries, applying any field
// name or font rename, and records them as roots of the merged form.
//
// A font rename is applied to the root field's own /DA. A /DA on a nested
// sub-field of a renamed input is not rewritten; such forms are rare, and the
// widgets' own appearance streams (which carry their resources) are unaffected.
func (c *Concat) finishForm(cp *pdf.Copier, prepared []preparedField, fontRename map[pdf.Name]pdf.Name) error {
	for _, p := range prepared {
		dict, err := cp.CopyDict(p.srcDict)
		if err != nil {
			return err
		}
		if p.rename != "" {
			dict["T"] = pdf.TextString(p.rename)
		}
		if da, ok := dict["DA"].(pdf.String); ok {
			dict["DA"] = rewriteDA(da, fontRename)
		}
		if err := c.w.Put(p.newRef, dict); err != nil {
			return err
		}
		c.formFields = append(c.formFields, p.newRef)
	}
	return nil
}

// freshFontName returns a /DR /Font resource name derived from name that is not
// yet in use, for resolving a cross-input collision.
func (c *Concat) freshFontName(name pdf.Name) pdf.Name {
	for i := 2; ; i++ {
		cand := pdf.Name(fmt.Sprintf("%s_%d", name, i))
		if !c.usedFontNames[cand] {
			return cand
		}
	}
}

// rewriteDA rewrites the renamed font resource names in a /DA default appearance
// string. The only name tokens a /DA contains are font names (the operand of the
// Tf operator), so every name token is looked up in rename. The pass is
// single-shot to avoid re-substituting a name that a rename just produced.
func rewriteDA(da pdf.String, rename map[pdf.Name]pdf.Name) pdf.String {
	if len(rename) == 0 {
		return da
	}
	s := []byte(da)
	var out []byte
	changed := false
	i := 0
	for i < len(s) {
		if s[i] != '/' {
			out = append(out, s[i])
			i++
			continue
		}
		// a name token: decode #XX escapes to match against the rename map
		j := i + 1
		var name []byte
		for j < len(s) && !isNameDelim(s[j]) {
			if s[j] == '#' && j+2 < len(s) && isHexDigit(s[j+1]) && isHexDigit(s[j+2]) {
				name = append(name, hexNibble(s[j+1])<<4|hexNibble(s[j+2]))
				j += 3
			} else {
				name = append(name, s[j])
				j++
			}
		}
		out = append(out, s[i:j]...) // keep the original token bytes
		if nn, ok := rename[pdf.Name(name)]; ok {
			// the new name is the old one plus a "_N" suffix of safe characters
			out = append(out, strings.TrimPrefix(string(nn), string(name))...)
			changed = true
		}
		i = j
	}
	if !changed {
		return da
	}
	return pdf.String(out)
}

// isNameDelim reports whether b ends a PDF name token (whitespace or delimiter).
func isNameDelim(b byte) bool {
	switch b {
	case 0, '\t', '\n', '\f', '\r', ' ',
		'(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return true
	}
	return false
}

func isHexDigit(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F'
}

func hexNibble(b byte) byte {
	switch {
	case b >= '0' && b <= '9':
		return b - '0'
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10
	default:
		return b - 'A' + 10
	}
}

// uniqueName reserves a partial field name for a merged root field, returning ""
// if the original name is free (keep it) or a fresh, non-colliding name to use
// instead. Anonymous fields (no name) never collide.
func (c *Concat) uniqueName(name string) string {
	if name == "" {
		return ""
	}
	if !c.usedNames[name] {
		c.usedNames[name] = true
		return ""
	}
	for i := 2; ; i++ {
		cand := fmt.Sprintf("%s_%d", name, i)
		if !c.usedNames[cand] {
			c.usedNames[cand] = true
			return cand
		}
	}
}

// CopyOutlineItems copies outline items from the source file to the target file.
func (c *Concat) CopyOutlineItems(cp *pdf.Copier, in []*outline.Item) ([]*outline.Item, error) {
	out := make([]*outline.Item, len(in))
	for i, child := range in {
		cc, err := c.CopyOutlineItems(cp, child.Children)
		if err != nil {
			return nil, err
		}

		entry := &outline.Item{
			Title:    child.Title,
			Children: cc,
			Open:     child.Open,
			Color:    child.Color,
			Bold:     child.Bold,
			Italic:   child.Italic,
		}

		// copy destination with page reference translation
		if child.Destination != nil {
			entry.Destination, err = copyDestination(cp, c.rm, child.Destination)
			if err != nil {
				return nil, err
			}
		}

		// copy action with page reference translation
		if child.Action != nil {
			entry.Action, err = copyAction(cp, c.rm, child.Action)
			if err != nil {
				return nil, err
			}
		}

		out[i] = entry
	}
	return out, nil
}

// copyDestination copies a destination, translating page references.
func copyDestination(cp *pdf.Copier, rm *pdf.ResourceManager,
	dest destination.Destination) (destination.Destination, error) {
	if _, ok := dest.(*destination.Named); ok {
		return dest, nil
	}
	encoded, err := dest.Encode(rm)
	if err != nil {
		return nil, err
	}
	copied, err := cp.Copy(encoded)
	if err != nil {
		return nil, err
	}
	return pdf.Decode(pdf.NewCursor(rm.Out), copied, destination.Decode)
}

// copyAction copies an action, translating page references.
func copyAction(cp *pdf.Copier, rm *pdf.ResourceManager,
	act pdf.Action) (pdf.Action, error) {
	encoded, err := act.Encode(rm)
	if err != nil {
		return nil, err
	}
	copied, err := cp.Copy(encoded)
	if err != nil {
		return nil, err
	}
	return pdf.Decode(pdf.NewCursor(rm.Out), copied, action.Decode)
}
