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

// Package testfonts locates optional external test fonts on the local
// machine.  These fonts are too large, or too restrictively licensed,
// to commit to the repository; run scripts/get-testfonts.sh at the
// workspace root to download them.
package testfonts

import (
	"os"
	"path/filepath"
	"testing"
)

// Path returns the path of an optional external test font, skipping the
// test when the font is not available locally.
//
// name is the font's file name (e.g. "Junicode-VF.ttf"), looked up in the
// directory named by the QUIRE_TESTFONTS environment variable.
func Path(t *testing.T, name string) string {
	t.Helper()

	dir := os.Getenv("QUIRE_TESTFONTS")
	if dir == "" {
		t.Skipf("external test font %s not available (set QUIRE_TESTFONTS)", name)
	}

	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err != nil {
		t.Skipf("external test font %s not available (set QUIRE_TESTFONTS)", name)
	}
	return path
}
