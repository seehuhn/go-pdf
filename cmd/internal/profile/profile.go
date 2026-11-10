// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package profile

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
)

// Start begins CPU profiling (if cpuprofile is non-empty) and returns a stop
// function that stops CPU profiling and writes the memory profile (if
// memprofile is non-empty). The caller should defer stop() inside run().
func Start(cpuprofile, memprofile string) (stop func(), err error) {
	var cpuFile *os.File
	if cpuprofile != "" {
		cpuFile, err = os.Create(cpuprofile)
		if err != nil {
			return nil, fmt.Errorf("could not create CPU profile: %w", err)
		}
		if err = pprof.StartCPUProfile(cpuFile); err != nil {
			cpuFile.Close()
			return nil, fmt.Errorf("could not start CPU profile: %w", err)
		}
	}

	stop = func() {
		if cpuFile != nil {
			pprof.StopCPUProfile()
			cpuFile.Close()
		}
		if memprofile != "" {
			f, err := os.Create(memprofile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "could not create memory profile: %v\n", err)
				return
			}
			runtime.GC()
			allocs := pprof.Lookup("allocs")
			if allocs == nil {
				fmt.Fprintln(os.Stderr, "could not lookup memory profile")
				f.Close()
				return
			}
			if err := allocs.WriteTo(f, 0); err != nil {
				fmt.Fprintf(os.Stderr, "could not write memory profile: %v\n", err)
			}
			f.Close()
		}
	}
	return stop, nil
}
