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

package printprep

import (
	"fmt"
	"io"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/fallback"
	"seehuhn.de/go/pdf/oc"
	"seehuhn.de/go/pdf/pagetree"
)

// Options control the print-safe simplification.  The zero value (or a nil
// *Options) selects every page, all layers on, markup shown, and the default
// appearance generator.
type Options struct {
	// Pages selects which pages to emit, as zero-based page indices in output
	// order.  A nil slice emits every page in document order.
	Pages []int

	// HiddenLayers lists the optional-content groups that are currently
	// switched off, by their object reference in the source document.  Content
	// belonging only to these groups is removed.  A nil slice treats every
	// group as on.
	HiddenLayers []pdf.Reference

	// HideMarkup suppresses markup annotations, matching a viewer's
	// "hide markup" state.
	HideMarkup bool

	// Generator supplies appearance streams for printable annotations that lack
	// one.  A nil value uses the library default,
	// [fallback.NewStyle] for the output version.
	Generator annotation.AppearanceGenerator
}

// page dictionary keys that affect the printed marks and are carried over.
// Everything else (metadata, structure, navigation, interactivity, prepress
// boxes, thumbnails, timestamps, ...) is dropped.
var keepPageKeys = []pdf.Name{
	"MediaBox",
	"CropBox",
	"Rotate",
	"Group",
	"UserUnit",
	"OutputIntents",
	"SeparationInfo",
}

// Write reads the document from r and writes a print-safe, unencrypted
// simplification to w.
func Write(w io.Writer, r *pdf.Reader, opts *Options) error {
	if opts == nil {
		opts = &Options{}
	}

	// the source version, floored at 1.3: converting glyf fonts to composite
	// CIDFontType2/Identity-H needs 1.3, and relabeling an older file upward is
	// safe since PDF 1.x is additive
	version := max(r.GetMeta().Version, pdf.V1_3)

	out, err := pdf.NewWriter(w, version, nil)
	if err != nil {
		return err
	}
	rm := pdf.NewResourceManager(out)

	c := &converter{
		r:            r,
		x:            pdf.NewExtractor(r),
		out:          out,
		rm:           rm,
		copy:         pdf.NewCopier(out, r),
		hideMkp:      opts.HideMarkup,
		gen:          opts.Generator,
		version:      version,
		formCache:    map[pdf.Reference]pdf.Reference{},
		patternCache: map[pdf.Reference]pdf.Reference{},
	}
	if c.gen == nil {
		c.gen = fallback.NewStyle(version)
	}
	if err := c.buildOCState(opts.HiddenLayers); err != nil {
		return err
	}

	pages, err := selectPages(r, opts.Pages)
	if err != nil {
		return err
	}

	tree := pagetree.NewWriter(out, rm)
	for _, pg := range pages {
		newRef := out.Alloc()
		newDict, err := c.convertPage(pg.dict)
		if err != nil {
			return err
		}
		if err := tree.AppendPageDict(newRef, newDict); err != nil {
			return err
		}
	}
	pagesRef, err := tree.Close()
	if err != nil {
		return err
	}

	meta := out.GetMeta()
	meta.Catalog.Pages = pagesRef
	if oi, ok := r.GetMeta().Catalog.OutputIntents.(pdf.Native); ok && oi != nil {
		copied, err := c.copy.Copy(oi)
		if err != nil {
			return err
		}
		meta.Catalog.OutputIntents = copied
	}

	if err := rm.Close(); err != nil {
		return err
	}
	return out.Close()
}

// converter holds the state shared while rewriting one document.
type converter struct {
	r       *pdf.Reader
	x       *pdf.Extractor // one shared extractor, for stable cached identities
	out     *pdf.Writer
	rm      *pdf.ResourceManager
	copy    *pdf.Copier
	ocState *oc.GroupStates
	hideMkp bool
	gen     annotation.AppearanceGenerator
	version pdf.Version

	// formCache and patternCache map a source form-XObject or tiling-pattern
	// reference to its rewritten output reference, deduplicating shared
	// resources and breaking reference cycles.
	formCache    map[pdf.Reference]pdf.Reference
	patternCache map[pdf.Reference]pdf.Reference
}

// buildOCState constructs the optional-content visibility state from the set of
// hidden group references.  Each of the document's optional-content groups is
// registered under both extraction identities it can acquire during the walk
// (as a [oc.Conditional] for a direct /OC entry, and as a [*oc.Group] for an
// OCMD member), so that visibility checks made through the shared extractor's
// cache resolve consistently.
func (c *converter) buildOCState(hidden []pdf.Reference) error {
	if len(hidden) == 0 {
		return nil
	}
	hiddenSet := make(map[pdf.Reference]bool, len(hidden))
	for _, ref := range hidden {
		hiddenSet[ref] = true
	}

	ocp, err := pdf.CursorAt(c.x, nil).Dict(c.r.GetMeta().Catalog.OCProperties)
	if err != nil || ocp == nil {
		return err
	}
	ocgs, err := pdf.CursorAt(c.x, nil).Array(ocp["OCGs"])
	if err != nil {
		return err
	}

	state := (&oc.Configuration{}).DefaultState(nil, oc.EventView, nil)
	for _, item := range ocgs {
		ref, ok := item.(pdf.Reference)
		if !ok {
			continue
		}
		on := !hiddenSet[ref]
		cur := pdf.CursorAt(c.x, nil)
		if cond, err := pdf.Decode(cur, ref, oc.ExtractConditional); err == nil {
			if g, ok := cond.(*oc.Group); ok {
				state.SetState(g, on)
			}
		}
		if g, err := pdf.DecodeOptional(cur, ref, oc.ExtractGroup); err == nil && g != nil {
			state.SetState(g, on)
		}
	}
	c.ocState = state
	return nil
}

// sourcePage pairs a page's reference with its inheritance-resolved dictionary.
type sourcePage struct {
	ref  pdf.Reference
	dict pdf.Dict
}

// selectPages returns the requested pages in output order, resolving page-tree
// inheritance.  A nil selection returns every page in document order.
func selectPages(r *pdf.Reader, sel []int) ([]sourcePage, error) {
	var all []sourcePage
	for ref, dict := range pagetree.NewIterator(r).All() {
		all = append(all, sourcePage{ref: ref, dict: dict})
	}
	if sel == nil {
		return all, nil
	}
	pages := make([]sourcePage, 0, len(sel))
	for _, i := range sel {
		if i < 0 || i >= len(all) {
			return nil, fmt.Errorf("page index %d out of range (have %d pages)", i, len(all))
		}
		pages = append(pages, all[i])
	}
	return pages, nil
}

// convertPage builds the print-safe page dictionary from a source page.
func (c *converter) convertPage(src pdf.Dict) (pdf.Dict, error) {
	dict := pdf.Dict{"Type": pdf.Name("Page")}
	for _, key := range keepPageKeys {
		// values read from a file are always Native; the comma-ok assertion
		// keeps a malformed value from panicking and simply drops the key
		v, ok := src[key].(pdf.Native)
		if !ok {
			continue
		}
		cv, err := c.copy.Copy(v)
		if err != nil {
			return nil, err
		}
		dict[key] = cv
	}

	srcRes, _ := pdf.CursorAt(c.x, nil).Dict(src["Resources"])
	fc := c.newFontContext(srcRes)

	// page marks: strip marked content and off optional-content groups, and
	// re-encode text shown in converted fonts (which populates fc)
	var body []byte
	var nesting int // unclosed "q" operators left by the page marks
	if contents, ok := src["Contents"]; ok {
		b, n, err := c.rewriteContentBytes(contents, srcRes, fc)
		if err != nil {
			return nil, err
		}
		body = b
		nesting = n
	}

	// annotations: flatten printable ones into an overlay after the page marks
	overlay, annotForms, err := c.flattenAnnots(src["Annots"], c.reservedXObjectNames(srcRes))
	if err != nil {
		return nil, err
	}
	if len(overlay) > 0 {
		// isolate the page marks so their residual graphics state (a leftover
		// CTM, clip, alpha, ...) cannot affect the flattened appearances, which
		// must draw in default user space (§12.5.5).  Wrap the marks in q/Q,
		// closing any q-regions they left open, then append the overlay.
		wrapped := append([]byte("q\n"), body...)
		wrapped = closeGraphicsState(wrapped, nesting)
		wrapped = append(wrapped, "Q\n"...)
		body = append(wrapped, overlay...)
	}

	ref, err := c.writeContentStream(body, nil)
	if err != nil {
		return nil, err
	}
	dict["Contents"] = ref

	// resources: converted fonts, rewritten forms/patterns, other resources copied
	fontDict, err := fc.subdict()
	if err != nil {
		return nil, err
	}
	var res pdf.Dict
	if resObj, ok := src["Resources"]; ok {
		newRes, err := c.convertResources(resObj, 0, fontDict)
		if err != nil {
			return nil, err
		}
		res, _ = newRes.(pdf.Dict)
	}
	if len(annotForms) > 0 {
		if res == nil {
			res = pdf.Dict{}
		}
		xobjs, _ := res["XObject"].(pdf.Dict)
		if xobjs == nil {
			xobjs = pdf.Dict{}
		}
		for name, ref := range annotForms {
			xobjs[name] = ref
		}
		res["XObject"] = xobjs
	}
	if res != nil {
		dict["Resources"] = res
	}

	return dict, nil
}
