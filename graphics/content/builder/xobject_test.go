// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package builder

import (
	"bytes"
	"io"
	"maps"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
)

func TestDrawInlineImageRawEntries(t *testing.T) {
	sampled := pdf.Dict{"W": pdf.Integer(1), "H": pdf.Integer(1), "CS": pdf.Name("G"), "BPC": pdf.Integer(8)}
	mask := pdf.Dict{"W": pdf.Integer(1), "H": pdf.Integer(1), "IM": pdf.Boolean(true)}

	for _, tc := range []struct {
		name    string
		base    pdf.Dict
		change  pdf.Dict // a nil value deletes the entry
		wantErr bool
	}{
		{"sampled", sampled, nil, false},
		{"full keys", pdf.Dict{
			"Width": pdf.Integer(1), "Height": pdf.Integer(1),
			"ColorSpace": pdf.Name("DeviceGray"), "BitsPerComponent": pdf.Integer(4),
		}, nil, false},

		{"missing W", sampled, pdf.Dict{"W": nil}, true},
		{"zero W", sampled, pdf.Dict{"W": pdf.Integer(0)}, true},
		{"real W", sampled, pdf.Dict{"W": pdf.Real(1)}, true},
		{"missing H", sampled, pdf.Dict{"H": nil}, true},
		{"negative H", sampled, pdf.Dict{"H": pdf.Integer(-1)}, true},

		{"missing CS", sampled, pdf.Dict{"CS": nil}, true},
		{"Indexed CS", sampled, pdf.Dict{"CS": pdf.Array{
			pdf.Name("I"), pdf.Name("G"), pdf.Integer(1), pdf.String([]byte{0, 255}),
		}}, false},
		{"CS resource", sampled, pdf.Dict{"CS": pdf.Name("C1")}, false},
		{"unknown CS resource", sampled, pdf.Dict{"CS": pdf.Name("Missing")}, true},
		{"Pattern CS", sampled, pdf.Dict{"CS": pdf.Name("P")}, true},

		{"missing BPC", sampled, pdf.Dict{"BPC": nil}, true},
		{"missing BPC with Flate", sampled, pdf.Dict{"BPC": nil, "F": pdf.Name("Fl")}, true},
		{"missing BPC with RunLength", sampled, pdf.Dict{"BPC": nil, "F": pdf.Name("RL")}, true},
		{"invalid BPC", sampled, pdf.Dict{"BPC": pdf.Integer(3)}, true},
		{"real BPC", sampled, pdf.Dict{"BPC": pdf.Real(8)}, true},
		{"BPC implied by DCT", sampled, pdf.Dict{"BPC": nil, "F": pdf.Name("DCT")}, false},
		{"BPC implied by CCF", sampled, pdf.Dict{"BPC": nil, "F": pdf.Name("CCF")}, false},
		{"BPC implied by last filter", sampled, pdf.Dict{
			"BPC": nil, "F": pdf.Array{pdf.Name("AHx"), pdf.Name("DCT")},
		}, false},
		{"BPC agrees with DCT", sampled, pdf.Dict{"F": pdf.Name("DCT")}, false},
		{"BPC contradicts DCT", sampled, pdf.Dict{"F": pdf.Name("DCT"), "BPC": pdf.Integer(4)}, true},

		{"mask", mask, nil, false},
		{"mask with BPC 1", mask, pdf.Dict{"BPC": pdf.Integer(1)}, false},
		{"mask with BPC 8", mask, pdf.Dict{"BPC": pdf.Integer(8)}, true},
		{"mask with CS", mask, pdf.Dict{"CS": pdf.Name("G")}, true},
		{"mask without W", mask, pdf.Dict{"W": nil}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dict := maps.Clone(tc.base)
			for k, v := range tc.change {
				if v == nil {
					delete(dict, k)
				} else {
					dict[k] = v
				}
			}
			res := &content.Resources{ColorSpace: map[pdf.Name]color.Space{
				"C1": color.SpaceDeviceRGB,
				"P":  color.SpacePatternColored,
			}}
			b := New(content.Page, res, pdf.V2_0)
			b.DrawInlineImageRaw(dict, []byte{0})
			if (b.Err != nil) != tc.wantErr {
				t.Errorf("got error %v, want error %t", b.Err, tc.wantErr)
			}
		})
	}
}

func TestDrawInlineImageRawLength(t *testing.T) {
	gray := func(extra pdf.Dict) pdf.Dict {
		dict := pdf.Dict{"W": pdf.Integer(4), "H": pdf.Integer(1), "CS": pdf.Name("G"), "BPC": pdf.Integer(8)}
		maps.Copy(dict, extra)
		return dict
	}
	marker := []byte{'a', ' ', 'E', 'I'}
	plain := []byte{1, 2, 3, 4}

	for _, tc := range []struct {
		name    string
		version pdf.Version
		dict    pdf.Dict
		data    []byte
		wantErr bool
	}{
		{"2.0 no L", pdf.V2_0, gray(nil), plain, false},
		{"2.0 correct L", pdf.V2_0, gray(pdf.Dict{"L": pdf.Integer(4)}), plain, false},
		{"2.0 correct Length", pdf.V2_0, gray(pdf.Dict{"Length": pdf.Integer(4)}), plain, false},
		{"2.0 wrong L", pdf.V2_0, gray(pdf.Dict{"L": pdf.Integer(5)}), plain, true},
		{"2.0 real L", pdf.V2_0, gray(pdf.Dict{"L": pdf.Real(4)}), plain, true},
		{"2.0 EI in data", pdf.V2_0, gray(nil), marker, false},
		{"2.0 L and Length", pdf.V2_0, gray(pdf.Dict{"L": pdf.Integer(4), "Length": pdf.Integer(4)}), plain, true},
		{"1.7 no L", pdf.V1_7, gray(nil), plain, false},
		{"1.7 wrong L", pdf.V1_7, gray(pdf.Dict{"L": pdf.Integer(3)}), plain, true},
		{"1.7 EI in data", pdf.V1_7, gray(nil), marker, true},
		{"1.7 EI at start of data", pdf.V1_7, gray(nil), []byte{'E', 'I', '\n', 4}, true},
		{"1.7 EI in data with L", pdf.V1_7, gray(pdf.Dict{"L": pdf.Integer(4)}), marker, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := New(content.Page, nil, tc.version)
			b.DrawInlineImageRaw(tc.dict, tc.data)
			if (b.Err != nil) != tc.wantErr {
				t.Fatalf("got error %v, want error %t", b.Err, tc.wantErr)
			}
		})
	}
}

// TestDrawInlineImageRawRoundTrip checks that inline image data containing
// the EI pattern is read back unchanged from a PDF 2.0 content stream.
func TestDrawInlineImageRawRoundTrip(t *testing.T) {
	data := []byte("a EI b\nEI>c")
	b := New(content.Page, nil, pdf.V2_0)
	b.DrawInlineImageRaw(pdf.Dict{
		"W": pdf.Integer(len(data)), "H": pdf.Integer(1), "CS": pdf.Name("G"), "BPC": pdf.Integer(8),
	}, data)
	b.SetLineWidth(2)
	if b.Err != nil {
		t.Fatal(b.Err)
	}

	buf := &bytes.Buffer{}
	for _, op := range b.Stream {
		if err := op.Format(buf, 0); err != nil {
			t.Fatal(err)
		}
	}

	scanner := content.NewScanner(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
	})
	var names []content.OpName
	for name, args := range scanner.NewIter().All() {
		names = append(names, name)
		if name == content.OpInlineImage {
			if got := []byte(args[1].(pdf.String)); !bytes.Equal(got, data) {
				t.Errorf("got data %q, want %q", got, data)
			}
		}
	}
	if diff := cmp.Diff([]content.OpName{content.OpInlineImage, content.OpSetLineWidth}, names); diff != "" {
		t.Errorf("operators differ (-want +got):\n%s", diff)
	}
}
