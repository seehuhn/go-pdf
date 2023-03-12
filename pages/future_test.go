// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package pages

import "testing"

func TestFutureInt1(t *testing.T) {
	xa := -1
	a := &futureInt{val: 1}
	a.WhenAvailable(func(n int) {
		xa = n
	})
	if xa != 1 {
		t.Fatalf("xa = %d, want 1", xa)
	}
	if len(a.cb) > 0 {
		t.Fatalf("len(a.cb) = %d, want empty", len(a.cb))
	}

	xa = -1
	a.numMissing++
	a.WhenAvailable(func(n int) {
		xa = n
	})
	if xa != -1 || len(a.cb) != 1 {
		t.Fatal("callback for a called too early")
	}

	a.AddMissing(2)
	if xa != 3 || len(a.cb) != 0 {
		t.Fatalf("xa = %d, want 3; len(a.cb) = %d, want empty", xa, len(a.cb))
	}
}

func TestFutureInt2(t *testing.T) {
	xa := -1
	xc := -1
	xd := -1

	a := &futureInt{val: 1}
	a.numMissing++
	a.WhenAvailable(func(n int) { xa = n })

	b := a.Inc()
	// no callback here, to allow the next Inc() to change b in place

	c := b.Inc()
	c.WhenAvailable(func(n int) { xc = n })

	// b and c are the same object now, so the sum should be 6
	d := b.Add(c)
	d.WhenAvailable(func(n int) { xd = n })

	if xa != -1 || xc != -1 || xd != -1 {
		t.Fatal("callbacks called too early")
	}

	a.AddMissing(0)
	if xa != 1 || xc != 3 || xd != 6 {
		t.Fatalf("xa = %d, want 1; xc = %d, want 3; xd = %d, want 6", xa, xc, xd)
	}
}

func TestFutureInt3(t *testing.T) {
	xa := -1
	xc := -1
	xd := -1

	a := &futureInt{val: 1}
	a.numMissing++
	a.WhenAvailable(func(n int) { xa = n })

	b := a.Inc()

	// Use add, to make sure that b is not changed in place.
	c := b.Add(&futureInt{val: 1})
	c.WhenAvailable(func(n int) { xc = n })

	// b and c are different objects now, so the sum should be 5
	d := b.Add(c)
	d.WhenAvailable(func(n int) { xd = n })

	if xa != -1 || xc != -1 || xd != -1 {
		t.Fatal("callbacks called too early")
	}

	a.AddMissing(0)
	if xa != 1 || xc != 3 || xd != 5 {
		t.Fatalf("xa = %d, want 1; xc = %d, want 3; xd = %d, want 5", xa, xc, xd)
	}
}
