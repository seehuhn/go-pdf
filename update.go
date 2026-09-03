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

package pdf

import (
	"bufio"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
)

// UpdateOptions controls an incremental update.
//
// In update mode, GetMeta().Catalog and GetMeta().Info are values decoded
// from the original file; they must be replaced by modified copies, never
// edited in place, or the edit is silently lost.
type UpdateOptions struct {
	// Password is used to open an encrypted original.  The empty string
	// is always tried first.
	Password string

	// Version, if non-zero, is the minimum version of the updated document.
	// If the original's version is lower, the document is raised to this
	// version by writing the catalog /Version entry; otherwise the option
	// has no effect.  Only objects written by the update are checked
	// against the raised version; the caller is responsible for the
	// original content still conforming.  The catalog is copied before the
	// version is raised.
	Version Version

	// HumanReadable requests pretty printing and disables object streams
	// and cross-reference streams.
	HumanReadable bool
}

// ReadWriteFile is the file type accepted by [NewUpdater].
// *os.File satisfies it.
type ReadWriteFile interface {
	io.ReaderAt
	io.WriteSeeker
}

// Update opens the named PDF file and prepares an incremental update
// which is appended to the file in place.  [Writer.Close] writes the
// update and closes the file.
//
// The update uses the encryption of the original, which cannot be
// changed.  Object numbers of freed objects are never reused.
func Update(name string, opt *UpdateOptions) (*Writer, error) {
	fd, err := os.OpenFile(name, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	fi, err := fd.Stat()
	if err != nil {
		fd.Close()
		return nil, err
	}
	w, err := NewUpdater(fd, fi.Size(), opt)
	if err != nil {
		fd.Close()
		return nil, err
	}
	w.closeOrigW = true
	return w, nil
}

// NewUpdater prepares an incremental update of the PDF file in f, whose
// current length is size.  New data is appended to f in place.
// [Writer.Close] writes the update but does not close f.
func NewUpdater(f ReadWriteFile, size int64, opt *UpdateOptions) (*Writer, error) {
	if _, err := f.Seek(size, io.SeekStart); err != nil {
		return nil, err
	}
	return newUpdater(f, size, f, opt)
}

// NewUpdaterTo copies the PDF file in src, of length size, to dst and
// prepares an incremental update which is appended to dst.  Objects
// written by the update can be read back through [Writer.Get] only if
// dst also implements [io.ReaderAt].
func NewUpdaterTo(src io.ReaderAt, size int64, dst io.Writer, opt *UpdateOptions) (*Writer, error) {
	_, err := io.Copy(dst, io.NewSectionReader(src, 0, size))
	if err != nil {
		return nil, err
	}
	return newUpdater(src, size, dst, opt)
}

func newUpdater(src io.ReaderAt, size int64, dst io.Writer, opt *UpdateOptions) (*Writer, error) {
	if opt == nil {
		opt = &UpdateOptions{}
	}

	base, err := NewReader(src, size, &ReaderOptions{Password: opt.Password})
	if err != nil {
		return nil, err
	}

	v := max(base.meta.Version, opt.Version)

	// the highest object number in use, whether listed by /Size or not
	next := base.trailerSize
	for n := range base.xref {
		next = max(next, int64(n)+1)
	}
	if next >= maxXRefSize {
		return nil, errors.New("object numbers exhausted")
	}

	bufferedW, ok := dst.(writeFlusher)
	if !ok {
		bufferedW = bufio.NewWriter(dst)
	}

	outOpt := outputOptionsFor(v, opt.HumanReadable)

	meta := base.meta
	meta.Version = v
	meta.Trailer = base.meta.Trailer.Clone()
	if opt.Version > base.meta.Version {
		// decoded values are immutable; raise the version on a copy
		cat := *base.meta.Catalog
		cat.Version = opt.Version
		meta.Catalog = &cat
	}

	w := &Writer{
		meta: meta,
		w: &posWriter{
			w:   bufferedW,
			enc: base.enc,
			pos: size,
		},
		origW:          dst,
		nextRef:        uint32(next),
		xref:           make(map[uint32]*xRefEntry),
		objects:        make(map[any]Native),
		reserved:       make(map[any]*ResourceManager),
		objstms:        newObjstmCache(),
		outputOptions:  outOpt,
		refIsPlaintext: map[Reference]bool{},
		base:           base,
		headerOffset:   base.headerOffset,
	}
	w.rm = NewResourceManager(w)

	w.baseCatalog = base.meta.Catalog
	w.baseInfo = base.meta.Info

	// the base Reader decoded these values, not the Writer, so their
	// provenance is recorded here.  The trailer entry may itself be a
	// reference to a further reference before reaching the object; the
	// chain is followed, as Decode does, and the last reference of the
	// chain is recorded, since that is where a replacement takes effect.
	if catalogObj, path, err := resolvePath(base, nil, base.meta.Trailer["Root"], true); err == nil && path != nil {
		ref := path.Ref
		w.recordOrigin(w.baseCatalog, ref)
		if m := w.baseCatalog.Metadata; m != nil {
			if dict, ok := catalogObj.(Dict); ok {
				if mref, ok := dict["Metadata"].(Reference); ok {
					w.recordOrigin(m, mref)
				}
			}
		}
	}
	if _, path, err := resolvePath(base, nil, base.meta.Trailer["Info"], true); err == nil && path != nil && w.baseInfo != nil {
		w.recordOrigin(w.baseInfo, path.Ref)
	}

	// separate the update from an original which does not end in white space
	var last [1]byte
	if _, err := src.ReadAt(last[:], size-1); err != nil {
		return nil, err
	}
	switch last[0] {
	case ' ', '\t', '\r', '\n', '\f', 0:
	default:
		if _, err := w.w.Write([]byte{'\n'}); err != nil {
			return nil, err
		}
	}

	return w, nil
}

// Free marks an object of the original file as deleted.  The reference
// must name an object in use in the original, with the same generation
// number.  The object number is not reused by [Writer.Alloc].
func (w *Writer) Free(ref Reference) error {
	if w.base == nil {
		return errors.New("Free requires an incremental update")
	}
	entry := w.base.xref[ref.Number()]
	if entry.IsFree() || entry.Generation != ref.Generation() {
		return fmt.Errorf("Writer.Free: object %s is not in use", ref)
	}
	gen := entry.Generation
	if gen < maxGeneration {
		gen++
	}
	err := w.setXRef(ref, &xRefEntry{Pos: -1, Generation: gen})
	if err != nil {
		return fmt.Errorf("Writer.Free: %w", err)
	}
	return nil
}

// getUpdate implements [Writer.get] in update mode: objects written in
// this session shadow the original.
func (w *Writer) getUpdate(ref Reference, canObjStm, scalarOnly bool) (Native, error) {
	entry := w.xref[ref.Number()]
	if entry == nil {
		return w.base.get(ref, canObjStm, scalarOnly)
	}
	if entry.IsFree() || entry.Generation != ref.Generation() {
		return nil, nil
	}

	if entry.InStream != 0 {
		if !canObjStm {
			return nil, &MalformedFileError{
				Err: errors.New("object in object stream"),
				Loc: []string{"object " + ref.String()},
			}
		}
		getInt := safeGetInteger(writerLengthGetter{w}, true)
		return getFromObjStm(w, ref.Number(), entry.InStream, getInt, w.objstms)
	}

	ra, ok := w.origW.(io.ReaderAt)
	if !ok {
		return nil, errors.New("Get() not supported by the underlying io.Writer")
	}
	if err := w.w.w.Flush(); err != nil {
		return nil, err
	}

	getInt := safeGetInteger(writerLengthGetter{w}, canObjStm)
	s := newScanner(io.NewSectionReader(ra, entry.Pos, w.w.pos-entry.Pos), getInt, w.w.enc)
	s.fileReader = ra
	s.filePos = entry.Pos
	s.scalarOnly = scalarOnly

	obj, fileRef, err := s.ReadIndirectObject()
	if err != nil {
		return nil, err
	}
	if ref != fileRef {
		return nil, &MalformedFileError{
			Err: errors.New("xref corrupted"),
			Loc: []string{"object " + ref.String() + "*"},
		}
	}
	return obj, nil
}

// closeUpdate writes the catalog and Info dictionary of an incremental
// update and returns the trailer dictionary.
func (w *Writer) closeUpdate() (Dict, error) {
	// a replaced metadata stream is embedded fresh, at a reference marked
	// plaintext when the original exempts metadata from encryption
	if m := w.meta.Catalog.Metadata; m != nil && w.Origin(m) == 0 {
		ref := w.Alloc()
		if w.w.enc != nil && w.w.enc.sec.unencryptedMetadata {
			w.refIsPlaintext[ref] = true
		}
		e := &EmbedHelper{rm: w.rm, copiers: map[*Extractor]*Copier{}}
		if _, err := e.EmbedAt(ref, m); err != nil {
			return nil, err
		}
	}

	trailer := w.meta.Trailer.Clone()

	var rootRef Reference
	switch {
	case w.meta.Catalog == w.baseCatalog && w.Origin(w.baseCatalog) != 0:
		rootRef = w.Origin(w.baseCatalog)
	case w.Origin(w.baseCatalog) != 0:
		var err error
		rootRef, err = w.rm.Replace(w.baseCatalog, w.meta.Catalog)
		if err != nil {
			return nil, fmt.Errorf("failed to write document catalog: %w", err)
		}
	default:
		var err error
		rootRef, err = w.rm.Store(w.meta.Catalog)
		if err != nil {
			return nil, fmt.Errorf("failed to write document catalog: %w", err)
		}
	}
	trailer["Root"] = rootRef

	baseInfoRef := w.Origin(w.baseInfo)
	switch {
	case w.meta.Info == nil || w.meta.Info.isEmpty():
		delete(trailer, "Info")
		if baseInfoRef != 0 {
			if err := w.Free(baseInfoRef); err != nil {
				return nil, err
			}
		}
	case w.meta.Info == w.baseInfo && baseInfoRef != 0:
		trailer["Info"] = baseInfoRef
	case baseInfoRef != 0:
		ref, err := w.rm.Replace(w.baseInfo, w.meta.Info)
		if err != nil {
			return nil, err
		}
		trailer["Info"] = ref
	default:
		ref, err := w.rm.Embed(w.meta.Info)
		if err != nil {
			return nil, err
		}
		trailer["Info"] = ref
	}

	if err := w.rm.Close(); err != nil {
		return nil, err
	}

	// ID[0] is the permanent identifier and feeds the encryption key.
	// ID[1] identifies the version last written.
	id1 := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, id1); err != nil {
		return nil, err
	}
	switch {
	case len(w.meta.ID) == 2:
		w.meta.ID = [][]byte{w.meta.ID[0], id1}
	case w.meta.Version >= V2_0:
		w.meta.ID = [][]byte{id1, id1}
	default:
		w.meta.ID = nil
	}
	if w.meta.ID != nil {
		trailer["ID"] = Array{String(w.meta.ID[0]), String(w.meta.ID[1])}
	} else {
		delete(trailer, "ID")
	}

	trailer["Prev"] = Integer(w.base.startXRef)
	return trailer, nil
}
