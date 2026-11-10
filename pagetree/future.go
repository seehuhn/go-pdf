// Copyright 2023 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package pagetree

import (
	"fmt"
	"strconv"
	"strings"
)

// A futureInt represents a positive integer which is not yet known.
// It is made up of a known value, plus a number of unknown values.
// Once all values have become known, the callbacks are called to report the
// final sum.
type futureInt struct {
	val        int
	numMissing int
	cb         []func(int)
}

func (f *futureInt) String() string {
	s := strconv.Itoa(f.val) + strings.Repeat("+?", f.numMissing)
	if len(f.cb) > 0 {
		s += fmt.Sprintf(" (%d callbacks)", len(f.cb))
	}
	return s
}

func (f *futureInt) WhenAvailable(cb func(int)) {
	if f.numMissing == 0 {
		cb(f.val)
	} else {
		f.cb = append(f.cb, cb)
	}
}

func (f *futureInt) Update(n int) {
	f.numMissing--
	if n < 0 || f.val < 0 {
		f.val = -1
	} else {
		f.val += n
	}
	if f.numMissing == 0 || f.val < 0 {
		for _, cb := range f.cb {
			cb(f.val)
		}
		f.cb = nil
	}
}

// Inc increases the value by 1, changing the futureInt in place where possible.
func (f *futureInt) Inc() *futureInt {
	if len(f.cb) == 0 {
		f.val++
		return f
	}

	res := &futureInt{
		val:        1,
		numMissing: 1,
	}
	f.WhenAvailable(res.Update)
	return res
}

func (f *futureInt) Add(g *futureInt) *futureInt {
	res := &futureInt{
		numMissing: 2,
	}
	f.WhenAvailable(res.Update)
	g.WhenAvailable(res.Update)
	return res
}
