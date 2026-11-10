// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package pdf

import (
	"sync"

	"seehuhn.de/go/pdf/internal/limits"
)

// maxObjstmCacheBytes bounds the total size of the decoded object streams
// retained by one objstmCache.  Individual streams are already capped at
// limits.MaxObjStmBytes; this bound additionally covers documents which
// spread their objects over many streams.
const maxObjstmCacheBytes = limits.MaxObjStmBytes

// objstmData is a decoded object stream: the stream body after filter
// decoding, together with the absolute byte offsets of the objects it
// contains.
type objstmData struct {
	data []byte
	idx  []stmObj
}

// objstmCache retains decoded object streams.  Documents commonly keep most
// of their objects in one ObjStm; without a cache, resolving each object
// would re-run the stream's whole filter chain, making N lookups cost N
// full decodes.
//
// The zero value is not usable; use [newObjstmCache].  All methods are safe
// for concurrent use, matching the documented guarantees of Reader.Get.
type objstmCache struct {
	mu      sync.Mutex
	used    int64
	order   []Reference // insertion order, for FIFO eviction
	entries map[Reference]*objstmData
}

func newObjstmCache() *objstmCache {
	return &objstmCache{entries: make(map[Reference]*objstmData)}
}

// get returns the cached decoding of the given object stream, or nil.
func (c *objstmCache) get(sRef Reference) *objstmData {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.entries[sRef]
}

// put adds a decoded object stream to the cache, evicting older entries if
// needed to stay within maxObjstmCacheBytes.  Streams larger than the bound
// are not retained (they still resolve correctly; only caching is skipped).
func (c *objstmCache) put(sRef Reference, data []byte, idx []stmObj) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.entries[sRef]; ok {
		return
	}
	size := int64(len(data))
	if size > maxObjstmCacheBytes {
		return
	}
	for c.used+size > maxObjstmCacheBytes && len(c.order) > 0 {
		old := c.order[0]
		c.order = c.order[1:]
		if d, ok := c.entries[old]; ok {
			c.used -= int64(len(d.data))
			delete(c.entries, old)
		}
	}
	c.entries[sRef] = &objstmData{data: data, idx: idx}
	c.used += size
	c.order = append(c.order, sRef)
}

// clear drops all cached entries.
func (c *objstmCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[Reference]*objstmData)
	c.order = nil
	c.used = 0
}
