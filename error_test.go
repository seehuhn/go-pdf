// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package pdf_test

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestIsWrongVersion checks that a VersionError is recognised whichever
// of its bounds are set.
func TestIsWrongVersion(t *testing.T) {
	for _, err := range []*pdf.VersionError{
		{Operation: "feature X", Earliest: pdf.V1_4},
		{Operation: "feature Y", Latest: pdf.V1_7},
		{Operation: "feature W", Earliest: pdf.V1_3, Latest: pdf.V1_7},
		{Operation: "feature Q"},
	} {
		if !pdf.IsWrongVersion(err) {
			t.Errorf("%v not recognised", err)
		}
	}
}

func TestCheckVersionAtMost(t *testing.T) {
	w20, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	if err := pdf.CheckVersionAtMost(w20, "op", pdf.V1_7); err == nil {
		t.Fatal("expected VersionError for V2.0 writer + max V1.7")
	} else if !pdf.IsWrongVersion(err) {
		t.Errorf("got %T, want *VersionError", err)
	}

	w17, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	if err := pdf.CheckVersionAtMost(w17, "op", pdf.V1_7); err != nil {
		t.Errorf("V1.7 writer + max V1.7 should pass, got %v", err)
	}

	w14, _ := memfile.NewPDFWriter(t, pdf.V1_4, nil)
	if err := pdf.CheckVersionAtMost(w14, "op", pdf.V1_7); err != nil {
		t.Errorf("V1.4 writer + max V1.7 should pass, got %v", err)
	}
}

func TestIsReadError(t *testing.T) {
	other := errors.New("some failure")
	malformed := &pdf.MalformedFileError{Err: errors.New("bad data")}
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"malformed", malformed, false},
		{"wrapped malformed", fmt.Errorf("context: %w", malformed), false},
		{"located malformed", pdf.Wrap(malformed, "object 1 0 R"), false},
		{"other", other, true},
		{"wrapped other", fmt.Errorf("context: %w", other), true},
		{"EOF", io.EOF, true},
		{"version", &pdf.VersionError{Operation: "x", Earliest: pdf.V1_5}, true},
	}
	for _, c := range cases {
		if got := pdf.IsReadError(c.err); got != c.want {
			t.Errorf("%s: IsReadError = %t, want %t", c.name, got, c.want)
		}
		if c.err != nil && pdf.IsReadError(c.err) == pdf.IsMalformed(c.err) {
			t.Errorf("%s: IsReadError and IsMalformed agree", c.name)
		}
	}
}
