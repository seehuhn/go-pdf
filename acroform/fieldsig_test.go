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
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/optional"
)

// embedErr embeds an Embedder into a fresh writer at the given version and
// returns the resulting error.
func embedErr(t *testing.T, version pdf.Version, obj pdf.Embedder) error {
	t.Helper()
	w, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(w)
	_, err := rm.Embed(obj)
	return err
}

func TestSigFieldLockInvalidAction(t *testing.T) {
	lock := &SigFieldLock{Action: "Bogus"}
	if err := embedErr(t, pdf.V2_0, lock); err == nil {
		t.Error("expected error for invalid lock action, got nil")
	}
}

func TestSigFieldLockPVersionGating(t *testing.T) {
	lock := &SigFieldLock{Action: SigFieldLockAll, P: 2}
	if err := embedErr(t, pdf.V1_7, lock); !pdf.IsWrongVersion(err) {
		t.Errorf("expected version error for Lock P at 1.7, got %v", err)
	}
	if err := embedErr(t, pdf.V2_0, lock); err != nil {
		t.Errorf("unexpected error for Lock P at 2.0: %v", err)
	}
}

func TestSigSeedValueInvalidLockDocument(t *testing.T) {
	sv := &SigSeedValue{LockDocument: "maybe"}
	if err := embedErr(t, pdf.V2_0, sv); err == nil {
		t.Error("expected error for invalid LockDocument, got nil")
	}
}

func TestSigSeedValueVersionGating(t *testing.T) {
	tests := []struct {
		name string
		sv   *SigSeedValue
	}{
		{"DigestMethod 1.7", &SigSeedValue{DigestMethod: []pdf.Name{"SHA256"}}},
		{"MDP 1.6", &SigSeedValue{MDP: optional.NewUInt(1)}},
		{"AddRevInfo 1.7", &SigSeedValue{AddRevInfo: true}},
		{"LockDocument 2.0", &SigSeedValue{LockDocument: "auto"}},
		{"AppearanceFilter 2.0", &SigSeedValue{AppearanceFilter: "x"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := embedErr(t, pdf.V1_5, tc.sv); !pdf.IsWrongVersion(err) {
				t.Errorf("expected version error, got %v", err)
			}
		})
	}
}

func TestSigSeedValueLockEntryRequiresIndirect(t *testing.T) {
	// the Lock and SV entries shall be indirect references; Embed must allocate
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)
	ref, err := rm.Embed(&SigFieldLock{Action: SigFieldLockAll})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ref.(pdf.Reference); !ok {
		t.Errorf("expected an indirect reference, got %T", ref)
	}
}
