// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cmap

import (
	"testing"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestIsStandardCollection(t *testing.T) {
	for _, tc := range []struct {
		ros  *cid.SystemInfo
		want bool
	}{
		{nil, false},
		{&cid.SystemInfo{Registry: "Adobe", Ordering: "CNS1", Supplement: 7}, true},
		{&cid.SystemInfo{Registry: "Adobe", Ordering: "GB1", Supplement: 5}, true},
		{&cid.SystemInfo{Registry: "Adobe", Ordering: "Japan1", Supplement: 7}, true},
		{&cid.SystemInfo{Registry: "Adobe", Ordering: "Japan2", Supplement: 0}, true},
		{&cid.SystemInfo{Registry: "Adobe", Ordering: "KR", Supplement: 9}, true},
		{&cid.SystemInfo{Registry: "Adobe", Ordering: "Korea1", Supplement: 2}, true},
		{&cid.SystemInfo{Registry: "Adobe", Ordering: "Identity", Supplement: 0}, false},
		{&cid.SystemInfo{Registry: "Quire", Ordering: "Japan1", Supplement: 0}, false},
		{&cid.SystemInfo{Registry: "Adobe", Ordering: "japan1", Supplement: 0}, false},
	} {
		if got := IsStandardCollection(tc.ros); got != tc.want {
			t.Errorf("%v: got %t, want %t", tc.ros, got, tc.want)
		}
	}
}

// TestPredefinedCollections checks that every predefined CMap except
// Identity-H and Identity-V uses a standardised character collection, so
// that all of them remain usable with non-embedded fonts.
func TestPredefinedCollections(t *testing.T) {
	for _, name := range allPredefined {
		c, err := Predefined(name)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := IsStandardCollection(c.ROS), !c.IsIdentity(); got != want {
			t.Errorf("%s: %v standard=%t, want %t", name, c.ROS, got, want)
		}
	}
}

func TestCheckCollectionVersion(t *testing.T) {
	kr := &cid.SystemInfo{Registry: "Adobe", Ordering: "KR", Supplement: 9}
	japan1 := &cid.SystemInfo{Registry: "Adobe", Ordering: "Japan1", Supplement: 7}
	for _, tc := range []struct {
		ros          *cid.SystemInfo
		v            pdf.Version
		wrongVersion bool
	}{
		{kr, pdf.V1_7, true},
		{kr, pdf.V2_0, false},
		{japan1, pdf.V1_7, false},
		{japan1, pdf.V2_0, false},
	} {
		w, _ := memfile.NewPDFWriter(t, tc.v, nil)
		err := CheckCollectionVersion(w, tc.ros)
		if got := pdf.IsWrongVersion(err); got != tc.wrongVersion || got != (err != nil) {
			t.Errorf("%v, PDF %s: got error %v", tc.ros, tc.v, err)
		}
	}
}
