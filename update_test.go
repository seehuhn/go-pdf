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
	"bytes"
	"io"
	"slices"
	"testing"

	"golang.org/x/text/language"
	"seehuhn.de/go/xmp"
)

// TestUpdateEmbeddedFileUsesEFF checks that an incremental update of a file
// whose /EFF differs from /StmF encrypts new embedded file streams with /EFF.
func TestUpdateEmbeddedFileUsesEFF(t *testing.T) {
	const payload = "attachment added by the update"
	for _, eff := range []Name{"Identity", "MyCF"} {
		t.Run(string(eff), func(t *testing.T) {
			f := buildEFFTestFile(t, eff)
			out := &bytes.Buffer{}
			w, err := NewUpdaterTo(bytes.NewReader(f.data), int64(len(f.data)), out, nil)
			if err != nil {
				t.Fatal(err)
			}

			writeStream := func(ref Reference, open func(Reference) (io.WriteCloser, error)) {
				t.Helper()
				s, err := open(ref)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := s.Write([]byte(payload)); err != nil {
					t.Fatal(err)
				}
				if err := s.Close(); err != nil {
					t.Fatal(err)
				}
			}
			var embedded []Reference
			ref := w.Alloc()
			writeStream(ref, func(ref Reference) (io.WriteCloser, error) {
				return w.OpenEmbeddedFileStream(ref, nil)
			})
			embedded = append(embedded, ref)
			ref = w.Alloc()
			writeStream(ref, func(ref Reference) (io.WriteCloser, error) {
				return w.OpenStream(ref, Dict{"Type": Name("EmbeddedFile")})
			})
			embedded = append(embedded, ref)
			ref = w.Alloc()
			if err := w.Put(ref, NewStream(nil, []byte(payload)).AsEmbeddedFile()); err != nil {
				t.Fatal(err)
			}
			embedded = append(embedded, ref)
			plainRef := w.Alloc()
			writeStream(plainRef, func(ref Reference) (io.WriteCloser, error) {
				return w.OpenStream(ref, nil)
			})
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}

			r, err := NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()), nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, ref := range embedded {
				stm, err := NewCursor(r).Stream(ref)
				if err != nil {
					t.Fatal(err)
				}

				// RC4 and Identity are deterministic, so the stored bytes
				// can be predicted
				want := &bytes.Buffer{}
				ew, err := f.enc.encryptStream(f.enc.efF, ref, withDummyClose{want})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := ew.Write([]byte(payload)); err != nil {
					t.Fatal(err)
				}
				if err := ew.Close(); err != nil {
					t.Fatal(err)
				}
				raw, err := io.ReadAll(stm.NewReader())
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(raw, want.Bytes()) {
					t.Errorf("object %d not encrypted with /EFF", ref.Number())
				}

				if got := decodeAll(t, r, stm.AsEmbeddedFile()); got != payload {
					t.Errorf("object %d: got %q, want %q", ref.Number(), got, payload)
				}
			}
			stm, err := NewCursor(r).Stream(plainRef)
			if err != nil {
				t.Fatal(err)
			}
			if got := decodeAll(t, r, stm); got != payload {
				t.Errorf("got %q, want %q", got, payload)
			}
		})
	}
}

// TestUpdateEmbeddedFileOwnCrypt checks that a new embedded file stream
// with its own leading /Crypt filter uses that filter rather than /EFF.
func TestUpdateEmbeddedFileOwnCrypt(t *testing.T) {
	const payload = "attachment added by the update"
	for _, eff := range []Name{"Identity", "MyCF"} {
		t.Run(string(eff), func(t *testing.T) {
			f := buildEFFTestFile(t, eff)
			out := &bytes.Buffer{}
			w, err := NewUpdaterTo(bytes.NewReader(f.data), int64(len(f.data)), out, nil)
			if err != nil {
				t.Fatal(err)
			}
			filters := map[Reference]Filter{
				w.Alloc(): FilterCryptStandard{},
				w.Alloc(): FilterCryptIdentity{},
			}
			for ref, filter := range filters {
				s, err := w.OpenEmbeddedFileStream(ref, nil, filter)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := s.Write([]byte(payload)); err != nil {
					t.Fatal(err)
				}
				if err := s.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}

			r, err := NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()), nil)
			if err != nil {
				t.Fatal(err)
			}
			for ref, filter := range filters {
				stm, err := NewCursor(r).Stream(ref)
				if err != nil {
					t.Fatal(err)
				}
				raw, err := io.ReadAll(stm.NewReader())
				if err != nil {
					t.Fatal(err)
				}
				_, isIdentity := filter.(FilterCryptIdentity)
				if isIdentity != bytes.Equal(raw, []byte(payload)) {
					t.Errorf("object %d: wrong stored form for %T", ref.Number(), filter)
				}
				if got := decodeAll(t, r, stm.AsEmbeddedFile()); got != payload {
					t.Errorf("object %d: got %q, want %q", ref.Number(), got, payload)
				}
			}
		})
	}
}

// TestUpdateDanglingEncrypt checks that a dangling /Encrypt entry is not
// carried into an update, where the object number it names may be reused.
func TestUpdateDanglingEncrypt(t *testing.T) {
	// the dangling number lies just above the two objects of the file, so
	// that the update would reach it after a few allocations
	data := buildDanglingEncryptFile(t, []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [] /Count 0 >>",
	}, "5 0 R")

	out := &bytes.Buffer{}
	w, err := NewUpdaterTo(bytes.NewReader(data), int64(len(data)), out, nil)
	if err != nil {
		t.Fatal(err)
	}
	var refs []Reference
	for range 4 {
		ref := w.Alloc()
		if err := w.Put(ref, Dict{"Filter": Name("Standard"), "V": Integer(1)}); err != nil {
			t.Fatal(err)
		}
		refs = append(refs, ref)
	}
	if slices.Contains(refs, NewReference(5, 0)) {
		t.Errorf("dangling object number 5 reused")
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()), nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.GetMeta().Encryption != nil {
		t.Error("updated file is encrypted")
	}
	if _, ok := r.GetMeta().Trailer["Encrypt"]; ok {
		t.Error("updated trailer has /Encrypt")
	}
}

// TestUpdatePutEncryptedStream checks that a stream read from an encrypted
// file can be written back to the same file in an update, and that one read
// from another encrypted file is refused.
func TestUpdatePutEncryptedStream(t *testing.T) {
	data, payload := buildNamedCFFile(t)

	out := &bytes.Buffer{}
	w, err := NewUpdaterTo(bytes.NewReader(data), int64(len(data)), out, nil)
	if err != nil {
		t.Fatal(err)
	}
	newRefs := map[uint32]Reference{}
	for num := range payload {
		stm, err := NewCursor(w).Stream(NewReference(num, 0))
		if err != nil {
			t.Fatal(err)
		}
		ref := w.Alloc()
		if err := w.Put(ref, stm); err != nil {
			t.Fatal(err)
		}
		newRefs[num] = ref
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()), nil)
	if err != nil {
		t.Fatal(err)
	}
	for num, ref := range newRefs {
		stm, err := NewCursor(r).Stream(ref)
		if err != nil {
			t.Fatal(err)
		}
		if got := decodeAll(t, r, stm); got != payload[num] {
			t.Errorf("object %d: got %q, want %q", num, got, payload[num])
		}
	}

	src, err := NewReader(bytes.NewReader(data), int64(len(data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	stm, err := NewCursor(src).Stream(NewReference(namedCFDefRef, 0))
	if err != nil {
		t.Fatal(err)
	}
	for _, opt := range []*WriterOptions{nil, {UserPassword: "x"}} {
		w, err := NewWriter(io.Discard, V1_7, opt)
		if err != nil {
			t.Fatal(err)
		}
		if err := w.Put(w.Alloc(), stm); err == nil {
			t.Error("foreign encrypted stream accepted")
		}
	}
}

// TestUpdatePlaintextMetadataPut checks that an update of a file with
// /EncryptMetadata false stores a replacement written at the original
// metadata reference as plaintext, as the reader expects.
func TestUpdatePlaintextMetadataPut(t *testing.T) {
	packet := xmp.NewPacket()
	dc := &xmp.DublinCore{}
	dc.Title.Set(language.Und, "original")
	if err := packet.Set(dc); err != nil {
		t.Fatal(err)
	}
	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, V2_0, &WriterOptions{
		UserPassword:     "user",
		DocumentMetadata: &MetadataStream{Data: packet, Plaintext: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := addPage(w); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	base := buf.Bytes()

	metaRef := func(r *Reader) Reference {
		t.Helper()
		cat, err := NewCursor(r).Dict(r.GetMeta().Trailer["Root"])
		if err != nil {
			t.Fatal(err)
		}
		ref, ok := cat["Metadata"].(Reference)
		if !ok {
			t.Fatalf("Metadata is %T, want Reference", cat["Metadata"])
		}
		return ref
	}
	opt := &UpdateOptions{Password: "user"}
	r, err := NewReader(bytes.NewReader(base), int64(len(base)), &ReaderOptions{Password: "user"})
	if err != nil {
		t.Fatal(err)
	}
	ref := metaRef(r)

	out := &bytes.Buffer{}
	u, err := NewUpdaterTo(bytes.NewReader(base), int64(len(base)), out, opt)
	if err != nil {
		t.Fatal(err)
	}
	const body = "<?xpacket begin='' id='W5M0MpCehiHzreSzNTczkc9d'?>replaced<?xpacket end='w'?>"
	stm := NewStream(Dict{"Type": Name("Metadata"), "Subtype": Name("XML")}, []byte(body))
	if err := u.Put(ref, stm); err != nil {
		t.Fatal(err)
	}
	if err := u.Close(); err != nil {
		t.Fatal(err)
	}

	upd := out.Bytes()
	r2, err := NewReader(bytes.NewReader(upd), int64(len(upd)), &ReaderOptions{Password: "user"})
	if err != nil {
		t.Fatal(err)
	}
	if got := metaRef(r2); got != ref {
		t.Fatalf("metadata moved to %v", got)
	}
	stm2, err := NewCursor(r2).Stream(ref)
	if err != nil {
		t.Fatal(err)
	}
	if got := decodeAll(t, r2, stm2); got != body {
		t.Errorf("metadata read back wrong: got %q, want %q", got, body)
	}
}

// TestUpdatePutIndirectCrypt checks that a stream from an unencrypted file
// whose indirect filter chain starts with a named /Crypt filter can be
// written back in an update, with the entry dropped.
func TestUpdatePutIndirectCrypt(t *testing.T) {
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [] /Count 0 >>",
		"<< /Length 3 /Filter 4 0 R /DecodeParms 5 0 R >>\nstream\nabc\nendstream",
		"[/Crypt]",
		"[<< /Name /StdCF >>]",
	}
	data := buildRawPDF(objs, 0)
	stmRef := NewReference(3, 0)

	out := &bytes.Buffer{}
	w, err := NewUpdaterTo(bytes.NewReader(data), int64(len(data)), out, nil)
	if err != nil {
		t.Fatal(err)
	}
	stm, err := NewCursor(w).Stream(stmRef)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Put(stmRef, stm); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()), nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := NewCursor(r).Stream(stmRef)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []Name{"Filter", "DecodeParms"} {
		if val, ok := got.Dict[key]; ok {
			t.Errorf("unexpected /%s %v", key, val)
		}
	}
	if s := decodeAll(t, r, got); s != "abc" {
		t.Errorf("got %q, want %q", s, "abc")
	}
}
