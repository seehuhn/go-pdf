// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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
	"bytes"
	"fmt"
	"io"
	"maps"
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
)

// The interactive forms of the inputs are merged into one.  The default
// resources (/DR) of all inputs are combined, and a resource whose name is
// already taken by an earlier input is renamed.  Default appearance strings
// (/DA) name these resources, so every /DA copied from that input, on fields
// at any depth, widgets, and FreeText and Redact annotations, is rewritten
// to use the new names.  A name which a /DA uses but its input's /DR lacks
// is reserved, so that it does not come to name another input's resource.
// The document-wide /DA and /Q entries, which fields inherit, are kept where
// all inputs agree and are moved down to each input's root fields otherwise.

// resRename maps a resource category of /DR (such as Font) to the renamed
// resources of that category, from the input's name to the output's.
type resRename map[pdf.Name]map[pdf.Name]pdf.Name

// formInput is the part of one input's interactive form which is written
// when the output is closed.
type formInput struct {
	// da is the input's document-wide default appearance, with its
	// resources renamed, or nil if it has none
	da pdf.String

	// q is the input's document-wide quadding
	q pdf.Integer

	fields []rootField
}

// rootField is a copied root field dictionary of an input's form.
type rootField struct {
	ref  pdf.Reference
	dict pdf.Dict
}

// preparedField records a source root field that has been redirected to a fresh
// reference in the output (so widgets copied with the pages point at it) and is
// to be (re)written, possibly with a new partial name, by finishForm.
type preparedField struct {
	newRef  pdf.Reference
	srcDict pdf.Dict
	rename  string // new partial name on a collision, "" to keep the original
}

// prepareForm reads one input's interactive form.  It merges the form's
// default resources and document-level flags, and for each root field
// allocates an output reference, redirects the source reference to it (so the
// field's widgets, copied with the pages, point back to it), and resolves any
// partial-name collision.  It returns the prepared root fields, and records
// the renamed resources in g so that /DA strings copied through g use the new
// names.  Pages must be copied after this, and finishForm called afterwards.
//
// The budget bounds the work spent on fingerprinting the input's default
// resources, see [fingerprinter].
func (c *Concat) prepareForm(r *pdf.Reader, g *daRenamer, cp *pdf.Copier, budget int64) ([]preparedField, *formInput, error) {
	cur := pdf.NewCursor(r)
	acro, err := pdf.Optional(cur.Dict(r.GetMeta().Catalog.AcroForm))
	if err != nil {
		return nil, nil, err
	} else if acro == nil {
		return nil, nil, nil
	}

	if err := c.mergeDR(r, g, cp, acro["DR"], budget); err != nil {
		return nil, nil, err
	}

	form := &formInput{}
	if da, err := pdf.Optional(cur.String(acro["DA"])); err != nil {
		return nil, nil, err
	} else if da != nil {
		form.da, _, err = rewriteDA(da, g.outputName)
		if err != nil {
			return nil, nil, err
		}
	}
	if q, err := pdf.Optional(cur.Integer(acro["Q"])); err != nil {
		return nil, nil, err
	} else if q >= 0 && q <= 2 {
		form.q = q
	}
	if na, _, err := cur.OptionalBoolean(acro["NeedAppearances"]); err != nil {
		return nil, nil, err
	} else if na {
		c.formNeedAppearances = true
	}
	if sf, err := pdf.Optional(cur.Integer(acro["SigFlags"])); err != nil {
		return nil, nil, err
	} else if sf > 0 {
		// only bits 1 (SignaturesExist) and 2 (AppendOnly) are defined
		c.formSigFlags |= sf & 3
	}

	fields, err := pdf.Optional(cur.Array(acro["Fields"]))
	if err != nil {
		return nil, nil, err
	}
	if c.usedNames == nil {
		c.usedNames = map[string]bool{}
	}

	var prepared []preparedField
	for _, el := range fields {
		ref, ok := el.(pdf.Reference)
		if !ok {
			continue
		}
		fieldDict, err := pdf.Optional(cur.Dict(ref))
		if err != nil {
			return nil, nil, err
		} else if fieldDict == nil {
			continue
		}
		name, err := pdf.Optional(cur.TextString(fieldDict["T"]))
		if err != nil {
			return nil, nil, err
		}

		newRef := c.w.Alloc()
		cp.Redirect(ref, newRef)
		prepared = append(prepared, preparedField{
			newRef:  newRef,
			srcDict: fieldDict,
			rename:  c.uniqueName(name.String()),
		})
	}
	return prepared, form, nil
}

// drCategory is one resource category of the merged /DR.
type drCategory struct {
	// res holds the copied resources, by output name
	res pdf.Dict

	// taken holds the output names in use, including those of resources
	// still to be copied
	taken map[pdf.Name]bool

	// bySum gives the output name of each resource with a fingerprint
	bySum map[resourceSum]pdf.Name

	// missing gives the output names reserved for names which /DA strings
	// use but their input's /DR lacks, by input name
	missing map[pdf.Name]pdf.Name
}

// category returns the given category of the merged /DR, creating it if
// needed.
func (c *Concat) category(name pdf.Name) *drCategory {
	if c.dr == nil {
		c.dr = map[pdf.Name]*drCategory{}
	}
	cat := c.dr[name]
	if cat == nil {
		cat = &drCategory{
			res:     pdf.Dict{},
			taken:   map[pdf.Name]bool{},
			bySum:   map[resourceSum]pdf.Name{},
			missing: map[pdf.Name]pdf.Name{},
		}
		c.dr[name] = cat
	}
	return cat
}

// mergeDR merges the default resources dr of one input into the output's
// /DR.  A resource whose name is free keeps it.  A resource whose name is
// taken shares an equivalent resource of an earlier input where there is
// one, and is copied under a fresh name otherwise.  The input's resources
// and their renames are recorded in g before any resource is copied, so
// that all /DA strings copied through g, including any reachable from the
// resources, use the output names.
//
// Equivalence is decided by fingerprints, which do not depend on the names
// of resources, so that renaming cannot make a shared resource differ from
// the one it stands for.
func (c *Concat) mergeDR(r *pdf.Reader, g *daRenamer, cp *pdf.Copier, dr pdf.Object, budget int64) error {
	cur := pdf.NewCursor(r)
	drDict, err := pdf.Optional(cur.Dict(dr))
	if err != nil {
		return err
	}
	categories := map[pdf.Name]pdf.Dict{}
	for catName, obj := range drDict {
		// the obsolete /ProcSet array names no resources
		res, err := pdf.Optional(cur.Dict(obj))
		if err != nil {
			return err
		} else if res != nil {
			categories[catName] = res
		}
	}
	g.dr = categories
	fp := newFingerprinter(r, categories, budget)

	type pending struct {
		cat  *drCategory
		name pdf.Name
		val  pdf.Object
	}
	var toCopy []pending

	// sorted: a name collision picks a fresh name from the order seen
	// here, and copying allocates object numbers in this order
	for _, catName := range slices.Sorted(maps.Keys(categories)) {
		res := categories[catName]
		cat := c.category(catName)
		for _, name := range slices.Sorted(maps.Keys(res)) {
			val := res[name]
			if val == nil {
				continue
			}
			sum, hasSum, err := fp.sum(catName, name)
			if err != nil {
				return err
			}

			outName := name
			if cat.taken[name] {
				if shared, ok := cat.bySum[sum]; hasSum && ok {
					if shared != name {
						addRename(g.rename, catName, name, shared)
					}
					continue
				}
				// the fresh name also avoids the input's own names, which
				// come later in sorted order and would need renaming too
				outName = freshName(name, func(n pdf.Name) bool {
					return cat.taken[n] || res[n] != nil
				})
				addRename(g.rename, catName, name, outName)
			}
			cat.taken[outName] = true
			if hasSum {
				if _, ok := cat.bySum[sum]; !ok {
					cat.bySum[sum] = outName
				}
			}
			toCopy = append(toCopy, pending{cat: cat, name: outName, val: val})
		}
	}
	for _, p := range toCopy {
		copied, err := cp.Copy(p.val.AsPDF(c.w.GetOptions()))
		if err != nil {
			return err
		}
		p.cat.res[p.name] = copied
	}
	return nil
}

func addRename(rename resRename, cat, from, to pdf.Name) {
	if rename[cat] == nil {
		rename[cat] = map[pdf.Name]pdf.Name{}
	}
	rename[cat][from] = to
}

// finishForm copies the prepared root field dictionaries, applying any field
// name or resource rename.  The copies are kept in form and written by
// writeForm, which may still have to move the input's document-wide /DA and
// /Q into them.
func (c *Concat) finishForm(g *daRenamer, cp *pdf.Copier, prepared []preparedField, form *formInput) error {
	for _, p := range prepared {
		src, _, err := g.patchDict(p.srcDict)
		if err != nil {
			return err
		}
		dict, err := cp.CopyDict(src)
		if err != nil {
			return err
		}
		if p.rename != "" {
			dict["T"] = pdf.Text(p.rename)
		}
		form.fields = append(form.fields, rootField{ref: p.newRef, dict: dict})
	}
	c.forms = append(c.forms, form)
	return nil
}

// writeForm writes the root fields of all inputs and returns the merged
// interactive form dictionary, or nil if no input has a form.
//
// Fields inherit /DA and /Q from the interactive form dictionary.  Where the
// inputs disagree on these values, the output's interactive form dictionary
// omits them, and each input's value is set on those of its root fields which
// do not give their own, so that every field keeps the value it inherited.
func (c *Concat) writeForm() (pdf.Dict, error) {
	if len(c.forms) == 0 {
		return nil, nil
	}

	commonDA, commonQ := c.forms[0].da, c.forms[0].q
	sameDA, sameQ := true, true
	for _, form := range c.forms[1:] {
		if (form.da == nil) != (commonDA == nil) || !bytes.Equal(form.da, commonDA) {
			sameDA = false
		}
		if form.q != commonQ {
			sameQ = false
		}
	}

	var fields pdf.Array
	for _, form := range c.forms {
		for _, f := range form.fields {
			if !sameDA && form.da != nil && f.dict["DA"] == nil {
				f.dict["DA"] = form.da
			}
			if !sameQ && form.q != 0 && f.dict["Q"] == nil {
				f.dict["Q"] = form.q
			}
			if err := c.w.Put(f.ref, f.dict); err != nil {
				return nil, err
			}
			fields = append(fields, f.ref)
		}
	}

	acro := pdf.Dict{"Fields": fields}
	dr := pdf.Dict{}
	for name, cat := range c.dr {
		if len(cat.res) > 0 {
			dr[name] = cat.res
		}
	}
	if len(dr) > 0 {
		acro["DR"] = dr
	}
	if sameDA && commonDA != nil {
		acro["DA"] = commonDA
	}
	if sameQ && commonQ != 0 {
		acro["Q"] = commonQ
	}
	if c.formNeedAppearances {
		acro["NeedAppearances"] = pdf.Boolean(true)
	}
	if c.formSigFlags != 0 {
		acro["SigFlags"] = c.formSigFlags
	}
	// /CO (calculation order) and /XFA are intentionally dropped: both
	// reference per-input state that cannot be merged across documents
	return acro, nil
}

// freshName returns a resource name derived from name for which taken
// reports false, for resolving a cross-input collision.
func freshName(name pdf.Name, taken func(pdf.Name) bool) pdf.Name {
	for i := 2; ; i++ {
		cand := pdf.Name(fmt.Sprintf("%s_%d", name, i))
		if !taken(cand) {
			return cand
		}
	}
}

// daRenamer is a [pdf.Getter] which reads objects from an input file and
// rewrites the /DA default appearance strings in them to use the output's
// /DR resource names, so that a [pdf.Copier] reading through it writes the
// renamed strings wherever in the input they occur.
type daRenamer struct {
	pdf.Getter
	c *Concat

	// dr holds the input's /DR, by category
	dr map[pdf.Name]pdf.Dict

	// rename maps the input's resources, by category, to their output
	// names where these differ
	rename resRename
}

func newDARenamer(r pdf.Getter, c *Concat) *daRenamer {
	return &daRenamer{Getter: r, c: c, rename: resRename{}}
}

// outputName returns the output name of the resource which the input names
// in the given category.
//
// A name which the input's /DR lacks names no resource, and must not come to
// name one in the output.  Such a name is reserved in the merged /DR: it is
// kept where it is free, and replaced by a fresh name otherwise.  Inputs
// lacking the same name share its reservation.
func (g *daRenamer) outputName(category, name pdf.Name) pdf.Name {
	if out, ok := g.rename[category][name]; ok {
		return out
	} else if g.dr[category][name] != nil {
		return name
	}
	cat := g.c.category(category)
	if out, ok := cat.missing[name]; ok {
		return out
	}
	out := name
	if cat.taken[name] {
		out = freshName(name, func(n pdf.Name) bool { return cat.taken[n] })
	}
	cat.taken[out] = true
	cat.missing[name] = out
	return out
}

// Get implements [pdf.Getter].
func (g *daRenamer) Get(ref pdf.Reference, canObjStm bool) (pdf.Native, error) {
	obj, err := g.Getter.Get(ref, canObjStm)
	if err != nil {
		return nil, err
	}
	res, _, err := g.patch(obj)
	return res, err
}

// patch returns obj with every /DA entry in it, or in the direct objects
// nested in it, rewritten.  Containers are copied where they change, so that
// objects held by the reader are left alone.  Streams are returned as they
// are, since a stream dictionary has no /DA entry.
func (g *daRenamer) patch(obj pdf.Native) (pdf.Native, bool, error) {
	switch x := obj.(type) {
	case pdf.Dict:
		return g.patchDict(x)
	case pdf.Array:
		var res pdf.Array
		for i, el := range x {
			native, ok := el.(pdf.Native)
			if !ok {
				continue
			}
			repl, changed, err := g.patch(native)
			if err != nil {
				return nil, false, err
			}
			if changed {
				if res == nil {
					res = slices.Clone(x)
				}
				res[i] = repl
			}
		}
		if res == nil {
			return x, false, nil
		}
		return res, true, nil
	default:
		return obj, false, nil
	}
}

// patchDict is the dictionary case of [daRenamer.patch].
func (g *daRenamer) patchDict(d pdf.Dict) (pdf.Dict, bool, error) {
	var res pdf.Dict
	set := func(key pdf.Name, val pdf.Object) {
		if res == nil {
			res = maps.Clone(d)
		}
		res[key] = val
	}
	// sorted: rewriting a /DA may reserve a fresh name, and the order
	// decides which name each reservation gets
	for _, key := range d.SortedKeys() {
		val := d[key]
		if key == "DA" {
			resolved, err := pdf.Optional(pdf.Resolve(g.Getter, val))
			if err != nil {
				return nil, false, err
			}
			da, ok := resolved.(pdf.String)
			if !ok {
				continue
			}
			newDA, changed, err := rewriteDA(da, g.outputName)
			if err != nil {
				return nil, false, err
			}
			if changed {
				set(key, newDA)
			}
			continue
		}
		native, ok := val.(pdf.Native)
		if !ok {
			continue
		}
		repl, changed, err := g.patch(native)
		if err != nil {
			return nil, false, err
		}
		if changed {
			set(key, repl)
		}
	}
	if res == nil {
		return d, false, nil
	}
	return res, true, nil
}

// daResourceOperand gives, for the operators which may occur in a default
// appearance string and name a resource, the resource category and the index
// of the operand which holds the name.  An index of -1 stands for the last
// operand.  A default appearance holds only operators allowed in text objects
// (PDF 2.0, 12.7.4.3), which excludes Do and sh.
var daResourceOperand = map[content.OpName]struct {
	category pdf.Name
	idx      int
}{
	content.OpTextSetFont:                      {"Font", 0},
	content.OpSetExtGState:                     {"ExtGState", 0},
	content.OpSetFillColorSpace:                {"ColorSpace", 0},
	content.OpSetStrokeColorSpace:              {"ColorSpace", 0},
	content.OpSetFillColorN:                    {"Pattern", -1},
	content.OpSetStrokeColorN:                  {"Pattern", -1},
	content.OpBeginMarkedContentWithProperties: {"Properties", 1},
	content.OpMarkedContentPointWithProperties: {"Properties", 1},
}

// daOperators returns the operators of a /DA default appearance string,
// tokenised as a content-stream fragment, without its comments.
func daOperators(da pdf.String) []content.Operator {
	stream := content.NewScanner(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(da)), nil
	})
	var ops []content.Operator
	for name, args := range stream.NewIter().All() {
		if name == content.OpRawContent {
			continue
		}
		// the scanner reuses its argument buffer
		ops = append(ops, content.Operator{Name: name, Args: slices.Clone(args)})
	}
	return ops
}

// colorSpaceFamilies are the colour space families which cs and CS name
// directly, without parameters and without a resource (PDF 2.0, 8.6.8).
var colorSpaceFamilies = map[pdf.Name]bool{
	"DeviceGray": true,
	"DeviceRGB":  true,
	"DeviceCMYK": true,
	"Pattern":    true,
}

// daResourceArg returns the resource category and the operand index of the
// resource named by op, if op is an operator of a /DA string which names a
// resource.
func daResourceArg(op content.Operator) (pdf.Name, int, bool) {
	operand, ok := daResourceOperand[op.Name]
	if !ok || len(op.Args) == 0 {
		return "", 0, false
	}
	idx := operand.idx
	if idx < 0 {
		idx = len(op.Args) - 1
	}
	if idx >= len(op.Args) {
		return "", 0, false
	}
	name, isName := op.Args[idx].(pdf.Name)
	if !isName || operand.category == "ColorSpace" && colorSpaceFamilies[name] {
		return "", 0, false
	}
	return operand.category, idx, true
}

// rewriteDA rewrites the resource names in a /DA default appearance string,
// replacing each by outputName(category, name).  The string is tokenised as a
// content-stream fragment, so that only the operands which name resources
// are replaced.  The string is returned unchanged where no name changes;
// otherwise it is written out again from its operators, without any
// comments it had.
func rewriteDA(da pdf.String, outputName func(category, name pdf.Name) pdf.Name) (pdf.String, bool, error) {
	ops := daOperators(da)
	changed := false
	for _, op := range ops {
		category, idx, ok := daResourceArg(op)
		if !ok {
			continue
		}
		name := op.Args[idx].(pdf.Name)
		if newName := outputName(category, name); newName != name {
			op.Args[idx] = newName
			changed = true
		}
	}
	if !changed {
		return da, false, nil
	}

	var out []byte
	for _, op := range ops {
		buf := &bytes.Buffer{}
		if err := op.Format(buf); err != nil {
			return nil, false, err
		}
		if len(out) > 0 {
			out = append(out, ' ')
		}
		out = append(out, bytes.TrimSuffix(buf.Bytes(), []byte("\n"))...)
	}
	return pdf.String(out), true, nil
}
