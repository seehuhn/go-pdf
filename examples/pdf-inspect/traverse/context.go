// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package traverse

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"syscall"

	"golang.org/x/term"

	"seehuhn.de/go/pdf"
)

type Context interface {
	// Next returns a child object.
	Next(string) (Context, error)

	// Show prints a textual description of the object to the standard output.
	Show() error

	// Keys lists the allowed keys for the Next method.
	// Keywords which need to be used verbatim are enclosed in backticks,
	// everything else is a human-readable description of what is allowed.
	Keys() []string
}

func Root(fileName string, passwords ...string) (Context, error) {
	tryPasswd := func(_ []byte, try int) string {
		if try < len(passwords) {
			return passwords[try]
		}
		fmt.Print("password: ")
		passwd, err := term.ReadPassword(syscall.Stdin)
		if err != nil {
			fmt.Println("XXX")
			return ""
		}
		fmt.Println("***")
		return string(passwd)
	}

	fd, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	opt := &pdf.ReaderOptions{
		ReadPassword:  tryPasswd,
		ErrorHandling: pdf.ErrorHandlingReport,
	}
	r, err := pdf.NewReader(fd, opt)
	if err != nil {
		return nil, err
	}

	// TODO(voss): where should fd and r be closed?

	c := &fileCtx{
		fd: fd,
		r:  r,
	}
	return c, nil
}

type fileCtx struct {
	fd *os.File
	r  pdf.Getter
}

func (c *fileCtx) Next(key string) (Context, error) {
	meta := c.r.GetMeta()

	var obj pdf.Object
	switch key {
	case "meta":
		return &metaCtx{r: c.r}, nil
	case "catalog":
		obj = pdf.AsDict(meta.Catalog)
	case "info":
		obj = pdf.AsDict(meta.Info)
	case "trailer":
		obj = meta.Trailer
	default:
		if m := objNumberRegexp.FindStringSubmatch(key); m != nil { // object reference ...
			number, err := strconv.ParseUint(m[1], 10, 32)
			if err != nil {
				return nil, err
			}
			var generation uint16
			if m[2] != "" {
				tmp, err := strconv.ParseUint(m[2], 10, 16)
				if err != nil {
					return nil, err
				}
				generation = uint16(tmp)
			}
			ref := pdf.NewReference(uint32(number), generation)
			obj, err = pdf.Resolve(c.r, ref)
			if err != nil {
				return nil, err
			}
		} else { // ... or catalog key
			cat := &objectCtx{r: c.r, obj: pdf.AsDict(meta.Catalog)}
			return cat.Next(key)
		}
	}
	return &objectCtx{r: c.r, obj: obj}, nil
}

func (c *fileCtx) Show() error {
	st, err := c.fd.Stat()
	if err != nil {
		return err
	}

	fmt.Println("file:", st.Name())
	fmt.Println("size:", st.Size())
	fmt.Println("modtime:", st.ModTime().Format("2006-01-02 15:04:05"))

	return nil
}

func (c *fileCtx) Keys() []string {
	return []string{
		"`meta`",
		"`catalog`",
		"`info`",
		"`trailer`",
		"object reference",
		"catalog key",
	}
}

var (
	intRegexp       = regexp.MustCompile(`^(\d+)$`)
	objNumberRegexp = regexp.MustCompile(`^(\d+)(?:\.(\d+))?$`)
)
