// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package type1glyphs

import (
	"bytes"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/internal/debug/makefont"
)

func TestRoundTrip(t *testing.T) {
	want, err := FromStream(ToStream(makefont.Type1()))
	if err != nil {
		t.Fatal(err)
	}

	got, err := FromStream(ToStream(want))
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

// TestFromStreamReleasesStream checks that FromStream has finished with the
// stream when it returns, also where the parser stops before the end of the
// font file.
func TestFromStreamReleasesStream(t *testing.T) {
	var font bytes.Buffer
	err := ToStream(makefont.Type1()).WriteTo(&font, nil)
	if err != nil {
		t.Fatal(err)
	}
	trailer := bytes.Repeat([]byte("% padding\n"), 10000)

	cases := []struct {
		name    string
		data    [][]byte
		wantErr bool
	}{
		{"valid", [][]byte{font.Bytes()}, false},
		{"error after font", [][]byte{font.Bytes(), []byte("\nundefinedOperator\n"), trailer}, false},
		{"not a font", [][]byte{[]byte("%!\nundefinedOperator\n"), trailer}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			done := make(chan struct{})
			stm := &glyphdata.Stream{
				Type: glyphdata.Type1,
				WriteTo: func(w io.Writer, _ *glyphdata.Lengths) error {
					defer close(done)
					for _, b := range c.data {
						if _, err := w.Write(b); err != nil {
							return err
						}
					}
					return nil
				},
			}

			_, err := FromStream(stm)
			if (err != nil) != c.wantErr {
				t.Fatalf("got error %v, want error: %t", err, c.wantErr)
			}

			select {
			case <-done:
			default:
				t.Error("stream still being written after FromStream returned")
			}
		})
	}
}
