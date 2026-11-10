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

	for _, r := range [][4]int{{-1, 0, 2, 2}, {0, -1, 2, 2}, {9, 0, 2, 1}, {0, 4, 1, 2}, {0, 0, 0, 1}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("crop %v outside the image accepted", r)
				}
			}()
			b.crop(r[0], r[1], r[2], r[3])
		}()
	}
}
