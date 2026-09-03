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

package pdf_test

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/text/language"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/xmp"
)

var sizePat = regexp.MustCompile(`/Size\s+(\d+)`)

// trailerSize returns the /Size named in the last trailer of data.
func trailerSize(t testing.TB, data []byte) uint32 {
	t.Helper()
	m := sizePat.FindAllSubmatch(data, -1)
	if m == nil {
		t.Fatal("no /Size entry")
	}
	n, err := strconv.Atoi(string(m[len(m)-1][1]))
	if err != nil {
		t.Fatal(err)
	}
	return uint32(n)
}

// newBaseFile writes a one-page document plus one extra object holding
// pdf.Name("old"), and returns the file and that object's reference.
func newBaseFile(t testing.TB, v pdf.Version, opt *pdf.WriterOptions) (*memfile.MemFile, pdf.Reference) {
	t.Helper()
	w, f := memfile.NewPDFWriter(t, v, opt)
	ref := w.Alloc()
	if err := w.Put(ref, pdf.Name("old")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return f, ref
}

func TestUpdaterReadsOriginal(t *testing.T) {
	f, oldRef := newBaseFile(t, pdf.V1_7, nil)
	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	pages := w.GetMeta().Catalog.Pages
	if pages == 0 {
		t.Fatal("catalog has no Pages reference")
	}
	obj, err := w.Get(pages, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := obj.(pdf.Dict); !ok {
		t.Errorf("pages object is %T, want Dict", obj)
	}
	obj, err = w.Get(oldRef, true)
	if err != nil {
		t.Fatal(err)
	}
	if obj != pdf.Name("old") {
		t.Errorf("got %v, want /old", obj)
	}
}

func TestUpdaterAllocStartsAtSize(t *testing.T) {
	f, _ := newBaseFile(t, pdf.V1_4, nil)
	size := trailerSize(t, f.Data)
	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if ref := w.Alloc(); ref.Number() != size {
		t.Errorf("first allocated number %d, want %d", ref.Number(), size)
	}
}

func TestUpdaterGetNewObject(t *testing.T) {
	f, _ := newBaseFile(t, pdf.V1_7, nil)
	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	ref := w.Alloc()
	if err := w.Put(ref, pdf.Name("added")); err != nil {
		t.Fatal(err)
	}
	obj, err := w.Get(ref, true)
	if err != nil {
		t.Fatal(err)
	}
	if obj != pdf.Name("added") {
		t.Errorf("got %v, want /added", obj)
	}
}

func TestUpdaterFree(t *testing.T) {
	f, oldRef := newBaseFile(t, pdf.V1_7, nil)
	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}

	// not in the original
	if err := w.Free(pdf.NewReference(trailerSize(t, f.Data)+5, 0)); err == nil {
		t.Error("freeing an unknown object succeeded")
	}
	// wrong generation
	if err := w.Free(pdf.NewReference(oldRef.Number(), 1)); err == nil {
		t.Error("freeing with a wrong generation succeeded")
	}

	if err := w.Free(oldRef); err != nil {
		t.Fatal(err)
	}
	obj, err := w.Get(oldRef, true)
	if err != nil {
		t.Fatal(err)
	}
	if obj != nil {
		t.Errorf("freed object still reads as %v", obj)
	}
	// twice
	if err := w.Free(oldRef); err == nil {
		t.Error("freeing twice succeeded")
	}

	// Free after Put
	pages := w.GetMeta().Catalog.Pages
	if err := w.Put(pages, pdf.Dict{"Type": pdf.Name("Pages"), "Kids": pdf.Array{}, "Count": pdf.Integer(0)}); err != nil {
		t.Fatal(err)
	}
	if err := w.Free(pages); err == nil {
		t.Error("Free after Put succeeded")
	}
}

func TestUpdaterRejectsWrongPassword(t *testing.T) {
	f, _ := newBaseFile(t, pdf.V1_7, &pdf.WriterOptions{UserPassword: "u", OwnerPassword: "o"})
	_, err := pdf.NewUpdater(f, int64(len(f.Data)), &pdf.UpdateOptions{Password: "wrong"})
	if err == nil {
		t.Error("update of an encrypted file opened without the password")
	}
}

func TestUpdaterVersionIsMinimum(t *testing.T) {
	f, _ := newBaseFile(t, pdf.V1_4, nil)
	w, err := pdf.NewUpdater(f, int64(len(f.Data)), &pdf.UpdateOptions{Version: pdf.V1_3})
	if err != nil {
		t.Fatal(err)
	}
	if w.GetMeta().Version != pdf.V1_4 {
		t.Errorf("version lowered to %s", w.GetMeta().Version)
	}
	if w.GetMeta().Catalog.Version != 0 {
		t.Errorf("catalog /Version set to %s", w.GetMeta().Catalog.Version)
	}
}

func TestUpdaterToCopiesOriginal(t *testing.T) {
	f, oldRef := newBaseFile(t, pdf.V1_4, nil)
	orig := bytes.Clone(f.Data)
	dst := memfile.New()
	w, err := pdf.NewUpdaterTo(bytes.NewReader(orig), int64(len(orig)), dst, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(dst.Data, orig) {
		t.Error("destination does not start with a copy of the original")
	}
	obj, err := w.Get(oldRef, true)
	if err != nil {
		t.Fatal(err)
	}
	if obj != pdf.Name("old") {
		t.Errorf("got %v, want /old", obj)
	}
}

// reopen parses the file and fails the test on error.
func reopen(t testing.TB, data []byte) *pdf.Reader {
	t.Helper()
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)), nil)
	if err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	return r
}

var prevPat = regexp.MustCompile(`/Prev\s+(\d+)`)

// lastStartXRef returns the value following the last startxref keyword.
func lastStartXRef(t testing.TB, data []byte) int64 {
	t.Helper()
	i := bytes.LastIndex(data, []byte("startxref"))
	if i < 0 {
		t.Fatal("no startxref keyword")
	}
	var pos int64
	if _, err := fmt.Sscanf(string(data[i+9:]), "\n%d", &pos); err != nil {
		t.Fatal(err)
	}
	return pos
}

func TestUpdateAddObject(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_4, pdf.V1_7} {
		t.Run(v.String(), func(t *testing.T) {
			f, oldRef := newBaseFile(t, v, nil)
			orig := bytes.Clone(f.Data)
			prevXRef := lastStartXRef(t, orig)

			w, err := pdf.NewUpdater(f, int64(len(orig)), nil)
			if err != nil {
				t.Fatal(err)
			}
			ref := w.Alloc()
			if err := w.Put(ref, pdf.Name("added")); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(f.Data[:len(orig)], orig) {
				t.Fatal("original bytes were modified")
			}
			appended := f.Data[len(orig):]
			m := prevPat.FindSubmatch(appended)
			if m == nil {
				t.Fatal("update has no /Prev")
			}
			if got, _ := strconv.ParseInt(string(m[1]), 10, 64); got != prevXRef {
				t.Errorf("/Prev = %d, want %d", got, prevXRef)
			}
			if lastStartXRef(t, f.Data) <= int64(len(orig)) {
				t.Error("startxref does not point into the update")
			}

			r := reopen(t, f.Data)
			obj, err := r.Get(ref, true)
			if err != nil {
				t.Fatal(err)
			}
			if obj != pdf.Name("added") {
				t.Errorf("got %v, want /added", obj)
			}
			obj, err = r.Get(oldRef, true)
			if err != nil {
				t.Fatal(err)
			}
			if obj != pdf.Name("old") {
				t.Errorf("got %v, want /old", obj)
			}
			if r.GetMeta().Catalog.Pages == 0 {
				t.Error("catalog lost")
			}
		})
	}
}

func TestUpdateReplaceAndFree(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_4, pdf.V1_7} {
		t.Run(v.String(), func(t *testing.T) {
			w0, f := memfile.NewPDFWriter(t, v, nil)
			replaceRef := w0.Alloc()
			freeRef := w0.Alloc()
			if err := w0.Put(replaceRef, pdf.Name("old")); err != nil {
				t.Fatal(err)
			}
			if err := w0.Put(freeRef, pdf.Name("doomed")); err != nil {
				t.Fatal(err)
			}
			if err := w0.Close(); err != nil {
				t.Fatal(err)
			}

			w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := w.Put(replaceRef, pdf.Name("new")); err != nil {
				t.Fatal(err)
			}
			if err := w.Free(freeRef); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}

			r := reopen(t, f.Data)
			obj, err := r.Get(replaceRef, true)
			if err != nil {
				t.Fatal(err)
			}
			if obj != pdf.Name("new") {
				t.Errorf("got %v, want /new", obj)
			}
			obj, err = r.Get(freeRef, true)
			if err != nil {
				t.Fatal(err)
			}
			if obj != nil {
				t.Errorf("freed object reads as %v", obj)
			}
		})
	}
}

// applyEmptyUpdate appends an update which changes nothing.
func applyEmptyUpdate(t *testing.T, f *memfile.MemFile) {
	t.Helper()
	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateID(t *testing.T) {
	t.Run("kept and refreshed", func(t *testing.T) {
		id0 := bytes.Repeat([]byte{0x11}, 16)
		f, _ := newBaseFile(t, pdf.V1_7, &pdf.WriterOptions{ID: [][]byte{id0}})
		id1 := slices.Clone(reopen(t, f.Data).GetMeta().ID[1])

		applyEmptyUpdate(t, f)
		got := reopen(t, f.Data).GetMeta().ID
		if len(got) != 2 || !bytes.Equal(got[0], id0) {
			t.Fatalf("ID[0] = %x, want %x", got, id0)
		}
		if len(got[1]) != 16 || bytes.Equal(got[1], id1) {
			t.Errorf("ID[1] = %x, want 16 fresh bytes", got[1])
		}

		applyEmptyUpdate(t, f)
		got2 := reopen(t, f.Data).GetMeta().ID
		if bytes.Equal(got2[1], got[1]) {
			t.Error("ID[1] unchanged by the second update")
		}
	})
	t.Run("2.0 gains an ID", func(t *testing.T) {
		f, _ := newBaseFile(t, pdf.V2_0, nil)
		applyEmptyUpdate(t, f)
		if got := reopen(t, f.Data).GetMeta().ID; len(got) != 2 {
			t.Errorf("ID = %x, want two strings", got)
		}
	})
	t.Run("1.4 stays without", func(t *testing.T) {
		f, _ := newBaseFile(t, pdf.V1_4, nil)
		if reopen(t, f.Data).GetMeta().ID != nil {
			t.Skip("base file unexpectedly has an ID")
		}
		applyEmptyUpdate(t, f)
		if got := reopen(t, f.Data).GetMeta().ID; got != nil {
			t.Errorf("ID = %x, want none", got)
		}
	})
}

func TestUpdateEmpty(t *testing.T) {
	f, oldRef := newBaseFile(t, pdf.V1_4, nil)
	n := len(f.Data)
	applyEmptyUpdate(t, f)
	if len(f.Data) == n {
		t.Fatal("empty update wrote nothing")
	}
	r := reopen(t, f.Data)
	if obj, _ := r.Get(oldRef, true); obj != pdf.Name("old") {
		t.Errorf("got %v, want /old", obj)
	}
	if !bytes.Contains(f.Data[n:], []byte("0 0\n")) {
		t.Error("empty table section lacks the 0 0 subsection")
	}
}

// objectHeader returns the token that starts the given object in the file.
func objectHeader(ref pdf.Reference) []byte {
	return fmt.Appendf(nil, "\n%d %d obj", ref.Number(), ref.Generation())
}

func TestUpdateCatalogUnchanged(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_4, pdf.V1_7} {
		t.Run(v.String(), func(t *testing.T) {
			f, _ := newBaseFile(t, v, nil)
			root0 := reopen(t, f.Data).GetMeta().Trailer["Root"]
			n := len(f.Data)

			w, err := pdf.NewUpdater(f, int64(n), nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := w.Put(w.Alloc(), pdf.Name("added")); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}

			root1 := reopen(t, f.Data).GetMeta().Trailer["Root"]
			if root0 != root1 {
				t.Errorf("Root changed from %v to %v", root0, root1)
			}
			if bytes.Contains(f.Data[n:], objectHeader(root0.(pdf.Reference))) {
				t.Error("unchanged catalog was written again")
			}
		})
	}
}

func TestUpdateCatalogChanged(t *testing.T) {
	f, _ := newBaseFile(t, pdf.V1_7, nil)
	root0 := reopen(t, f.Data).GetMeta().Trailer["Root"]

	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	cat := *w.GetMeta().Catalog
	cat.PageMode = "UseOutlines"
	w.GetMeta().Catalog = &cat
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r := reopen(t, f.Data)
	if r.GetMeta().Catalog.PageMode != "UseOutlines" {
		t.Errorf("PageMode = %q after update", r.GetMeta().Catalog.PageMode)
	}
	if r.GetMeta().Trailer["Root"] != root0 {
		t.Errorf("changed catalog moved from %v to %v", root0, r.GetMeta().Trailer["Root"])
	}
}

func TestUpdateInPlaceEditIsIgnored(t *testing.T) {
	f, _ := newBaseFile(t, pdf.V1_7, nil)
	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	// decoded values are immutable: an in-place edit is not written
	w.GetMeta().Catalog.PageMode = "UseOutlines"
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if got := reopen(t, f.Data).GetMeta().Catalog.PageMode; got != "" {
		t.Errorf("in-place edit was written: PageMode = %q", got)
	}
}

func TestUpdateInfo(t *testing.T) {
	f, _ := newBaseFile(t, pdf.V1_7, nil)
	if reopen(t, f.Data).GetMeta().Info != nil {
		t.Fatal("base file unexpectedly has an Info dictionary")
	}

	// add
	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	w.GetMeta().Info = &pdf.Info{Title: "first", Custom: map[string]string{"Team": "docs"}}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	r := reopen(t, f.Data)
	if r.GetMeta().Info == nil || r.GetMeta().Info.Title != "first" || r.GetMeta().Info.Custom["Team"] != "docs" {
		t.Fatalf("Info after adding: %+v", r.GetMeta().Info)
	}
	infoRef, ok := r.GetMeta().Trailer["Info"].(pdf.Reference)
	if !ok {
		t.Fatal("Info is not an indirect object")
	}

	// unchanged
	n := len(f.Data)
	applyEmptyUpdate(t, f)
	r = reopen(t, f.Data)
	if r.GetMeta().Trailer["Info"] != infoRef {
		t.Errorf("unchanged Info moved to %v", r.GetMeta().Trailer["Info"])
	}
	if bytes.Contains(f.Data[n:], objectHeader(infoRef)) {
		t.Error("unchanged Info was written again")
	}

	// change
	w, err = pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	info := *w.GetMeta().Info
	info.Title = "second"
	w.GetMeta().Info = &info
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	r = reopen(t, f.Data)
	if r.GetMeta().Info.Title != "second" || r.GetMeta().Trailer["Info"] != infoRef {
		t.Errorf("changed Info: %+v at %v", r.GetMeta().Info, r.GetMeta().Trailer["Info"])
	}

	// remove
	w, err = pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	w.GetMeta().Info = nil
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	r = reopen(t, f.Data)
	if r.GetMeta().Info != nil {
		t.Errorf("Info still present: %+v", r.GetMeta().Info)
	}
	if obj, _ := r.Get(infoRef, true); obj != nil {
		t.Errorf("old Info object not freed: %v", obj)
	}
}

func newMetadata(t testing.TB, title string) *pdf.MetadataStream {
	t.Helper()
	packet := xmp.NewPacket()
	dc := &xmp.DublinCore{}
	dc.Title.Set(language.Und, title)
	if err := packet.Set(dc); err != nil {
		t.Fatal(err)
	}
	return &pdf.MetadataStream{Data: packet}
}

func TestUpdateMetadata(t *testing.T) {
	w0, f := memfile.NewPDFWriter(t, pdf.V1_7, &pdf.WriterOptions{DocumentMetadata: newMetadata(t, "one")})
	if err := w0.Close(); err != nil {
		t.Fatal(err)
	}
	catalogMetadataRef := func(t *testing.T) pdf.Reference {
		t.Helper()
		r := reopen(t, f.Data)
		dict, err := r.Get(r.GetMeta().Trailer["Root"].(pdf.Reference), true)
		if err != nil {
			t.Fatal(err)
		}
		ref, ok := dict.(pdf.Dict)["Metadata"].(pdf.Reference)
		if !ok {
			t.Fatal("catalog has no Metadata reference")
		}
		return ref
	}
	meta0 := catalogMetadataRef(t)

	// unchanged
	n := len(f.Data)
	applyEmptyUpdate(t, f)
	if got := catalogMetadataRef(t); got != meta0 {
		t.Errorf("unchanged metadata moved from %v to %v", meta0, got)
	}
	if bytes.Contains(f.Data[n:], objectHeader(meta0)) {
		t.Error("unchanged metadata stream was written again")
	}

	// replaced
	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	cat := *w.GetMeta().Catalog
	cat.Metadata = newMetadata(t, "two")
	w.GetMeta().Catalog = &cat
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if got := catalogMetadataRef(t); got == meta0 {
		t.Error("replaced metadata keeps the old reference")
	}
	r := reopen(t, f.Data)
	var dc xmp.DublinCore
	if err := r.GetMeta().Catalog.Metadata.Data.Get(&dc); err != nil {
		t.Fatal(err)
	}
	if got := dc.Title.Best(language.Und); got != "two" {
		t.Errorf("metadata title = %q, want two", got)
	}
}

func TestUpdateEncrypted(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_4, pdf.V1_7, pdf.V2_0} {
		t.Run(v.String(), func(t *testing.T) {
			opt := &pdf.WriterOptions{UserPassword: "u", OwnerPassword: "o"}
			f, _ := newBaseFile(t, v, opt)
			n := len(f.Data)

			w, err := pdf.NewUpdater(f, int64(n), &pdf.UpdateOptions{Password: "u"})
			if err != nil {
				t.Fatal(err)
			}
			strRef := w.Alloc()
			if err := w.Put(strRef, pdf.String("top secret text")); err != nil {
				t.Fatal(err)
			}
			stmRef := w.Alloc()
			ws, err := w.OpenStream(stmRef, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ws.Write([]byte("top secret stream")); err != nil {
				t.Fatal(err)
			}
			if err := ws.Close(); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}

			if bytes.Contains(f.Data[n:], []byte("top secret")) {
				t.Error("update contains plaintext")
			}

			r, err := pdf.NewReader(bytes.NewReader(f.Data), int64(len(f.Data)), &pdf.ReaderOptions{Password: "u"})
			if err != nil {
				t.Fatal(err)
			}
			obj, err := r.Get(strRef, true)
			if err != nil {
				t.Fatal(err)
			}
			if s, _ := obj.(pdf.String); string(s) != "top secret text" {
				t.Errorf("string = %q", obj)
			}
			obj, err = r.Get(stmRef, true)
			if err != nil {
				t.Fatal(err)
			}
			stm, ok := obj.(*pdf.Stream)
			if !ok {
				t.Fatalf("stream object is %T", obj)
			}
			body, err := pdf.ReadAll(r, nil, stm, 1<<20)
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != "top secret stream" {
				t.Errorf("stream = %q", body)
			}
		})
	}
}

func TestUpdateHeaderJunk(t *testing.T) {
	base, _ := newBaseFile(t, pdf.V1_7, nil)
	f := &memfile.MemFile{Data: append([]byte("JUNK BEFORE HEADER\n"), base.Data...)}

	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	ref := w.Alloc()
	if err := w.Put(ref, pdf.Name("added")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	r := reopen(t, f.Data)
	if obj, _ := r.Get(ref, true); obj != pdf.Name("added") {
		t.Errorf("got %v, want /added", obj)
	}
	if r.GetMeta().Catalog.Pages == 0 {
		t.Error("catalog lost")
	}
}

func TestUpdateNoTrailingNewline(t *testing.T) {
	f, _ := newBaseFile(t, pdf.V1_4, nil)
	f.Data = bytes.TrimRight(f.Data, "\r\n")
	if !bytes.HasSuffix(f.Data, []byte("%%EOF")) {
		t.Fatal("base file does not end in the EOF marker")
	}
	n := len(f.Data)

	w, err := pdf.NewUpdater(f, int64(n), nil)
	if err != nil {
		t.Fatal(err)
	}
	ref := w.Alloc()
	if err := w.Put(ref, pdf.Name("added")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if f.Data[n] != '\n' {
		t.Errorf("update starts with %q, want a newline", f.Data[n])
	}
	r := reopen(t, f.Data)
	if obj, _ := r.Get(ref, true); obj != pdf.Name("added") {
		t.Errorf("got %v, want /added", obj)
	}
}

func TestUpdateChainOfThree(t *testing.T) {
	f, _ := newBaseFile(t, pdf.V1_7, nil)
	var refs []pdf.Reference
	for i := range 3 {
		w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
		if err != nil {
			t.Fatal(err)
		}
		ref := w.Alloc()
		if err := w.Put(ref, pdf.Integer(i)); err != nil {
			t.Fatal(err)
		}
		refs = append(refs, ref)
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if got := bytes.Count(f.Data, []byte("startxref")); got != 4 {
		t.Errorf("%d startxref keywords, want 4", got)
	}
	r := reopen(t, f.Data)
	for i, ref := range refs {
		if obj, _ := r.Get(ref, true); obj != pdf.Integer(i) {
			t.Errorf("object %v = %v, want %d", ref, obj, i)
		}
	}
}

func TestUpdateToMatchesInPlace(t *testing.T) {
	f, oldRef := newBaseFile(t, pdf.V1_7, nil)
	orig := bytes.Clone(f.Data)

	edit := func(t *testing.T, w *pdf.Writer) pdf.Reference {
		t.Helper()
		if err := w.Put(oldRef, pdf.Name("new")); err != nil {
			t.Fatal(err)
		}
		ref := w.Alloc()
		if err := w.Put(ref, pdf.Name("added")); err != nil {
			t.Fatal(err)
		}
		cat := *w.GetMeta().Catalog
		cat.PageMode = "UseThumbs"
		w.GetMeta().Catalog = &cat
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		return ref
	}

	wIn, err := pdf.NewUpdater(f, int64(len(orig)), nil)
	if err != nil {
		t.Fatal(err)
	}
	refIn := edit(t, wIn)

	dst := memfile.New()
	wTo, err := pdf.NewUpdaterTo(bytes.NewReader(orig), int64(len(orig)), dst, nil)
	if err != nil {
		t.Fatal(err)
	}
	refTo := edit(t, wTo)

	if refIn != refTo {
		t.Errorf("allocated %v in place but %v in copy", refIn, refTo)
	}
	if !bytes.Equal(dst.Data[:len(orig)], orig) {
		t.Error("copy does not start with the original")
	}
	for _, data := range [][]byte{f.Data, dst.Data} {
		r := reopen(t, data)
		if obj, _ := r.Get(oldRef, true); obj != pdf.Name("new") {
			t.Errorf("got %v, want /new", obj)
		}
		if obj, _ := r.Get(refIn, true); obj != pdf.Name("added") {
			t.Errorf("got %v, want /added", obj)
		}
		if r.GetMeta().Catalog.PageMode != "UseThumbs" {
			t.Errorf("PageMode = %q", r.GetMeta().Catalog.PageMode)
		}
	}
}

func TestUpdateVersionRaised(t *testing.T) {
	f, _ := newBaseFile(t, pdf.V1_4, nil)
	n := len(f.Data)
	w, err := pdf.NewUpdater(f, int64(n), &pdf.UpdateOptions{Version: pdf.V1_7})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	r := reopen(t, f.Data)
	if r.GetMeta().Version != pdf.V1_7 || r.GetMeta().Catalog.Version != pdf.V1_7 {
		t.Errorf("version %s, catalog version %s, want 1.7", r.GetMeta().Version, r.GetMeta().Catalog.Version)
	}
	// a 1.7 update uses a cross-reference stream even after a 1.4 original
	if !bytes.Contains(f.Data[n:], []byte("XRef")) {
		t.Error("raised-version update did not write a cross-reference stream")
	}
}

func TestUpdateSectionTypeFollowsVersion(t *testing.T) {
	f14, _ := newBaseFile(t, pdf.V1_4, nil)
	n := len(f14.Data)
	applyEmptyUpdate(t, f14)
	if !bytes.Contains(f14.Data[n:], []byte("trailer")) {
		t.Error("1.4 update did not write a cross-reference table")
	}

	f17, _ := newBaseFile(t, pdf.V1_7, nil)
	n = len(f17.Data)
	applyEmptyUpdate(t, f17)
	if bytes.Contains(f17.Data[n:], []byte("trailer")) {
		t.Error("1.7 update wrote a cross-reference table")
	}

	// HumanReadable forces a table
	w, err := pdf.NewUpdater(f17, int64(len(f17.Data)), &pdf.UpdateOptions{HumanReadable: true})
	if err != nil {
		t.Fatal(err)
	}
	n = len(f17.Data)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(f17.Data[n:], []byte("trailer")) {
		t.Error("HumanReadable update did not write a cross-reference table")
	}
	reopen(t, f17.Data)
}

func TestUpdateCompressedObjects(t *testing.T) {
	f, _ := newBaseFile(t, pdf.V1_7, nil)
	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	refs := []pdf.Reference{w.Alloc(), w.Alloc()}
	if err := w.WriteCompressed(refs, pdf.Name("a"), pdf.Name("b")); err != nil {
		t.Fatal(err)
	}
	if obj, err := w.Get(refs[1], true); err != nil || obj != pdf.Name("b") {
		t.Errorf("Get of compressed object before Close: %v, %v", obj, err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	r := reopen(t, f.Data)
	if obj, _ := r.Get(refs[0], true); obj != pdf.Name("a") {
		t.Errorf("got %v, want /a", obj)
	}
}

// TestUpdateOnHybridOriginal verifies that an update can be applied to a
// hybrid-reference original, whose newest section is a classic
// cross-reference table with an /XRefStm pointing at a hidden
// cross-reference stream (PDF 32000-1, 7.5.8.4).
func TestUpdateOnHybridOriginal(t *testing.T) {
	base, _ := newBaseFile(t, pdf.V1_4, nil)
	data := base.Data
	prev := lastStartXRef(t, data)
	size := trailerSize(t, data)
	hidden := size     // only listed in the cross-reference stream
	stmNum := size + 1 // the cross-reference stream object

	root, ok := reopen(t, data).GetMeta().Trailer["Root"].(pdf.Reference)
	if !ok {
		t.Fatal("base file has no direct Root reference")
	}

	buf := bytes.NewBuffer(bytes.Clone(data))
	hiddenPos := buf.Len()
	fmt.Fprintf(buf, "%d 0 obj\n/hidden\nendobj\n", hidden)

	// one type-1 entry for hidden: W [1 4 2]
	entry := []byte{1,
		byte(hiddenPos >> 24), byte(hiddenPos >> 16), byte(hiddenPos >> 8), byte(hiddenPos),
		0, 0}
	stmPos := buf.Len()
	fmt.Fprintf(buf, "%d 0 obj\n<< /Type /XRef /Size %d /W [1 4 2] /Index [%d 1] /Length %d >>\nstream\n",
		stmNum, size+2, hidden, len(entry))
	buf.Write(entry)
	buf.WriteString("\nendstream\nendobj\n")

	xrefPos := buf.Len()
	fmt.Fprintf(buf, "xref\n0 0\ntrailer\n<< /Size %d /Root %d 0 R /Prev %d /XRefStm %d >>\nstartxref\n%d\n%%%%EOF\n",
		size+2, root.Number(), prev, stmPos, xrefPos)

	f := &memfile.MemFile{Data: buf.Bytes()}

	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	ref := w.Alloc()
	if err := w.Put(ref, pdf.Name("added")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r := reopen(t, f.Data)
	if obj, err := r.Get(pdf.NewReference(hidden, 0), true); err != nil || obj != pdf.Name("hidden") {
		t.Errorf("hidden object = %v, %v, want /hidden", obj, err)
	}
	if obj, err := r.Get(ref, true); err != nil || obj != pdf.Name("added") {
		t.Errorf("added object = %v, %v, want /added", obj, err)
	}
	if r.GetMeta().Catalog.Pages == 0 {
		t.Error("catalog lost")
	}
	if got, ok := r.GetMeta().Trailer["Root"].(pdf.Reference); !ok || got != root {
		t.Errorf("Root = %v, want %v", r.GetMeta().Trailer["Root"], root)
	}
}

func TestUpdaterToGetNewObjectBeforeClose(t *testing.T) {
	f, _ := newBaseFile(t, pdf.V1_4, nil)
	orig := bytes.Clone(f.Data)
	dst := memfile.New()
	w, err := pdf.NewUpdaterTo(bytes.NewReader(orig), int64(len(orig)), dst, nil)
	if err != nil {
		t.Fatal(err)
	}
	ref := w.Alloc()
	if err := w.Put(ref, pdf.Name("added")); err != nil {
		t.Fatal(err)
	}
	obj, err := w.Get(ref, true)
	if err != nil {
		t.Fatal(err)
	}
	if obj != pdf.Name("added") {
		t.Errorf("got %v, want /added", obj)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateHeaderJunkPreservedExactly(t *testing.T) {
	base, _ := newBaseFile(t, pdf.V1_7, nil)
	combined := append([]byte("JUNK BEFORE HEADER\n"), base.Data...)
	orig := bytes.Clone(combined)
	f := &memfile.MemFile{Data: combined}

	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	ref := w.Alloc()
	if err := w.Put(ref, pdf.Name("added")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(f.Data[:len(orig)], orig) {
		t.Error("header junk not preserved exactly")
	}
}

func TestUpdateRootDirectDict(t *testing.T) {
	base, _ := newBaseFile(t, pdf.V1_4, nil)
	data := bytes.Clone(base.Data)
	pagesRef := reopen(t, data).GetMeta().Catalog.Pages
	prev := lastStartXRef(t, data)
	size := trailerSize(t, data)

	buf := bytes.NewBuffer(data)
	pos := buf.Len()
	fmt.Fprintf(buf, "xref\n0 0\ntrailer\n<< /Size %d /Root << /Type /Catalog /Pages %d 0 R >> /Prev %d >>\nstartxref\n%d\n%%%%EOF\n",
		size, pagesRef.Number(), prev, pos)
	f := &memfile.MemFile{Data: buf.Bytes()}

	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r := reopen(t, f.Data)
	if r.GetMeta().Catalog.Pages != pagesRef {
		t.Errorf("Pages = %v, want %v", r.GetMeta().Catalog.Pages, pagesRef)
	}
	if _, ok := r.GetMeta().Trailer["Root"].(pdf.Reference); !ok {
		t.Errorf("Root is %T, want pdf.Reference", r.GetMeta().Trailer["Root"])
	}
}

// TestUpdateInfoDirectDict verifies that an untouched update of an original
// whose trailer carries a direct (non-conforming but readable) /Info
// dictionary allocates a fresh indirect object, rather than writing the
// zero reference into the trailer.
func TestUpdateInfoDirectDict(t *testing.T) {
	base, _ := newBaseFile(t, pdf.V1_4, nil)
	data := bytes.Clone(base.Data)
	root, ok := reopen(t, data).GetMeta().Trailer["Root"].(pdf.Reference)
	if !ok {
		t.Fatal("base file has no direct Root reference")
	}
	prev := lastStartXRef(t, data)
	size := trailerSize(t, data)

	buf := bytes.NewBuffer(data)
	pos := buf.Len()
	fmt.Fprintf(buf, "xref\n0 0\ntrailer\n<< /Size %d /Root %d 0 R /Info << /Title (x) >> /Prev %d >>\nstartxref\n%d\n%%%%EOF\n",
		size, root.Number(), prev, pos)
	f := &memfile.MemFile{Data: buf.Bytes()}

	if info := reopen(t, f.Data).GetMeta().Info; info == nil || info.Title != "x" {
		t.Fatalf("base file Info = %+v, want direct dictionary with Title \"x\"", info)
	}

	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r := reopen(t, f.Data)
	if r.GetMeta().Info == nil || r.GetMeta().Info.Title != "x" {
		t.Fatalf("Info = %+v, want Title \"x\"", r.GetMeta().Info)
	}
	ref, ok := r.GetMeta().Trailer["Info"].(pdf.Reference)
	if !ok {
		t.Fatalf("Info is %T, want pdf.Reference", r.GetMeta().Trailer["Info"])
	}
	if ref.Number() == 0 {
		t.Errorf("Info reference is %v, want a non-zero object number", ref)
	}
}

// buildRootChainFile returns a base file whose /Root points at an object
// that is itself an indirect reference to the catalog dictionary, plus the
// reference of that extra chain link and the reference of the catalog
// dictionary it ultimately points to.  The catalog carries a document
// metadata stream, which exercises the code path that follows /Metadata
// off the resolved catalog dictionary.
func buildRootChainFile(t testing.TB, v pdf.Version) (f *memfile.MemFile, chainRef, root pdf.Reference) {
	t.Helper()
	opt := &pdf.WriterOptions{DocumentMetadata: newMetadata(t, "root-chain")}
	base, _ := newBaseFile(t, v, opt)
	data := bytes.Clone(base.Data)
	root, ok := reopen(t, data).GetMeta().Trailer["Root"].(pdf.Reference)
	if !ok {
		t.Fatal("base file has no direct Root reference")
	}
	prev := lastStartXRef(t, data)
	size := trailerSize(t, data)
	chainRef = pdf.NewReference(size, 0)

	buf := bytes.NewBuffer(data)
	chainPos := buf.Len()
	fmt.Fprintf(buf, "%d 0 obj\n%d 0 R\nendobj\n", chainRef.Number(), root.Number())
	pos := buf.Len()
	fmt.Fprintf(buf, "xref\n%d 1\n%010d %05d n\r\ntrailer\n<< /Size %d /Root %d 0 R /Prev %d >>\nstartxref\n%d\n%%%%EOF\n",
		chainRef.Number(), chainPos, 0, size+1, chainRef.Number(), prev, pos)
	return &memfile.MemFile{Data: buf.Bytes()}, chainRef, root
}

// TestUpdateRootReferenceChain regresses a panic in NewUpdater when /Root
// chains through an extra reference before reaching the catalog dictionary
// (e.g. "1 0 obj 2 0 R endobj" as the /Root object): NewUpdater must not
// panic, an empty update must reopen unchanged, and a changed catalog must
// be written at the last reference of the chain rather than as a further
// copy.  The trailer's /Root entry, which closeUpdate always reconstructs,
// collapses to that last reference directly; the intermediate chain link is
// never rewritten.
func TestUpdateRootReferenceChain(t *testing.T) {
	f, chainRef, root := buildRootChainFile(t, pdf.V1_4)
	n := len(f.Data)

	w, err := pdf.NewUpdater(f, int64(n), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(f.Data[n:], objectHeader(root)) {
		t.Error("unchanged catalog was written again")
	}
	if bytes.Contains(f.Data[n:], objectHeader(chainRef)) {
		t.Error("chain link rewritten unnecessarily")
	}

	r := reopen(t, f.Data)
	if r.GetMeta().Catalog.Pages == 0 {
		t.Error("catalog lost")
	}
	if got, ok := r.GetMeta().Trailer["Root"].(pdf.Reference); !ok || got != root {
		t.Errorf("Root = %v, want the catalog's own reference %v", r.GetMeta().Trailer["Root"], root)
	}

	// a changed catalog is written at the last reference of the chain,
	// not appended as a further link
	n = len(f.Data)
	w, err = pdf.NewUpdater(f, int64(n), nil)
	if err != nil {
		t.Fatal(err)
	}
	cat := *w.GetMeta().Catalog
	cat.PageMode = "UseOutlines"
	w.GetMeta().Catalog = &cat
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	// the object header may start right at n, without a preceding
	// newline of its own (the prior section already ends in one)
	if !bytes.Contains(f.Data[n-1:], objectHeader(root)) {
		t.Error("changed catalog not written at the chain's last reference")
	}
	if bytes.Contains(f.Data[n-1:], objectHeader(chainRef)) {
		t.Error("chain link rewritten unnecessarily")
	}

	r2 := reopen(t, f.Data)
	if r2.GetMeta().Catalog.PageMode != "UseOutlines" {
		t.Errorf("PageMode = %q after update", r2.GetMeta().Catalog.PageMode)
	}
	if got, ok := r2.GetMeta().Trailer["Root"].(pdf.Reference); !ok || got != root {
		t.Errorf("Root = %v, want the catalog's own reference %v", r2.GetMeta().Trailer["Root"], root)
	}
}

func FuzzUpdate(f *testing.F) {
	for i := range 4 {
		for _, v := range []pdf.Version{pdf.V1_1, pdf.V1_4, pdf.V1_5, pdf.V1_7, pdf.V2_0} {
			w, file := memfile.NewPDFWriter(f, v, &pdf.WriterOptions{HumanReadable: i%2 == 0})
			switch i {
			case 1:
				w.GetMeta().Info.Title = "test"
			case 2:
				w.GetMeta().Info.ModDate = pdf.Date(time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC))
			case 3:
				if err := w.Put(w.Alloc(), pdf.String("AAAAAAAAAAAAAAAAAA")); err != nil {
					f.Fatal(err)
				}
			}
			if err := w.Close(); err != nil {
				f.Fatal(err)
			}
			f.Add(bytes.Clone(file.Data))
		}
	}

	// /Root chains through an extra reference before reaching the
	// catalog (see TestUpdateRootReferenceChain)
	if chainFile, _, _ := buildRootChainFile(f, pdf.V1_4); chainFile != nil {
		f.Add(bytes.Clone(chainFile.Data))
	}

	f.Fuzz(func(t *testing.T, in []byte) {
		r1, err := pdf.NewReader(bytes.NewReader(in), int64(len(in)), nil)
		if err != nil {
			return
		}
		cat1 := r1.GetMeta().Catalog
		info1 := r1.GetMeta().Info

		file := &memfile.MemFile{Data: bytes.Clone(in)}

		// an update which changes nothing
		w, err := pdf.NewUpdater(file, int64(len(file.Data)), nil)
		if err != nil {
			return
		}
		if err := w.Close(); err != nil {
			t.Fatalf("empty update failed: %v", err)
		}

		// an update which adds one object
		w, err = pdf.NewUpdater(file, int64(len(file.Data)), nil)
		if err != nil {
			t.Fatalf("second open failed: %v", err)
		}
		ref := w.Alloc()
		if err := w.Put(ref, pdf.Name("x")); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("update failed: %v", err)
		}

		r2, err := pdf.NewReader(bytes.NewReader(file.Data), int64(len(file.Data)), nil)
		if err != nil {
			t.Fatalf("reopen failed: %v", err)
		}
		if obj, err := r2.Get(ref, true); err != nil || obj != pdf.Name("x") {
			t.Errorf("added object reads as %v, %v", obj, err)
		}
		ignore := cmpopts.IgnoreFields(pdf.Catalog{}, "Metadata")
		equateLang := cmpopts.EquateComparable(language.Tag{})
		if diff := cmp.Diff(cat1, r2.GetMeta().Catalog, ignore, equateLang); diff != "" {
			t.Errorf("catalog changed (-before +after):\n%s", diff)
		}
		if diff := cmp.Diff(info1, r2.GetMeta().Info); diff != "" {
			t.Errorf("info changed (-before +after):\n%s", diff)
		}
	})
}
