// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package bundled

import (
	"sync"

	"seehuhn.de/go/pdf/font/type1"
)

// Cache hands out instances of a fixed set of bundled fonts.
//
// Parsing a font program and deriving its glyph tables costs far more than the
// encoding state a clone gets of its own, and the result is the same every
// time, so each font is read at most once.  A Cache is safe for concurrent
// use.
type Cache[K comparable] struct {
	read   func(K) (*type1.Instance, error)
	shared map[K]func() (*type1.Instance, error)
}

// New returns a Cache for the fonts named by keys.  read builds a font instance
// from the bundled data; the Cache calls it at most once for each of the keys,
// and afresh for any other key, which has nothing to share.
func New[K comparable](keys []K, read func(K) (*type1.Instance, error)) *Cache[K] {
	c := &Cache[K]{
		read:   read,
		shared: make(map[K]func() (*type1.Instance, error), len(keys)),
	}
	for _, key := range keys {
		c.shared[key] = sync.OnceValues(func() (*type1.Instance, error) {
			return read(key)
		})
	}
	return c
}

// Get returns a new instance of the given font.  The instance allocates
// character codes of its own, but the data behind them is shared with every
// other instance of the same font and must be treated as read-only.
func (c *Cache[K]) Get(key K) (*type1.Instance, error) {
	var tmpl *type1.Instance
	var err error
	if load, ok := c.shared[key]; ok {
		tmpl, err = load()
	} else {
		// not one of the bundled fonts, so there is nothing to share
		tmpl, err = c.read(key)
	}
	if err != nil {
		return nil, err
	}
	return tmpl.Clone(), nil
}
