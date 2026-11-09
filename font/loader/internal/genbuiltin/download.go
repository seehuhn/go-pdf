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

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// cacheDir is the directory for downloaded files.  It is set by main.
var cacheDir string

// srcDir, if non-empty, is a directory with local copies of the sources,
// named by the base name of the source URL.
var srcDir string

// httpClient is used for all downloads.  The timeout covers the whole
// request, including reading the body, so that a stalled connection cannot
// hang the generator.
var httpClient = &http.Client{Timeout: 5 * time.Minute}

// maxDownload limits the size of a downloaded file.
const maxDownload = 64 << 20

// fetch returns the content of url, which must have the given SHA-256
// checksum (hex encoded).  Verified downloads are cached in cacheDir.
// A cached file which fails the check is downloaded again.
func fetch(url, sha string) ([]byte, error) {
	name := path.Base(url)

	if srcDir != "" {
		data, err := os.ReadFile(filepath.Join(srcDir, name))
		if err != nil {
			return nil, err
		}
		if err := verify(data, sha); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		return data, nil
	}

	// cache key includes the checksum, so that equal base names from
	// different URLs cannot clash
	cacheFile := filepath.Join(cacheDir, sha[:16]+"-"+name)
	if data, err := os.ReadFile(cacheFile); err == nil {
		if verify(data, sha) == nil {
			return data, nil
		}
	}

	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDownload+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxDownload {
		return nil, fmt.Errorf("%s: file too large", url)
	}
	if err := verify(data, sha); err != nil {
		return nil, fmt.Errorf("%s: %w", url, err)
	}

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(cacheDir, "download-*")
	if err != nil {
		return nil, err
	}
	_, err = tmp.Write(data)
	if err2 := tmp.Close(); err == nil {
		err = err2
	}
	if err == nil {
		err = os.Rename(tmp.Name(), cacheFile)
	}
	if err != nil {
		os.Remove(tmp.Name())
		return nil, err
	}
	return data, nil
}

var errChecksum = errors.New("checksum mismatch")

func verify(data []byte, sha string) error {
	sum := sha256.Sum256(data)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), sha) {
		return errChecksum
	}
	return nil
}

// extractLiberation returns the twelve Liberation TrueType fonts (keyed by
// file name, e.g. "LiberationSans-Regular.ttf") and the LICENSE file from
// the gzipped tar archive in data.
func extractLiberation(data []byte) (fonts map[string][]byte, license []byte, err error) {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, nil, err
	}
	defer zr.Close()

	fonts = make(map[string][]byte)
	var total int64 // bytes extracted so far
	tr := tar.NewReader(zr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		base := path.Base(hdr.Name)
		isFont := liberationFonts[base]
		if !isFont && base != "LICENSE" {
			continue
		}
		total += hdr.Size
		if hdr.Size > maxDownload || total > maxDownload {
			return nil, nil, fmt.Errorf("%s: archive too large", hdr.Name)
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			return nil, nil, err
		}
		if isFont {
			fonts[base] = body
		} else {
			license = body
		}
	}

	for name := range liberationFonts {
		if _, ok := fonts[name]; !ok {
			return nil, nil, fmt.Errorf("missing %s in archive", name)
		}
	}
	if license == nil {
		return nil, nil, errors.New("missing LICENSE in archive")
	}
	return fonts, license, nil
}

// liberationFonts is the set of font files used from the Liberation archive.
var liberationFonts = func() map[string]bool {
	m := make(map[string]bool)
	for _, family := range []string{"Sans", "Serif", "Mono"} {
		for _, style := range []string{"Regular", "Bold", "Italic", "BoldItalic"} {
			m["Liberation"+family+"-"+style+".ttf"] = true
		}
	}
	return m
}()
