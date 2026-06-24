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

package acroform

import (
	"errors"
	"fmt"
	"maps"
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/internal/formhooks"
)

// PDF 2.0 sections: 12.7.3

// InteractiveForm represents a document's interactive form, referenced from
// the AcroForm entry in the document catalog.
//
// The form must be encoded via [InteractiveForm.Encode] after the pages
// containing its fields are written; encoding it via
// [seehuhn.de/go/pdf.ResourceManager.StoreDeferred] arranges this automatically.
//
// Use [seehuhn.de/go/pdf/annotation/decode.Form] to decode an interactive form
// from a PDF file.
type InteractiveForm struct {
	// Fields are the roots of the form's field trees.
	// Entries are either of type [Group] or [Field].
	Fields []Node

	// NeedAppearances indicates that the viewer must construct appearance
	// streams and appearance dictionaries for all widget annotations in the
	// document.
	//
	// This entry is deprecated in PDF 2.0, where appearance streams are
	// required.
	NeedAppearances bool

	// SigFlags is a set of flags describing document-level characteristics
	// related to signature fields.
	SigFlags SignatureFlags

	// CalculationOrder (optional) lists the fields with calculation actions, in
	// the order their values are recalculated when the value of any field
	// changes. Each entry must also appear in the field tree reachable from
	// Fields.
	//
	// This corresponds to the /CO entry in the interactive form dictionary.
	CalculationOrder []Field

	// DefaultResources (optional) contains resources, such as fonts, that are
	// used by form field appearance streams.
	//
	// This corresponds to the /DR entry in the interactive form dictionary.
	DefaultResources *content.Resources

	// XFA (optional) holds an XFA resource, as a stream or an array. The
	// library treats this value as opaque.
	//
	// This entry is deprecated in PDF 2.0.
	XFA pdf.Object
}

// SignatureFlags is a set of document-level flags related to signature fields.
type SignatureFlags uint32

const (
	// SignaturesExist indicates that the document contains at least one
	// signature field.
	SignaturesExist SignatureFlags = 1 << 0

	// AppendOnly indicates that the document contains signatures that may be
	// invalidated if the file is saved in a way that alters its previous
	// contents, rather than by an incremental update.
	AppendOnly SignatureFlags = 1 << 1
)

var _ pdf.Encoder = (*InteractiveForm)(nil)

// encNode mirrors one node of the field tree during encoding. The encoder works
// on this mirror so it never mutates the caller's [Group] and [Field] values.
type encNode struct {
	field Field      // nil for a group
	group *Group     // nil for a terminal field
	kids  []*encNode // children of a group
	dict  pdf.Dict   // the node's own dictionary entries
}

// Encode returns the interactive form dictionary, suitable for use as the
// AcroForm entry in the document catalog. It writes the field tree and all of
// the form's widget annotations as a side effect.
//
// Encode must run after the pages whose widgets the form owns: each form widget
// reserves its reference when its page is written, and the form fills in those
// references here. Use [seehuhn.de/go/pdf.ResourceManager.StoreDeferred] to have
// the form encoded automatically at the end, when the resource manager is
// closed.
//
// This implements the [pdf.Encoder] interface.
func (f *InteractiveForm) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "interactive form", pdf.V1_2); err != nil {
		return nil, err
	}

	// phase 1: build the mirror tree, validate, and emit each node's own entries
	seenNode := map[Node]bool{}
	seenWidget := map[Widget]bool{}
	inTree := map[Field]bool{}
	roots := make([]*encNode, 0, len(f.Fields))
	for _, node := range f.Fields {
		n, err := f.buildEncNode(rm, node, seenNode, seenWidget, inTree)
		if err != nil {
			return nil, err
		}
		roots = append(roots, n)
	}

	// phase 2: hoist inheritable entries into group nodes
	for _, n := range roots {
		factorNode(n)
	}

	// phase 3: hoist the document-wide DA/Q into the form dictionary
	formDA, formQ := factorForm(roots)

	dict := pdf.Dict{}

	// phase 4: allocate references and write the tree top-down
	fieldRefs := map[Field]pdf.Reference{}
	fields := make(pdf.Array, 0, len(roots))
	for _, n := range roots {
		ref, err := writeEncNode(rm, n, 0, fieldRefs)
		if err != nil {
			return nil, err
		}
		fields = append(fields, ref)
	}

	// phase 5: assemble the form dictionary
	dict["Fields"] = fields

	if f.NeedAppearances {
		dict["NeedAppearances"] = pdf.Boolean(true)
	}

	if f.SigFlags != 0 {
		if err := pdf.CheckVersion(rm.Out, "interactive form SigFlags entry", pdf.V1_3); err != nil {
			return nil, err
		}
		dict["SigFlags"] = pdf.Integer(f.SigFlags)
	}

	if len(f.CalculationOrder) > 0 {
		if err := pdf.CheckVersion(rm.Out, "interactive form CO entry", pdf.V1_3); err != nil {
			return nil, err
		}
		co := make(pdf.Array, 0, len(f.CalculationOrder))
		for _, fld := range f.CalculationOrder {
			if !inTree[fld] {
				return nil, errors.New("CalculationOrder field is not in the form")
			}
			co = append(co, fieldRefs[fld])
		}
		dict["CO"] = co
	}

	if f.DefaultResources != nil {
		dr, err := rm.Embed(f.DefaultResources)
		if err != nil {
			return nil, err
		}
		dict["DR"] = dr
	}

	if formDA != nil {
		dict["DA"] = formDA
	}
	// a document-wide Q of 0 (left-justified) is the default and is omitted
	if q, ok := formQ.(pdf.Integer); ok && q != pdf.Integer(pdf.TextAlignLeft) {
		dict["Q"] = formQ
	}

	if f.XFA != nil {
		// the stream form dates from PDF 1.5, the array form from PDF 1.6
		// TODO(voss): an indirect reference resolving to an array is gated
		// at 1.5 instead of 1.6 here, since we only inspect the direct value.
		xfaVersion := pdf.V1_5
		if _, ok := f.XFA.(pdf.Array); ok {
			xfaVersion = pdf.V1_6
		}
		if err := pdf.CheckVersion(rm.Out, "interactive form XFA entry", xfaVersion); err != nil {
			return nil, err
		}
		dict["XFA"] = f.XFA
	}

	return dict, nil
}

// buildEncNode builds the mirror node for one tree node, validating it and
// emitting its own dictionary entries (phase 1). It recurses into groups and
// records every terminal field in inTree, rejecting any node or widget that
// appears in the tree more than once.
func (f *InteractiveForm) buildEncNode(rm *pdf.ResourceManager, node Node, seenNode map[Node]bool, seenWidget map[Widget]bool, inTree map[Field]bool) (*encNode, error) {
	if seenNode[node] {
		return nil, errors.New("field or group appears more than once in the form")
	}
	seenNode[node] = true

	switch t := node.(type) {
	case *Group:
		if strings.Contains(t.Name, ".") {
			return nil, errors.New("group partial name must not contain a period")
		}
		if len(t.Kids) == 0 {
			return nil, errors.New("group without children")
		}
		dict := pdf.Dict{}
		if t.Name != "" {
			dict["T"] = pdf.TextString(t.Name)
		}
		n := &encNode{group: t, dict: dict}
		for _, kid := range t.Kids {
			kn, err := f.buildEncNode(rm, kid, seenNode, seenWidget, inTree)
			if err != nil {
				return nil, err
			}
			n.kids = append(n.kids, kn)
		}
		return n, nil

	case Field:
		for _, w := range t.GetCommon().Widgets {
			if seenWidget[w] {
				return nil, errors.New("widget appears more than once in the form")
			}
			seenWidget[w] = true
		}
		dict, err := terminalEntries(rm, t)
		if err != nil {
			return nil, err
		}
		inTree[t] = true
		return &encNode{field: t, dict: dict}, nil

	default:
		return nil, errors.New("unknown field tree node type")
	}
}

// factorNode hoists inheritable entries from a group's children into the group,
// bottom-up (phase 2). Terminal nodes have no children and are left unchanged.
func factorNode(n *encNode) {
	if n.group == nil {
		return
	}
	for _, kid := range n.kids {
		factorNode(kid)
	}
	children := make([]pdf.Dict, len(n.kids))
	for i, kid := range n.kids {
		children[i] = kid.dict
	}
	hoistUnanimous("FT", n.dict, children)
	hoistUnanimous("DA", n.dict, children)
	hoistUnanimous("MaxLen", n.dict, children)
	hoistWithDefault("Ff", pdf.Integer(0), n.dict, children)
	hoistWithDefault("Q", pdf.Integer(pdf.TextAlignLeft), n.dict, children)
}

// factorForm hoists the document-wide default appearance and quadding into the
// form dictionary (phase 3), returning the values for its /DA and /Q entries
// (either may be nil). Only DA and Q are inheritable at the form level; FT, Ff
// and MaxLen have no place in the interactive form dictionary.
func factorForm(roots []*encNode) (da, q pdf.Object) {
	rootDicts := make([]pdf.Dict, len(roots))
	for i, n := range roots {
		rootDicts[i] = n.dict
	}
	formDict := pdf.Dict{}
	hoistUnanimous("DA", formDict, rootDicts)
	hoistWithDefault("Q", pdf.Integer(pdf.TextAlignLeft), formDict, rootDicts)
	return formDict["DA"], formDict["Q"]
}

// writeEncNode allocates references and writes a node and its subtree top-down
// (phase 4), returning the reference that names the node. parentRef is the
// enclosing group's reference, or 0 at the root. Terminal fields are written
// together with their widgets: a single-widget field is merged into its widget
// and has no object of its own. Each terminal field's reference is recorded in
// fieldRefs for the /CO array.
func writeEncNode(rm *pdf.ResourceManager, n *encNode, parentRef pdf.Reference, fieldRefs map[Field]pdf.Reference) (pdf.Reference, error) {
	if n.group != nil {
		ref := rm.Out.Alloc()
		kidRefs := make(pdf.Array, 0, len(n.kids))
		for _, kid := range n.kids {
			kidRef, err := writeEncNode(rm, kid, ref, fieldRefs)
			if err != nil {
				return 0, err
			}
			kidRefs = append(kidRefs, kidRef)
		}
		n.dict["Kids"] = kidRefs
		if err := putNode(rm, ref, n.dict, parentRef); err != nil {
			return 0, err
		}
		return ref, nil
	}

	widgets := n.field.GetCommon().Widgets
	if len(widgets) == 1 {
		// a single-widget terminal: no object of its own; the field's entries
		// are folded into the widget, forming one merged field/widget dictionary
		// whose /Parent is the field's enclosing group
		entries := n.dict
		if !hasMergedDetectionKey(entries) {
			// keep the merged dictionary recognisable as a field
			entries["FT"] = n.field.FieldType()
		}
		ref, err := writeWidget(rm, widgets[0], entries, parentRef)
		if err != nil {
			return 0, err
		}
		fieldRefs[n.field] = ref
		return ref, nil
	}

	// a field with its own object; each widget gets a /Parent pointing at it
	ref := rm.Out.Alloc()
	kidRefs := make(pdf.Array, 0, len(widgets))
	for _, w := range widgets {
		wref, err := writeWidget(rm, w, nil, ref)
		if err != nil {
			return 0, err
		}
		kidRefs = append(kidRefs, wref)
	}
	if len(kidRefs) > 0 {
		n.dict["Kids"] = kidRefs
	}
	if err := putNode(rm, ref, n.dict, parentRef); err != nil {
		return 0, err
	}
	fieldRefs[n.field] = ref
	return ref, nil
}

// writeWidget encodes the widget annotation w and writes it at the reference w
// reserved while its page was written. fieldEntries are the owning field's own
// dictionary entries to fold in; they are non-nil only for a single-widget
// field merged with its widget. Field entries take precedence over the widget's,
// except for the shared /AA, whose field half (K/F/V/C) and widget half
// (E/X/Fo/Bl/…) are combined. parentRef becomes the widget's /Parent (the
// enclosing group for a merged field, or the field's own object for a widget of
// a multi-widget field); it is omitted when zero.
func writeWidget(rm *pdf.ResourceManager, w Widget, fieldEntries pdf.Dict, parentRef pdf.Reference) (pdf.Reference, error) {
	if formhooks.EncodeWidgetEntries == nil {
		return 0, errors.New("annotation package not linked")
	}
	dict, err := formhooks.EncodeWidgetEntries(rm, w)
	if err != nil {
		return 0, err
	}

	for k, v := range fieldEntries {
		if existing, exists := dict[k]; exists && k == "AA" {
			merged, err := mergeAADicts(v, existing)
			if err != nil {
				return 0, err
			}
			dict[k] = merged
			continue
		}
		dict[k] = v
	}
	if parentRef != 0 {
		dict["Parent"] = parentRef
	}

	return rm.StoreEncoded(w, dict)
}

// mergeAADicts combines the field-level (K/F/V/C) and annotation-level
// (E/X/Fo/Bl/…) halves of a merged field's shared /AA dictionary. The two key
// sets are disjoint by construction, so an overlap signals a bug.
func mergeAADicts(field, widget pdf.Object) (pdf.Dict, error) {
	fd, ok := field.(pdf.Dict)
	if !ok {
		return nil, errors.New("field AA did not encode to a dictionary")
	}
	wd, ok := widget.(pdf.Dict)
	if !ok {
		return nil, errors.New("widget AA did not encode to a dictionary")
	}
	merged := maps.Clone(fd)
	for k, v := range wd {
		if _, exists := merged[k]; exists {
			return nil, fmt.Errorf("conflicting AA entry %q", k)
		}
		merged[k] = v
	}
	return merged, nil
}

// putNode sets the node's /Parent (omitted at the root, where parentRef is 0)
// and writes the node's dictionary under ref.
func putNode(rm *pdf.ResourceManager, ref pdf.Reference, dict pdf.Dict, parentRef pdf.Reference) error {
	if parentRef != 0 {
		dict["Parent"] = parentRef
	}
	return rm.Out.Put(ref, dict)
}
