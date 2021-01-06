package main

// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// Package rc4 implements RC4 encryption, as defined in Bruce Schneier's
// Applied Cryptography.
//
// RC4 is cryptographically broken and should not be used for secure
// applications.

// An ArcFour is an instance of the ArcFour cipher using a particular key.
type ArcFour struct {
	s    [256]uint8
	i, j uint8
}

// Reset restarts the key stream using a new key.
func (c *ArcFour) Reset(key []byte) {
	s := c.s[:256]
	for i := 0; i < 256; i++ {
		s[i] = uint8(i)
	}
	var j uint8
	k := uint8(len(key))
	for ii := 0; ii < 256; ii++ {
		i := uint8(ii)
		j += s[i] + key[i%k]
		s[i], s[j] = s[j], s[i]
	}
	c.i = 0
	c.j = 0
}

// XORKeyStream sets dst to the result of XORing src with the key stream.
// Dst and src must overlap entirely or not at all.
func (c *ArcFour) XORKeyStream(dst, src []byte) {
	if len(src) == 0 {
		return
	}

	i, j := c.i, c.j
	_ = dst[len(src)-1]
	dst = dst[:len(src)] // eliminate bounds check from loop
	for k, v := range src {
		i++
		x := c.s[i]
		j += x
		y := c.s[j]
		c.s[i], c.s[j] = y, x
		dst[k] = v ^ uint8(c.s[uint8(x+y)])
	}
	c.i, c.j = i, j
}
