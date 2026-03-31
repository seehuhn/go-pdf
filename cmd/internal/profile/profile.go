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
