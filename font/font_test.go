// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package font

import (
	"testing"

	"seehuhn.de/go/pdf/font/charcode"
)

// codecInstance is an [Instance] with the given codec.  Only the methods
// used by [InstancesEqual] are implemented.
type codecInstance struct {
	Instance
	codec *charcode.Codec
}

func (f codecInstance) PostScriptName() string   { return "Test" }
func (f codecInstance) WritingMode() WritingMode { return Horizontal }
func (f codecInstance) Codec() *charcode.Codec   { return f.codec }

func TestInstancesEqualCodeSpace(t *testing.T) {
	one := charcode.Range{Low: []byte{0x00}, High: []byte{0x7F}}
	two := charcode.Range{Low: []byte{0x80, 0x00}, High: []byte{0xFF, 0xFF}}
	half := charcode.Range{Low: []byte{0x00}, High: []byte{0x3F}}
	other := charcode.Range{Low: []byte{0x40}, High: []byte{0x7F}}
	newInstance := func(csr ...charcode.Range) Instance {
		codec, err := charcode.NewCodec(csr)
		if err != nil {
			t.Fatal(err)
		}
		return codecInstance{codec: codec}
	}

	a := newInstance(one, two)
	cases := []struct {
		b    Instance
		want bool
	}{
		{newInstance(one, two), true},
		{newInstance(two, one, two), true},
		{newInstance(one), false},
		{newInstance(half, other, two), false}, // same codes, different ranges
	}
	for i, c := range cases {
		if got := InstancesEqual(a, c.b); got != c.want {
			t.Errorf("%d: got %t, want %t", i, got, c.want)
		}
	}
}
