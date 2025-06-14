// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"seehuhn.de/go/pdf/examples/pdf-inspect/traverse"
)

var (
	passwdArg = flag.String("p", "", "PDF password")
)

func main() {
	flag.Parse()
	flag.CommandLine.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(),
			"Usage: %s [options] <file.pdf> <path>...\n",
			filepath.Base(os.Args[0]))
		fmt.Fprintln(flag.CommandLine.Output())
		fmt.Fprintln(flag.CommandLine.Output(), "The given path describes an object in the PDF file,")
		fmt.Fprintln(flag.CommandLine.Output(), "starting from the document catalog.")
		fmt.Fprintln(flag.CommandLine.Output())
		fmt.Fprintln(flag.CommandLine.Output(), "Options:")
		flag.PrintDefaults()
	}
	args := flag.Args()

	if len(args) == 0 {
		flag.CommandLine.Usage()
		os.Exit(1)
	}

	err := showObject(args...)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func showObject(args ...string) error {
	passwords := []string{}
	if *passwdArg != "" {
		passwords = append(passwords, *passwdArg)
	}

	obj, err := traverse.Root(args[0], passwords...)
	if err != nil {
		return err
	}

	for _, key := range args[1:] {
		obj, err = obj.Next(key)
		if err != nil {
			return err
		}
	}
	err = obj.Show()
	if err != nil {
		return err
	}

	keys := obj.Keys()
	if len(keys) > 0 {
		fmt.Println("")
		fmt.Println("next:")
		for _, key := range keys {
			fmt.Printf("  • %s\n", key)
		}
	}

	return nil
}
