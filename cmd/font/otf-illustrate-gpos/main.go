// Copyright 2021 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"fmt"
	"log"
	"os"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/opentype/gtab/builder"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: otf-illustrate-gpos font.otf")
		os.Exit(1)
	}
	fontFileName := os.Args[1]

	info, err := sfnt.ReadFile(fontFileName)
	if err != nil {
		log.Fatal(err)
	}

	if info.Gpos == nil {
		log.Fatal("font has no GPOS table")
	}

	explained := builder.ExplainGpos(info)
	for i, e := range explained {
		fmt.Printf("%d: %s\n", i, e)
	}
}
