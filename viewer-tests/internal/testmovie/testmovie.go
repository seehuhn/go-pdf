// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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
