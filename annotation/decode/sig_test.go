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

package decode

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/mock"
	"seehuhn.de/go/pdf/optional"
)

var sigSeedValueTestCases = []struct {
	name    string
	version pdf.Version
	sv      *acroform.SigSeedValue
}{
	{
		name:    "empty",
		version: pdf.V1_5,
		sv:      &acroform.SigSeedValue{},
	},
	{
		name:    "handler",
		version: pdf.V1_5,
		sv: &acroform.SigSeedValue{
			Flags:     acroform.SigSeedFilter | acroform.SigSeedSubFilter,
			Filter:    "Adobe.PPKLite",
			SubFilter: []pdf.Name{"adbe.pkcs7.detached", "ETSI.CAdES.detached"},
			V:         2,
			Reasons:   []string{"I approve this document", "I am the author"},
		},
	},
	{
		name:    "timestamp",
		version: pdf.V1_6,
		sv: &acroform.SigSeedValue{
			TimeStamp: &acroform.SigSeedValueTimeStamp{
				URL: "https://timestamp.example.com/tsa",
			},
		},
	},
	{
		name:    "timestamp_required",
		version: pdf.V1_6,
		sv: &acroform.SigSeedValue{
			TimeStamp: &acroform.SigSeedValueTimeStamp{
				URL:      "https://timestamp.example.com/tsa",
				Required: true,
			},
			LegalAttestation: []string{"no changes allowed"},
			MDP:              optional.NewUInt(2),
		},
	},
	{
		name:    "certificate",
		version: pdf.V1_7,
		sv: &acroform.SigSeedValue{
			DigestMethod: []pdf.Name{"SHA256", "SHA512"},
			AddRevInfo:   true,
			Cert: &acroform.SigCertSeedValue{
				Flags:     acroform.SigCertSubject,
				Subject:   [][]byte{{0x30, 0x82, 0x01, 0x0a}},
				Issuer:    [][]byte{{0x30, 0x82, 0x02, 0x0b}},
				OID:       [][]byte{[]byte("2.16.840.1.113733.1.7.1.1")},
				SubjectDN: []map[pdf.Name]string{{"CN": "Test Signer", "O": "Example Ltd"}},
				KeyUsage:  []string{"1XXXXXXXX"},
				URL:       "https://ca.example.com/enroll",
				URLType:   "Browser",
			},
		},
	},
	{
		name:    "all_fields",
		version: pdf.V2_0,
		sv: &acroform.SigSeedValue{
			Flags:            acroform.SigSeedFilter | acroform.SigSeedLockDocument | acroform.SigSeedAppearanceFilter,
			Filter:           "Adobe.PPKLite",
			SubFilter:        []pdf.Name{"ETSI.CAdES.detached"},
			DigestMethod:     []pdf.Name{"SHA384"},
			V:                3,
			Reasons:          []string{"I approve this document"},
			MDP:              optional.NewUInt(3),
			TimeStamp:        &acroform.SigSeedValueTimeStamp{URL: "https://timestamp.example.com/tsa", Required: true},
			LegalAttestation: []string{"attestation one", "attestation two"},
			AddRevInfo:       true,
			LockDocument:     "auto",
			AppearanceFilter: "Formal Signature",
		},
	},
}

// roundTripSigSeedValue writes sv to a new file, reads it back and checks that
// the two agree.
func roundTripSigSeedValue(t *testing.T, version pdf.Version, sv *acroform.SigSeedValue) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(t, version, nil)
	rm := pdf.NewResourceManager(w)

	ref, err := rm.Embed(sv)
	if err != nil {
		if pdf.IsWrongVersion(err) {
			t.Skip("version not supported")
		}
		t.Fatalf("embed failed: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatalf("rm.Close failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("w.Close failed: %v", err)
	}

	decoded, err := pdf.Decode(pdf.NewCursor(w), ref, sigSeedValue)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	if diff := cmp.Diff(sv, decoded); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func TestSigSeedValueRoundTrip(t *testing.T) {
	for _, tc := range sigSeedValueTestCases {
		t.Run(tc.name, func(t *testing.T) {
			roundTripSigSeedValue(t, tc.version, tc.sv)
		})
	}
}

func FuzzSigSeedValueRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{HumanReadable: true}
	for _, tc := range sigSeedValueTestCases {
		w, buf := memfile.NewPDFWriter(f, tc.version, opt)
		rm := pdf.NewResourceManager(w)
		embedded, err := rm.Embed(tc.sv)
		if err != nil {
			continue
		}
		if err := rm.Close(); err != nil {
			continue
		}
		w.GetMeta().Trailer["Quir:E"] = embedded
		if err := w.Close(); err != nil {
			continue
		}
		f.Add(buf.Data)
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}
		defer r.Close()

		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("missing test object")
		}

		sv, err := pdf.Decode(pdf.NewCursor(r), obj, sigSeedValue)
		if err != nil {
			t.Skip("malformed object")
		}

		// reading accepts versions the writer cannot reproduce
		version := pdf.GetVersion(r)
		if !version.IsSupported() {
			t.Skip("version cannot be written")
		}

		roundTripSigSeedValue(t, version, sv)
	})
}

// TestSigSeedValueTimeStampWithoutURL checks that a timestamp dictionary which
// names no server is dropped.  There is no server to fall back on, and keeping
// the dictionary would yield a seed value which cannot be written back.
func TestSigSeedValueTimeStampWithoutURL(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)

	for _, ts := range []pdf.Dict{
		{"Ff": pdf.Integer(1)},
		{"URL": pdf.String(""), "Ff": pdf.Integer(1)},
	} {
		sv, err := sigSeedValue(c, pdf.Dict{"TimeStamp": ts}, false)
		if err != nil {
			t.Fatalf("%v: read failed: %v", ts, err)
		}
		if sv.TimeStamp != nil {
			t.Errorf("%v: got TimeStamp %+v, want none", ts, sv.TimeStamp)
		}
	}
}

// Malformed array elements are skipped rather than read as empty values, and
// an array without valid elements reads as absent.
func TestSigArraysSkipMalformed(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)

	sv, err := sigSeedValue(c, pdf.Dict{
		"Reasons":   pdf.Array{pdf.TextString("a"), pdf.Integer(1), pdf.TextString(".")},
		"SubFilter": pdf.Array{pdf.Integer(1), pdf.Integer(2)},
		"Cert": pdf.Dict{
			"Subject":   pdf.Array{pdf.Integer(1), pdf.String("s")},
			"SubjectDN": pdf.Array{pdf.Dict{"CN": pdf.Integer(1)}, pdf.Integer(2)},
			"KeyUsage":  pdf.Array{pdf.Name("x")},
		},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{"a", "."}, sv.Reasons); diff != "" {
		t.Errorf("Reasons (-want +got):\n%s", diff)
	}
	if sv.SubFilter != nil {
		t.Errorf("SubFilter = %v, want nil", sv.SubFilter)
	}
	if diff := cmp.Diff([][]byte{[]byte("s")}, sv.Cert.Subject); diff != "" {
		t.Errorf("Subject (-want +got):\n%s", diff)
	}
	if sv.Cert.SubjectDN != nil {
		t.Errorf("SubjectDN = %v, want nil", sv.Cert.SubjectDN)
	}
	if sv.Cert.KeyUsage != nil {
		t.Errorf("KeyUsage = %v, want nil", sv.Cert.KeyUsage)
	}
}

// The signature policy entries are ignored without a policy OID.
func TestSigCertPolicyWithoutOID(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)

	cert, err := sigCertSeedValue(c, pdf.Dict{
		"SignaturePolicyHashValue":      pdf.String("hash"),
		"SignaturePolicyHashAlgorithm":  pdf.Name("SHA256"),
		"SignaturePolicyCommitmentType": pdf.Array{pdf.String("1.2.3")},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if cert.SignaturePolicyHashValue != nil || cert.SignaturePolicyHashAlgorithm != "" ||
		cert.SignaturePolicyCommitmentType != nil {
		t.Errorf("policy entries kept without OID: %+v", cert)
	}
}

// A lock dictionary without a valid Action, or an Include / Exclude lock
// without Fields, is dropped instead of being repaired.
func TestSigFieldLockInvalid(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)

	for _, dict := range []pdf.Dict{
		{},
		{"Action": pdf.Name("Bogus")},
		{"Action": pdf.Name("Include")},
		{"Action": pdf.Name("Exclude"), "Fields": pdf.Integer(1)},
	} {
		lock, err := pdf.DecodeOptional(c, dict, sigFieldLock)
		if err != nil {
			t.Errorf("%v: unexpected error %v", dict, err)
		}
		if lock != nil {
			t.Errorf("%v: got %+v, want nil", dict, lock)
		}
	}

	lock, err := sigFieldLock(c, pdf.Dict{"Action": pdf.Name("Exclude"), "Fields": pdf.Array{}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if lock.Action != acroform.SigFieldLockExclude || lock.Fields != nil {
		t.Errorf("got %+v, want an Exclude lock without fields", lock)
	}
}
