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
	"compress/zlib"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestEncryptionScopeRoundTrip writes files with different encryption
// methods for streams, strings and embedded files, and checks the
// encryption dictionary, which data is stored as plaintext, the reported
// encryption, and that all data reads back.
func TestEncryptionScopeRoundTrip(t *testing.T) {
	const (
		stmPayload = "ordinary stream payload"
		strPayload = "string payload"
		efPayload  = "embedded file payload"
	)
	type cryptEntries struct {
		StmF, StrF, EFF pdf.Object
		CF              map[pdf.Name]pdf.Name // filter name -> CFM
		AuthEvent       map[pdf.Name]pdf.Name // filter name -> AuthEvent, if given
	}
	const (
		def = pdf.CryptDefault
		id  = pdf.CryptIdentity
		rc4 = pdf.CryptRC4
		aes = pdf.CryptAES
	)
	stdCF := pdf.Name("StdCF")
	identity := pdf.Name("Identity")
	cases := []struct {
		name         string
		version      pdf.Version
		stm, str, ef pdf.CryptMethod
		want         cryptEntries
		wantEnc      pdf.Encryption
	}{
		{"V1_7/default", pdf.V1_7, def, def, def,
			cryptEntries{stdCF, stdCF, nil, map[pdf.Name]pdf.Name{"StdCF": "AESV2"}, nil},
			pdf.Encryption{Streams: aes, Strings: aes, EmbeddedFiles: aes, KeyLength: 128}},
		{"V1_7/rc4", pdf.V1_7, rc4, rc4, rc4,
			cryptEntries{stdCF, stdCF, nil, map[pdf.Name]pdf.Name{"StdCF": "V2"}, nil},
			pdf.Encryption{Streams: rc4, Strings: rc4, EmbeddedFiles: rc4, KeyLength: 128}},
		{"V1_7/attachments-only", pdf.V1_7, id, id, aes,
			cryptEntries{identity, identity, stdCF, map[pdf.Name]pdf.Name{"StdCF": "AESV2"},
				map[pdf.Name]pdf.Name{"StdCF": "EFOpen"}},
			pdf.Encryption{Streams: id, Strings: id, EmbeddedFiles: aes, KeyLength: 128}},
		{"V1_7/plaintext-embedded-files", pdf.V1_7, def, def, id,
			cryptEntries{stdCF, stdCF, identity, map[pdf.Name]pdf.Name{"StdCF": "AESV2"}, nil},
			pdf.Encryption{Streams: aes, Strings: aes, EmbeddedFiles: id, KeyLength: 128}},
		{"V1_7/strings-only", pdf.V1_7, id, aes, id,
			cryptEntries{identity, stdCF, nil, map[pdf.Name]pdf.Name{"StdCF": "AESV2"}, nil},
			pdf.Encryption{Streams: id, Strings: aes, EmbeddedFiles: id, KeyLength: 128}},
		{"V1_7/mixed", pdf.V1_7, aes, rc4, id,
			cryptEntries{stdCF, pdf.Name("StdCF2"), identity,
				map[pdf.Name]pdf.Name{"StdCF": "AESV2", "StdCF2": "V2"}, nil},
			pdf.Encryption{Streams: aes, Strings: rc4, EmbeddedFiles: id, KeyLength: 128}},
		{"V1_7/mixed-strings-first", pdf.V1_7, id, rc4, aes,
			cryptEntries{identity, stdCF, pdf.Name("StdCF2"),
				map[pdf.Name]pdf.Name{"StdCF": "V2", "StdCF2": "AESV2"},
				map[pdf.Name]pdf.Name{"StdCF2": "EFOpen"}},
			pdf.Encryption{Streams: id, Strings: rc4, EmbeddedFiles: aes, KeyLength: 128}},
		{"V2_0/default", pdf.V2_0, def, def, def,
			cryptEntries{stdCF, stdCF, nil, map[pdf.Name]pdf.Name{"StdCF": "AESV3"}, nil},
			pdf.Encryption{Streams: aes, Strings: aes, EmbeddedFiles: aes, KeyLength: 256}},
		{"V2_0/attachments-only", pdf.V2_0, id, id, def,
			cryptEntries{identity, identity, stdCF, map[pdf.Name]pdf.Name{"StdCF": "AESV3"},
				map[pdf.Name]pdf.Name{"StdCF": "EFOpen"}},
			pdf.Encryption{Streams: id, Strings: id, EmbeddedFiles: aes, KeyLength: 256}},
		{"V2_0/plaintext-streams", pdf.V2_0, id, aes, aes,
			cryptEntries{identity, stdCF, stdCF, map[pdf.Name]pdf.Name{"StdCF": "AESV3"}, nil},
			pdf.Encryption{Streams: id, Strings: aes, EmbeddedFiles: aes, KeyLength: 256}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			opt := &pdf.WriterOptions{
				UserPassword:         "user",
				OwnerPassword:        "owner",
				EncryptStreams:       c.stm,
				EncryptStrings:       c.str,
				EncryptEmbeddedFiles: c.ef,
				HumanReadable:        true, // no object streams
			}
			buf := &bytes.Buffer{}
			w, err := pdf.NewWriter(buf, c.version, opt)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(&c.wantEnc, w.GetMeta().Encryption); diff != "" {
				t.Errorf("writer Encryption (-want +got):\n%s", diff)
			}

			stmRef := w.Alloc()
			s, err := w.OpenStream(stmRef, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.Write([]byte(stmPayload)); err != nil {
				t.Fatal(err)
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			strRef := w.Alloc()
			if err := w.Put(strRef, pdf.Array{pdf.String(strPayload)}); err != nil {
				t.Fatal(err)
			}
			rm := pdf.NewResourceManager(w)
			efObj, err := rm.Embed(&file.Stream{
				WriteData: func(w io.Writer) error {
					_, err := w.Write([]byte(efPayload))
					return err
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := rm.Close(); err != nil {
				t.Fatal(err)
			}
			pagesRef := w.Alloc()
			pages := pdf.Dict{"Type": pdf.Name("Pages"), "Kids": pdf.Array{}, "Count": pdf.Integer(0)}
			if err := w.Put(pagesRef, pages); err != nil {
				t.Fatal(err)
			}
			w.GetMeta().Catalog.Pages = pagesRef
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			data := buf.Bytes()

			// data is stored as plaintext exactly for Identity
			for _, p := range []struct {
				payload string
				method  pdf.CryptMethod
			}{
				{stmPayload, c.wantEnc.Streams},
				{strPayload, c.wantEnc.Strings},
				{efPayload, c.wantEnc.EmbeddedFiles},
			} {
				plain := bytes.Contains(data, []byte(p.payload))
				if plain != (p.method == pdf.CryptIdentity) {
					t.Errorf("%q: plaintext=%t, method %s", p.payload, plain, p.method)
				}
			}

			r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)),
				&pdf.ReaderOptions{Password: "user"})
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(&c.wantEnc, r.GetMeta().Encryption); diff != "" {
				t.Errorf("reader Encryption (-want +got):\n%s", diff)
			}

			cur := pdf.NewCursor(r)
			encDict, err := cur.Dict(r.GetMeta().Trailer["Encrypt"])
			if err != nil {
				t.Fatal(err)
			}
			cfDict, err := cur.Dict(encDict["CF"])
			if err != nil {
				t.Fatal(err)
			}
			got := cryptEntries{
				StmF: encDict["StmF"],
				StrF: encDict["StrF"],
				EFF:  encDict["EFF"],
				CF:   map[pdf.Name]pdf.Name{},
			}
			for name, obj := range cfDict {
				d, err := cur.Dict(obj)
				if err != nil {
					t.Fatal(err)
				}
				got.CF[name], _ = d["CFM"].(pdf.Name)
				if ev, ok := d["AuthEvent"].(pdf.Name); ok {
					if got.AuthEvent == nil {
						got.AuthEvent = map[pdf.Name]pdf.Name{}
					}
					got.AuthEvent[name] = ev
				}
			}
			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Errorf("wrong encryption dictionary (-want +got):\n%s", diff)
			}

			stm, err := cur.Stream(stmRef)
			if err != nil {
				t.Fatal(err)
			}
			rc, err := pdf.DecodeStream(r, nil, stm)
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(rc)
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != stmPayload {
				t.Errorf("stream: got %q, want %q", body, stmPayload)
			}

			arr, err := cur.Array(strRef)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(pdf.Array{pdf.String(strPayload)}, arr); diff != "" {
				t.Errorf("string (-want +got):\n%s", diff)
			}

			ef, err := file.ExtractStream(pdf.CursorAt(pdf.NewExtractor(r), nil),
				efObj, false)
			if err != nil {
				t.Fatal(err)
			}
			efBuf := &bytes.Buffer{}
			if err := ef.WriteData(efBuf); err != nil {
				t.Fatal(err)
			}
			if efBuf.String() != efPayload {
				t.Errorf("embedded file: got %q, want %q", efBuf.String(), efPayload)
			}
		})
	}
}

// TestEncryptionScopeObjectStreams checks that strings written through
// WriteCompressed are encrypted with the string filter when streams and
// strings use different methods, with object streams enabled.
func TestEncryptionScopeObjectStreams(t *testing.T) {
	const strPayload = "compressed string payload"
	cases := []struct {
		name     string
		password string
		stm, str pdf.CryptMethod
	}{
		{"unencrypted", "", pdf.CryptDefault, pdf.CryptDefault},
		{"same filter", "user", pdf.CryptAES, pdf.CryptAES},
		{"strings-only", "user", pdf.CryptIdentity, pdf.CryptAES},
		{"mixed", "user", pdf.CryptAES, pdf.CryptRC4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			opt := &pdf.WriterOptions{
				UserPassword:   c.password,
				EncryptStreams: c.stm,
				EncryptStrings: c.str,
			}
			buf := &bytes.Buffer{}
			w, err := pdf.NewWriter(buf, pdf.V1_7, opt)
			if err != nil {
				t.Fatal(err)
			}
			strRef := w.Alloc()
			pagesRef := w.Alloc()
			pages := pdf.Dict{"Type": pdf.Name("Pages"), "Kids": pdf.Array{}, "Count": pdf.Integer(0)}
			err = w.WriteCompressed([]pdf.Reference{strRef, pagesRef},
				pdf.Array{pdf.String(strPayload)}, pages)
			if err != nil {
				t.Fatal(err)
			}
			w.GetMeta().Catalog.Pages = pagesRef
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			data := buf.Bytes()

			// unencrypted, the string sits in a flate-compressed object
			// stream, or in a plain object where no object stream is used
			plain := bytes.Contains(data, []byte(strPayload)) ||
				bytes.Contains(objStmBodies(t, data, c.password == "" || c.stm == pdf.CryptIdentity), []byte(strPayload))
			if plain != (c.password == "") {
				t.Errorf("plaintext=%t", plain)
			}
			// object streams are used exactly when streams and strings
			// share a filter
			hasObjStm := bytes.Contains(data, []byte("/ObjStm"))
			if hasObjStm != (c.stm == c.str) {
				t.Errorf("object stream=%t", hasObjStm)
			}

			r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)),
				&pdf.ReaderOptions{Password: c.password})
			if err != nil {
				t.Fatal(err)
			}
			arr, err := pdf.NewCursor(r).Array(strRef)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(pdf.Array{pdf.String(strPayload)}, arr); diff != "" {
				t.Errorf("string (-want +got):\n%s", diff)
			}
		})
	}
}

// TestEncryptionScopeObjectStreamsUpdate checks that an update of a file
// whose streams and strings use different filters writes no object stream.
func TestEncryptionScopeObjectStreamsUpdate(t *testing.T) {
	const strPayload = "compressed string payload"
	opt := &pdf.WriterOptions{
		UserPassword:   "user",
		EncryptStreams: pdf.CryptIdentity,
		EncryptStrings: pdf.CryptAES,
	}
	buf := &bytes.Buffer{}
	w, err := pdf.NewWriter(buf, pdf.V1_7, opt)
	if err != nil {
		t.Fatal(err)
	}
	pagesRef := w.Alloc()
	pages := pdf.Dict{"Type": pdf.Name("Pages"), "Kids": pdf.Array{}, "Count": pdf.Integer(0)}
	if err := w.Put(pagesRef, pages); err != nil {
		t.Fatal(err)
	}
	w.GetMeta().Catalog.Pages = pagesRef
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	base := buf.Bytes()

	out := &bytes.Buffer{}
	u, err := pdf.NewUpdaterTo(bytes.NewReader(base), int64(len(base)), out,
		&pdf.UpdateOptions{Password: "user"})
	if err != nil {
		t.Fatal(err)
	}
	strRef := u.Alloc()
	otherRef := u.Alloc()
	err = u.WriteCompressed([]pdf.Reference{strRef, otherRef},
		pdf.Array{pdf.String(strPayload)}, pdf.Dict{"X": pdf.Integer(1)})
	if err != nil {
		t.Fatal(err)
	}
	if err := u.Close(); err != nil {
		t.Fatal(err)
	}
	update := out.Bytes()[len(base):]
	if bytes.Contains(update, []byte("/ObjStm")) {
		t.Error("update uses an object stream")
	}
	if bytes.Contains(update, []byte(strPayload)) ||
		bytes.Contains(objStmBodies(t, out.Bytes(), true), []byte(strPayload)) {
		t.Error("string stored as plaintext")
	}

	r, err := pdf.NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()),
		&pdf.ReaderOptions{Password: "user"})
	if err != nil {
		t.Fatal(err)
	}
	arr, err := pdf.NewCursor(r).Array(strRef)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(pdf.Array{pdf.String(strPayload)}, arr); diff != "" {
		t.Errorf("string (-want +got):\n%s", diff)
	}
}

// objStmBodies returns the inflated bodies of all object streams in data,
// located by a sequential scan and read without decryption.  If plain is
// set, the object streams are stored without encryption and one which does
// not inflate fails the test.  Otherwise their bodies are ciphertext, and
// one which does not inflate is skipped.
func objStmBodies(t *testing.T, data []byte, plain bool) []byte {
	t.Helper()
	fi, err := pdf.SequentialScan(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var out []byte
	for _, sec := range fi.Sections {
		for _, info := range sec.ObjectStreams {
			obj, err := fi.Read(info)
			if err != nil {
				t.Fatal(err)
			}
			stm, ok := obj.(*pdf.Stream)
			if !ok {
				t.Fatalf("object stream %v is %T", info.Reference, obj)
			}
			raw, err := io.ReadAll(stm.NewReader())
			if err != nil {
				t.Fatal(err)
			}
			var body []byte
			zr, err := zlib.NewReader(bytes.NewReader(raw))
			if err == nil {
				body, err = io.ReadAll(zr)
			}
			if err != nil {
				if plain {
					t.Fatalf("object stream %v: %v", info.Reference, err)
				}
				continue
			}
			out = append(out, body...)
		}
	}
	return out
}

// TestEncryptionScopeValidation checks that NewWriter rejects invalid
// combinations of encryption methods.
func TestEncryptionScopeValidation(t *testing.T) {
	cases := []struct {
		name         string
		version      pdf.Version
		password     string
		stm, str, ef pdf.CryptMethod
		wrongVersion bool
	}{
		{"V1_5", pdf.V1_5, "pw", pdf.CryptAES, pdf.CryptAES, pdf.CryptIdentity, true},
		{"all Identity", pdf.V1_7, "pw", pdf.CryptIdentity, pdf.CryptIdentity, pdf.CryptIdentity, false},
		{"no password", pdf.V1_7, "", pdf.CryptDefault, pdf.CryptDefault, pdf.CryptIdentity, false},
		{"RC4 in PDF 2.0", pdf.V2_0, "pw", pdf.CryptDefault, pdf.CryptRC4, pdf.CryptDefault, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			opt := &pdf.WriterOptions{
				UserPassword:         c.password,
				EncryptStreams:       c.stm,
				EncryptStrings:       c.str,
				EncryptEmbeddedFiles: c.ef,
			}
			_, err := pdf.NewWriter(io.Discard, c.version, opt)
			if err == nil {
				t.Fatal("missing error")
			}
			if pdf.IsWrongVersion(err) != c.wrongVersion {
				t.Errorf("version error: got %t, want %t", pdf.IsWrongVersion(err), c.wrongVersion)
			}
		})
	}
}

// TestEncryptedWriterReadAfterClose checks that an encrypted Writer still
// decrypts objects read back from it after Close.
func TestEncryptedWriterReadAfterClose(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, &pdf.WriterOptions{UserPassword: "user"})
	ref := w.Alloc()
	if err := w.Put(ref, pdf.Dict{"S": pdf.String("hello")}); err != nil {
		t.Fatal(err)
	}
	check := func(when string) {
		t.Helper()
		d, err := pdf.NewCursor(w).Dict(ref)
		if err != nil {
			t.Fatalf("%s: %v", when, err)
		}
		if s, _ := d["S"].(pdf.String); string(s) != "hello" {
			t.Errorf("%s: got %q, want %q", when, s, "hello")
		}
	}
	check("before close")
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	check("after close")
}
