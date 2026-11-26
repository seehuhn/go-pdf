// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation/fallback"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/internal/limits"
	"seehuhn.de/go/pdf/internal/redactmark"
	"seehuhn.de/go/pdf/pagetree"
)

// Options control how redactions are applied.  A nil *Options is the same as
// the zero value.
type Options struct {
	// Style (optional) supplies the font for overlay text, the same font the
	// fallback preview appearance of a redaction annotation uses.  If this
	// is nil, [fallback.NewStyle] is used.
	Style *fallback.Style

	// Writer (optional) is passed to [pdf.NewWriter], for example to encrypt
	// the output.  With nil the output is unencrypted.  The ID and
	// DocumentMetadata fields are ignored: the document metadata is copied
	// from the input, and the output gets a file identifier as described at
	// [Write].
	Writer *pdf.WriterOptions
}

// ErrTooComplex is returned when applying the redactions would need work or
// output out of proportion to the size of the input file.
var ErrTooComplex = errors.New("redact: document too complex")

// ErrEncrypted is returned when the input is encrypted and Options.Writer is
// nil.  The caller must then decide explicitly whether the output is to be
// encrypted.
var ErrEncrypted = errors.New("redact: input is encrypted but Options.Writer is nil")

// ErrUnsupportedVersion is returned when the input declares a PDF version,
// in the file header or in the catalog's /Version entry, which the library
// cannot write.
var ErrUnsupportedVersion = errors.New("redact: unsupported PDF version")

// UnusableRedactionError is returned when a page has an annotation of
// subtype Redact which cannot be read, or whose region encloses no area.
// Deleting the annotation would silently leave its content in the output,
// so no output is produced.  Use [errors.As] to test for this error.
type UnusableRedactionError struct {
	// Page is the zero-based index of the page.
	Page int
}

// Error implements the error interface.
func (e *UnusableRedactionError) Error() string {
	return fmt.Sprintf("redact: unusable redaction annotation on page %d", e.Page)
}

// Write reads the document from r and writes to w a copy in which the
// redaction annotations of all pages are applied.  The copy has the PDF
// version of the input, or a higher one; an input whose version the library
// cannot write gives [ErrUnsupportedVersion].
//
// If the input has a file identifier, and the output version allows its
// length (PDF 2.0 requires at least 16 bytes), the output keeps the
// permanent identifier and gets a new changing identifier.  Otherwise, an
// encrypted or PDF 2.0 output gets new identifiers, and any other output
// has no identifier.
//
// Nothing is written to w if the error is [ErrUnsupportedVersion],
// [ErrEncrypted] or an [UnusableRedactionError], or if the document has no
// pages.  After any other error, w may hold an incomplete file.
func Write(w io.Writer, r *pdf.Reader, opts *Options) error {
	if opts == nil {
		opts = &Options{}
	}
	meta := r.GetMeta()
	if !meta.Version.IsSupported() {
		return ErrUnsupportedVersion
	}
	if meta.Encryption != nil && opts.Writer == nil {
		return ErrEncrypted
	}

	// 1.2 is the first version with BDC and Flate
	version := max(meta.Version, pdf.V1_2)
	version = max(version, meta.Catalog.MinVersion())
	if meta.Info != nil {
		version = max(version, meta.Info.MinVersion())
	}

	wo := &pdf.WriterOptions{}
	if opts.Writer != nil {
		*wo = *opts.Writer
	}
	wo.DocumentMetadata = meta.Catalog.Metadata
	wo.ID = nil
	if len(meta.ID) > 0 && (version < pdf.V2_0 || len(meta.ID[0]) >= 16) {
		fresh := make([]byte, 16)
		if _, err := rand.Read(fresh); err != nil {
			return err
		}
		wo.ID = [][]byte{meta.ID[0], fresh}
	}

	red := &redactor{
		r:       r,
		x:       pdf.NewExtractor(r),
		version: version,
		style:   opts.Style,

		redirects: map[pdf.Reference]pdf.Reference{},
		rewritten: map[pdf.Reference]*annotRewrite{},
		fonts:     glyphBoxCache{},

		workLimit: limits.StreamBudgetBase + 64*r.FileSize(),
		tainted:   map[pdf.Reference]bool{},
		fixes:     map[pdf.Reference]*fixJob{},
		apEdits:   map[annotID]apEdit{},
		shared:    map[pdf.Reference]pdf.Reference{},
		fontSrc:   map[font.Instance]pdf.Object{},
		nulled:    map[pdf.Reference]bool{},
		dicts:     map[pdf.Reference]pdf.Dict{},
		marks:     map[pdf.Reference]*redactmark.Mark{},
		uses:      map[valueKey]int{},
		copies:    map[valueKey]pdf.Reference{},
	}
	if err := red.checkPages(); err != nil {
		return err
	}

	out, err := pdf.NewWriter(w, version, wo)
	if err != nil {
		return err
	}
	red.out = out
	red.rm = pdf.NewResourceManager(out)
	if err := red.run(); err != nil {
		return err
	}

	if err := red.rm.Close(); err != nil {
		return err
	}
	return out.Close()
}

// redactor holds the state shared while rewriting one document.
//
// The rewrite has two parts.  The first part decides what happens to the
// objects of the input, and writes only objects which do not refer to the
// input: the page content streams, the redacted images and the objects of
// the resource manager.  The second part copies the objects of the input.
// No object is copied before all redirections are known, since a copy made
// earlier could keep the original of an object which is redirected later.
type redactor struct {
	r       *pdf.Reader
	out     *pdf.Writer
	x       *pdf.Extractor
	rm      *pdf.ResourceManager
	version pdf.Version

	// redirects maps source objects to the output objects which replace
	// them.  A later redirection of an object overrides an earlier one.
	redirects map[pdf.Reference]pdf.Reference

	// copier copies objects of the input, with the redirections applied.
	// It is nil until all redirections are known.
	copier *pdf.Copier

	// style supplies the overlay font, and gen is made from it on first use
	style *fallback.Style
	gen   *fallback.Generator

	pages []*pageState

	// regions is set if a redaction region was applied on any page
	regions bool

	// structRoot replaces a direct structure tree root, or is nil
	structRoot pdf.Dict

	// deleted lists the source annotations which are removed from the output
	deleted []pdf.Reference

	// annotRewrites lists the kept annotations which are written again,
	// and rewritten the source references of these
	annotRewrites []*annotRewrite
	rewritten     map[pdf.Reference]*annotRewrite

	// fonts caches the glyph boxes of the fonts of the document
	fonts glyphBoxCache

	// work counts the work done for nested streams and image samples, and
	// workLimit is the largest value which is allowed
	work, workLimit int64

	// tainted lists the source streams which no reference may bring back
	// unchanged, see [redactor.decideRedirects]
	tainted map[pdf.Reference]bool

	// nulled lists the source objects which are redirected to null: the
	// page tree nodes which are not pages, the deleted and the tainted
	// objects.  null is the output object written as null, or 0 before it
	// is needed.
	nulled map[pdf.Reference]bool
	null   pdf.Reference

	// dicts caches the annotation dictionaries read through /Annots and
	// /Popup, and marks the marks of the redaction annotations, by reference
	dicts map[pdf.Reference]pdf.Dict
	marks map[pdf.Reference]*redactmark.Mark

	// uses counts the uses of the direct values of the page dictionaries
	// and their resources, and copies holds the copies of the values which
	// are used more than once; see [redactor.copyPageValue]
	uses   map[valueKey]int
	copies map[valueKey]pdf.Reference

	// fixes maps source streams to their copies with new resources, and
	// fixOrder lists the copies in the order of creation
	fixes    map[pdf.Reference]*fixJob
	fixOrder []*fixJob

	// forms holds the changes to the interactive form
	forms formChanges

	// apEdits holds the changes to the appearance dictionaries of the kept
	// annotations, and apOrder lists the annotations in the order in which
	// their entries were made
	apEdits map[annotID]apEdit
	apOrder []annotID

	// pending lists the redacted copies of nested streams, which are
	// written once all redirections are known
	pending []*pendingStream

	// copiesOf maps source streams to their redacted copies which hold
	// content: forms, soft-mask groups, patterns and images
	copiesOf map[pdf.Reference][]pdf.Reference

	// shared maps the source objects which values embedded by the resource
	// manager refer to, to the output objects which hold their copies, and
	// sharedOrder lists the keys in the order of creation.  The copies are
	// written once all redirections are known.  See [redactor.share].
	shared      map[pdf.Reference]pdf.Reference
	sharedOrder []pdf.Reference

	// fontSrc maps the decoded fonts of the analysed resources to the source
	// objects they were read from
	fontSrc map[font.Instance]pdf.Object

	// structEd holds the changes to the structure tree, or is nil
	structEd *structEditor

	// keys holds the parent tree keys of the redacted copies of streams
	keys parentKeys
}

// pageState tracks one page between the phases.
type pageState struct {
	index          int
	oldRef, newRef pdf.Reference

	// src is the page dictionary with inherited attributes resolved and
	// without /Parent.
	src pdf.Dict

	// repl holds entries of the output page dictionary which replace the
	// copy of the source entry.  The values live in the output file.  A nil
	// value omits the entry.
	repl pdf.Dict

	// lostMCIDs lists the marked-content identifiers in the page content
	// whose content was removed.  They belong to the page's /StructParents
	// key.  The identifiers in form XObjects belong to the keys of the
	// forms, and are listed in lostForms.
	lostMCIDs []int
	lostForms []mcidRecord

	// lostObjs lists the /StructParent keys of the form XObjects which
	// the page content drops or replaces by redacted copies
	lostObjs []int

	// res describes the resources of a page whose content was rewritten, or
	// whose resources refer to a tainted stream, and is nil otherwise
	res *resPlan

	// hulls are the regions of the page, or nil for a page without regions
	hulls []poly

	// annots are the entries of the page's /Annots array, each reference
	// listed once, and marked is set if the page has redaction annotations
	annots []*annotEntry
	marked bool

	// annotEdits is set if the appearance of a direct annotation dictionary
	// in /Annots changes, and aliasContents if the content of a page without
	// regions includes a tainted stream.  /Annots or /Contents is then
	// written again.
	annotEdits, aliasContents bool
}

// run executes the phases of the rewrite.
func (red *redactor) run() error {
	if err := red.setup(); err != nil {
		return err
	}

	// page content comes first: it decides which source objects are
	// replaced by redacted copies
	for _, pg := range red.pages {
		if err := red.redactPage(pg); err != nil {
			return err
		}
	}
	if err := red.settleAnnots(); err != nil {
		return err
	}

	if err := red.prepareForms(); err != nil {
		return err
	}
	if err := red.analyzeAppearances(); err != nil {
		return err
	}
	if err := red.decideRedirects(); err != nil {
		return err
	}
	if err := red.redactStructure(); err != nil {
		return err
	}
	if err := red.planFixes(); err != nil {
		return err
	}
	if err := red.rewriteEditedAnnots(); err != nil {
		return err
	}

	// all redirections are known: objects of the input can be copied
	red.copier = pdf.NewCopier(red.out, red.r)
	for src, dst := range red.redirects {
		red.copier.Redirect(src, dst)
	}
	if err := red.writePending(); err != nil {
		return err
	}
	if err := red.writeShared(); err != nil {
		return err
	}
	if err := red.writeFixes(); err != nil {
		return err
	}
	if err := red.writeAnnotRewrites(); err != nil {
		return err
	}
	if err := red.writeFormRewrites(); err != nil {
		return err
	}
	if red.structEd != nil {
		if err := red.structEd.write(); err != nil {
			return err
		}
	}

	red.countUses()
	tree := pagetree.NewWriter(red.out, red.rm)
	for _, pg := range red.pages {
		dict, err := red.copyPageDict(pg)
		if err != nil {
			return err
		}
		if err := tree.AppendPageDict(pg.newRef, dict); err != nil {
			return err
		}
	}
	pagesRef, err := tree.Close()
	if err != nil {
		return err
	}
	return red.copyCatalog(pagesRef)
}

// checkPages checks, before any output is written, that the overlay font can
// be made, that the document has pages and that all redaction annotations can
// be used.
func (red *redactor) checkPages() error {
	if _, err := red.generator(); err != nil {
		return err
	}
	cur := pdf.CursorAt(red.x, nil)
	it := pagetree.NewIterator(red.r)
	n := 0
	for _, dict := range it.All() {
		entries, _, err := red.readAnnots(cur, n, dict["Annots"])
		if err != nil {
			return err
		}
		if err := red.charge(int64(len(entries)) + 1); err != nil {
			return err
		}
		n++
	}
	if it.Err != nil {
		return it.Err
	}
	if n == 0 {
		// the output needs a page tree with at least one page
		return pdf.Error("document has no pages")
	}
	return nil
}

// setup collects the pages and redirects every page reference to a new one.
// The other nodes of the page tree, including leaves which are not read as
// pages, are redirected to null: a copy would bring back the old page tree
// and the unredacted content of such a leaf.
func (red *redactor) setup() error {
	it := pagetree.NewIterator(red.r)
	for ref, dict := range it.All() {
		pg := &pageState{
			index:  len(red.pages),
			oldRef: ref,
			newRef: red.out.Alloc(),
			src:    dict,
			repl:   pdf.Dict{},
		}
		red.redirect(ref, pg.newRef)
		red.pages = append(red.pages, pg)
	}
	if it.Err != nil {
		return it.Err
	}

	nodes, err := red.treeNodes()
	if err != nil {
		return err
	}
	for _, ref := range nodes {
		if _, isPage := red.redirects[ref]; isPage {
			continue
		}
		null, err := red.nullRef()
		if err != nil {
			return err
		}
		red.nulled[ref] = true
		red.redirect(ref, null)
	}
	return nil
}

// treeNodes returns the dictionaries which the page tree reaches through
// /Kids, the root included, whose /Type is missing, Pages or Page.
func (red *redactor) treeNodes() ([]pdf.Reference, error) {
	cur := pdf.CursorAt(red.x, nil)
	root := red.r.GetMeta().Catalog.Pages
	if root == 0 {
		return nil, nil
	}
	var res []pdf.Reference
	seen := map[pdf.Reference]bool{root: true}
	todo := []pdf.Reference{root}
	for len(todo) > 0 {
		ref := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if err := red.charge(1); err != nil {
			return nil, err
		}
		node, err := pdf.Optional(cur.Dict(ref))
		if err != nil {
			return nil, err
		}
		if node == nil {
			continue
		}
		tp, err := pdf.Optional(cur.Name(node["Type"]))
		if err != nil {
			return nil, err
		}
		if tp != "" && tp != "Pages" && tp != "Page" {
			continue
		}
		res = append(res, ref)
		kids, err := pdf.Optional(cur.Array(node["Kids"]))
		if err != nil {
			return nil, err
		}
		for _, kid := range slices.Backward(kids) {
			if kidRef, ok := kid.(pdf.Reference); ok && !seen[kidRef] {
				seen[kidRef] = true
				todo = append(todo, kidRef)
			}
		}
	}
	return res, nil
}

// nullRef returns the output object which is written as null.
func (red *redactor) nullRef() (pdf.Reference, error) {
	if red.null == 0 {
		ref := red.out.Alloc()
		if err := red.out.Put(ref, nil); err != nil {
			return 0, err
		}
		red.null = ref
	}
	return red.null, nil
}

// redirect records that the copies of the source object src are replaced
// by the object dst of the output.
func (red *redactor) redirect(src, dst pdf.Reference) {
	red.redirects[src] = dst
}

// decideRedirects redirects the deleted and the tainted source objects to a
// reference written as null, so that nothing which refers to them can bring
// back the original.
//
// A source stream is tainted if, in some placement on a page with regions,
// the rewritten content uses a copy of the stream or no copy at all: its
// content lies under a region, or it cannot be decoded completely.  The
// content streams of the pages with regions are tainted, and so are the
// streams whose use is removed without a copy: images which cannot be
// decoded, forms and soft-mask groups which cannot be rewritten, and
// XObjects which are neither images nor forms.  So are the appearance
// streams of the deleted annotations and the replaced appearance streams of
// widgets which get a new appearance, unless a kept annotation shows them.
//
// A use which paints a tainted stream unchanged, for example on a page
// without regions, refers to an alias instead: a copy of the stream under a
// new object number.  This holds for the content of pages, the appearance
// streams of kept annotations, the rewritten streams, and the streams which
// these paint.  A resource dictionary of such a stream which has an entry
// for a tainted stream is written again for the stream: the entries which
// the content uses refer to the aliases, and the other entries for tainted
// streams are left out.  A stream whose copy changes in this way is
// redirected to the copy; see [redactor.planFixes].  All other references to
// a tainted stream, for example from the structure tree or from a resource
// dictionary which nothing paints, give null.
func (red *redactor) decideRedirects() error {
	if err := red.taintAppearances(); err != nil {
		return err
	}
	for _, ref := range red.deleted {
		red.nulled[ref] = true
	}
	for ref := range red.tainted {
		red.nulled[ref] = true
	}
	if len(red.nulled) == 0 {
		return nil
	}
	null, err := red.nullRef()
	if err != nil {
		return err
	}
	for ref := range red.nulled {
		red.redirect(ref, null)
	}
	return nil
}

// charge accounts for n units of work.  The work is the bytes written for
// image samples and nested streams, a fixed amount for each nested stream,
// the bytes read and an amount for each operator of an analysed content
// stream, the page content included, and an amount for each test of a shape
// against the regions.
func (red *redactor) charge(n int64) error {
	red.work += n
	if red.work > red.workLimit {
		return ErrTooComplex
	}
	return nil
}

// redactImageXObject writes a redacted copy of an image XObject.  The error
// is errUndecodable if the image cannot be read.
func (red *redactor) redactImageXObject(src *imageSrc, regions imageRegions) (pdf.Native, error) {
	x := &imgRedaction{regions: regions, charge: red.charge}
	img, err := x.image(src.img)
	if err != nil {
		return nil, err
	}
	if err := red.renumberImage(img); err != nil {
		return nil, err
	}
	if err := red.shareImage(img, src); err != nil {
		return nil, err
	}
	obj, ok := img.(pdf.Embedder)
	if !ok {
		return nil, errUndecodable
	}
	res, err := red.rm.Embed(obj)
	if ref, ok := res.(pdf.Reference); ok && err == nil {
		red.noteCopy(src.ref, ref)
	}
	return res, err
}

// writeAnnotRewrites writes the kept annotations which lost their pop-up or
// whose appearance changes.
func (red *redactor) writeAnnotRewrites() error {
	for _, rw := range red.annotRewrites {
		if rw.taken || red.nulled[rw.oldRef] {
			continue
		}
		dict, err := red.copyAnnotDict(rw.dict, red.apEdits[annotID{ref: rw.oldRef}])
		if err != nil {
			return err
		}
		if err := red.out.Put(rw.newRef, dict); err != nil {
			return err
		}
	}
	return nil
}

// copyPageDict builds the output dictionary of a page.  Entries which
// redactPage replaced come from pg.repl, the resources from pg.res if this
// is set, and all others are copied.
func (red *redactor) copyPageDict(pg *pageState) (pdf.Dict, error) {
	rest := pageRest(pg)
	if err := red.charge(int64(len(rest)) + 1); err != nil {
		return nil, err
	}
	dict := pdf.Dict{}
	for _, key := range rest.SortedKeys() {
		val, err := red.copyPageValue(rest[key])
		if err != nil {
			return nil, err
		}
		dict[key] = val
	}
	var err error
	if pg.annotEdits {
		if dict["Annots"], err = red.copyAnnots(pg); err != nil {
			return nil, err
		}
	}
	if pg.aliasContents {
		if dict["Contents"], err = red.copyContents(pg.src["Contents"]); err != nil {
			return nil, err
		}
	}
	if pg.res != nil {
		res, err := red.buildResources(pg.res)
		if err != nil {
			return nil, err
		}
		dict["Resources"] = res
	}
	for key, val := range pg.repl {
		if val != nil {
			dict[key] = val
		}
	}
	return dict, nil
}

// pageRest returns the entries of the source page dictionary which
// [redactor.copyPageDict] copies.
func pageRest(pg *pageState) pdf.Dict {
	rest := pdf.Dict{}
	for key, val := range pg.src {
		if _, ok := pg.repl[key]; !ok && val != nil {
			rest[key] = val
		}
	}
	if pg.res != nil {
		delete(rest, "Resources")
	}
	if pg.annotEdits {
		delete(rest, "Annots")
	}
	if pg.aliasContents {
		delete(rest, "Contents")
	}
	return rest
}

// valueKey identifies a direct dictionary or array by its storage.
type valueKey struct {
	ptr uintptr
	n   int
}

// keyOf returns the key of a direct dictionary or array with at least one
// entry, and false for all other objects.
func keyOf(obj pdf.Object) (valueKey, bool) {
	switch o := obj.(type) {
	case pdf.Dict:
		return valueKey{reflect.ValueOf(o).Pointer(), len(o)}, len(o) > 0
	case pdf.Array:
		return valueKey{reflect.ValueOf(o).Pointer(), len(o)}, len(o) > 0
	}
	return valueKey{}, false
}

// countUses counts the uses of the direct values which [redactor.copyPageValue]
// may copy: the entries of the page dictionaries, and of the source resources
// of the rewritten pages.
func (red *redactor) countUses() {
	note := func(d pdf.Dict) {
		for _, val := range d {
			if key, ok := keyOf(val); ok {
				red.uses[key]++
			}
		}
	}
	for _, pg := range red.pages {
		note(pageRest(pg))
		if pg.res != nil {
			note(pg.res.src)
		}
	}
}

// copyPageValue copies a value of a page dictionary or of the resources of a
// page.  The page tree iterator places the value of an inherited attribute,
// for example a direct /Resources dictionary of an intermediate node, into
// the dictionary of every page which inherits it.  A direct value which is
// used more than once is copied once, into an indirect object which all uses
// refer to.
func (red *redactor) copyPageValue(obj pdf.Object) (pdf.Native, error) {
	key, ok := keyOf(obj)
	if !ok || red.uses[key] < 2 {
		return red.copier.Copy(obj.AsPDF(red.out.GetOptions()))
	}
	if ref, done := red.copies[key]; done {
		return ref, nil
	}
	ref := red.out.Alloc()
	red.copies[key] = ref
	val, err := red.copier.Copy(obj.AsPDF(red.out.GetOptions()))
	if err != nil {
		return nil, err
	}
	if err := red.out.Put(ref, val); err != nil {
		return nil, err
	}
	return ref, nil
}

// copyCatalog copies the catalog entries and the Info dictionary of the
// input to the output.
func (red *redactor) copyCatalog(pagesRef pdf.Reference) error {
	in := red.r.GetMeta()
	meta := red.out.GetMeta()

	cat := meta.Catalog
	src := in.Catalog

	cat.Pages = pagesRef
	cat.PageLayout = src.PageLayout
	cat.PageMode = src.PageMode
	cat.Lang = src.Lang
	cat.NeedsRendering = src.NeedsRendering

	fields := []struct {
		name string
		src  pdf.Object
		dst  *pdf.Object
	}{
		{"Extensions", src.Extensions, &cat.Extensions},
		{"PageLabels", src.PageLabels, &cat.PageLabels},
		{"Names", src.Names, &cat.Names},
		{"Dests", src.Dests, &cat.Dests},
		{"ViewerPreferences", src.ViewerPreferences, &cat.ViewerPreferences},
		{"OpenAction", src.OpenAction, &cat.OpenAction},
		{"AA", src.AA, &cat.AA},
		{"URI", src.URI, &cat.URI},
		{"AcroForm", src.AcroForm, &cat.AcroForm},
		{"StructTreeRoot", src.StructTreeRoot, &cat.StructTreeRoot},
		{"MarkInfo", src.MarkInfo, &cat.MarkInfo},
		{"SpiderInfo", src.SpiderInfo, &cat.SpiderInfo},
		{"OutputIntents", src.OutputIntents, &cat.OutputIntents},
		{"PieceInfo", src.PieceInfo, &cat.PieceInfo},
		{"OCProperties", src.OCProperties, &cat.OCProperties},
		{"Perms", src.Perms, &cat.Perms},
		{"Legal", src.Legal, &cat.Legal},
		{"Requirements", src.Requirements, &cat.Requirements},
		{"Collection", src.Collection, &cat.Collection},
		{"DSS", src.DSS, &cat.DSS},
		{"AF", src.AF, &cat.AF},
		{"DPartRoot", src.DPartRoot, &cat.DPartRoot},
	}
	for _, f := range fields {
		if ov, ok, err := red.catalogOverride(f.name); err != nil {
			return err
		} else if ok {
			if ov != nil {
				*f.dst = ov
			}
			continue
		}
		n, ok := f.src.(pdf.Native)
		if !ok {
			continue
		}
		cp, err := red.copier.Copy(n)
		if err != nil {
			return err
		}
		*f.dst = cp
	}

	if src.Outlines != 0 {
		cp, err := red.copier.CopyReference(src.Outlines)
		if err != nil {
			return err
		}
		cat.Outlines = cp
	}
	if src.Threads != 0 {
		cp, err := red.copier.CopyReference(src.Threads)
		if err != nil {
			return err
		}
		cat.Threads = cp
	}

	meta.Info = in.Info
	return nil
}
