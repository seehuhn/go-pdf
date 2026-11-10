// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package annotation

import (
	"testing"
	"time"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/graphics/trapnet"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// trapNetRect is the annotation rectangle shared by the test cases below.
var trapNetRect = pdf.Rectangle{URx: 612, URy: 792}

// trapNetAppearance returns an appearance a trap network annotation can carry.
// A trap network needs one at every PDF version, and its normal appearance is
// the trap network itself, so the form has to hold the trap network entries.
func trapNetAppearance(rect pdf.Rectangle) *appearance.Dict {
	return &appearance.Dict{
		Normal: &form.Form{
			BBox:    rect,
			Res:     &content.Resources{},
			TrapNet: &trapnet.Attributes{PCM: trapnet.DefaultPCM},
		},
		SingleUse: true,
	}
}

// TestTrapNetEncodeValidCombinations verifies that Encode accepts valid
// field combinations for LastModified/Version/AnnotStates.
func TestTrapNetEncodeValidCombinations(t *testing.T) {
	for _, tc := range []struct {
		name string
		data TrapNet
	}{
		{
			name: "LastModified only",
			data: TrapNet{
				Common:       Common{Rect: trapNetRect, Flags: FlagPrint | FlagReadOnly, Appearance: trapNetAppearance(trapNetRect)},
				LastModified: time.Date(2023, 12, 15, 10, 30, 0, 0, time.UTC),
			},
		},
		{
			name: "Version and AnnotStates",
			data: TrapNet{
				Common:      Common{Rect: trapNetRect, Flags: FlagPrint | FlagReadOnly, Appearance: trapNetAppearance(trapNetRect)},
				Version:     []pdf.Reference{pdf.NewReference(1, 0)},
				AnnotStates: []pdf.Name{"N"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			rm := pdf.NewResourceManager(w)
			_, err := tc.data.Encode(rm)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestTrapNetEncodeInvalidCombinations verifies that Encode rejects invalid
// field combinations for LastModified/Version/AnnotStates.
func TestTrapNetEncodeInvalidCombinations(t *testing.T) {
	for _, tc := range []struct {
		name string
		data TrapNet
	}{
		{
			name: "none present",
			data: TrapNet{
				Common: Common{Rect: trapNetRect, Flags: FlagPrint | FlagReadOnly, Appearance: trapNetAppearance(trapNetRect)},
			},
		},
		{
			name: "all three present",
			data: TrapNet{
				Common:       Common{Rect: trapNetRect, Flags: FlagPrint | FlagReadOnly, Appearance: trapNetAppearance(trapNetRect)},
				LastModified: time.Date(2023, 12, 15, 10, 30, 0, 0, time.UTC),
				Version:      []pdf.Reference{pdf.NewReference(1, 0)},
				AnnotStates:  []pdf.Name{"N"},
			},
		},
		{
			name: "LastModified and Version",
			data: TrapNet{
				Common:       Common{Rect: trapNetRect, Flags: FlagPrint | FlagReadOnly, Appearance: trapNetAppearance(trapNetRect)},
				LastModified: time.Date(2023, 12, 15, 10, 30, 0, 0, time.UTC),
				Version:      []pdf.Reference{pdf.NewReference(1, 0)},
			},
		},
		{
			name: "LastModified and AnnotStates",
			data: TrapNet{
				Common:       Common{Rect: trapNetRect, Flags: FlagPrint | FlagReadOnly, Appearance: trapNetAppearance(trapNetRect)},
				LastModified: time.Date(2023, 12, 15, 10, 30, 0, 0, time.UTC),
				AnnotStates:  []pdf.Name{"N"},
			},
		},
		{
			name: "Version only",
			data: TrapNet{
				Common:  Common{Rect: trapNetRect, Flags: FlagPrint | FlagReadOnly, Appearance: trapNetAppearance(trapNetRect)},
				Version: []pdf.Reference{pdf.NewReference(1, 0)},
			},
		},
		{
			name: "AnnotStates only",
			data: TrapNet{
				Common:      Common{Rect: trapNetRect, Flags: FlagPrint | FlagReadOnly, Appearance: trapNetAppearance(trapNetRect)},
				AnnotStates: []pdf.Name{"N"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			rm := pdf.NewResourceManager(w)
			_, err := tc.data.Encode(rm)
			if err == nil {
				t.Fatal("expected error for invalid field combination")
			}
		})
	}
}

// TestTrapNetEncodeLastModifiedV13 verifies that LastModified-only fails on
// PDF 1.3 because LastModified requires PDF 1.4.
func TestTrapNetEncodeLastModifiedV13(t *testing.T) {
	tn := TrapNet{
		Rect: trapNetRect, Flags: FlagPrint | FlagReadOnly, Appearance: trapNetAppearance(trapNetRect),
		LastModified: time.Date(2023, 12, 15, 10, 30, 0, 0, time.UTC),
	}
	w, _ := memfile.NewPDFWriter(t, pdf.V1_3, nil)
	rm := pdf.NewResourceManager(w)
	_, err := tn.Encode(rm)
	if err == nil {
		t.Fatal("expected error for LastModified on PDF 1.3")
	}
}
