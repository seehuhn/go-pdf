// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package pdfenc

import "testing"

func TestStdLatinDoesNotHaveEmpty(t *testing.T) {
	if StandardLatin.Has[""] {
		t.Errorf("StandardLatin.Has[\"\"] is true")
	}
}
