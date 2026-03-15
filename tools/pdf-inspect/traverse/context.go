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
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"syscall"

	"golang.org/x/term"

	"seehuhn.de/go/pdf"
)

type Context interface {
	// Show prints a textual description of the object to the standard output.
	Show() error

	// Keys lists the allowed keys for the Next method.
	// Keywords which need to be used verbatim are enclosed in backticks,
	// everything else is a human-readable description of what is allowed.
	Next() []Step
}

// Step represents an action which can be performed on a context to either move
// to a child object or to get a new view of the same object.
type Step struct {
	// Match is a regular expression which is used to select a step
	// from the key chosen by the user.
	Match *regexp.Regexp

	// Desc is a human-readable description of the step.
	// For keywords this should be enclosed in backticks, e.g. "`meta`".
	// Otherwise this should be a short description, e.g. "object reference".
	Desc string

	// Next returns the next context reached by this step.
	// The caller must ensure that the key matches the Match regular expression.
	Next func(key string) (Context, error)
}

func Root(fileName string, passwords ...string) (Context, func(), error) {
	fd, err := os.Open(fileName)
	if err != nil {
		return nil, nil, err
	}
	fi, err := fd.Stat()
	if err != nil {
		fd.Close()
		return nil, nil, err
	}
	opt := &pdf.ReaderOptions{
		ErrorHandling: pdf.ErrorHandlingReport,
	}

	// try each password from the command line
	var r *pdf.Reader
	for i := range len(passwords) + 1 {
		if i > 0 {
			opt.Password = passwords[i-1]
		}
		r, err = pdf.NewReader(fd, fi.Size(), opt)
		if err == nil {
			break
		}
		var authErr *pdf.AuthenticationError
		if !errors.As(err, &authErr) {
			fd.Close()
			return nil, nil, err
		}
	}

	// prompt interactively until the user enters an empty password
	for err != nil {
		fmt.Print("password: ")
		passwd, readErr := term.ReadPassword(syscall.Stdin)
		fmt.Println("***")
		if readErr != nil || len(passwd) == 0 {
			break
		}
		opt.Password = string(passwd)
		r, err = pdf.NewReader(fd, fi.Size(), opt)
	}
	if err != nil {
		fd.Close()
		return nil, nil, err
	}

	c := &fileCtx{
		fd: fd,
		r:  r,
	}
	cleanup := func() {
		r.Close()
		fd.Close()
	}
	return c, cleanup, nil
}

type fileCtx struct {
	fd *os.File
	r  pdf.Getter
}

func (c *fileCtx) Next() []Step {
	meta := c.r.GetMeta()

	return []Step{
		{
			Match: regexp.MustCompile(`^meta$`),
			Desc:  "`meta`",
			Next: func(key string) (Context, error) {
				return &metaCtx{r: c.r}, nil
			},
		},
		{
			Match: regexp.MustCompile(`^catalog$`),
			Desc:  "`catalog`",
			Next: func(key string) (Context, error) {
				obj := pdf.AsDict(meta.Catalog)
				return &objectCtx{r: c.r, obj: obj}, nil
			},
		},
		{
			Match: regexp.MustCompile(`^info$`),
			Desc:  "`info`",
			Next: func(key string) (Context, error) {
				obj := pdf.AsDict(meta.Info)
				return &objectCtx{r: c.r, obj: obj}, nil
			},
		},
		{
			Match: regexp.MustCompile(`^trailer$`),
			Desc:  "`trailer`",
			Next: func(key string) (Context, error) {
				obj := meta.Trailer
				return &objectCtx{r: c.r, obj: obj}, nil
			},
		},
		{
			Match: objNumberRegexp,
			Desc:  "object reference",
			Next: func(key string) (Context, error) {
				m := objNumberRegexp.FindStringSubmatch(key)
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
				obj, err := pdf.Resolve(c.r, ref)
				if err != nil {
					return nil, err
				}
				return &objectCtx{r: c.r, obj: obj}, nil
			},
		},
		{
			Match: regexp.MustCompile(`^.+$`),
			Desc:  "catalog key",
			Next: func(key string) (Context, error) {
				cat := &objectCtx{r: c.r, obj: pdf.AsDict(meta.Catalog)}
				steps := cat.Next()
				for _, step := range steps {
					if step.Match.MatchString(key) {
						return step.Next(key)
					}
				}
				return nil, &KeyError{Key: key, Ctx: "catalog key"}
			},
		},
	}
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

var (
	intRegexp       = regexp.MustCompile(`^(\d+)$`)
	objNumberRegexp = regexp.MustCompile(`^(\d+)(?:\.(\d+))?$`)
)
