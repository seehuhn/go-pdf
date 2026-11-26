// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

import (
	"io"
	"maps"
	"math"
	"reflect"
	"slices"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/vec"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/redactmark"
	"seehuhn.de/go/pdf/page"
)

const (
	// maxNestDepth bounds the nesting of forms, soft-mask groups and
	// patterns.
	maxNestDepth = 40

	// nestCost is the work charged for each nested stream which is analysed,
	// and opCost the work charged for each operator of an analysed content
	// stream.
	nestCost = 2048
	opCost   = 8

	// maxLatticeCells is the number of pattern cells which may meet the
	// regions before the whole pattern cell is redacted, and
	// maxLatticeCandidates bounds the number of cells which are tested.
	maxLatticeCells      = 64
	maxLatticeCandidates = 4096
)

// formSkipKeys lists the entries of a form or pattern dictionary which are
// not copied into a rewritten copy.  The content-defining keys are rewritten,
// and the other keys can repeat the removed content.  /Ref makes a form a
// reference XObject, whose target page can stand in for the content (PDF 2.0
// section 8.10.4).
var formSkipKeys = []pdf.Name{
	"Length", "Filter", "DecodeParms", "DL", "F", "FFilter", "FDecodeParms",
	"Resources", "PieceInfo", "Metadata", "AF", "OPI", "Ref",
}

// A streamSrc is a stream among the resources of a content stream.
type streamSrc struct {
	// ref is the reference of the stream, or 0 for a direct stream
	ref pdf.Reference
	stm *pdf.Stream
}

// A gsSrc is an extended graphics state with a soft mask whose group is a
// form XObject.
type gsSrc struct {
	dict  pdf.Dict
	smask pdf.Dict
	group *streamSrc
}

// A resSources holds the resources of a content stream in the form which the
// rewrite needs.
type resSources struct {
	// raw is the resource dictionary of the source, which can be nil
	raw pdf.Dict

	// res are the decoded resources
	res *content.Resources

	props    map[pdf.Name]pdf.Dict
	images   map[pdf.Name]*imageSrc
	forms    map[pdf.Name]*streamSrc
	patterns map[pdf.Name]*streamSrc
	gstates  map[pdf.Name]*gsSrc
	xnames   map[pdf.Name]bool
	gsnames  map[pdf.Name]bool

	// shadings are the shading patterns with a soft-mask group
	shadings map[pdf.Name]*shadingSrc

	// others are the XObjects which are neither images nor forms
	others map[pdf.Name]*streamSrc
}

// newResSources analyses the resource dictionary raw.  The decoded
// resources res can be nil.
func newResSources(cur pdf.Cursor, raw pdf.Dict, res *content.Resources) (*resSources, error) {
	rs := &resSources{raw: raw, res: res}
	var err error
	if rs.props, err = namedProperties(cur, raw); err != nil {
		return nil, err
	}
	srcXObj, err := pdf.Optional(cur.Dict(raw["XObject"]))
	if err != nil {
		return nil, err
	}
	rs.xnames = keySet(srcXObj)
	if rs.images, rs.forms, rs.others, err = xobjectSources(cur, srcXObj, res); err != nil {
		return nil, err
	}

	patterns, err := pdf.Optional(cur.Dict(raw["Pattern"]))
	if err != nil {
		return nil, err
	}
	rs.patterns = map[pdf.Name]*streamSrc{}
	rs.shadings = map[pdf.Name]*shadingSrc{}
	for name, obj := range patterns {
		src, err := streamOf(cur, obj)
		if err != nil {
			return nil, err
		}
		if src == nil {
			sh, err := shadingOf(cur, obj)
			if err != nil {
				return nil, err
			}
			if sh != nil {
				rs.shadings[name] = sh
			}
			continue
		}
		if t, err := pdf.Optional(cur.Integer(src.stm.Dict["PatternType"])); err != nil {
			return nil, err
		} else if t == 1 {
			rs.patterns[name] = src
		}
	}

	states, err := pdf.Optional(cur.Dict(raw["ExtGState"]))
	if err != nil {
		return nil, err
	}
	rs.gstates = map[pdf.Name]*gsSrc{}
	rs.gsnames = keySet(states)
	for name, obj := range states {
		g, err := gsSourceOf(cur, obj)
		if err != nil {
			return nil, err
		}
		if g != nil {
			rs.gstates[name] = g
		}
	}
	return rs, nil
}

// gsSourceOf returns the extended graphics state which obj refers to, if its
// soft mask has a group, or nil.
func gsSourceOf(cur pdf.Cursor, obj pdf.Object) (*gsSrc, error) {
	d, err := pdf.Optional(cur.Dict(obj))
	if err != nil {
		return nil, err
	}
	smask, err := pdf.Optional(cur.Dict(d["SMask"]))
	if err != nil || smask == nil {
		return nil, err
	}
	g, err := streamOf(cur, smask["G"])
	if err != nil || g == nil {
		return nil, err
	}
	return &gsSrc{dict: d, smask: smask, group: g}, nil
}

// A shadingSrc is a shading pattern whose graphics state has a soft mask
// with a group.
type shadingSrc struct {
	dict pdf.Dict
	gs   *gsSrc
}

// shadingOf returns the shading pattern which obj refers to, if its graphics
// state has a soft mask with a group, or nil.  Every pattern which is a
// dictionary, and not a stream, counts as a shading pattern.
func shadingOf(cur pdf.Cursor, obj pdf.Object) (*shadingSrc, error) {
	d, err := pdf.Optional(cur.Dict(obj))
	if err != nil || d == nil {
		return nil, err
	}
	gs, err := gsSourceOf(cur, d["ExtGState"])
	if err != nil || gs == nil {
		return nil, err
	}
	return &shadingSrc{dict: d, gs: gs}, nil
}

// shadingGroupRef returns the reference to the soft-mask group in the
// graphics state of the shading pattern obj, or 0.
func shadingGroupRef(cur pdf.Cursor, obj pdf.Object) (pdf.Reference, error) {
	sh, err := shadingOf(cur, obj)
	if err != nil || sh == nil {
		return 0, err
	}
	return sh.gs.group.ref, nil
}

// streamOf returns the stream which obj refers to, or nil.
func streamOf(cur pdf.Cursor, obj pdf.Object) (*streamSrc, error) {
	stm, err := pdf.Optional(cur.Stream(obj))
	if err != nil || stm == nil {
		return nil, err
	}
	src := &streamSrc{stm: stm}
	src.ref, _ = obj.(pdf.Reference)
	return src, nil
}

// A scope holds the state of the nested rewrites of one page.
type scope struct {
	// copies caches the outcome of nested rewrites
	copies map[copyKey]*copyResult

	// active lists the streams which are being rewritten
	active map[pdf.Reference]bool

	// lastHull is the last identifier given to a set of regions
	lastHull int

	// lost lists the marked-content identifiers of the spans in nested
	// forms which lost content
	lost []mcidRecord

	// lostObjs lists the /StructParent keys of the form XObjects which
	// are dropped or rewritten
	lostObjs []int
}

func newScope() *scope {
	return &scope{
		copies: map[copyKey]*copyResult{},
		active: map[pdf.Reference]bool{},
	}
}

// An mcidRecord names a marked-content sequence which lost content.  The key
// is the /StructParents entry of the stream which holds the sequence.  The
// key [unknownKey] stands for content whose structure element cannot be
// found.
type mcidRecord struct {
	key, mcid int
}

// unknownKey is the parent tree key recorded for content whose structure
// element cannot be found.  It is not a valid parent tree key.
const unknownKey = -1

// A copyKey identifies a rewritten copy of a stream.  A copy depends on the
// place where the stream is used, which the matrix and the regions fix, and
// on the kind of use.
type copyKey struct {
	ref    pdf.Reference
	base   matrix.Matrix
	hullID int

	// kind is the type of the content, which tells a form, a soft-mask group
	// and a pattern apart
	kind content.Type

	// inh holds the inherited parameters which affect the rewrite
	inh inheritKey

	// inherit is set for a form without resources
	inherit *resSources
}

// A copyResult is the outcome of the rewrite of a nested stream.
type copyResult struct {
	// changed is set if the stream is replaced by the copy ref, and lost if
	// the copy lost content
	changed, lost bool
	ref           pdf.Reference

	// fail is set if the stream cannot be rewritten: it is cyclic or nested
	// too deeply
	fail bool

	// used lists the resources of the parent which an unchanged stream
	// without resources of its own uses
	used *usedNames
}

// A usedNames lists resource names which a stream uses, for the categories
// from which a rewritten stream drops the entries it does not use.
type usedNames struct {
	images, forms, gstates, props map[pdf.Name]bool
}

// useNames records that the rewritten stream of wk uses the resources u.
func (wk *walker) useNames(u *usedNames) {
	for name := range u.images {
		touch(&wk.keptImages, name)
	}
	for name := range u.forms {
		touch(&wk.keptForms, name)
	}
	for name := range u.gstates {
		touch(&wk.keptGS, name)
	}
	for name := range u.props {
		wk.keep(name)
	}
}

// A nestSpec describes a nested stream which is to be rewritten.
type nestSpec struct {
	src *streamSrc

	// key identifies the placement.  key.base maps the default space of the
	// stream to the space of the hulls.
	key copyKey

	// hulls are the regions, and hullID identifies them.  For a pattern,
	// these are the regions moved onto the pattern cell, which differ from
	// the regions of the placement in key.
	hulls  []poly
	hullID int

	// parent is the walker of the stream which uses the stream.  A form
	// without resources uses the resources of the parent.
	parent *walker

	// inherit is the initial graphics state, which is nil for the defaults;
	// key.kind is the type of the content
	inherit *graphics.State

	// pat holds the patterns selected as colours at the start of the stream
	pat patTracker

	depth int
}

// noteMCIDs records the marked-content identifiers of a stream which lost
// content.  If the stream has no valid /StructParents key, the structure
// elements of the content cannot be found.
func (red *redactor) noteMCIDs(sc *scope, dict pdf.Dict, mcids []int) error {
	if len(mcids) == 0 {
		return nil
	}
	key, ok, err := red.parentKey(dict["StructParents"])
	if err != nil {
		return err
	}
	if !ok {
		sc.lost = append(sc.lost, mcidRecord{key: unknownKey, mcid: -1})
		return nil
	}
	for _, m := range mcids {
		sc.lost = append(sc.lost, mcidRecord{key: int(key), mcid: m})
	}
	return nil
}

// rewriteNested rewrites a form, soft-mask group or pattern for the regions
// of its placement.  The copy is added to red.pending, to be written once all
// redirections are known.
//
// No copy is made if the rewrite changes nothing.  A copy is always made for
// a stream with a malformed /Matrix, which the analysis reads as the identity
// and the copy leaves out, and for a reference XObject, whose copy has no
// /Ref.  A stream without a copy which uses the resources of its parent adds
// the resources it uses to those which the parent keeps.
func (red *redactor) rewriteNested(sc *scope, sp *nestSpec) (*copyResult, error) {
	src := sp.src
	cur := pdf.CursorAt(red.x, nil)
	rawRes, err := pdf.Optional(cur.Dict(src.stm.Dict["Resources"]))
	if err != nil {
		return nil, err
	}
	inherited := rawRes == nil && sp.parent != nil
	_, matrixOK, err := parseMatrix(cur, src.stm.Dict["Matrix"])
	if err != nil {
		return nil, err
	}
	_, isRef := src.stm.Dict["Ref"]
	needCopy := !matrixOK || isRef
	key := sp.key
	if inherited {
		key.inherit = sp.parent.rs
	}

	// the outcome for a direct stream is not cached: other direct streams
	// share its key
	store := func(r *copyResult) *copyResult {
		if src.ref != 0 {
			sc.copies[key] = r
		}
		return r
	}
	if src.ref != 0 {
		if r := sc.copies[key]; r != nil {
			if r.used != nil {
				sp.parent.useNames(r.used)
			}
			return r, nil
		}
		if sc.active[src.ref] || sp.depth > maxNestDepth {
			return &copyResult{fail: true}, nil
		}
		sc.active[src.ref] = true
		defer delete(sc.active, src.ref)
	} else if sp.depth > maxNestDepth {
		return &copyResult{fail: true}, nil
	}
	if err := red.charge(nestCost); err != nil {
		return nil, err
	}

	var rs *resSources
	if inherited {
		rs = sp.parent.rs
	} else {
		_, resRef := src.stm.Dict["Resources"].(pdf.Reference)
		res, err := pdf.Optional(extract.Resources(cur, src.stm.Dict["Resources"], !resRef))
		if err != nil {
			return nil, err
		}
		if rs, err = newResSources(cur, rawRes, res); err != nil {
			return nil, err
		}
		if err := red.noteFontSources(cur, rawRes, res); err != nil {
			return nil, err
		}
	}

	var obj pdf.Object = src.stm
	if src.ref != 0 {
		obj = src.ref
	}
	open := func() (io.ReadCloser, error) {
		segments, err := page.ExtractContents(cur, obj)
		if err != nil {
			return nil, err
		}
		return page.SegmentsReader(segments), nil
	}
	wk := newWalker(open, rs.res, key.base, sp.hulls, red.fonts)
	wk.red, wk.sc = red, sc
	wk.rs = rs
	wk.useResources(rs)
	wk.depth, wk.hullID = sp.depth, sp.hullID
	wk.kind, wk.inherit = key.kind, sp.inherit
	wk.pat = patTracker{fill: sp.pat.fill, stroke: sp.pat.stroke}
	wk.startPat = wk.pat
	wk.inherited = inherited
	wk.charge = red.charge

	p, err := wk.analyze()
	if err != nil {
		return nil, err
	}
	if !p.changed && !needCopy {
		r := &copyResult{}
		if inherited {
			r.used = &usedNames{
				images: wk.keptImages, forms: wk.keptForms,
				gstates: wk.keptGS, props: wk.keptProps,
			}
			sp.parent.useNames(r.used)
		}
		return store(r), nil
	}

	// the copy keeps the source of the content and the plan, but not the
	// walker with its decoded resources
	res, err := red.planResources(cur, rs.raw, wk, p, nil)
	if err != nil {
		return nil, err
	}
	dict := copiedEntries(src.stm.Dict)
	if !matrixOK {
		delete(dict, "Matrix")
	}
	if err := red.renumberCopy(dict); err != nil {
		return nil, err
	}
	ref := red.out.Alloc()
	red.pending = append(red.pending, &pendingStream{
		ref: ref, dict: dict, res: res, open: wk.open, plan: p,
	})
	red.taint(src.ref)
	red.noteCopy(src.ref, ref)
	if err := red.noteMCIDs(sc, src.stm.Dict, wk.lostMCIDs); err != nil {
		return nil, err
	}
	return store(&copyResult{changed: true, lost: p.lost, ref: ref}), nil
}

// renumberCopy gives the dictionary of a redacted copy of a stream its own
// /StructParents and /StructParent keys, which repeat the parent tree
// entries of the keys of the source.  The source can stay in use next to
// the copy, and the two must not share a key.  A key which cannot be added
// to the parent tree is removed from the copy, and an invalid key is kept.
func (red *redactor) renumberCopy(dict pdf.Dict) error {
	for _, name := range []pdf.Name{"StructParents", "StructParent"} {
		obj, present := dict[name]
		if !present {
			continue
		}
		old, ok, err := red.parentKey(obj)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		key, ok, err := red.copyParentKey(old)
		if err != nil {
			return err
		}
		if ok {
			dict[name] = key
		} else {
			delete(dict, name)
		}
	}
	return nil
}

// emptyCopy makes a tiling pattern without content, of the given kind.  This
// is the replacement for a pattern which meets a region and which cannot be
// rewritten.  The copy is added to red.pending.  Since it paints nothing, it
// keeps only the paint type of the source, and has valid values for the
// other required entries and no parent tree keys.
func (red *redactor) emptyCopy(src *streamSrc, kind content.Type) pdf.Reference {
	paintType := pdf.Integer(1)
	if kind == content.PatternUncolored {
		paintType = 2
	}
	one := pdf.Integer(1)
	dict := pdf.Dict{
		"Type":        pdf.Name("Pattern"),
		"PatternType": one,
		"PaintType":   paintType,
		"TilingType":  one,
		"BBox":        pdf.Array{pdf.Integer(0), pdf.Integer(0), one, one},
		"XStep":       one,
		"YStep":       one,
	}
	ref := red.out.Alloc()
	red.pending = append(red.pending, &pendingStream{ref: ref, dict: dict})
	red.taint(src.ref)
	return ref
}

// copiedEntries returns the entries of a form or pattern dictionary which a
// rewritten copy keeps.
func copiedEntries(d pdf.Dict) pdf.Dict {
	rest := pdf.Dict{}
	for k, v := range d {
		if !slices.Contains(formSkipKeys, k) {
			rest[k] = v
		}
	}
	return rest
}

// A pendingStream is a redacted copy of a form, soft-mask group or pattern,
// which is written once all redirections are known.
type pendingStream struct {
	ref pdf.Reference

	// dict holds the entries of the source dictionary which the copy keeps
	dict pdf.Dict

	// res describes the resources of the copy.  If res is nil, the copy has
	// empty resources and no content.
	res *resPlan

	// open reads the source content, and plan describes how it is rewritten
	open func() (io.ReadCloser, error)
	plan *plan

	// shading, if it is not nil, describes a copy of a shading pattern,
	// which is written in place of a stream
	shading *shadingPlan
}

// A shadingPlan is a copy of a shading pattern.  If gs is not nil, the
// graphics state of the copy is the one which gs describes.  If matrixOK is
// not set, the copy has no /Matrix.
type shadingPlan struct {
	src      *shadingSrc
	gs       *gsPlan
	matrixOK bool
}

// writePending writes the redacted copies of the nested streams.
func (red *redactor) writePending() error {
	for _, ps := range red.pending {
		var err error
		if ps.shading != nil {
			err = red.writeShading(ps.ref, ps.shading)
		} else {
			err = red.writeStream(ps)
		}
		if err != nil {
			return err
		}
	}
	red.pending = nil
	return nil
}

// writeShading writes the copy of a shading pattern which sp describes.
func (red *redactor) writeShading(ref pdf.Reference, sp *shadingPlan) error {
	rest := maps.Clone(sp.src.dict)
	if !sp.matrixOK {
		delete(rest, "Matrix")
	}
	if sp.gs != nil {
		delete(rest, "ExtGState")
	}
	d, err := red.copier.CopyDict(rest)
	if err != nil {
		return err
	}
	if sp.gs != nil {
		if d["ExtGState"], err = red.copyGS(sp.gs); err != nil {
			return err
		}
	}
	return red.out.Put(ref, d)
}

// aliasShading returns the copy of the shading pattern obj whose soft-mask
// group is tainted, for a use which paints the pattern unchanged.  The copy
// refers to the alias of the group.
func (red *redactor) aliasShading(cur pdf.Cursor, obj pdf.Object, alias pdf.Reference) (pdf.Native, error) {
	sh, err := shadingOf(cur, obj)
	if err != nil {
		return nil, err
	}
	rest := maps.Clone(sh.dict)
	delete(rest, "ExtGState")
	d, err := red.copier.CopyDict(rest)
	if err != nil {
		return nil, err
	}
	if d["ExtGState"], err = red.copyGS(&gsPlan{src: sh.gs, group: alias}); err != nil {
		return nil, err
	}
	return d, nil
}

// writeStream writes a redacted copy of a nested stream.  The content is
// written as the plan of the analysis says.
func (red *redactor) writeStream(ps *pendingStream) error {
	res := pdf.Dict{}
	if ps.res != nil {
		var err error
		res, err = red.buildResources(ps.res)
		if err != nil {
			return err
		}
	}
	dict, err := red.copier.CopyDict(ps.dict)
	if err != nil {
		return err
	}
	dict["Resources"] = res
	w, err := red.out.OpenStream(ps.ref, dict, pdf.FilterCompress{})
	if err != nil {
		return err
	}
	if ps.res != nil {
		cw := &chargeWriter{w: w, charge: red.charge}
		tail, err := emit(cw, ps.open, ps.plan, red.out.GetOptions())
		if err == nil {
			err = red.writeOps(cw, tail)
		}
		if err != nil {
			w.Close()
			return err
		}
	}
	return w.Close()
}

// A chargeWriter accounts for the bytes which are written.
type chargeWriter struct {
	w      io.Writer
	charge func(n int64) error
}

func (c *chargeWriter) Write(p []byte) (int, error) {
	if err := c.charge(int64(len(p))); err != nil {
		return 0, err
	}
	return c.w.Write(p)
}

// matrixOf returns the matrix in a dictionary entry, see [parseMatrix].
func matrixOf(cur pdf.Cursor, obj pdf.Object) (matrix.Matrix, error) {
	m, _, err := parseMatrix(cur, obj)
	return m, err
}

// parseMatrix returns the matrix in a dictionary entry.  An absent or
// malformed entry gives the identity, which is the default of the /Matrix
// entries, and ok is false for a malformed entry.  A rewritten copy leaves out
// a malformed entry, so that it reads as the matrix which the analysis used.
// A singular matrix is returned as it is.
func parseMatrix(cur pdf.Cursor, obj pdf.Object) (m matrix.Matrix, ok bool, err error) {
	if obj == nil {
		return matrix.Identity, true, nil
	}
	m, err = cur.Matrix(obj)
	if pdf.IsMalformed(err) {
		return matrix.Identity, false, nil
	} else if err != nil {
		return matrix.Matrix{}, false, err
	}
	for _, v := range m {
		if !isFinite(v) {
			return matrix.Identity, false, nil
		}
	}
	return m, true, nil
}

// analyzeFormDo decides what happens to the form which a Do operator paints.
// The form is always analysed: text extraction ignores the clip of the /BBox,
// so the box does not bound the content which a region can hide.
func (wk *walker) analyzeFormDo(p *plan, gs *graphics.State, idx int, name pdf.Name, src *streamSrc) error {
	cur := pdf.CursorAt(wk.red.x, nil)
	m, err := matrixOf(cur, src.stm.Dict["Matrix"])
	if err != nil {
		return err
	}
	fm := m.Mul(gs.CTM).Mul(wk.base)

	inherit := gs.Clone()
	inherit.CTM = matrix.Identity
	res, err := wk.red.rewriteNested(wk.sc, &nestSpec{
		src: src,
		key: copyKey{
			ref: src.ref, base: fm, hullID: wk.hullID,
			kind: content.Form, inh: inheritKeyOf(gs, wk.pat),
		},
		hulls:  wk.hulls,
		hullID: wk.hullID,
		parent: wk,
		depth:  wk.depth + 1,

		inherit: inherit,
		pat:     wk.pat,
	})
	if err != nil {
		return err
	}
	switch {
	case res.changed:
		newName, ok := wk.formNames[res.ref]
		if !ok {
			newName = wk.freshXObjectName()
			wk.extraXObjects[newName] = res.ref
			if wk.formNames == nil {
				wk.formNames = map[pdf.Reference]pdf.Name{}
			}
			wk.formNames[res.ref] = newName
		}
		touch(&wk.touchedForms, name)
		p.ops[idx] = &opPlan{repl: []content.Operator{
			{Name: content.OpXObject, Args: []pdf.Object{newName}},
		}}
		if !res.lost {
			p.change()
			return nil
		}
		p.lose()
		return wk.red.noteObj(wk.sc, src.stm.Dict)
	case res.fail:
		touch(&wk.touchedForms, name)
		p.ops[idx] = &opPlan{drop: true}
		p.lose()
		wk.red.taint(src.ref)
		return wk.red.scanDropped(wk.sc, src, wk.rs, map[visit]bool{})
	default:
		touch(&wk.keptForms, name)
		return nil
	}
}

// An inheritKey holds the inherited graphics state parameters which affect
// how the content of a form is classified.  The pointers keep the objects
// alive, so that a key cannot match a later object at the same address.
type inheritKey struct {
	// font is the text font, or a pointer which matches no other key if the
	// font cannot serve as a map key
	font any

	fontSize, charSpacing, wordSpacing, hScaling, leading, rise float64
	renderMode                                                  graphics.TextRenderingMode
	knockout                                                    bool

	lineWidth  float64
	lineCap    graphics.LineCapStyle
	lineJoin   graphics.LineJoinStyle
	miterLimit float64

	// fill and stroke are the patterns of the colours, if these are pattern
	// colours
	fill, stroke patRef
}

// inheritKeyOf returns the inherited graphics state parameters which affect
// how the content of a form or pattern is classified.  The tracker pat holds
// the patterns of the colours of gs.
func inheritKeyOf(gs *graphics.State, pat patTracker) inheritKey {
	k := inheritKey{
		fontSize:    gs.TextFontSize,
		charSpacing: gs.TextCharacterSpacing,
		wordSpacing: gs.TextWordSpacing,
		hScaling:    gs.TextHorizontalScaling,
		leading:     gs.TextLeading,
		rise:        gs.TextRise,
		renderMode:  gs.TextRenderingMode,
		knockout:    gs.TextKnockout,
		lineWidth:   gs.LineWidth,
		lineCap:     gs.LineCap,
		lineJoin:    gs.LineJoin,
		miterLimit:  gs.MiterLimit,
	}
	if f := gs.TextFont; f != nil {
		if reflect.TypeOf(f).Kind() == reflect.Pointer {
			k.font = f
		} else {
			k.font = new(byte)
		}
	}
	if c := gs.FillColor; c != nil && color.IsPattern(c.ColorSpace()) {
		k.fill = pat.fill
	}
	if c := gs.StrokeColor; c != nil && color.IsPattern(c.ColorSpace()) {
		k.stroke = pat.stroke
	}
	return k
}

// A visit is a stream which [redactor.scanDropped] has scanned.  For a
// stream without resources of its own, rs holds the resources of the stream
// which uses it, since its names refer to these.
type visit struct {
	ref pdf.Reference
	rs  *resSources
}

// scanDropped records the marked-content identifiers in a form which is
// removed from the content, and in the forms, soft-mask groups, tiling
// patterns and soft-mask groups of shading patterns which it uses.  The scan
// reads the operators only, and does not follow the graphics state.  The
// form's /StructParent key is recorded too.  The resources parent are those
// of the stream which uses the form.
//
// Every stream which the scan reaches is tainted: it painted on a page with
// regions, and none of its content is kept there.
func (red *redactor) scanDropped(sc *scope, src *streamSrc, parent *resSources, visited map[visit]bool) error {
	cur := pdf.CursorAt(red.x, nil)
	rawRes, err := pdf.Optional(cur.Dict(src.stm.Dict["Resources"]))
	if err != nil {
		return err
	}
	if src.ref != 0 {
		v := visit{ref: src.ref}
		if rawRes == nil {
			v.rs = parent
		}
		if visited[v] {
			return nil
		}
		visited[v] = true
	}
	if err := red.charge(nestCost); err != nil {
		return err
	}
	red.taint(src.ref)
	if err := red.noteObj(sc, src.stm.Dict); err != nil {
		return err
	}

	rs := parent
	if rawRes != nil {
		if rs, err = newResSources(cur, rawRes, nil); err != nil {
			return err
		}
	}
	var obj pdf.Object = src.stm
	if src.ref != 0 {
		obj = src.ref
	}
	segments, err := page.ExtractContents(cur, obj)
	if err != nil {
		return err
	}
	it := content.NewScanner(func() (io.ReadCloser, error) {
		return page.SegmentsReader(segments), nil
	}).NewIter()
	var inner []*streamSrc
	var mcids []int
	for name, args := range it.All() {
		if err := red.charge(opCost); err != nil {
			return err
		}
		switch name {
		case content.OpBeginMarkedContentWithProperties:
			if len(args) < 2 {
				continue
			}
			var props pdf.Dict
			switch x := args[1].(type) {
			case pdf.Dict:
				props = x
			case pdf.Name:
				props = rs.props[x]
			}
			if m, ok := props["MCID"].(pdf.Integer); ok && m >= 0 && int64(m) <= math.MaxInt32 {
				mcids = append(mcids, int(m))
			}
		case content.OpXObject:
			if len(args) > 0 {
				if n, ok := args[0].(pdf.Name); ok && rs.forms[n] != nil {
					inner = append(inner, rs.forms[n])
				}
			}
		case content.OpSetExtGState:
			if len(args) > 0 {
				if n, ok := args[0].(pdf.Name); ok && rs.gstates[n] != nil {
					inner = append(inner, rs.gstates[n].group)
				}
			}
		case content.OpSetFillColorN, content.OpSetStrokeColorN:
			if len(args) > 0 {
				n, _ := args[len(args)-1].(pdf.Name)
				if rs.patterns[n] != nil {
					inner = append(inner, rs.patterns[n])
				} else if sh := rs.shadings[n]; sh != nil {
					inner = append(inner, sh.gs.group)
				}
			}
		}
	}
	// the scanner reads malformed data as a reader does, and reports only
	// other errors
	if err := it.Err(); err != nil {
		return err
	}
	if err := red.noteMCIDs(sc, src.stm.Dict, mcids); err != nil {
		return err
	}
	for _, f := range inner {
		if err := red.scanDropped(sc, f, rs, visited); err != nil {
			return err
		}
	}
	return nil
}

// noteObj records the /StructParent key of a dropped or rewritten form.  A
// key which is present but not valid is recorded as [unknownKey].
func (red *redactor) noteObj(sc *scope, d pdf.Dict) error {
	obj, present := d["StructParent"]
	if !present {
		return nil
	}
	key, ok, err := red.parentKey(obj)
	if err != nil {
		return err
	}
	if ok {
		sc.lostObjs = append(sc.lostObjs, int(key))
	} else {
		sc.lostObjs = append(sc.lostObjs, unknownKey)
	}
	return nil
}

// touch adds a name to a set of names.
func touch(set *map[pdf.Name]bool, name pdf.Name) {
	if *set == nil {
		*set = map[pdf.Name]bool{}
	}
	(*set)[name] = true
}

// A gsKey identifies a rewritten extended graphics state.
type gsKey struct {
	name pdf.Name
	base matrix.Matrix
}

// analyzeGS decides what happens to the soft mask which a gs operator sets.
func (wk *walker) analyzeGS(p *plan, gs *graphics.State, idx int, args []pdf.Object) error {
	if len(args) < 1 {
		return nil
	}
	name, ok := args[0].(pdf.Name)
	if !ok {
		return nil
	}
	src := wk.gstates[name]
	if src == nil {
		return nil
	}

	// as for forms, the /BBox does not bound the content
	cur := pdf.CursorAt(wk.red.x, nil)
	m, err := matrixOf(cur, src.group.stm.Dict["Matrix"])
	if err != nil {
		return err
	}
	fm := m.Mul(gs.CTM).Mul(wk.base)

	key := gsKey{name, fm}
	newName, done := wk.gsCache[key]
	if !done {
		var group pdf.Object = pdf.Name("None")
		lost := true
		res, err := wk.red.rewriteNested(wk.sc, &nestSpec{
			src:    src.group,
			key:    copyKey{ref: src.group.ref, base: fm, hullID: wk.hullID, kind: content.Page},
			hulls:  wk.hulls,
			hullID: wk.hullID,
			parent: wk,
			depth:  wk.depth + 1,
		})
		if err != nil {
			return err
		}
		switch {
		case res.changed:
			group, lost = res.ref, res.lost
			if lost {
				err = wk.red.noteObj(wk.sc, src.group.stm.Dict)
			}
		case res.fail:
			wk.red.taint(src.group.ref)
			err = wk.red.scanDropped(wk.sc, src.group, wk.rs, map[visit]bool{})
		default:
			touch(&wk.keptGS, name)
			return nil
		}
		if err != nil {
			return err
		}

		newName = wk.freshGSName()
		wk.extraGStates[newName] = &gsPlan{src: src, group: group, lost: lost}
		if wk.gsCache == nil {
			wk.gsCache = map[gsKey]pdf.Name{}
		}
		wk.gsCache[key] = newName
	}
	touch(&wk.touchedGS, name)
	p.ops[idx] = &opPlan{repl: []content.Operator{
		{Name: content.OpSetExtGState, Args: []pdf.Object{newName}},
	}}
	if wk.extraGStates[newName].lost {
		p.lose()
	} else {
		p.change()
	}
	return nil
}

// A gsPlan is an extended graphics state of a rewritten stream in which the
// soft mask group is replaced.  The group is a reference to the output file,
// or the name None.  lost is set if the soft mask lost content.
type gsPlan struct {
	src   *gsSrc
	group pdf.Object
	lost  bool
}

// copyGS returns the copy of an extended graphics state which g describes.
func (red *redactor) copyGS(g *gsPlan) (pdf.Dict, error) {
	rest := maps.Clone(g.src.dict)
	delete(rest, "SMask")
	d, err := red.copier.CopyDict(rest)
	if err != nil {
		return nil, err
	}
	if _, none := g.group.(pdf.Name); none {
		d["SMask"] = g.group
		return d, nil
	}
	mrest := maps.Clone(g.src.smask)
	delete(mrest, "G")
	sm, err := red.copier.CopyDict(mrest)
	if err != nil {
		return nil, err
	}
	sm["G"] = g.group
	d["SMask"] = sm
	return d, nil
}

// freshGSName returns a resource name for a rewritten extended graphics
// state.
func (wk *walker) freshGSName() pdf.Name {
	if wk.extraGStates == nil {
		wk.extraGStates = map[pdf.Name]*gsPlan{}
	}
	return freshName("RdG", &wk.nextGS, func(name pdf.Name) bool {
		_, used := wk.extraGStates[name]
		return used || wk.rs.gsnames[name]
	})
}

// A patRef names a pattern together with the walker whose resources hold
// it.
type patRef struct {
	name  pdf.Name
	owner *walker
}

// A patTracker follows the patterns which are selected as fill and stroke
// colour.
type patTracker struct {
	fill, stroke patRef
	stack        [][2]patRef
}

// update applies the effect of a graphics-state operator of the stream of wk.
func (t *patTracker) update(wk *walker, name content.OpName, args []pdf.Object) {
	set := func(dst *patRef) {
		*dst = patRef{}
		if len(args) > 0 {
			if n, ok := args[len(args)-1].(pdf.Name); ok {
				*dst = patRef{n, wk}
			}
		}
	}
	switch name {
	case content.OpPushGraphicsState:
		t.stack = append(t.stack, [2]patRef{t.fill, t.stroke})
	case content.OpPopGraphicsState:
		if n := len(t.stack); n > 0 {
			t.fill, t.stroke = t.stack[n-1][0], t.stack[n-1][1]
			t.stack = t.stack[:n-1]
		}
	case content.OpSetFillColorN:
		set(&t.fill)
	case content.OpSetStrokeColorN:
		set(&t.stroke)
	case content.OpSetFillColorSpace, content.OpSetFillColor, content.OpSetFillGray,
		content.OpSetFillRGB, content.OpSetFillCMYK:
		t.fill = patRef{}
	case content.OpSetStrokeColorSpace, content.OpSetStrokeColor, content.OpSetStrokeGray,
		content.OpSetStrokeRGB, content.OpSetStrokeCMYK:
		t.stroke = patRef{}
	}
}

// hitPattern rewrites a tiling pattern which a painting operator uses
// where the painted object meets a region.  The pattern is replaced in the
// resources of the stream where the colour was set.
func (wk *walker) hitPattern(p *plan, pr patRef) error {
	o, name := pr.owner, pr.name
	src := o.patterns[name]
	if src == nil {
		return nil
	}
	res, ok := o.patDone[name]
	if !ok {
		if o.patDone == nil {
			o.patDone = map[pdf.Name]*copyResult{}
			o.patRepl = map[pdf.Name]pdf.Native{}
		}
		o.patDone[name] = &copyResult{}

		var err error
		res, err = o.rewritePattern(src)
		if err != nil {
			return err
		}
		o.patDone[name] = res
		if res.changed {
			o.patRepl[name] = res.ref
		}
	}
	switch {
	case res.lost:
		p.lose()
	case res.changed:
		p.change()
	}
	return nil
}

// analyzeShading rewrites the soft mask of a shading pattern which a scn or
// SCN operator selects.  The pattern is replaced in the resources of the
// stream.  The soft mask can hold content wherever the pattern is selected,
// whether or not the painted objects meet a region.
func (wk *walker) analyzeShading(p *plan, args []pdf.Object) error {
	if len(args) == 0 {
		return nil
	}
	name, _ := args[len(args)-1].(pdf.Name)
	src := wk.rs.shadings[name]
	if src == nil {
		return nil
	}
	res, ok := wk.patDone[name]
	if !ok {
		var err error
		res, err = wk.rewriteShading(src)
		if err != nil {
			return err
		}
		if wk.patDone == nil {
			wk.patDone = map[pdf.Name]*copyResult{}
			wk.patRepl = map[pdf.Name]pdf.Native{}
		}
		wk.patDone[name] = res
		if res.changed {
			wk.patRepl[name] = res.ref
		}
	}
	switch {
	case res.lost:
		p.lose()
	case res.changed:
		p.change()
	}
	return nil
}

// rewriteShading returns the copy of a shading pattern among the resources
// of the stream of wk, in which the content of the soft-mask group under the
// regions is removed.
//
// The graphics state of the pattern is put into effect while the pattern is
// painted (PDF 2.0 section 8.7.4.1), and the shading is painted in pattern
// space.  The soft mask is therefore taken to be established with the
// pattern matrix as the current transformation matrix (section 11.6.5.1).
func (wk *walker) rewriteShading(src *shadingSrc) (*copyResult, error) {
	red, sc := wk.red, wk.sc
	cur := pdf.CursorAt(red.x, nil)
	pm, matrixOK, err := parseMatrix(cur, src.dict["Matrix"])
	if err != nil {
		return nil, err
	}
	group := src.gs.group
	gm, err := matrixOf(cur, group.stm.Dict["Matrix"])
	if err != nil {
		return nil, err
	}
	base := gm.Mul(pm).Mul(wk.base)
	res, err := red.rewriteNested(sc, &nestSpec{
		src:    group,
		key:    copyKey{ref: group.ref, base: base, hullID: wk.hullID, kind: content.Page},
		hulls:  wk.hulls,
		hullID: wk.hullID,
		parent: wk,
		depth:  wk.depth + 1,
	})
	if err != nil {
		return nil, err
	}

	plan := &shadingPlan{src: src, matrixOK: matrixOK}
	switch {
	case res.changed:
		plan.gs = &gsPlan{src: src.gs, group: res.ref, lost: res.lost}
		if res.lost {
			err = red.noteObj(sc, group.stm.Dict)
		}
	case res.fail:
		plan.gs = &gsPlan{src: src.gs, group: pdf.Name("None"), lost: true}
		red.taint(group.ref)
		err = red.scanDropped(sc, group, wk.rs, map[visit]bool{})
	case matrixOK:
		return &copyResult{}, nil
	}
	if err != nil {
		return nil, err
	}
	ref := red.out.Alloc()
	red.pending = append(red.pending, &pendingStream{ref: ref, shading: plan})
	lost := plan.gs != nil && plan.gs.lost
	return &copyResult{changed: true, lost: lost, ref: ref}, nil
}

// rewritePattern returns the copy of a tiling pattern among the resources of
// the stream of wk, in which the content under the regions is removed.
//
// The content of the pattern runs with the graphics state at the start of
// the stream of wk (PDF 2.0 section 8.7.3.1).
func (wk *walker) rewritePattern(src *streamSrc) (*copyResult, error) {
	red, sc := wk.red, wk.sc
	cur := pdf.CursorAt(red.x, nil)
	pm, err := matrixOf(cur, src.stm.Dict["Matrix"])
	if err != nil {
		return nil, err
	}
	paintType, err := pdf.Optional(cur.Integer(src.stm.Dict["PaintType"]))
	if err != nil {
		return nil, err
	}
	kind := content.PatternColored
	if paintType == 2 {
		kind = content.PatternUncolored
	}
	start := wk.startPat
	key := copyKey{ref: src.ref, base: pm.Mul(wk.base), hullID: wk.hullID, kind: kind}
	if wk.inherit != nil {
		key.inh = inheritKeyOf(wk.inherit, start)
	}
	if src.ref != 0 {
		if r := sc.copies[key]; r != nil {
			return r, nil
		}
	}

	hulls, all, err := wk.patternRegions(cur, src, key.base)
	if err != nil {
		return nil, err
	}
	res := &copyResult{fail: true}
	if !all {
		sc.lastHull++
		res, err = red.rewriteNested(sc, &nestSpec{
			src:     src,
			key:     key,
			hulls:   hulls,
			hullID:  sc.lastHull,
			depth:   wk.depth + 1,
			inherit: wk.inherit,
			pat:     start,
		})
		if err != nil {
			return nil, err
		}
	}
	if res.fail {
		ref := red.emptyCopy(src, kind)
		if err := red.scanDropped(sc, src, &resSources{}, map[visit]bool{}); err != nil {
			return nil, err
		}
		res = &copyResult{changed: true, lost: true, ref: ref}
		if src.ref != 0 {
			sc.copies[key] = res
		}
	}
	return res, nil
}

// patternRegions returns the regions for which the content of a tiling
// pattern is removed, in the space of the hulls.  The matrix base maps
// pattern space to the space of the hulls.  The result all is set if the
// whole content is to be removed.
//
// A region meets the copies of the pattern cell in some cells of the
// lattice.  For each of these, the result holds the region moved by the
// offset of the cell, so that it lies over the part of the cell which it
// hides.  The tests are made in the space of the hulls, where [touchTol]
// has its meaning.
func (wk *walker) patternRegions(cur pdf.Cursor, src *streamSrc, base matrix.Matrix) (hulls []poly, all bool, err error) {
	inv, ok := base.Inv()
	if !ok {
		return nil, true, nil
	}
	bbox, err := pdf.Optional(cur.Rectangle(src.stm.Dict["BBox"]))
	if err != nil {
		return nil, false, err
	}
	if bbox == nil {
		return nil, true, nil
	}
	xs, err := pdf.Optional(cur.Number(src.stm.Dict["XStep"]))
	if err != nil {
		return nil, false, err
	}
	ys, err := pdf.Optional(cur.Number(src.stm.Dict["YStep"]))
	if err != nil {
		return nil, false, err
	}
	xs, ys = math.Abs(xs), math.Abs(ys)
	if !(xs > 0 && ys > 0 && isFinite(xs) && isFinite(ys)) {
		return nil, true, nil
	}

	// Where coordinates are too large for the tests to be accurate, or not
	// finite, the whole content is removed.
	q := redactmark.RectQuad(*bbox)
	cell := convexHull(q[:])
	if !moderate(cell) {
		return nil, true, nil
	}
	norm := math.Abs(base[0]) + math.Abs(base[1]) + math.Abs(base[2]) + math.Abs(base[3])
	cellSize := max(math.Abs(bbox.LLx), math.Abs(bbox.LLy), math.Abs(bbox.URx), math.Abs(bbox.URy))

	var res []poly
	for _, h := range wk.hulls {
		hp := h.transform(inv)
		if !moderate(hp) {
			return nil, true, nil
		}
		b := pdf.RectangleFromPoints(hp...)
		i0, i1 := cellRange(bbox.LLx, bbox.URx, xs, b.LLx, b.URx)
		j0, j1 := cellRange(bbox.LLy, bbox.URy, ys, b.LLy, b.URy)
		ni, nj := i1-i0+1, j1-j0+1
		if !(ni*nj <= maxLatticeCandidates) {
			return nil, true, nil
		}
		if ni < 1 || nj < 1 {
			continue
		}
		if err := wk.chargeOp(int64(ni*nj) + 1); err != nil {
			return nil, false, err
		}
		for ki := range int(ni) {
			i := i0 + float64(ki)
			for kj := range int(nj) {
				j := j0 + float64(kj)
				tx, ty := i*xs, j*ys

				// a bound on the rounding error of the computed positions
				reach := roundErr * (norm*(math.Abs(tx)+math.Abs(ty)+cellSize) + math.Abs(base[4]) + math.Abs(base[5]))

				moved := cell.transform(matrix.Translate(tx, ty).Mul(base))
				if !moderate(moved) {
					return nil, true, nil
				}
				if !intersects(moved.grow(reach), h, touchTol) {
					continue
				}
				if len(res) >= maxLatticeCells {
					return nil, true, nil
				}

				// the offset of the cell, in the space of the hulls
				o := base.Apply(vec.Vec2{X: tx, Y: ty}).Sub(base.Apply(vec.Vec2{}))
				back := h.transform(matrix.Translate(-o.X, -o.Y)).grow(reach)
				if !moderate(back) {
					return nil, true, nil
				}
				res = append(res, back)
			}
		}
	}
	return res, false, nil
}

// maxPatternCoord bounds the coordinates for which the lattice of pattern
// cells is tested.
const maxPatternCoord = 1e12

// moderate reports whether the coordinates of the vertices of h are finite
// and at most [maxPatternCoord] in absolute value.  Near 1e12 the resolution
// of float64 is about 1e-4, well below [touchTol], and the margin shrinks
// quickly beyond.  Emptying the pattern past the limit is the conservative
// choice.
func moderate(h poly) bool {
	for _, v := range h {
		if !(math.Abs(v.X) <= maxPatternCoord && math.Abs(v.Y) <= maxPatternCoord) {
			return false
		}
	}
	return true
}

// cellRange returns the range of the integers i for which the interval
// [a+i*step, b+i*step] can meet the interval [lo, hi].  The result is
// empty or huge if the range is not finite.
func cellRange(a, b, step, lo, hi float64) (first, last float64) {
	first = math.Ceil((lo-b)/step - 1e-9)
	last = math.Floor((hi-a)/step + 1e-9)
	if math.IsNaN(first) || math.IsNaN(last) {
		return 0, math.Inf(1)
	}
	return first, last
}
