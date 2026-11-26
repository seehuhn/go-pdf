// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package redact applies redaction annotations (PDF 2.0 section 12.5.6.23).
//
// [Write] copies a document and, on every page, removes the content
// that lies under the page's redaction annotations, draws the mark each
// annotation describes, and deletes the annotations.  Text and image data in
// the regions are removed from page content, form XObjects, tiling patterns,
// soft-mask groups and the appearance streams of the kept annotations.
// Characters of the overlay text which the font of [Options.Style] lacks are
// shown by its .notdef glyph.
//
// A redaction annotation which cannot be read, or whose region encloses no
// area, makes [Write] fail with an [UnusableRedactionError] before anything
// is written.
//
// # Annotations
//
// An appearance stream counts as a form placed at the annotation rectangle.
// On pages with regions, an appearance stream which cannot be placed, since
// the annotation has no rectangle or the bounding box encloses no area, is
// removed.
//
// Annotations that can be shown over a region are deleted.  The values of
// form fields whose widgets are deleted are removed, and the export values of
// the deleted widgets of a check box or radio button field are removed too.
// Deleting an annotation removes it from every page which lists it; its
// pop-up and, transitively, the replies to it (/IRT) are deleted with it.  On
// pages with redaction annotations, /Annots entries which are not
// dictionaries are removed as well.
//
// On a rotated page, an annotation with the NoRotate flag, which includes
// every text annotation, is also tested, and its appearance analysed, as
// turned about the upper-left corner of its rectangle.  An annotation with
// the NoZoom flag is tested at its size for the default magnification.  At
// other magnifications it can cover a region, but it then shows only content
// of its appearance which lies outside the regions at the default
// magnification.
//
// # Rewriting content
//
// On pages with regions, the page content, the forms, soft-mask groups and
// appearance streams, and the tiling patterns which paint under a region are
// read and rewritten.  Such a stream which cannot be decoded completely, for
// example because its data is truncated or its filter is unknown, is
// replaced by a rewritten copy of the part which can be decoded; a stream
// with an unknown filter is emptied.  A malformed /Matrix entry is read as the
// identity, and such a stream is replaced by a copy without the entry.  A
// reference XObject (PDF 2.0 section 8.10.4) is replaced by a copy without its
// /Ref entry, since the page it refers to can show the original content.  The
// rewritten streams are properly nested and hold no unknown operators: an
// operator which would close a pair such as q and Q across another pair is
// left out, or for Q and ET, the pairs in between are closed first, and before
// PDF 2.0, q and Q in a text object are left out.
//
// A tiling pattern which paints under a region is rewritten, and every tile of
// the pattern shows the rewritten cell.  Content may therefore also be removed
// from tiles outside the regions, and a pattern may be emptied.  The soft-mask
// group in the graphics state of a shading pattern is rewritten wherever the
// pattern is selected as a colour on a page with regions, with the group
// placed in pattern space.
//
// XObjects which are neither images nor forms, such as PostScript XObjects,
// are removed from pages with regions, since their data can hold text.  So
// are images which cannot be decoded and meet a region, and forms and
// soft-mask groups which cannot be rewritten because they are cyclic or
// nested too deeply.
//
// # Shared objects
//
// Where content paints an image, form, pattern or soft-mask group unchanged,
// for example on a page without regions or in the appearance of a kept
// annotation, the output refers to an unchanged copy of the object.  All
// other references to the object are removed, for example the resource
// entries which no content paints.
//
// An object which is removed or replaced by a rewritten copy where it is
// painted is not copied as it is.  The same holds for the content streams of
// the pages with regions, and for the appearance streams of deleted
// annotations and the replaced appearances of widgets, unless a kept
// annotation shows them.  The glyph procedures of a Type 3 font paint
// wherever text is shown in the font, and the masks of an image wherever the
// image is painted.  A content item of the structure tree which refers to the
// object is replaced by one for each such copy and each rewritten copy.
// Rewritten copies get new parent tree keys, whose entries repeat those of the
// originals.
//
// # Tagged content
//
// In a tagged document, the structure elements of removed content, and their
// ancestors, keep only the entries which the specification defines, without
// ActualText, Alt, E, Phoneme, T and AF.  These are the elements which the
// parent tree names for the content, and those whose content items refer to
// it.  Of their own attribute objects they keep only the standard attributes
// of the Layout, List, PrintField, Table and Artifact owners, without Desc,
// Summary and Short.  The attribute classes which they use lose the same
// attributes, also for the other elements which use them.  If the element of
// some removed content cannot be found, every structure element is treated
// this way.
//
// In marked-content property lists which apply to removed content, the
// ActualText, Alt, E and Phoneme entries are removed.
//
// # Known gaps
//
// Vector graphics are hidden but not always removed.  A path is clipped to the
// area outside the regions.  A subpath which crosses the boundary of a region,
// or which is covered only by the union of adjacent regions, is kept
// unchanged, so that its coordinates remain in the file.  The sh operator and
// shading patterns are also kept unchanged: they are hidden inside the regions
// by the clip, but all shading data, including mesh vertices, remains in the
// file.
//
// Removed text is replaced by a gap of the same width, so that the kept
// glyphs stay in place, and its extent can be inferred from the layout.  Fonts
// are copied whole: embedded font programs, ToUnicode maps and the glyph
// procedures of Type 3 fonts can still hold the glyphs of removed text.  A
// Type 3 font which shows text keeps what all of its glyph procedures paint.
// The extent of a TrueType glyph is taken from its outline before hinting:
// the instructions of fonts seen in practice move points by about a pixel,
// but those of a crafted font can move them anywhere.  The glyphs which a
// viewer substitutes for a font which is not embedded are taken to extend
// at most half an em to the left of and below the glyph origin, and one and
// a half em to the right and above, scaled horizontally for glyphs wider
// than one em.
//
// Text held at document level is copied unchanged: outlines, the document
// information dictionary, XMP metadata, embedded files, JavaScript, named
// destinations, and the catalog's /AF and /PieceInfo.  The /Contents and /RC
// of annotations which are kept are also copied unchanged.
//
// Template pages (the /Templates name tree) are not part of the page tree,
// and are copied unchanged.  So are the additional actions (/AA) of the
// pages and of the document, which can hold JavaScript.  A page dictionary
// which the page tree does not reach but which is referred to from elsewhere,
// for example by a destination, is copied unchanged in the same way.
//
// # Output
//
// The output is a complete rewrite and holds no earlier revisions.  Every byte
// offset in the file changes, which invalidates the ByteRange of every
// signature, whether or not anything was redacted.  For this reason the values
// of signature fields reachable from the interactive form or from a widget,
// the form's /SigFlags (if some signature field has a value) and the catalog's
// /Perms entry are removed on every call.
//
// The output has a new page tree.  References to the nodes of the input page
// tree which are not pages, for example a destination which names an
// intermediate node, give null.
//
// On pages with a redaction annotation, the page thumbnail (/Thumb), the
// page-piece dictionary (/PieceInfo), the page metadata (/Metadata) and the
// associated files (/AF) are removed, since they may show the original
// content.  The interactive form's /XFA entry is removed if any region is
// applied.
package redact
