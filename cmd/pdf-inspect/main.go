// Copyright 2021 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"flag"
	"fmt"
	"os"

	"seehuhn.de/go/pdf/cmd/internal/buildinfo"
	"seehuhn.de/go/pdf/cmd/internal/profile"
	"seehuhn.de/go/pdf/cmd/pdf-inspect/traverse"
)

var (
	passwdArg  = flag.String("p", "", "PDF password")
	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
	memprofile = flag.String("memprofile", "", "write memory profile to `file`")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "pdf-inspect \u2014 inspect PDF file structure\n")
		fmt.Fprintf(os.Stderr, "%s\n\n", buildinfo.Short("pdf-inspect"))
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  pdf-inspect [options] <file.pdf> [path...]\n\n")
		fmt.Fprintf(os.Stderr, "Arguments:\n")
		fmt.Fprintf(os.Stderr, "  file.pdf   PDF file to inspect\n")
		fmt.Fprintf(os.Stderr, "  path       sequence of selectors to navigate the document structure\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  pdf-inspect file.pdf\n")
		fmt.Fprintf(os.Stderr, "  pdf-inspect file.pdf Pages 1\n")
		fmt.Fprintf(os.Stderr, "  pdf-inspect -p secret file.pdf Pages 1 @contents\n")
	}
	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(1)
	}

	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	stop, err := profile.Start(*cpuprofile, *memprofile)
	if err != nil {
		return err
	}
	defer stop()

	return showObject(flag.Args()...)
}

func showObject(args ...string) error {
	passwords := []string{}
	if *passwdArg != "" {
		passwords = append(passwords, *passwdArg)
	}

	obj, cleanup, err := traverse.Root(args[0], passwords...)
	if err != nil {
		return err
	}
	defer cleanup()

	for _, key := range args[1:] {
		steps := obj.Next()
		found := false
		for _, step := range steps {
			if step.Match.MatchString(key) {
				obj, err = step.Next(key)
				if err != nil {
					return err
				}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("no match for key %q", key)
		}
	}
	err = obj.Show()
	if err != nil {
		return err
	}

	steps := obj.Next()
	if len(steps) > 0 {
		fmt.Println("")
		fmt.Println("next:")
		for _, step := range steps {
			fmt.Printf("  • %s\n", step.Desc)
		}
	}

	return nil
}
