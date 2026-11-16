// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package main

import "testing"

func TestCrop(t *testing.T) {
	b := newBilevel(10, 5)
	b.set(9, 4)
	b.set(3, 1)
	c := b.crop(3, 1, 7, 4)
	if c.w != 7 || c.h != 4 || !c.get(0, 0) || !c.get(6, 3) {
		t.Error("wrong crop")
	}
	if c.blank() {
		t.Error("crop reported as blank")
	}
	if !b.crop(0, 2, 3, 3).blank() {
		t.Error("blank crop has black pixels")
	}
}
