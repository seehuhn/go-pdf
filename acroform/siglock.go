// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package acroform

import (
	"fmt"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 12.7.5.5

// SigFieldLockAction selects which form fields are locked when the signature
// field is signed.
type SigFieldLockAction pdf.Name

const (
	// SigFieldLockAll locks all fields in the document.
	SigFieldLockAll SigFieldLockAction = "All"

	// SigFieldLockInclude locks the fields named in [SigFieldLock.Fields].
	SigFieldLockInclude SigFieldLockAction = "Include"

	// SigFieldLockExclude locks all fields except those named in
	// [SigFieldLock.Fields].
	SigFieldLockExclude SigFieldLockAction = "Exclude"
)

// SigFieldLock is a signature field lock dictionary. It specifies the set of
// form fields that are locked when the containing signature field is signed.
//
// It corresponds to the /Lock entry of a signature field and is always written
// as an indirect object.
type SigFieldLock struct {
	// Action selects which fields are locked, in conjunction with Fields.
	Action SigFieldLockAction

	// Fields lists the field names that Action refers to. It is used only when
	// Action is [SigFieldLockInclude] or [SigFieldLockExclude].
	Fields []string

	// P specifies the access permissions granted for the document after signing,
	// as one of the values 1, 2, or 3. The value 0 indicates that no permission
	// constraint is set.
	//
	// This entry requires PDF 2.0.
	P int
}

var _ pdf.Embedder = (*SigFieldLock)(nil)

// Embed writes the signature field lock dictionary to the PDF file and returns a
// reference to it.
//
// This implements the [pdf.Embedder] interface.
func (l *SigFieldLock) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "signature field lock dictionary", pdf.V1_5); err != nil {
		return nil, err
	}

	dict := pdf.Dict{}
	if e.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("SigFieldLock")
	}

	switch l.Action {
	case SigFieldLockAll:
		dict["Action"] = pdf.Name(SigFieldLockAll)
	case SigFieldLockInclude, SigFieldLockExclude:
		dict["Action"] = pdf.Name(l.Action)
		// Fields is required for Include / Exclude
		arr := make(pdf.Array, len(l.Fields))
		for i, s := range l.Fields {
			arr[i] = pdf.Text(s)
		}
		dict["Fields"] = arr
	default:
		return nil, fmt.Errorf("invalid signature field lock action %q", l.Action)
	}

	if l.P != 0 {
		if l.P < 1 || l.P > 3 {
			return nil, fmt.Errorf("invalid signature field lock permission %d", l.P)
		}
		if err := pdf.CheckVersion(e.Out(), "signature field lock P entry", pdf.V2_0); err != nil {
			return nil, err
		}
		dict["P"] = pdf.Integer(l.P)
	}

	ref := e.Alloc()
	if err := e.Out().Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}
