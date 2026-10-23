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

package pdf

import (
	"bytes"
	"io"
	"maps"
)

// A Copier is used to copy objects from one PDF file to another. The Copier
// keeps track of the objects that have already been copied and ensures that
// each object is copied only once.
//
// Indirect objects are allocated in the target file as needed, and references
// are translated accordingly.
type Copier struct {
	// trans maps source references to their copies.  If the source or the
	// destination encrypts embedded file streams differently from other
	// streams, a source object reached in both roles is copied twice, since
	// the two copies differ in their decryption or encryption.
	trans map[transKey]Reference
	r     Getter
	w     *Writer

	// sameFile is set when r reads the file w is appending to, so that
	// references are already valid in the output and pass through unchanged
	sameFile bool

	// rolesDiffer is set when the source or the destination uses different
	// crypt filters for embedded file streams and for other streams
	rolesDiffer bool

	// redirect holds the replacements installed by [Copier.Redirect], which
	// apply in every role
	redirect map[Reference]Reference
}

// copyRole says how an object is reached during a copy.  Embedded file
// streams are encrypted with the crypt filter named by /EFF, but a stream
// without /Type /EmbeddedFile can be recognised as an embedded file only
// from the /EF or /RF entry of the file specification referring to it.
type copyRole int

const (
	rolePlain        copyRole = iota
	roleEmbeddedFile          // an embedded file stream
	roleEF                    // an /EF dictionary, whose values are embedded file streams
	roleRF                    // an /RF dictionary, whose values are related files arrays
	roleRFArray               // a related files array, whose odd elements are embedded file streams
)

// childRole gives the role of the value stored under key in a dictionary
// which has the given role.
func childRole(role copyRole, key Name) copyRole {
	switch role {
	case roleEF:
		return roleEmbeddedFile
	case roleRF:
		return roleRFArray
	}
	switch key {
	case "EF":
		return roleEF
	case "RF":
		return roleRF
	}
	return rolePlain
}

// elementRole gives the role of element i in an array which has the given
// role.
func elementRole(role copyRole, i int) copyRole {
	if role == roleRFArray && i%2 == 1 {
		return roleEmbeddedFile
	}
	return rolePlain
}

type transKey struct {
	ref  Reference
	role copyRole
}

// NewCopier creates a new Copier.
//
// If r reads the file which w updates (r is w itself, or the reader w was
// opened from), references are passed through unchanged instead of being
// copied.
func NewCopier(w *Writer, r Getter) *Copier {
	c := &Copier{
		trans:    make(map[transKey]Reference),
		w:        w,
		r:        r,
		redirect: make(map[Reference]Reference),
	}
	switch r := r.(type) {
	case *Writer:
		c.sameFile = r == w
	case *Reader:
		c.sameFile = w.base != nil && r == w.base
	}
	if enc := w.w.enc; enc != nil {
		c.rolesDiffer = enc.stmF != enc.efF
	}
	if enc := r.GetMeta().Encryption; enc != nil {
		c.rolesDiffer = c.rolesDiffer || enc.Streams != enc.EmbeddedFiles
	}
	return c
}

// Copy copies an object from the source file to the target file, recursively.
//
// The returned object is guaranteed to be the same type as the input object.
func (c *Copier) Copy(obj Native) (Native, error) {
	return c.copy(obj, rolePlain)
}

func (c *Copier) copy(obj Native, role copyRole) (Native, error) {
	switch x := obj.(type) {
	case Dict:
		return c.copyDict(x, role)
	case Array:
		return c.copyArray(x, role)
	case *Stream:
		return c.copyStream(x, role == roleEmbeddedFile)
	case Reference:
		return c.copyReference(x, role)
	default:
		return obj, nil
	}
}

// copyStream copies a stream.  If embeddedFile is set, the stream is
// treated as an embedded file both when decrypting the source bytes and
// when encrypting the copy.
func (c *Copier) copyStream(x *Stream, embeddedFile bool) (*Stream, error) {
	if embeddedFile {
		x = x.AsEmbeddedFile()
	}
	dict, err := c.copyStreamDict(x.Dict)
	if err != nil {
		return nil, err
	}
	res := &Stream{
		Dict:         dict,
		data:         x.data,
		start:        x.start,
		length:       x.length,
		embeddedFile: x.embeddedFile,
	}
	// Decide whether the source's on-disk bytes need decryption
	// before being installed in the destination.  For /Identity
	// (and unencrypted sources) the bytes are already plaintext
	// and can be reused in place; otherwise we strip the source
	// encryption so the destination writer can apply its own.
	recipe, cfName, err := streamCryptRecipe(c.r, x)
	if err != nil {
		return nil, err
	}
	switch recipe {
	case cryptDefault, cryptNamed:
		rc, err := RawStreamReader(c.r, x)
		if err != nil {
			return nil, err
		}
		raw, err := io.ReadAll(rc)
		if closeErr := rc.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return nil, err
		}
		res.data = bytes.NewReader(raw)
		res.start = 0
		res.length = int64(len(raw))
	}
	// A named crypt filter is kept if the destination defines it.
	// Otherwise the stream falls back to the destination's default.
	if cfName != "" {
		if enc := c.w.w.enc; enc == nil || enc.cf[cfName] == nil {
			stripLeadingCrypt(res.Dict)
		}
	}
	return res, nil
}

// CopyDict copies a dictionary from the source file to the target file.
// Entries with a nil value are omitted from the copy.
//
// The keys are visited in the order [Dict.SortedKeys] returns them.  Copying a
// reference allocates an object number in the target file, so map order would
// make the output depend on the run.
func (c *Copier) CopyDict(obj Dict) (Dict, error) {
	return c.copyDict(obj, rolePlain)
}

func (c *Copier) copyDict(obj Dict, role copyRole) (Dict, error) {
	res := Dict{}
	for _, key := range obj.SortedKeys() {
		val := obj[key]
		if val == nil {
			continue
		}
		repl, err := c.copy(val.AsPDF(c.w.GetOptions()), childRole(role, key))
		if err != nil {
			return nil, err
		}
		res[key] = repl
	}

	return res, nil
}

// copyStreamDict copies a stream's dictionary, inlining /Filter and
// /DecodeParms so that the destination dict carries direct values
// rather than references into the destination file.  This keeps the
// filter chain self-describing for downstream consumers (notably the
// cheap /Crypt probe in [Writer.OpenStream]) and matches the canonical
// PDF form used by all writers in the library.
func (c *Copier) copyStreamDict(src Dict) (Dict, error) {
	// the filter entries are inlined below, so an indirect one must not
	// be copied as an object of its own
	rest := maps.Clone(src)
	delete(rest, "Filter")
	delete(rest, "DecodeParms")
	res, err := c.CopyDict(rest)
	if err != nil {
		return nil, err
	}
	for _, key := range []Name{"Filter", "DecodeParms"} {
		val, ok := src[key]
		if !ok {
			continue
		}
		inlined, err := inlineFilterRefs(c.r, val)
		if err != nil {
			return nil, err
		}
		repl, err := c.Copy(inlined)
		if err != nil {
			return nil, err
		}
		res[key] = repl
	}
	return res, nil
}

// inlineFilterRefs resolves indirect references in a /Filter or
// /DecodeParms entry at the top level and, for arrays, at the element
// level.  The returned value contains only direct entries at those
// levels; nested references inside dictionary entries are left alone
// (the caller's [Copier.Copy] handles them).
func inlineFilterRefs(r Getter, val Object) (Native, error) {
	resolved, err := Resolve(r, val)
	if err != nil {
		return nil, err
	}
	arr, ok := resolved.(Array)
	if !ok {
		return resolved, nil
	}
	out := make(Array, len(arr))
	for i, v := range arr {
		elem, err := Resolve(r, v)
		if err != nil {
			return nil, err
		}
		out[i] = elem
	}
	return out, nil
}

// CopyArray copies an array from the source file to the target file.
func (c *Copier) CopyArray(obj Array) (Array, error) {
	return c.copyArray(obj, rolePlain)
}

func (c *Copier) copyArray(obj Array, role copyRole) (Array, error) {
	var res Array
	for i, val := range obj {
		var repl Native
		if val != nil {
			var err error
			repl, err = c.copy(val.AsPDF(c.w.GetOptions()), elementRole(role, i))
			if err != nil {
				return nil, err
			}
		}
		res = append(res, repl)
	}
	return res, nil
}

// CopyReference copies a reference from the source file to the target file.
//
// This method shortens chains of indirect references, the returned reference
// always points to a direct object.
func (c *Copier) CopyReference(obj Reference) (Reference, error) {
	return c.copyReference(obj, rolePlain)
}

func (c *Copier) copyReference(obj Reference, role copyRole) (Reference, error) {
	if newRef, ok := c.redirect[obj]; ok {
		return newRef, nil
	}
	if c.sameFile {
		return obj, nil
	}
	key := transKey{ref: obj, role: role}
	if !c.rolesDiffer {
		// the copies are the same in every role
		key.role = rolePlain
	}
	if newRef, ok := c.trans[key]; ok {
		return newRef, nil
	}
	newRef := c.w.Alloc()
	c.trans[key] = newRef

	val, err := Resolve(c.r, obj)
	if IsReadError(err) {
		return 0, err
	}
	// a reference to a malformed or undefined source object resolves to
	// null (PDF 2.0, 7.3.10); leave val nil and copy null in its place
	trans, err := c.copy(val, role)
	if err != nil {
		return 0, err
	}
	err = c.w.Put(newRef, trans)
	if err != nil {
		return 0, err
	}

	return newRef, nil
}

// Redirect replaces an indirect object in the old file with one in the new file.
func (c *Copier) Redirect(origRef, newRef Reference) {
	c.redirect[origRef] = newRef
}
