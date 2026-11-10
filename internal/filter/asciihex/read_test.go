// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package asciihex

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestDecode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []byte
		wantErr  bool
	}{
		{
			name:     "Simple hex string",
			input:    "48656C6C6F20576F726C64>",
			expected: []byte("Hello World"),
		},
		{
			name:     "Hex string with whitespace",
			input:    "48 65 6C 6C 6F 20 57 6F 72 6C 64>",
			expected: []byte("Hello World"),
		},
		{
			name:     "Hex string with mixed case",
			input:    "48656c6C6F20576f726C64>",
			expected: []byte("Hello World"),
		},
		{
			name:     "Hex string with newlines and tabs",
			input:    "48656C6C6F\n20576F\t726C64>",
			expected: []byte("Hello World"),
		},
		{
			name:     "Empty string",
			input:    ">",
			expected: []byte{},
		},
		{
			name:     "Odd number of digits",
			input:    "202>",
			expected: []byte("  "),
		},
		{
			name:     "Invalid character",
			input:    "48X848>",
			expected: []byte("H"),
			wantErr:  true,
		},
		{
			name:     "No EOD marker",
			input:    "48656C6C6F20576F726C64",
			expected: []byte("Hello World"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := Decode(strings.NewReader(tt.input))
			result, err := io.ReadAll(reader)

			if !bytes.Equal(result, tt.expected) {
				t.Errorf("Decode() = %v, want %v", result, tt.expected)
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("Decode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestDecodeInChunks(t *testing.T) {
	input := "48656C6C6F20576F726C64>"
	expected := []byte("Hello World")

	reader := Decode(strings.NewReader(input))
	result := make([]byte, 3)
	var fullResult []byte

	for {
		n, err := reader.Read(result)
		fullResult = append(fullResult, result[:n]...)
		if err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	}

	if !bytes.Equal(fullResult, expected) {
		t.Errorf("Decode() in chunks = %v, want %v", fullResult, expected)
	}
}

func TestRoundTrip(t *testing.T) {
	inputs := []string{
		"",
		"Hello World",
		"großartig",
		"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
	}
	buf := &bytes.Buffer{}
	for w := 2; w < 20; w++ {
		for _, input := range inputs {
			buf.Reset()
			enc := Encode(withDummyClose{buf}, w)
			_, err := enc.Write([]byte(input))
			if err != nil {
				t.Fatal(err)
			}
			err = enc.Close()
			if err != nil {
				t.Fatal(err)
			}

			dec := Decode(buf)
			result, err := io.ReadAll(dec)
			if err != nil {
				t.Fatal(err)
			}

			if string(result) != input {
				t.Errorf("round-trip failed: %q -> %q", input, result)
			}
		}
	}
}
