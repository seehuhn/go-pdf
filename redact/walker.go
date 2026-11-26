// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

import (
	"bytes"
	"errors"
	"io"
	"maps"
	"math"
	"slices"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/page"
)

// A walker rewrites one content stream.  It works in two passes over the
// stream.  The analysis pass ([walker.analyze]) follows the graphics state
// exactly as the reader does and records what happens to each operator in a
// plan.  The emission pass ([emit]) replays the stream and applies the
// plan.  Operators are identified by their index in the stream.
type walker struct {
	// hulls are the regions to remove, in page space
	hulls []poly

	// base maps the default space of the stream to page space
	base matrix.Matrix

	// res are the resources of the stream
	res *content.Resources

	// open opens the stream; it can be called repeatedly
	open func() (io.ReadCloser, error)

	// reader is the reader of the analysis pass, if open returns a
	// [page.ContentReader]
	reader *page.ContentReader

	// fonts caches the glyph boxes of the fonts
	fonts glyphBoxCache

	// extraFonts lists fonts which the rewritten stream uses and which the
	// resources lack, by the names the stream uses for them
	extraFonts map[pdf.Name]font.Instance

	// fontNames maps the fonts of the resources and of extraFonts to the
	// names which the rewritten stream uses for them.  It is built on first
	// use.
	fontNames map[font.Instance]pdf.Name

	// nextFont, nextXObject, nextGS and nextProp are the numbers from which
	// the searches for fresh resource names start
	nextFont, nextXObject, nextGS, nextProp int

	// props are the property lists of the stream resources, by name
	props map[pdf.Name]pdf.Dict

	// extraProps lists the property lists which the rewritten stream uses
	// and which the resources lack, by the names the stream uses for them.
	// These are copies of entries of props, without the text-bearing keys.
	extraProps map[pdf.Name]pdf.Dict

	// strippedNames maps the names in props to the names of the stripped
	// copies in extraProps
	strippedNames map[pdf.Name]pdf.Name

	// keptProps are the names in props which the rewritten stream still uses
	keptProps map[pdf.Name]bool

	// lostMCIDs lists the marked-content identifiers of the spans which lost
	// content, in the order in which the spans end
	lostMCIDs []int

	// images are the image XObjects among the resources, by name
	images map[pdf.Name]*imageSrc

	// xobjectNames are all keys of the XObject resources
	xobjectNames map[pdf.Name]bool

	// extraXObjects lists the redacted images which the rewritten stream
	// uses, by the names the stream uses for them
	extraXObjects map[pdf.Name]pdf.Native

	// keptImages are the names in images which the rewritten stream still
	// uses
	keptImages map[pdf.Name]bool

	// charge accounts for work done
	charge func(n int64) error

	// err is the first error of charge in a function which cannot return it
	err error

	// red rewrites nested streams, and sc holds the state of the rewrites
	// of the page
	red *redactor
	sc  *scope

	// rs holds the source resources of the stream
	rs *resSources

	// depth is the nesting depth of the stream, and hullID identifies the
	// set of regions
	depth, hullID int

	// kind is the type of the content, and inherit, if it is not nil, the
	// graphics state at the start of the stream
	kind    content.Type
	inherit *graphics.State

	// forms, patterns and gstates are the nested streams among the
	// resources, by name
	forms    map[pdf.Name]*streamSrc
	patterns map[pdf.Name]*streamSrc
	gstates  map[pdf.Name]*gsSrc

	// keptForms are the names of the forms which the rewritten stream still
	// paints, and touchedForms the names of the forms which are replaced or
	// dropped.  The same holds for extended graphics states.
	keptForms, touchedForms map[pdf.Name]bool
	keptGS, touchedGS       map[pdf.Name]bool

	// extraGStates lists the extended graphics states which the rewritten
	// stream uses and which the resources lack
	extraGStates map[pdf.Name]*gsPlan
	gsCache      map[gsKey]pdf.Name

	// patRepl lists the tiling patterns which are replaced, by name.
	// The patDone entry for a name records the outcome of the rewrite.
	patRepl map[pdf.Name]pdf.Native
	patDone map[pdf.Name]*copyResult

	// pat tracks the patterns of the fill and stroke colour, and startPat
	// holds the patterns at the start of the stream, with which a tiling
	// pattern among the resources of the stream starts
	pat, startPat patTracker

	// formNames maps the redacted copies of forms to the resource names
	// which the stream uses for them
	formNames map[pdf.Reference]pdf.Name

	// inherited is set if the stream has no resources of its own and uses
	// the resources of the stream which paints it.  The rewritten stream
	// then keeps only the entries which it uses.
	inherited bool

	// others are the XObjects among the resources which are neither images
	// nor forms, by name
	others map[pdf.Name]*streamSrc
}

// useResources sets the fields which hold the resources of the stream.
func (wk *walker) useResources(rs *resSources) {
	wk.props = rs.props
	wk.images = rs.images
	wk.xobjectNames = rs.xnames
	wk.forms = rs.forms
	wk.patterns = rs.patterns
	wk.gstates = rs.gstates
	wk.others = rs.others
}

// A plan is the outcome of the analysis pass.
type plan struct {
	// ops holds an entry for every operator which is changed
	ops map[int]*opPlan

	// lost is set if any content was removed, and changed if the rewritten
	// stream differs from the source in any other way which needs a copy:
	// the source could not be decoded completely, or a nested stream is
	// replaced by a copy which lost no content
	lost, changed bool

	// spans are the currently open marked-content spans, outermost first.
	// The spans spans[:lostDepth] have lost content, and the others have
	// not.
	spans     []*span
	lostDepth int
}

// op returns the entry of the plan for the operator with index idx.  The
// plan p can be nil, for a stream which is kept unchanged.
func (p *plan) op(idx int) *opPlan {
	if p == nil {
		return nil
	}
	return p.ops[idx]
}

// lose records that content was removed.  Every open marked-content span
// loses content.
func (p *plan) lose() {
	p.lost, p.changed = true, true
	for _, s := range p.spans[p.lostDepth:] {
		s.lost = true
	}
	p.lostDepth = len(p.spans)
}

// popSpan removes the innermost open span and returns it.
func (p *plan) popSpan() *span {
	n := len(p.spans) - 1
	s := p.spans[n]
	p.spans = p.spans[:n]
	p.lostDepth = min(p.lostDepth, n)
	return s
}

// change records that the stream needs a copy, although it lost no content.
func (p *plan) change() {
	p.changed = true
}

// A span is a marked-content sequence.
type span struct {
	idx  int // index of the BMC or BDC operator
	args []pdf.Object
	lost bool

	// for BDC spans with a property list
	props pdf.Dict // nil if there is none
	name  pdf.Name // the resource name, or empty for an inline list
	mcid  int      // -1 if there is none

	// badMCID is set if the property list has an MCID entry which is not a
	// valid marked-content identifier
	badMCID bool
}

// textKeys are the keys of a property list which can repeat the content of
// the span.
var textKeys = []pdf.Name{"ActualText", "Alt", "E", "Phoneme"}

// hasTextKeys reports whether a property list has one of the [textKeys].
func hasTextKeys(d pdf.Dict) bool {
	for _, key := range textKeys {
		if _, ok := d[key]; ok {
			return true
		}
	}
	return false
}

// An opPlan describes how an operator is rewritten.
type opPlan struct {
	// drop removes the operator
	drop bool

	// repl replaces the operator, if it is non-nil.  An empty non-nil slice
	// removes the operator.
	repl []content.Operator
}

// A nesting tracks the operator pairs (q and Q, BT and ET, BMC or BDC and
// EMC) which the rewritten stream leaves open.  The analysis pass uses it to
// make the rewritten stream properly nested: an operator which would close a
// pair across another pair is left out, or for Q and ET, the pairs in between
// are closed first.  The [content.State] of the analysis sees only the
// operators which are written, so that it interprets the rewritten stream.
type nesting struct {
	// frames lists the open pairs, outermost first.  The last frame is
	// not gone.
	frames []nestFrame

	// q and mc are the indices in frames of the open q and of the open BMC
	// and BDC operators, and text is one more than the index of the open BT
	// operator, or 0
	q, mc []int
	text  int

	// qInText is set if q and Q may appear in a text object (PDF 2.0)
	qInText bool
}

// A nestFrame is an open operator pair.
type nestFrame struct {
	// closer is the operator which closes the pair
	closer content.OpName

	// dropped is set for a marked-content sequence whose BMC or BDC
	// [content.State.Operands] rejects.  The sequence is left out, and
	// content.State rejects its EMC as well.
	dropped bool

	// gone is set for a dropped sequence whose EMC has been seen
	gone bool
}

// check decides about an operator which [content.State.Operands] accepts.
// The result keep is false if the operator is left out.  For Q, ET and
// EMC, at is the index of the frame which the operator closes; the frames
// after it are closed first.
func (n *nesting) check(name content.OpName) (at int, keep bool) {
	switch name {
	case content.OpBeginCompatibility, content.OpEndCompatibility:
		// the rewritten stream holds no unknown operators
		return 0, false
	case content.OpPushGraphicsState:
		return 0, n.qInText || n.text == 0
	case content.OpTextBegin:
		return 0, n.text == 0
	case content.OpPopGraphicsState:
		// closing the text object early would leave its operators outside
		if len(n.q) == 0 || n.text > n.q[len(n.q)-1] {
			return 0, false
		}
		return n.q[len(n.q)-1], true
	case content.OpTextEnd:
		return n.text - 1, n.text > 0
	case content.OpEndMarkedContent:
		// closing q or BT early would change the graphics state or
		// the text object of the operators which follow
		if len(n.mc) == 0 || n.mc[len(n.mc)-1] != len(n.frames)-1 {
			return 0, false
		}
		return len(n.frames) - 1, true
	}
	return 0, true
}

// push records an operator which opens a pair.
func (n *nesting) push(closer content.OpName, dropped bool) {
	i := len(n.frames)
	n.frames = append(n.frames, nestFrame{closer: closer, dropped: dropped})
	switch closer {
	case content.OpPopGraphicsState:
		n.q = append(n.q, i)
	case content.OpTextEnd:
		n.text = i + 1
	case content.OpEndMarkedContent:
		n.mc = append(n.mc, i)
	}
}

// pop removes the innermost pair and returns it.
func (n *nesting) pop() nestFrame {
	i := len(n.frames) - 1
	f := n.frames[i]
	n.frames = n.frames[:i]
	switch f.closer {
	case content.OpPopGraphicsState:
		n.q = n.q[:len(n.q)-1]
	case content.OpTextEnd:
		n.text = 0
	case content.OpEndMarkedContent:
		n.mc = n.mc[:len(n.mc)-1]
	}
	n.trim()
	return f
}

// reject records an operator which [content.State.Operands] rejects.  A
// rejected BMC or BDC opens a dropped sequence, and a rejected EMC closes
// the innermost marked-content sequence, which is a dropped one.
func (n *nesting) reject(name content.OpName) {
	switch name {
	case content.OpBeginMarkedContent, content.OpBeginMarkedContentWithProperties:
		n.push(content.OpEndMarkedContent, true)
	case content.OpEndMarkedContent:
		if k := len(n.mc); k > 0 {
			n.frames[n.mc[k-1]].gone = true
			n.mc = n.mc[:k-1]
			n.trim()
		}
	}
}

// trim removes the gone frames at the end of n.frames.
func (n *nesting) trim() {
	for k := len(n.frames); k > 0 && n.frames[k-1].gone; k-- {
		n.frames = n.frames[:k-1]
	}
}

// newWalker returns a walker for the stream which open reads.  The
// resources res may be nil.
func newWalker(open func() (io.ReadCloser, error), res *content.Resources, base matrix.Matrix, hulls []poly, fonts glyphBoxCache) *walker {
	return &walker{
		hulls: hulls,
		base:  base,
		res:   res,
		open:  open,
		fonts: fonts,
	}
}

// analyze is the first pass.
func (wk *walker) analyze() (*plan, error) {
	p := &plan{ops: map[int]*opPlan{}}
	state := content.NewState(wk.kind, wk.res)
	if wk.inherit != nil {
		state.GState = wk.inherit.Clone()
	}
	it := content.NewScanner(wk.openCharged).NewIter()

	// the glyphs of the current text object shown in a clipping mode
	var clip clipCount

	// the path under construction
	var pb pathBuf

	// the operator pairs which the rewritten stream leaves open
	nest := &nesting{qInText: wk.red.version >= pdf.V2_0}

	idx := -1
	for name, args := range it.All() {
		idx++
		if err := wk.chargeOp(opCost); err != nil {
			return nil, err
		}
		if skipOperator(name) {
			continue
		}
		if state.Ignored(name, args) {
			p.ops[idx] = &opPlan{drop: true}
			continue
		}

		// The rewritten stream holds only what the analysis interprets:
		// operators which the analysis ignores are dropped, and the others
		// are written with the operands which the analysis uses.
		used, usedArgs, changed, ok := operands(state, name, args)
		if !ok {
			// the state records the marked-content sequences which are
			// left out
			if err := state.ApplyStateChanges(name, args); err != nil {
				return nil, err
			}
			nest.reject(name)
			p.ops[idx] = &opPlan{drop: true}
			continue
		}
		at, keep := nest.check(used)
		if !keep {
			p.ops[idx] = &opPlan{drop: true}
			continue
		}

		// the pairs which Q or ET closes first
		var pre []content.Operator
		switch used {
		case content.OpPopGraphicsState, content.OpTextEnd:
			for len(nest.frames)-1 > at {
				f := nest.pop()
				if err := state.ApplyStateChanges(f.closer, nil); err != nil {
					return nil, err
				}
				if f.dropped {
					continue
				}
				wk.pat.update(wk, f.closer, nil)
				pre = append(pre, content.Operator{Name: f.closer})
				if f.closer == content.OpEndMarkedContent {
					wk.endSpan(p, p.popSpan())
				}
			}
		}

		if state.ApplyStateChanges(used, usedArgs) != nil {
			if pre == nil {
				p.ops[idx] = &opPlan{drop: true}
			} else {
				p.ops[idx] = &opPlan{repl: pre}
			}
			continue
		}
		switch used {
		case content.OpPushGraphicsState:
			nest.push(content.OpPopGraphicsState, false)
		case content.OpTextBegin:
			nest.push(content.OpTextEnd, false)
		case content.OpBeginMarkedContent, content.OpBeginMarkedContentWithProperties:
			nest.push(content.OpEndMarkedContent, false)
		case content.OpPopGraphicsState, content.OpTextEnd, content.OpEndMarkedContent:
			nest.pop()
		}
		if v, ok := storedOperand(used, usedArgs, state.GState); ok {
			usedArgs, changed = []pdf.Object{v}, true
		}
		if changed {
			p.ops[idx] = &opPlan{repl: []content.Operator{{Name: used, Args: slices.Clone(usedArgs)}}}
		}
		name, args = used, usedArgs
		wk.pat.update(wk, name, args)

		if isPathOp(name) {
			pb.add(idx, name, args)
		}
		switch name {
		case content.OpStroke, content.OpCloseAndStroke, content.OpFill,
			content.OpFillCompat, content.OpFillEvenOdd, content.OpFillAndStroke,
			content.OpFillAndStrokeEvenOdd, content.OpCloseFillAndStroke,
			content.OpCloseFillAndStrokeEvenOdd, content.OpEndPath:
			meets, err := wk.pathMeets(&pb, state.GState, name)
			if err != nil {
				return nil, err
			}
			if err := wk.paint(p, &pb, state.GState, idx, name); err != nil {
				return nil, err
			}
			if meets {
				// the path is hidden in part
				p.lose()
			}
		case content.OpSetExtGState:
			if err := wk.analyzeGS(p, state.GState, idx, args); err != nil {
				return nil, err
			}
		case content.OpSetFillColorN, content.OpSetStrokeColorN:
			if err := wk.analyzeShading(p, args); err != nil {
				return nil, err
			}
		case content.OpShading:
			// TODO(voss): remove mesh vertices inside regions
			// The shading fills the clip, which can reach the regions.
			if len(wk.hulls) > 0 {
				p.lose()
			}
		case content.OpXObject:
			if err := wk.analyzeDo(p, state.GState, idx, args); err != nil {
				return nil, err
			}
		case content.OpInlineImage:
			if err := wk.analyzeInline(p, state, idx, args); err != nil {
				return nil, err
			}
		case content.OpMarkedContentPointWithProperties:
			if name, ok := args[1].(pdf.Name); ok {
				wk.keep(name)
			}
		case content.OpBeginMarkedContent:
			p.spans = append(p.spans, &span{idx: idx, mcid: -1})
		case content.OpBeginMarkedContentWithProperties:
			s, err := wk.newSpan(idx, args)
			if err != nil {
				return nil, err
			}
			p.spans = append(p.spans, s)
		case content.OpEndMarkedContent:
			wk.endSpan(p, p.popSpan())
		case content.OpTextBegin:
			clip = clipCount{}
		case content.OpTextShow, content.OpTextShowArray,
			content.OpTextShowMoveNextLine, content.OpTextShowMoveNextLineSetSpacing:
			op, lost := wk.analyzeText(state.GState, name, args, &clip)
			if wk.err != nil {
				return nil, wk.err
			}
			if op != nil {
				p.ops[idx] = op
			}
			if lost {
				p.lose()
			}
		case content.OpTextEnd:
			if clip.removed > 0 && clip.kept == 0 {
				// the clip must be empty, and not absent
				p.ops[idx] = &opPlan{repl: []content.Operator{
					{Name: content.OpTextEnd},
					{Name: content.OpRectangle, Args: []pdf.Object{
						pdf.Integer(0), pdf.Integer(0), pdf.Integer(0), pdf.Integer(0)}},
					{Name: content.OpClipNonZero},
					{Name: content.OpEndPath},
				}}
			}
		}

		if pre != nil {
			op := p.ops[idx]
			if op == nil {
				op = &opPlan{repl: []content.Operator{{Name: name}}}
				p.ops[idx] = op
			}
			op.repl = append(pre, op.repl...)
		}
	}
	if wk.err != nil {
		return nil, wk.err
	}
	if err := it.Err(); err != nil {
		return nil, err
	}
	if wk.reader != nil && wk.reader.Incomplete() {
		// the copy holds only the part which was analysed
		p.change()
	}

	// spans which the stream leaves open
	for _, v := range slices.Backward(p.spans) {
		wk.endSpan(p, v)
	}
	p.spans = nil
	return p, nil
}

// openCharged opens the stream with wk.open, and charges one unit of work
// for every byte read.  A failed charge is kept in wk.err, and ends the
// reading.
func (wk *walker) openCharged() (io.ReadCloser, error) {
	r, err := wk.open()
	if cr, ok := r.(*page.ContentReader); ok {
		wk.reader = cr
	}
	if err != nil {
		return r, err
	}
	return &chargedReader{ReadCloser: r, wk: wk}, nil
}

// chargedReader charges the work of reading a content stream, see
// [walker.openCharged].
type chargedReader struct {
	io.ReadCloser
	wk *walker
}

func (r *chargedReader) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if n > 0 {
		if e := r.wk.chargeOp(int64(n)); e != nil {
			r.wk.err = e
			return n, e
		}
	}
	return n, err
}

// analyzeDo decides what happens to the XObject which a Do operator paints.
// Where there are regions, an XObject which is neither an image nor a form
// is removed, since its data can hold text which cannot be found.
func (wk *walker) analyzeDo(p *plan, gs *graphics.State, idx int, args []pdf.Object) error {
	name := args[0].(pdf.Name)
	if f := wk.forms[name]; f != nil {
		return wk.analyzeFormDo(p, gs, idx, name, f)
	}
	if o := wk.others[name]; o != nil {
		if len(wk.hulls) == 0 {
			return nil
		}
		p.ops[idx] = &opPlan{drop: true}
		p.lose()
		return wk.lostXObject(o.ref, o.stm.Dict)
	}
	src := wk.images[name]
	if src == nil {
		return nil
	}

	if err := wk.chargeTests(1); err != nil {
		return err
	}
	how, hulls := placeImage(gs.CTM.Mul(wk.base), wk.hulls)
	if how != placeKeep && wk.pat.fill.name != "" && color.IsPattern(gs.FillColor.ColorSpace()) {
		// an image mask is painted in the fill colour
		if err := wk.hitPattern(p, wk.pat.fill); err != nil {
			return err
		}
	}
	if how == placeRedact {
		var ref pdf.Native
		err := errUndecodable
		if src.img != nil {
			ref, err = wk.red.redactImageXObject(src, hulls)
		}
		switch {
		case err == nil:
			newName := wk.freshXObjectName()
			wk.extraXObjects[newName] = ref
			p.ops[idx] = &opPlan{repl: []content.Operator{
				{Name: content.OpXObject, Args: []pdf.Object{newName}},
			}}
			p.lose()
			return wk.lostXObject(src.ref, src.dict)
		case errors.Is(err, errUndecodable):
			how = placeDrop
		default:
			return err
		}
	}

	switch how {
	case placeKeep:
		if wk.keptImages == nil {
			wk.keptImages = map[pdf.Name]bool{}
		}
		wk.keptImages[name] = true
	case placeDrop:
		p.ops[idx] = &opPlan{drop: true}
		p.lose()
		return wk.lostXObject(src.ref, src.dict)
	}
	return nil
}

// lostXObject records that an image or other XObject which is not a form,
// with reference ref and dictionary dict, is removed or replaced by a
// redacted copy.
func (wk *walker) lostXObject(ref pdf.Reference, dict pdf.Dict) error {
	wk.red.taint(ref)
	return wk.red.noteObj(wk.sc, dict)
}

// analyzeInline decides what happens to an inline image.
func (wk *walker) analyzeInline(p *plan, state *content.State, idx int, args []pdf.Object) error {
	if err := wk.chargeTests(1); err != nil {
		return err
	}
	how, hulls := placeImage(state.GState.CTM.Mul(wk.base), wk.hulls)
	if how == placeKeep {
		return nil
	}
	if wk.pat.fill.name != "" && color.IsPattern(state.GState.FillColor.ColorSpace()) {
		if err := wk.hitPattern(p, wk.pat.fill); err != nil {
			return err
		}
	}
	if how == placeRedact {
		op, err := redactInline(content.Operator{Name: content.OpInlineImage, Args: args}, wk.res, hulls, wk.charge)
		if err == nil {
			p.ops[idx] = &opPlan{repl: []content.Operator{op}}
			p.lose()
			return nil
		}
		if !errors.Is(err, errUndecodable) {
			return err
		}
		// an image which cannot be decoded is removed
	}
	p.ops[idx] = &opPlan{drop: true}
	p.lose()
	return nil
}

// freshXObjectName returns a resource name for a redacted image or form
// which is not used by the XObject resources.
func (wk *walker) freshXObjectName() pdf.Name {
	if wk.extraXObjects == nil {
		wk.extraXObjects = map[pdf.Name]pdf.Native{}
	}
	return freshName("RdI", &wk.nextXObject, func(name pdf.Name) bool {
		_, used := wk.extraXObjects[name]
		return used || wk.xobjectNames[name]
	})
}

// newSpan returns the span which a BDC operator opens.  The scanner reuses
// the operand slice, so the span keeps a copy.
func (wk *walker) newSpan(idx int, args []pdf.Object) (*span, error) {
	s := &span{idx: idx, args: slices.Clone(args), mcid: -1}
	if len(args) < 2 {
		return s, nil
	}
	switch x := args[1].(type) {
	case pdf.Dict:
		s.props = x
	case pdf.Name:
		s.name = x
		s.props = wk.props[x]
	}
	obj, present := s.props["MCID"]
	if !present {
		return s, nil
	}
	val, err := pdf.Optional(pdf.CursorAt(wk.red.x, nil).Resolve(obj))
	if err != nil {
		return nil, err
	}
	if m, ok := val.(pdf.Integer); ok && m >= 0 && int64(m) <= math.MaxInt32 {
		s.mcid = int(m)
	} else {
		s.badMCID = true
	}
	return s, nil
}

// endSpan finishes the analysis of a span.  A span which lost content
// is recorded, and its property list loses the keys which can repeat
// the removed content.
func (wk *walker) endSpan(p *plan, s *span) {
	if !s.lost || !hasTextKeys(s.props) {
		wk.keep(s.name)
	}
	if !s.lost {
		return
	}
	if s.mcid >= 0 {
		wk.lostMCIDs = append(wk.lostMCIDs, s.mcid)
	}
	if s.badMCID {
		// the structure element of the content cannot be found
		wk.sc.lost = append(wk.sc.lost, mcidRecord{key: unknownKey, mcid: -1})
	}
	if !hasTextKeys(s.props) {
		return
	}
	stripped := maps.Clone(s.props)
	for _, key := range textKeys {
		delete(stripped, key)
	}
	args := slices.Clone(s.args)
	if s.name == "" {
		args[1] = stripped
	} else {
		args[1] = wk.strippedName(s.name, stripped)
	}
	p.ops[s.idx] = &opPlan{repl: []content.Operator{
		{Name: content.OpBeginMarkedContentWithProperties, Args: args},
	}}
}

// keep records that the rewritten stream uses a property list resource.
func (wk *walker) keep(name pdf.Name) {
	if name == "" {
		return
	}
	if wk.keptProps == nil {
		wk.keptProps = map[pdf.Name]bool{}
	}
	wk.keptProps[name] = true
}

// strippedProps returns the names of the property list resources which the
// rewritten stream replaces and which the rewritten content does not use.
func (wk *walker) strippedProps() []pdf.Name {
	var res []pdf.Name
	for name := range wk.strippedNames {
		if !wk.keptProps[name] {
			res = append(res, name)
		}
	}
	return res
}

// strippedName returns the name under which the rewritten stream's resources
// hold the stripped copy of the property list name.
func (wk *walker) strippedName(name pdf.Name, stripped pdf.Dict) pdf.Name {
	if n, ok := wk.strippedNames[name]; ok {
		return n
	}
	n := freshName("RdP", &wk.nextProp, func(name pdf.Name) bool {
		_, used := wk.props[name]
		_, extra := wk.extraProps[name]
		return used || extra
	})
	if wk.extraProps == nil {
		wk.extraProps = map[pdf.Name]pdf.Dict{}
		wk.strippedNames = map[pdf.Name]pdf.Name{}
	}
	wk.extraProps[n] = stripped
	wk.strippedNames[name] = n
	return n
}

// skipOperator reports whether an operator is dropped from rewritten streams
// without further analysis: comments, and unknown operators which may carry
// strings.
func skipOperator(name content.OpName) bool {
	return name == content.OpRawContent || !content.Known(name)
}

// operands returns the operator and the operands which the analysis
// interprets in place of an operator of the stream, as given by
// [content.State.Operands].  The result changed tells whether these differ
// from the operator, and ok is false if the analysis ignores the operator.
func operands(state *content.State, name content.OpName, args []pdf.Object) (_ content.OpName, _ []pdf.Object, changed, ok bool) {
	used, usedArgs, same, ok := state.Operands(name, args)
	if !ok {
		return name, nil, false, false
	}
	changed = !same || len(usedArgs) != len(args)
	return used, usedArgs, changed, true
}

// storedOperand returns the operand which sets a graphics state parameter to
// the value which the state gs holds after the operator.  This covers the
// operators whose operand the state clamps or rounds.  The result is false
// if the operand gives the stored value already.
func storedOperand(name content.OpName, args []pdf.Object, gs *graphics.State) (pdf.Object, bool) {
	integer := func(v int64) (pdf.Object, bool) {
		if x, ok := args[0].(pdf.Integer); ok && int64(x) == v {
			return nil, false
		}
		return pdf.Integer(v), true
	}
	number := func(v float64) (pdf.Object, bool) {
		if x, _ := getNumber(args[0]); x == v {
			return nil, false
		}
		return pdf.Number(pdf.Round(v, 4)), true
	}
	switch name {
	case content.OpSetLineCap:
		return integer(int64(gs.LineCap))
	case content.OpSetLineJoin:
		return integer(int64(gs.LineJoin))
	case content.OpTextSetRenderingMode:
		return integer(int64(gs.TextRenderingMode))
	case content.OpSetLineWidth:
		return number(gs.LineWidth)
	case content.OpSetMiterLimit:
		return number(gs.MiterLimit)
	case content.OpSetFlatnessTolerance:
		return number(gs.FlatnessTolerance)
	}
	return nil, false
}

// pathMeets reports whether a subpath of the path which a painting operator
// paints meets a region.
func (wk *walker) pathMeets(pb *pathBuf, gs *graphics.State, name content.OpName) (bool, error) {
	pp, _ := content.PathPainting(name)
	if len(pb.subs) == 0 || !pp.Fill && !pp.Stroke {
		return false, nil
	}
	// the tests are linear in the number of points
	npts := 0
	for _, s := range pb.subs {
		npts += len(s.pts)
	}
	if err := wk.chargeTests(npts); err != nil {
		return false, err
	}
	userToPage := gs.CTM.Mul(wk.base)
	reach := 0.0
	if pp.Stroke {
		reach = strokeReach(gs.LineWidth, gs.MiterLimit, gs.LineJoin, gs.LineCap, userToPage)
	}
	for _, s := range pb.subs {
		hull := convexHull(transformPoints(s.pts, userToPage))
		if reach > 0 {
			hull = hull.grow(reach)
		}
		for _, h := range wk.hulls {
			if intersects(hull, h, touchTol) {
				return true, nil
			}
		}
	}
	return false, nil
}

// emit is the second pass.  It reads the stream with open, which must give
// the stream the plan p was made for, and writes the rewritten stream to w.
// The result is the list of operators which end the path object and close
// the graphics states, text objects and marked-content sequences which the
// new stream leaves open, innermost first.
//
// The analysis plans a properly nested stream.  Independently of the plan,
// a "Q", "ET" or "EMC" which does not close the innermost open pair of the
// new stream is dropped, so that the content cannot undo the clip which
// precedes it.
func emit(w io.Writer, open func() (io.ReadCloser, error), p *plan, opt pdf.OutputOptions) ([]content.Operator, error) {
	it := content.NewScanner(open).NewIter()
	var buf bytes.Buffer

	// closers lists the closing operators for what the written operators
	// leave open, innermost last
	var closers []content.OpName

	// inPath is set if the written operators leave a path object open
	inPath := false

	// put formats and writes an operator
	put := func(op content.Operator) error {
		closes := false
		switch op.Name {
		case content.OpPopGraphicsState, content.OpTextEnd, content.OpEndMarkedContent:
			n := len(closers)
			if n == 0 || closers[n-1] != op.Name {
				return nil
			}
			closes = true
		}
		buf.Reset()
		if err := op.Format(&buf, opt); err != nil {
			return err
		}
		buf.WriteByte('\n')
		if _, err := w.Write(buf.Bytes()); err != nil {
			return err
		}
		switch op.Name {
		case content.OpPushGraphicsState:
			closers = append(closers, content.OpPopGraphicsState)
		case content.OpTextBegin:
			closers = append(closers, content.OpTextEnd)
		case content.OpBeginMarkedContent, content.OpBeginMarkedContentWithProperties:
			closers = append(closers, content.OpEndMarkedContent)
		}
		if closes {
			closers = closers[:len(closers)-1]
		}
		if _, paints := content.PathPainting(op.Name); paints {
			inPath = false
		} else if isPathOp(op.Name) {
			inPath = true
		}
		return nil
	}

	idx := -1
	for name, args := range it.All() {
		idx++
		if skipOperator(name) {
			continue
		}
		op := p.ops[idx]
		if op != nil && op.drop {
			continue
		}
		if op != nil && op.repl != nil {
			for _, r := range op.repl {
				if err := put(r); err != nil {
					return nil, err
				}
			}
			continue
		}
		if err := put(content.Operator{Name: name, Args: args}); err != nil {
			return nil, err
		}
	}
	if err := it.Err(); err != nil {
		return nil, err
	}

	var tail []content.Operator
	if inPath {
		tail = append(tail, content.Operator{Name: content.OpEndPath})
	}
	for _, name := range slices.Backward(closers) {
		tail = append(tail, content.Operator{Name: name})
	}
	return tail, nil
}

// isPathOp reports whether an operator constructs a path or sets the clip.
func isPathOp(name content.OpName) bool {
	switch name {
	case content.OpMoveTo, content.OpLineTo, content.OpCurveTo,
		content.OpCurveToV, content.OpCurveToY, content.OpClosePath,
		content.OpRectangle, content.OpClipNonZero, content.OpClipEvenOdd:
		return true
	}
	return false
}

// clipCount counts the glyphs of a text object which are shown in a clipping
// text rendering mode.
type clipCount struct {
	removed, kept int
}

// A showItem is one element of the operand of a text-showing operator: a
// code, or a number of a TJ array.
type showItem struct {
	// for a code
	bytes   pdf.String
	removed bool
	adv     float64 // advance in unscaled text space units, without Th

	// for a number
	isNum bool
	num   float64
	orig  pdf.Object
}

// analyzeText decides how a text-showing operator is rewritten.  The state
// must already include the effect of the operator on the text line matrix,
// and the text matrix is advanced past the text.  The result is nil if the
// operator is kept, and lost reports whether text was removed.  The glyphs
// shown in a clipping mode are counted in clip.
func (wk *walker) analyzeText(gs *graphics.State, name content.OpName, args []pdf.Object, clip *clipCount) (op *opPlan, lost bool) {
	var strs []pdf.Object // strings and numbers
	switch name {
	case content.OpTextShow, content.OpTextShowMoveNextLine:
		strs = args[:1]
	case content.OpTextShowMoveNextLineSetSpacing:
		strs = args[2:3]
	case content.OpTextShowArray:
		strs = args[0].(pdf.Array)
	}

	prefix := textPrefix(name, args)
	f := gs.TextFont
	if f == nil {
		// paints nothing and does not advance
		return &opPlan{repl: prefix}, false
	}

	vertical := f.WritingMode() == font.Vertical
	var items []showItem
	needed := false
	for _, obj := range strs {
		if s, ok := obj.(pdf.String); ok {
			for rest := s; len(rest) > 0 && wk.err == nil; {
				_, k, _ := f.Codec().Decode(rest)
				var code font.Code
				for c := range f.Codes(rest[:k]) {
					code = c
					break
				}
				it := showItem{bytes: rest[:k]}
				it.removed = wk.hitsRegion(gs, f, &code)
				needed = needed || it.removed
				if mode := gs.TextRenderingMode; mode >= graphics.TextRenderingModeFillClip &&
					mode <= graphics.TextRenderingModeClip {
					if it.removed {
						clip.removed++
					} else {
						clip.kept++
					}
				}
				if vertical {
					it.adv = code.VerticalAdvance*gs.TextFontSize + gs.TextCharacterSpacing
				} else {
					it.adv = code.Width*gs.TextFontSize + gs.TextCharacterSpacing
				}
				if code.UseWordSpacing {
					it.adv += gs.TextWordSpacing
				}
				items = append(items, it)
				gs.AdvanceTextMatrix(&code)
				rest = rest[k:]
			}
			continue
		}
		if v, ok := getNumber(obj); ok {
			items = append(items, showItem{isNum: true, num: v, orig: obj})
			gs.ApplyTextKern(v)
		}
	}
	if !needed {
		return nil, false
	}

	return &opPlan{repl: append(prefix, wk.keptOps(items, gs)...)}, true
}

// textPrefix returns the operators which, in place of a text-showing
// operator, have the same effect on the text state apart from the text shown.
// The result is not nil.
func textPrefix(name content.OpName, args []pdf.Object) []content.Operator {
	res := []content.Operator{}
	switch name {
	case content.OpTextShowMoveNextLine:
		res = append(res, content.Operator{Name: content.OpTextNextLine})
	case content.OpTextShowMoveNextLineSetSpacing:
		res = append(res,
			content.Operator{Name: content.OpTextSetWordSpacing, Args: []pdf.Object{args[0]}},
			content.Operator{Name: content.OpTextSetCharacterSpacing, Args: []pdf.Object{args[1]}},
			content.Operator{Name: content.OpTextNextLine},
		)
	}
	return res
}

// fontName returns a name under which the resources of the rewritten stream
// hold the font f.  A font which the resources lack is recorded in
// [walker.extraFonts].
func (wk *walker) fontName(f font.Instance) pdf.Name {
	if wk.fontNames == nil {
		// the first name in sorted order, so that the choice is reproducible
		wk.fontNames = map[font.Instance]pdf.Name{}
		if wk.res != nil {
			for _, name := range slices.Sorted(maps.Keys(wk.res.Font)) {
				g := wk.res.Font[name]
				if _, seen := wk.fontNames[g]; g != nil && !seen {
					wk.fontNames[g] = name
				}
			}
		}
	}
	if name, ok := wk.fontNames[f]; ok {
		return name
	}
	name := freshName("RdF", &wk.nextFont, func(name pdf.Name) bool {
		_, used := wk.extraFonts[name]
		if !used && wk.res != nil {
			_, used = wk.res.Font[name]
		}
		return used
	})
	if wk.extraFonts == nil {
		wk.extraFonts = map[pdf.Name]font.Instance{}
	}
	wk.extraFonts[name] = f
	wk.fontNames[f] = name
	return name
}

// keptOps returns the operators which show the kept codes at their original
// positions.  Each run of removed codes, together with the numbers in and
// next to it, is replaced by a gap: a single TJ number which moves the text
// position as far as the run.  Where the font size gives no such number
// within the precision written, for example at font size zero, the number
// is shown at another font size, and the font size is restored afterwards.
func (wk *walker) keptOps(items []showItem, gs *graphics.State) []content.Operator {
	// Both a TJ number n and the advance a of a code move the text position
	// by an amount proportional to Th.  The factor cancels, and a run with
	// total advance sum is replaced by the number -1000 sum / Tfs.  With
	// Th = 0 nothing moves and no number is needed.
	size := gs.TextFontSize
	noMove := gs.TextHorizontalScaling == 0 && gs.TextFont.WritingMode() == font.Horizontal

	var ops []content.Operator
	var arr pdf.Array
	var cur pdf.String
	var nums []pdf.Object
	sum := 0.0
	removed := false

	flushStr := func() {
		if len(cur) > 0 {
			arr = append(arr, cur)
			cur = nil
		}
	}
	flushArr := func() {
		flushStr()
		if len(arr) > 0 {
			ops = append(ops, content.Operator{Name: content.OpTextShowArray, Args: []pdf.Object{arr}})
			arr = nil
		}
	}
	flushRun := func() {
		switch {
		case removed && !noMove:
			s, n, ok := gapNumber(sum, size)
			switch {
			case !ok || n == 0:
				// A gap which overflows cannot be written.  The glyphs after
				// it have positions which are not finite, and are removed.
			case s == size:
				flushStr()
				arr = append(arr, pdf.Number(n))
			default:
				flushArr()
				name := wk.fontName(gs.TextFont)
				ops = append(ops,
					content.Operator{Name: content.OpTextSetFont, Args: []pdf.Object{name, pdf.Number(s)}},
					content.Operator{Name: content.OpTextShowArray, Args: []pdf.Object{pdf.Array{pdf.Number(n)}}},
					content.Operator{Name: content.OpTextSetFont, Args: []pdf.Object{name, pdf.Number(size)}},
				)
			}
		case !removed && len(nums) > 0:
			flushStr()
			arr = append(arr, nums...)
		}
		nums, sum, removed = nil, 0, false
	}

	for _, it := range items {
		switch {
		case it.isNum:
			nums = append(nums, it.orig)
			sum -= it.num * size / 1000
		case it.removed:
			removed = true
			sum += it.adv
		default:
			flushRun()
			cur = append(cur, it.bytes...)
		}
	}
	flushRun()
	flushArr()
	return ops
}

// maxGapNumber is the largest magnitude of a TJ number which replaces
// removed text at the current font size or at size 1, and maxGapSize the
// largest magnitude of such a font size.  Beyond these limits, [gapNumber]
// shows a number of magnitude at most 1000 at a power of ten.  This bounds
// the number of digits written, and the error of rounding the number.
const (
	maxGapNumber = 1e6
	maxGapSize   = 1e3
)

// gapNumber returns a font size s and a TJ number n, rounded as written,
// such that n shown at font size s moves the text position by the
// advance sum, in unscaled text space units.  The size s is the font size
// size where possible, and otherwise a power of ten.  The result ok is false
// if sum is not finite.
func gapNumber(sum, size float64) (s, n float64, ok bool) {
	if !isFinite(sum) {
		return 0, 0, false
	}
	for _, try := range []float64{size, 1} {
		x := -1000 * sum / try
		if isFinite(x) && math.Abs(x) <= maxGapNumber && math.Abs(try) <= maxGapSize {
			return try, pdf.Round(x, 4), true
		}
	}
	// the smallest power of ten not below |sum|, which gives |n| <= 1000
	k := int(math.Ceil(math.Log10(math.Abs(sum))))
	s = math.Pow10(min(max(k, -300), 300))
	return s, pdf.Round(-1000*sum/s, 4), true
}

// chargeOp accounts for n units of work.  Once a charge has failed, the
// error is kept in wk.err, and every later call returns it.
func (wk *walker) chargeOp(n int64) error {
	if wk.err != nil {
		return wk.err
	}
	return wk.charge(n)
}

// chargeTests accounts for testing n shapes against all regions.
func (wk *walker) chargeTests(n int) error {
	return wk.chargeOp(int64(n) * int64(len(wk.hulls)))
}

// hitsRegion reports whether the box of a glyph meets a region.  The
// graphics state gs must hold the text position of the glyph.  If the work
// budget is used up, the error is kept in wk.err.
func (wk *walker) hitsRegion(gs *graphics.State, f font.Instance, code *font.Code) bool {
	if wk.err = wk.chargeTests(1); wk.err != nil {
		return false
	}
	gb := wk.fonts.get(f)
	pts, bounded, err := gb.points(code, wk.chargeOp)
	if err != nil {
		wk.err = err
		return false
	}
	if !bounded {
		return len(wk.hulls) > 0
	}

	userToPage := gs.CTM.Mul(wk.base)
	corners := transformPoints(pts, gs.GlyphMatrix(code).Mul(userToPage))
	if !poly(corners).isFinite() {
		// The position cannot be represented, for example after an
		// advance which overflows.  Such a glyph is removed, so that the
		// rewritten stream need not reproduce its position.
		return true
	}
	box := convexHull(corners)
	switch gs.TextRenderingMode {
	case graphics.TextRenderingModeStroke, graphics.TextRenderingModeFillStroke,
		graphics.TextRenderingModeStrokeClip, graphics.TextRenderingModeFillStrokeClip:
		box = box.grow(strokeReach(gs.LineWidth, gs.MiterLimit, gs.LineJoin, gs.LineCap, userToPage))
	}
	for _, h := range wk.hulls {
		if intersects(box, h, touchTol) {
			return true
		}
	}
	return false
}

// fontDict returns the font dictionary of f, or nil.
func fontDict(f font.Instance) dict.Dict {
	if fd, ok := f.(interface{ GetDict() dict.Dict }); ok {
		return fd.GetDict()
	}
	return nil
}

// getNumber returns the value of a numeric operand.
func getNumber(obj pdf.Object) (float64, bool) {
	switch x := obj.(type) {
	case pdf.Integer:
		return float64(x), true
	case pdf.Real:
		return float64(x), true
	case pdf.Number:
		return float64(x), true
	}
	return 0, false
}

// dropNames returns the names of the form resources and extended graphics
// states which are replaced and not used any more.
func dropNames(touched, kept map[pdf.Name]bool) []pdf.Name {
	var res []pdf.Name
	for name := range touched {
		if !kept[name] {
			res = append(res, name)
		}
	}
	slices.Sort(res)
	return res
}
