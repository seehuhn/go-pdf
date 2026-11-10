// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

//go:generate go run gen_movie.go

// Package testmovie provides a shared test movie for the media and
// movie viewer tests: a 30-second 320x240 (4:3) H.264/MP4 clip showing
// the playback timestamp on each frame, with an audio track that ticks
// once per second.
//
// The clip is committed to the repository so that the viewer tests
// build on a fresh checkout.  To regenerate it after changing
// gen_movie.go, run `go generate` in this directory; this requires the
// ffmpeg command-line tool.
package testmovie

import _ "embed"

//go:embed movie.mp4
var movieData []byte

// Data returns the raw bytes of the test movie.  The returned slice
// must not be modified.
func Data() []byte {
	return movieData
}
