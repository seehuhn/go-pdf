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

// ExtractSigFieldLock reads a signature field lock dictionary from a PDF file.
func ExtractSigFieldLock(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, _ bool) (*SigFieldLock, error) {
	dict, err := x.GetDict(path, obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing signature field lock dictionary")
	}

	lock := &SigFieldLock{}

	// Action; snap an unrecognised value to a safe default so the result stays
	// writable
	action, err := pdf.Optional(x.GetName(path, dict["Action"]))
	if err != nil {
		return nil, err
	}
	switch SigFieldLockAction(action) {
	case SigFieldLockInclude:
		lock.Action = SigFieldLockInclude
	case SigFieldLockExclude:
		lock.Action = SigFieldLockExclude
	default:
		lock.Action = SigFieldLockAll
	}

	// Fields applies only to Include / Exclude
	if lock.Action != SigFieldLockAll {
		if fields, err := readTextStringArray(x, path, dict["Fields"]); err != nil {
			return nil, err
		} else {
			lock.Fields = fields
		}
	}

	if p, err := pdf.Optional(x.GetInteger(path, dict["P"])); err != nil {
		return nil, err
	} else if p >= 1 && p <= 3 {
		lock.P = int(p)
	}

	return lock, nil
}

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
			arr[i] = pdf.TextString(s)
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
