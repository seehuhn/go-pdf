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
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestCopierEmbeddedFileEFF checks that the Copier decrypts untyped embedded
// file streams, referenced from /EF and /RF, with the source's /EFF, and
// stores them using the destination's crypt filter for embedded files.
func TestCopierEmbeddedFileEFF(t *testing.T) {
	dests := []struct {
		name string
		opt  WriterOptions
	}{
		{"unencrypted", WriterOptions{}},
		{"encrypted", WriterOptions{UserPassword: "dst"}},
		{"plaintext-embedded-files", WriterOptions{
			UserPassword:         "dst",
			EncryptEmbeddedFiles: CryptIdentity,
		}},
	}
	for _, eff := range []Name{"Identity", "MyCF"} {
		for _, dest := range dests {
			t.Run(fmt.Sprintf("%s/%s", eff, dest.name), func(t *testing.T) {
				f := buildEFFTestFile(t, eff)
				src, err := NewReader(bytes.NewReader(f.data), int64(len(f.data)), nil)
				if err != nil {
					t.Fatal(err)
				}

				opt := dest.opt
				opt.HumanReadable = true // no object streams
				buf := &bytes.Buffer{}
				w, err := NewWriter(buf, V1_6, &opt)
				if err != nil {
					t.Fatal(err)
				}
				copier := NewCopier(w, src)
				specRef, err := copier.CopyReference(f.specRef)
				if err != nil {
					t.Fatal(err)
				}
				stmRef, err := copier.CopyReference(f.stmRef)
				if err != nil {
					t.Fatal(err)
				}
				if err := addPage(w); err != nil {
					t.Fatal(err)
				}
				if err := w.Close(); err != nil {
					t.Fatal(err)
				}

				// embedded files are stored as plaintext exactly when the
				// destination does not encrypt them
				efPlain := dest.opt.UserPassword == "" || dest.opt.EncryptEmbeddedFiles == CryptIdentity
				for _, ref := range []Reference{f.efRef, f.rfRef} {
					if got := bytes.Contains(buf.Bytes(), []byte(f.payload[ref])); got != efPlain {
						t.Errorf("%q stored as plaintext: %t, want %t", f.payload[ref], got, efPlain)
					}
				}
				if got := bytes.Contains(buf.Bytes(), []byte(f.payload[f.stmRef])); got != (dest.opt.UserPassword == "") {
					t.Errorf("ordinary stream stored as plaintext: %t", got)
				}

				dst, err := NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()),
					&ReaderOptions{Password: "dst"})
				if err != nil {
					t.Fatal(err)
				}
				c := NewCursor(dst)
				spec, err := c.Dict(specRef)
				if err != nil {
					t.Fatal(err)
				}
				ef, err := c.Dict(spec["EF"])
				if err != nil {
					t.Fatal(err)
				}
				rf, err := c.Dict(spec["RF"])
				if err != nil {
					t.Fatal(err)
				}
				rfArr, err := c.Array(rf["F"])
				if err != nil {
					t.Fatal(err)
				}
				if len(rfArr) != 2 {
					t.Fatalf("got %d /RF entries, want 2", len(rfArr))
				}
				for ref, want := range map[Object]string{
					ef["F"]:  f.payload[f.efRef],
					rfArr[1]: f.payload[f.rfRef],
				} {
					stm, err := c.Stream(ref)
					if err != nil {
						t.Fatal(err)
					}
					if got := decodeAll(t, dst, stm.AsEmbeddedFile()); got != want {
						t.Errorf("got %q, want %q", got, want)
					}
				}
				stm, err := c.Stream(stmRef)
				if err != nil {
					t.Fatal(err)
				}
				if got := decodeAll(t, dst, stm); got != f.payload[f.stmRef] {
					t.Errorf("got %q, want %q", got, f.payload[f.stmRef])
				}
			})
		}
	}
}

// TestCopierUnencryptedSourceCrypt checks copying a stream with a named
// /Crypt filter from an unencrypted source.  The reader drops the entry, so
// the copy is encrypted with the destination's default filter.
func TestCopierUnencryptedSourceCrypt(t *testing.T) {
	const payload = "stream with a named crypt filter"
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [] /Count 0 >>",
		fmt.Sprintf("<< /Length %d /Filter /Crypt /DecodeParms << /Name /StdCF >> >>\n"+
			"stream\n%s\nendstream", len(payload), payload),
	}
	data := buildRawPDF(objs, 0)
	src, err := NewReader(bytes.NewReader(data), int64(len(data)), nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, encrypted := range []bool{true, false} {
		t.Run(fmt.Sprintf("encrypted=%t", encrypted), func(t *testing.T) {
			opt := &WriterOptions{}
			if encrypted {
				opt.UserPassword = "dst"
			}
			buf := &bytes.Buffer{}
			w, err := NewWriter(buf, V1_6, opt)
			if err != nil {
				t.Fatal(err)
			}
			ref, err := NewCopier(w, src).CopyReference(NewReference(3, 0))
			if err != nil {
				t.Fatal(err)
			}
			if err := addPage(w); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}

			dst, err := NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()),
				&ReaderOptions{Password: "dst"})
			if err != nil {
				t.Fatal(err)
			}
			stm, err := NewCursor(dst).Stream(ref)
			if err != nil {
				t.Fatal(err)
			}
			// the reader drops the named filter, since the source is not
			// encrypted; the copy uses the destination's default
			if f := stm.Dict["Filter"]; f != nil {
				t.Errorf("unexpected /Filter %v", f)
			}
			if p := stm.Dict["DecodeParms"]; p != nil {
				t.Errorf("unexpected /DecodeParms %v", p)
			}
			if got := decodeAll(t, dst, stm); got != payload {
				t.Errorf("got %q, want %q", got, payload)
			}
		})
	}
}

// TestCopierStreamInBothRoles checks that a stream reached both as an
// embedded file and as an ordinary stream is copied once per role when the
// destination encrypts the two roles differently, and once otherwise.  The
// ordinary reference is copied first, so the role cannot be known when the
// stream is first seen.
func TestCopierStreamInBothRoles(t *testing.T) {
	const payload = "stream used as an embedded file and as an ordinary stream"
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [] /Count 0 >>",
		"<< /Type /Filespec /F (a.txt) /EF << /F 4 0 R >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(payload), payload),
	}
	data := buildRawPDF(objs, 0)
	srcSpecRef := NewReference(3, 0)
	srcStmRef := NewReference(4, 0)

	dests := []struct {
		name      string
		opt       WriterOptions
		twoCopies bool
	}{
		{"unencrypted", WriterOptions{}, false},
		{"same-filter", WriterOptions{UserPassword: "dst"}, false},
		{"different-filters", WriterOptions{
			UserPassword:         "dst",
			EncryptStreams:       CryptRC4,
			EncryptEmbeddedFiles: CryptAES,
		}, true},
	}
	for _, dest := range dests {
		t.Run(dest.name, func(t *testing.T) {
			src, err := NewReader(bytes.NewReader(data), int64(len(data)), nil)
			if err != nil {
				t.Fatal(err)
			}

			opt := dest.opt
			buf := &bytes.Buffer{}
			w, err := NewWriter(buf, V1_6, &opt)
			if err != nil {
				t.Fatal(err)
			}
			copier := NewCopier(w, src)
			plainRef, err := copier.CopyReference(srcStmRef)
			if err != nil {
				t.Fatal(err)
			}
			specRef, err := copier.CopyReference(srcSpecRef)
			if err != nil {
				t.Fatal(err)
			}
			if err := addPage(w); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}

			dst, err := NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()),
				&ReaderOptions{Password: "dst"})
			if err != nil {
				t.Fatal(err)
			}
			c := NewCursor(dst)
			spec, err := c.Dict(specRef)
			if err != nil {
				t.Fatal(err)
			}
			ef, err := c.Dict(spec["EF"])
			if err != nil {
				t.Fatal(err)
			}
			efRef, ok := ef["F"].(Reference)
			if !ok {
				t.Fatalf("/EF /F is %T, want Reference", ef["F"])
			}
			if (efRef != plainRef) != dest.twoCopies {
				t.Errorf("embedded file copy %v, plain copy %v, want two copies: %t",
					efRef, plainRef, dest.twoCopies)
			}

			want := payload
			stm, err := c.Stream(efRef)
			if err != nil {
				t.Fatal(err)
			}
			if got := decodeAll(t, dst, stm.AsEmbeddedFile()); got != want {
				t.Errorf("embedded file: got %q, want %q", got, want)
			}
			stm, err = c.Stream(plainRef)
			if err != nil {
				t.Fatal(err)
			}
			if got := decodeAll(t, dst, stm); got != want {
				t.Errorf("ordinary stream: got %q, want %q", got, want)
			}
		})
	}
}

// TestCopierSourceRoles checks that a stream which the source encrypts as
// an embedded file is decrypted as such when reached through /EF, even if
// it was reached as an ordinary stream first and the destination does not
// distinguish the two roles.
func TestCopierSourceRoles(t *testing.T) {
	const payload = "embedded file also referenced as an ordinary stream"

	srcBuf := &bytes.Buffer{}
	w, err := NewWriter(srcBuf, V1_7, &WriterOptions{
		UserPassword:         "src",
		EncryptStreams:       CryptIdentity,
		EncryptStrings:       CryptIdentity,
		EncryptEmbeddedFiles: CryptAES,
		HumanReadable:        true,
	})
	if err != nil {
		t.Fatal(err)
	}
	srcStmRef := w.Alloc()
	s, err := w.OpenEmbeddedFileStream(srcStmRef, nil) // no /Type
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Write([]byte(payload)); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	srcSpecRef := w.Alloc()
	err = w.Put(srcSpecRef, Dict{
		"Type": Name("Filespec"),
		"F":    String("a.txt"),
		"EF":   Dict{"F": srcStmRef},
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

	for _, plainFirst := range []bool{true, false} {
		t.Run(fmt.Sprintf("plainFirst=%t", plainFirst), func(t *testing.T) {
			src, err := NewReader(bytes.NewReader(srcBuf.Bytes()), int64(srcBuf.Len()),
				&ReaderOptions{Password: "src"})
			if err != nil {
				t.Fatal(err)
			}
			buf := &bytes.Buffer{}
			w, err := NewWriter(buf, V1_7, &WriterOptions{UserPassword: "dst", HumanReadable: true})
			if err != nil {
				t.Fatal(err)
			}
			copier := NewCopier(w, src)
			var plainRef, specRef Reference
			if plainFirst {
				plainRef, err = copier.CopyReference(srcStmRef)
				if err != nil {
					t.Fatal(err)
				}
			}
			specRef, err = copier.CopyReference(srcSpecRef)
			if err != nil {
				t.Fatal(err)
			}
			if !plainFirst {
				plainRef, err = copier.CopyReference(srcStmRef)
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := addPage(w); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}

			dst, err := NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()),
				&ReaderOptions{Password: "dst"})
			if err != nil {
				t.Fatal(err)
			}
			c := NewCursor(dst)
			spec, err := c.Dict(specRef)
			if err != nil {
				t.Fatal(err)
			}
			ef, err := c.Dict(spec["EF"])
			if err != nil {
				t.Fatal(err)
			}
			efRef, ok := ef["F"].(Reference)
			if !ok {
				t.Fatalf("/EF /F is %T, want Reference", ef["F"])
			}
			// the two roles decrypt the source differently, so they
			// cannot share a copy
			if efRef == plainRef {
				t.Errorf("embedded file and ordinary stream share copy %v", efRef)
			}
			stm, err := c.Stream(efRef)
			if err != nil {
				t.Fatal(err)
			}
			if got := decodeAll(t, dst, stm.AsEmbeddedFile()); got != payload {
				t.Errorf("embedded file: got %q, want %q", got, payload)
			}
		})
	}
}

// TestCopierInlinesFilterRefs checks that an indirect /Filter or
// /DecodeParms entry of a copied stream is inlined, and the referenced
// object is not copied on its own.
func TestCopierInlinesFilterRefs(t *testing.T) {
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [] /Count 0 >>",
		"<< /Length 3 /Filter 4 0 R /DecodeParms 5 0 R >>\nstream\nabc\nendstream",
		"[/ASCIIHexDecode]",
		"[null]",
	}
	data := buildRawPDF(objs, 0)
	src, err := NewReader(bytes.NewReader(data), int64(len(data)), nil)
	if err != nil {
		t.Fatal(err)
	}

	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, V1_7, &WriterOptions{HumanReadable: true})
	if err != nil {
		t.Fatal(err)
	}
	ref, err := NewCopier(w, src).CopyReference(NewReference(3, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := addPage(w); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	fi, err := SequentialScan(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, sec := range fi.Sections {
		for _, info := range sec.Objects {
			obj, err := fi.Read(info)
			if err != nil {
				t.Fatal(err)
			}
			if _, isArray := obj.(Array); isArray {
				t.Errorf("filter entry copied as object %v", info.Reference)
			}
		}
	}

	r, err := NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()), nil)
	if err != nil {
		t.Fatal(err)
	}
	stm, err := NewCursor(r).Stream(ref)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(Array{Name("ASCIIHexDecode")}, stm.Dict["Filter"]); diff != "" {
		t.Errorf("wrong /Filter (-want +got):\n%s", diff)
	}
}
