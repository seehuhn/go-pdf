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

package shading

import (
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/membudget"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// meshFunctions are a valid shading function for DeviceGray, and two
// functions of the wrong shape: two inputs, and three outputs.
func meshFunctions() map[string]pdf.Function {
	return map[string]pdf.Function{
		"valid":     &function.Type2{XMin: 0, XMax: 1, C0: []float64{0}, C1: []float64{1}, N: 1},
		"2 inputs":  &function.Type4{Domain: []float64{0, 1, 0, 1}, Range: []float64{0, 1}, Program: "add"},
		"3 outputs": &function.Type2{XMin: 0, XMax: 1, C0: []float64{0, 0, 0}, C1: []float64{1, 1, 1}, N: 1},
	}
}

// meshTypes lists, for each mesh shading type, the length of an all-zero
// stream which holds one triangle or patch, with one colour value per vertex
// and 16-bit coordinates.
var meshTypes = []struct {
	shadingType int
	dataLen     int
}{
	{4, 3 * 6},                // 3 vertices: flag, 2 coordinates, 1 colour value
	{5, 4 * 5},                // 4 vertices: 2 coordinates, 1 colour value
	{6, (8 + 24*16 + 32) / 8}, // flag, 12 points, 4 colour values
	{7, (8 + 32*16 + 32) / 8}, // flag, 16 points, 4 colour values
}

// writeMeshShading writes a mesh shading stream with all-zero data and
// returns its reference.  The Decode array ends with colorDecode.
func writeMeshShading(t *testing.T, buf *pdf.Writer, shadingType, dataLen int, cs pdf.Object, fnObj pdf.Object, colorDecode ...float64) pdf.Reference {
	t.Helper()

	decode := pdf.Array{pdf.Integer(0), pdf.Integer(1), pdf.Integer(0), pdf.Integer(1)}
	for _, x := range colorDecode {
		decode = append(decode, pdf.Number(x))
	}
	dict := pdf.Dict{
		"ShadingType":       pdf.Integer(shadingType),
		"ColorSpace":        cs,
		"BitsPerCoordinate": pdf.Integer(16),
		"BitsPerComponent":  pdf.Integer(8),
		"Decode":            decode,
		"Function":          fnObj,
	}
	if shadingType == 5 {
		dict["VerticesPerRow"] = pdf.Integer(2)
	} else {
		dict["BitsPerFlag"] = pdf.Integer(8)
	}
	ref := buf.Alloc()
	w, err := buf.OpenStream(ref, dict)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(make([]byte, dataLen)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return ref
}

// embedFunction writes fn to buf and returns the embedded object.
func embedFunction(t *testing.T, buf *pdf.Writer, fn pdf.Function) pdf.Object {
	t.Helper()
	rm := pdf.NewResourceManager(buf)
	fnObj, err := rm.Embed(fn)
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	return fnObj
}

// Reading a mesh shading fails where the function does not map one input to
// the colour space's components.
func TestMeshReadFunctionShape(t *testing.T) {
	for name, fn := range meshFunctions() {
		for _, tc := range meshTypes {
			t.Run(fmt.Sprintf("type%d %s", tc.shadingType, name), func(t *testing.T) {
				buf, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
				fnObj := embedFunction(t, buf, fn)

				// the Decode array matches the function's input count
				m, _ := fn.Shape()
				var colorDecode []float64
				for range m {
					colorDecode = append(colorDecode, 0, 1)
				}
				ref := writeMeshShading(t, buf, tc.shadingType, tc.dataLen, pdf.Name("DeviceGray"), fnObj, colorDecode...)

				x := pdf.NewExtractor(buf)
				_, err := Extract(pdf.CursorAt(x, nil), ref, false)
				if wantErr := name != "valid"; (err != nil) != wantErr {
					t.Errorf("type %d: error = %v, want error %v", tc.shadingType, err, wantErr)
				}
			})
		}
	}
}

// Reading a mesh shading drops a function given with an Indexed colour
// space, so that the result can be written again.
func TestMeshIndexedFunctionDropped(t *testing.T) {
	indexed := pdf.Array{
		pdf.Name("Indexed"),
		pdf.Name("DeviceRGB"),
		pdf.Integer(1),
		pdf.String("\x00\x00\x00\xff\xff\xff"),
	}
	fn := meshFunctions()["valid"]
	for _, tc := range meshTypes {
		t.Run(fmt.Sprintf("type%d", tc.shadingType), func(t *testing.T) {
			buf, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			fnObj := embedFunction(t, buf, fn)
			ref := writeMeshShading(t, buf, tc.shadingType, tc.dataLen, indexed, fnObj, 0, 1)

			x := pdf.NewExtractor(buf)
			sh, err := Extract(pdf.CursorAt(x, nil), ref, false)
			if err != nil {
				t.Fatal(err)
			}
			var f pdf.Function
			switch sh := sh.(type) {
			case *Type4:
				f = sh.F
			case *Type5:
				f = sh.F
			case *Type6:
				f = sh.F
			case *Type7:
				f = sh.F
			default:
				t.Fatalf("unexpected shading type %T", sh)
			}
			if f != nil {
				t.Error("function not dropped")
			}

			buf2, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			rm := pdf.NewResourceManager(buf2)
			if _, err := rm.Embed(sh); err != nil {
				t.Errorf("writing the shading failed: %v", err)
			}
			if err := rm.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Embedding a mesh shading fails where the function does not map one input
// to the colour space's components.
func TestMeshEmbedFunctionShape(t *testing.T) {
	cs := color.SpaceDeviceGray
	decode := []float64{0, 1, 0, 1, 0, 1}
	corners := [][]float64{{0}, {0}, {0}, {0}}
	for name, fn := range meshFunctions() {
		shadings := []graphics.Shading{
			&Type4{
				Common: Common{ColorSpace: cs}, BitsPerCoordinate: 16, BitsPerComponent: 8, BitsPerFlag: 8,
				Decode: decode, F: fn,
				Vertices: []Type4Vertex{{Color: []float64{0}}, {Color: []float64{0}}, {Color: []float64{0}}},
			},
			&Type5{
				Common: Common{ColorSpace: cs}, BitsPerCoordinate: 16, BitsPerComponent: 8, VerticesPerRow: 2,
				Decode: decode, F: fn,
				Vertices: []Type5Vertex{{Color: []float64{0}}, {Color: []float64{0}}, {Color: []float64{0}}, {Color: []float64{0}}},
			},
			&Type6{
				Common: Common{ColorSpace: cs}, BitsPerCoordinate: 16, BitsPerComponent: 8, BitsPerFlag: 8,
				Decode: decode, F: fn,
				Patches: []Type6Patch{{ControlPoints: [12]vec.Vec2{}, CornerColors: corners}},
			},
			&Type7{
				Common: Common{ColorSpace: cs}, BitsPerCoordinate: 16, BitsPerComponent: 8, BitsPerFlag: 8,
				Decode: decode, F: fn,
				Patches: []Type7Patch{{ControlPoints: [16]vec.Vec2{}, CornerColors: corners}},
			},
		}
		for _, sh := range shadings {
			t.Run(fmt.Sprintf("type%d %s", sh.ShadingType(), name), func(t *testing.T) {
				buf, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
				rm := pdf.NewResourceManager(buf)
				_, err := rm.Embed(sh)
				if wantErr := name != "valid"; (err != nil) != wantErr {
					t.Errorf("type %d: error = %v, want error %v", sh.ShadingType(), err, wantErr)
				}
			})
		}
	}
}

// Embedding a mesh shading with a non-finite vertex value fails.
func TestMeshEmbedNonFinite(t *testing.T) {
	cs := color.SpaceDeviceGray
	decode := []float64{0, 100, 0, 100, 0, 1}
	var cp [12]vec.Vec2
	var cp16 [16]vec.Vec2
	corners := func() [][]float64 { return [][]float64{{0}, {0}, {0}, {0}} }
	nan := math.NaN()

	cases := map[string]graphics.Shading{
		"type4 X": &Type4{
			Common: Common{ColorSpace: cs}, BitsPerCoordinate: 16, BitsPerComponent: 8, BitsPerFlag: 8,
			Decode:   decode,
			Vertices: []Type4Vertex{{X: nan, Y: 0, Color: []float64{0}}, {Color: []float64{0}}, {Color: []float64{0}}},
		},
		"type4 color": &Type4{
			Common: Common{ColorSpace: cs}, BitsPerCoordinate: 16, BitsPerComponent: 8, BitsPerFlag: 8,
			Decode:   decode,
			Vertices: []Type4Vertex{{X: 0, Y: 0, Color: []float64{math.Inf(1)}}, {Color: []float64{0}}, {Color: []float64{0}}},
		},
		"type5 Y": &Type5{
			Common: Common{ColorSpace: cs}, BitsPerCoordinate: 16, BitsPerComponent: 8, VerticesPerRow: 2,
			Decode: decode,
			Vertices: []Type5Vertex{
				{X: 0, Y: math.Inf(-1), Color: []float64{0}}, {X: 1, Y: 0, Color: []float64{0}},
				{X: 0, Y: 1, Color: []float64{0}}, {X: 1, Y: 1, Color: []float64{0}},
			},
		},
		"type6 control point": &Type6{
			Common: Common{ColorSpace: cs}, BitsPerCoordinate: 16, BitsPerComponent: 8, BitsPerFlag: 8,
			Decode:  decode,
			Patches: []Type6Patch{{ControlPoints: func() [12]vec.Vec2 { p := cp; p[5].X = nan; return p }(), CornerColors: corners()}},
		},
		"type6 color": &Type6{
			Common: Common{ColorSpace: cs}, BitsPerCoordinate: 16, BitsPerComponent: 8, BitsPerFlag: 8,
			Decode:  decode,
			Patches: []Type6Patch{{ControlPoints: cp, CornerColors: [][]float64{{0}, {nan}, {0}, {0}}}},
		},
		"type7 control point": &Type7{
			Common: Common{ColorSpace: cs}, BitsPerCoordinate: 16, BitsPerComponent: 8, BitsPerFlag: 8,
			Decode:  decode,
			Patches: []Type7Patch{{ControlPoints: func() [16]vec.Vec2 { p := cp16; p[15].Y = nan; return p }(), CornerColors: corners()}},
		},
		"type7 color": &Type7{
			Common: Common{ColorSpace: cs}, BitsPerCoordinate: 16, BitsPerComponent: 8, BitsPerFlag: 8,
			Decode:  decode,
			Patches: []Type7Patch{{ControlPoints: cp16, CornerColors: [][]float64{{0}, {0}, {0}, {math.Inf(-1)}}}},
		},
		"type4 decode": &Type4{
			Common: Common{ColorSpace: cs}, BitsPerCoordinate: 16, BitsPerComponent: 8, BitsPerFlag: 8,
			Decode:   []float64{0, math.Inf(1), 0, 100, 0, 1},
			Vertices: []Type4Vertex{{X: 0, Y: 0, Color: []float64{0}}, {Color: []float64{0}}, {Color: []float64{0}}},
		},
		"type5 decode": &Type5{
			Common: Common{ColorSpace: cs}, BitsPerCoordinate: 16, BitsPerComponent: 8, VerticesPerRow: 2,
			Decode: []float64{0, 100, nan, 100, 0, 1},
			Vertices: []Type5Vertex{
				{X: 0, Y: 0, Color: []float64{0}}, {X: 1, Y: 0, Color: []float64{0}},
				{X: 0, Y: 1, Color: []float64{0}}, {X: 1, Y: 1, Color: []float64{0}},
			},
		},
		"type6 decode": &Type6{
			Common: Common{ColorSpace: cs}, BitsPerCoordinate: 16, BitsPerComponent: 8, BitsPerFlag: 8,
			Decode:  []float64{0, 100, 0, 100, 0, nan},
			Patches: []Type6Patch{{ControlPoints: cp, CornerColors: corners()}},
		},
		"type7 decode": &Type7{
			Common: Common{ColorSpace: cs}, BitsPerCoordinate: 16, BitsPerComponent: 8, BitsPerFlag: 8,
			Decode:  []float64{math.Inf(-1), 100, 0, 100, 0, 1},
			Patches: []Type7Patch{{ControlPoints: cp16, CornerColors: corners()}},
		},
	}
	for name, sh := range cases {
		t.Run(name, func(t *testing.T) {
			buf, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			rm := pdf.NewResourceManager(buf)
			if _, err := rm.Embed(sh); err == nil {
				t.Error("non-finite value was accepted")
			}
		})
	}
}

// Embedding a type 3 shading fails where the function does not map one input
// to the colour space's components.
func TestType3EmbedFunctionShape(t *testing.T) {
	for name, fn := range meshFunctions() {
		t.Run(name, func(t *testing.T) {
			sh := &Type3{
				Common:  Common{ColorSpace: color.SpaceDeviceGray},
				Center2: vec.Vec2{X: 10, Y: 10},
				R2:      10,
				F:       fn,
				TMax:    1,
			}
			buf, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			rm := pdf.NewResourceManager(buf)
			_, err := rm.Embed(sh)
			if wantErr := name != "valid"; (err != nil) != wantErr {
				t.Errorf("error = %v, want error %v", err, wantErr)
			}
		})
	}
}

// Reading a mesh shading fails where the function's domain does not
// contain the parametric range of the Decode array.
func TestMeshReadFunctionDomain(t *testing.T) {
	fn := meshFunctions()["valid"] // domain [0, 1]
	cases := []struct {
		t0, t1 float64
		ok     bool
	}{
		{0, 1, true},
		{1, 0, true},
		{0.25, 0.5, true},
		{0, 2, false},
		{-1, 1, false},
	}
	for _, tc := range meshTypes {
		for _, c := range cases {
			t.Run(fmt.Sprintf("type%d [%g %g]", tc.shadingType, c.t0, c.t1), func(t *testing.T) {
				buf, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
				fnObj := embedFunction(t, buf, fn)
				ref := writeMeshShading(t, buf, tc.shadingType, tc.dataLen, pdf.Name("DeviceGray"), fnObj, c.t0, c.t1)

				x := pdf.NewExtractor(buf)
				_, err := Extract(pdf.CursorAt(x, nil), ref, false)
				if (err == nil) != c.ok {
					t.Errorf("error = %v, want success %v", err, c.ok)
				}
			})
		}
	}
}

// Embedding a mesh shading fails where the function's domain does not
// contain the parametric range of the Decode array.
func TestMeshEmbedFunctionDomain(t *testing.T) {
	cs := color.SpaceDeviceGray
	fn := meshFunctions()["valid"] // domain [0, 1]
	corners := [][]float64{{0}, {0}, {0}, {0}}
	for _, decode := range [][]float64{{0, 1, 0, 1, 0, 2}, {0, 1, 0, 1, -0.5, 1}} {
		shadings := []graphics.Shading{
			&Type4{
				Common: Common{ColorSpace: cs}, BitsPerCoordinate: 16, BitsPerComponent: 8, BitsPerFlag: 8,
				Decode: decode, F: fn,
				Vertices: []Type4Vertex{{Color: []float64{0}}, {Color: []float64{0}}, {Color: []float64{0}}},
			},
			&Type5{
				Common: Common{ColorSpace: cs}, BitsPerCoordinate: 16, BitsPerComponent: 8, VerticesPerRow: 2,
				Decode: decode, F: fn,
				Vertices: []Type5Vertex{{Color: []float64{0}}, {Color: []float64{0}}, {Color: []float64{0}}, {Color: []float64{0}}},
			},
			&Type6{
				Common: Common{ColorSpace: cs}, BitsPerCoordinate: 16, BitsPerComponent: 8, BitsPerFlag: 8,
				Decode: decode, F: fn,
				Patches: []Type6Patch{{CornerColors: corners}},
			},
			&Type7{
				Common: Common{ColorSpace: cs}, BitsPerCoordinate: 16, BitsPerComponent: 8, BitsPerFlag: 8,
				Decode: decode, F: fn,
				Patches: []Type7Patch{{CornerColors: corners}},
			},
		}
		for _, sh := range shadings {
			t.Run(fmt.Sprintf("type%d %v", sh.ShadingType(), decode[4:]), func(t *testing.T) {
				buf, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
				rm := pdf.NewResourceManager(buf)
				if _, err := rm.Embed(sh); err == nil {
					t.Error("function domain not checked")
				}
			})
		}
	}
}

func TestRepairType4Mesh(t *testing.T) {
	cases := []struct {
		flags []uint8
		want  []uint8 // flags of the kept vertices, by original index
		keep  []int
	}{
		{flags: []uint8{0, 1, 2, 1, 2}, keep: []int{0, 1, 2, 3, 4}, want: []uint8{0, 0, 0, 1, 2}},
		{flags: []uint8{1, 2, 0, 0, 0}, keep: []int{2, 3, 4}, want: []uint8{0, 0, 0}},
		{flags: []uint8{0, 0, 0, 3, 1, 0, 3, 3, 2}, keep: []int{0, 1, 2, 5, 6, 7, 8}, want: []uint8{0, 0, 0, 0, 0, 0, 2}},
		{flags: []uint8{0, 0, 0, 1, 0, 0}, keep: []int{0, 1, 2, 3}, want: []uint8{0, 0, 0, 1}},
		{flags: []uint8{0, 0}, keep: nil, want: nil},
		{flags: nil, keep: nil, want: nil},
	}
	for _, c := range cases {
		var vertices []Type4Vertex
		for i, f := range c.flags {
			vertices = append(vertices, Type4Vertex{X: float64(i), Flag: f})
		}
		got := repairType4Mesh(vertices)
		var keep []int
		var flags []uint8
		for _, v := range got {
			keep = append(keep, int(v.X))
			flags = append(flags, v.Flag)
		}
		if diff := cmp.Diff(c.keep, keep); diff != "" {
			t.Errorf("flags %v: kept vertices differ (-want +got):\n%s", c.flags, diff)
		}
		if diff := cmp.Diff(c.want, flags); diff != "" {
			t.Errorf("flags %v: repaired flags differ (-want +got):\n%s", c.flags, diff)
		}
	}
}

// Reading a Type 4 shading uses only the two low bits of each flag and
// ignores trailing bytes which do not form a whole vertex.
func TestType4ReadFlagBits(t *testing.T) {
	s := &Type4{
		Common: Common{ColorSpace: color.SpaceDeviceGray}, BitsPerCoordinate: 16, BitsPerComponent: 8, BitsPerFlag: 8,
		Decode: []float64{0, 1, 0, 1, 0, 1},
		Vertices: []Type4Vertex{
			{Color: []float64{0}}, {X: 1, Color: []float64{0}}, {Y: 1, Color: []float64{0}},
			{X: 1, Y: 1, Flag: 1, Color: []float64{1}},
		},
	}
	data := meshStreamData(t, s)
	const vertexBytes = 6
	data[0] |= 0xFC
	data[vertexBytes] |= 0x05
	data[3*vertexBytes] |= 0x10
	data = append(data, 0, 0, 0)

	got, err := parseType4Vertices(data, s, membudget.New(1<<20))
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(s.Vertices, got); diff != "" {
		t.Errorf("vertices differ (-want +got):\n%s", diff)
	}
}

// Reading a patch mesh uses only the two low bits of each flag, and skips
// connected patches before the first patch with flag 0.
func TestPatchReadFlags(t *testing.T) {
	cs := color.SpaceDeviceGray
	decode := []float64{0, 1, 0, 1, 0, 1}
	corners := [][]float64{{0}, {1}, {0}, {1}}

	t.Run("type6", func(t *testing.T) {
		s := &Type6{
			Common: Common{ColorSpace: cs}, BitsPerCoordinate: 16, BitsPerComponent: 8, BitsPerFlag: 8,
			Decode:  decode,
			Patches: []Type6Patch{{CornerColors: corners}},
		}
		prev := &s.Patches[0]
		for i := range prev.ControlPoints {
			prev.ControlPoints[i] = vec.Vec2{X: float64(i) / 16, Y: 1 - float64(i)/16}
		}
		next := Type6Patch{Flag: 1, CornerColors: [][]float64{nil, nil, {1}, {1}}}
		conn := edgeConnections[next.Flag]
		for i := range next.ControlPoints {
			next.ControlPoints[i] = vec.Vec2{X: 1, Y: 1}
		}
		for i, src := range conn.ImplicitPoints {
			next.ControlPoints[i] = prev.ControlPoints[src]
		}
		for i, src := range conn.ImplicitColors {
			next.CornerColors[i] = prev.CornerColors[src]
		}
		s.Patches = append(s.Patches, next)
		data := meshStreamData(t, s)
		want, err := parseType6Patches(data, s, membudget.New(1<<20))
		if err != nil {
			t.Fatal(err)
		}
		const first = 53 // bytes in the first patch, followed by the connected one

		// a leading connected patch, with flag 1 in the high bits as well
		lead := slices.Clone(data[first:])
		lead[0] |= 0x40
		data[0] |= 0x04
		data = append(lead, data...)

		got, err := parseType6Patches(data, s, membudget.New(1<<20))
		if err != nil {
			t.Fatal(err)
		}
		if len(want) != 2 {
			t.Fatalf("got %d patches from the unmodified data, want 2", len(want))
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("patches differ (-want +got):\n%s", diff)
		}
	})

	t.Run("type7", func(t *testing.T) {
		s := &Type7{
			Common: Common{ColorSpace: cs}, BitsPerCoordinate: 16, BitsPerComponent: 8, BitsPerFlag: 8,
			Decode:  decode,
			Patches: []Type7Patch{{CornerColors: corners}},
		}
		prev := &s.Patches[0]
		for i := range prev.ControlPoints {
			prev.ControlPoints[i] = vec.Vec2{X: float64(i) / 16, Y: 1 - float64(i)/16}
		}
		next := Type7Patch{Flag: 2, CornerColors: [][]float64{nil, nil, {1}, {1}}}
		conn := type7EdgeConnections[next.Flag]
		for i := range next.ControlPoints {
			next.ControlPoints[i] = vec.Vec2{X: 1, Y: 1}
		}
		for i, src := range conn.ImplicitStreamIndices {
			next.ControlPoints[i] = prev.ControlPoints[src]
		}
		for i, src := range conn.ImplicitColorIndices {
			next.CornerColors[i] = prev.CornerColors[src]
		}
		s.Patches = append(s.Patches, next)
		data := meshStreamData(t, s)
		want, err := parseType7Patches(data, s, membudget.New(1<<20))
		if err != nil {
			t.Fatal(err)
		}
		const first = 69 // bytes in the first patch, followed by the connected one

		lead := slices.Clone(data[first:])
		lead[0] |= 0x80
		data[0] |= 0x08
		data = append(lead, data...)

		got, err := parseType7Patches(data, s, membudget.New(1<<20))
		if err != nil {
			t.Fatal(err)
		}
		if len(want) != 2 {
			t.Fatalf("got %d patches from the unmodified data, want 2", len(want))
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("patches differ (-want +got):\n%s", diff)
		}
	})
}

// Embedding fails where the edge flags do not describe whole triangles or
// patches.
func TestMeshEmbedFlags(t *testing.T) {
	cs := color.SpaceDeviceGray
	decode := []float64{0, 1, 0, 1, 0, 1}
	v := func(flag uint8) Type4Vertex { return Type4Vertex{Flag: flag, Color: []float64{0}} }
	corners := [][]float64{{0}, {0}, {0}, {0}}
	cases := []struct {
		name string
		sh   graphics.Shading
		ok   bool
	}{
		{"type4 valid", &Type4{Vertices: []Type4Vertex{v(0), v(2), v(1), v(1), v(2), v(0), v(0), v(0)}}, true},
		{"type4 first flag", &Type4{Vertices: []Type4Vertex{v(1), v(0), v(0)}}, false},
		{"type4 incomplete", &Type4{Vertices: []Type4Vertex{v(0), v(0), v(0), v(0), v(1)}}, false},
		{"type4 flag 3", &Type4{Vertices: []Type4Vertex{v(0), v(0), v(0), v(3)}}, false},
		{"type6 first flag", &Type6{Patches: []Type6Patch{{Flag: 1, CornerColors: corners}}}, false},
		{"type7 first flag", &Type7{Patches: []Type7Patch{{Flag: 2, CornerColors: corners}}}, false},
		{"type6 valid", &Type6{Patches: []Type6Patch{{CornerColors: corners}, {Flag: 3, CornerColors: corners}}}, true},
		{"type7 valid", &Type7{Patches: []Type7Patch{{CornerColors: corners}, {Flag: 3, CornerColors: corners}}}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			switch sh := c.sh.(type) {
			case *Type4:
				sh.Common, sh.Decode = Common{ColorSpace: cs}, decode
				sh.BitsPerCoordinate, sh.BitsPerComponent, sh.BitsPerFlag = 16, 8, 8
			case *Type6:
				sh.Common, sh.Decode = Common{ColorSpace: cs}, decode
				sh.BitsPerCoordinate, sh.BitsPerComponent, sh.BitsPerFlag = 16, 8, 8
			case *Type7:
				sh.Common, sh.Decode = Common{ColorSpace: cs}, decode
				sh.BitsPerCoordinate, sh.BitsPerComponent, sh.BitsPerFlag = 16, 8, 8
			}
			buf, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			rm := pdf.NewResourceManager(buf)
			_, err := rm.Embed(c.sh)
			if (err == nil) != c.ok {
				t.Errorf("error = %v, want success %v", err, c.ok)
			}
		})
	}
}

// Embedding a mesh shading fails where a value lies outside its Decode
// range.
func TestMeshEmbedOutsideDecode(t *testing.T) {
	cs := color.SpaceDeviceGray
	decode := []float64{0, 100, 100, 0, 0, 1} // the Y range is reversed
	vertices := func(x, y, c float64) []Type4Vertex {
		return []Type4Vertex{{X: x, Y: y, Color: []float64{c}}, {Color: []float64{0}}, {Color: []float64{0}}}
	}
	lattice := func(x, y, c float64) []Type5Vertex {
		return []Type5Vertex{{X: x, Y: y, Color: []float64{c}}, {Color: []float64{0}}, {Color: []float64{0}}, {Color: []float64{0}}}
	}
	corners := func(c float64) [][]float64 { return [][]float64{{0}, {0}, {c}, {0}} }
	cp6 := func(p vec.Vec2) (cp [12]vec.Vec2) { cp[5] = p; return cp }
	cp7 := func(p vec.Vec2) (cp [16]vec.Vec2) { cp[15] = p; return cp }

	cases := []struct {
		name string
		sh   graphics.Shading
		ok   bool
	}{
		{"type4 inside", &Type4{Vertices: vertices(100, 100, 1)}, true},
		{"type4 X", &Type4{Vertices: vertices(100.5, 0, 0)}, false},
		{"type4 Y", &Type4{Vertices: vertices(0, -1, 0)}, false},
		{"type4 color", &Type4{Vertices: vertices(0, 0, 1.5)}, false},
		{"type5 inside", &Type5{Vertices: lattice(50, 50, 0.5)}, true},
		{"type5 Y", &Type5{Vertices: lattice(0, 101, 0)}, false},
		{"type5 color", &Type5{Vertices: lattice(0, 0, -0.1)}, false},
		{"type6 inside", &Type6{Patches: []Type6Patch{{ControlPoints: cp6(vec.Vec2{X: 100, Y: 100}), CornerColors: corners(1)}}}, true},
		{"type6 point", &Type6{Patches: []Type6Patch{{ControlPoints: cp6(vec.Vec2{X: -1}), CornerColors: corners(0)}}}, false},
		{"type6 color", &Type6{Patches: []Type6Patch{{CornerColors: corners(2)}}}, false},
		{"type7 inside", &Type7{Patches: []Type7Patch{{ControlPoints: cp7(vec.Vec2{X: 100, Y: 100}), CornerColors: corners(1)}}}, true},
		{"type7 point", &Type7{Patches: []Type7Patch{{ControlPoints: cp7(vec.Vec2{Y: 200}), CornerColors: corners(0)}}}, false},
		{"type7 color", &Type7{Patches: []Type7Patch{{CornerColors: corners(-1)}}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			switch sh := c.sh.(type) {
			case *Type4:
				sh.Common, sh.Decode = Common{ColorSpace: cs}, decode
				sh.BitsPerCoordinate, sh.BitsPerComponent, sh.BitsPerFlag = 16, 8, 8
			case *Type5:
				sh.Common, sh.Decode = Common{ColorSpace: cs}, decode
				sh.BitsPerCoordinate, sh.BitsPerComponent, sh.VerticesPerRow = 16, 8, 2
			case *Type6:
				sh.Common, sh.Decode = Common{ColorSpace: cs}, decode
				sh.BitsPerCoordinate, sh.BitsPerComponent, sh.BitsPerFlag = 16, 8, 8
			case *Type7:
				sh.Common, sh.Decode = Common{ColorSpace: cs}, decode
				sh.BitsPerCoordinate, sh.BitsPerComponent, sh.BitsPerFlag = 16, 8, 8
			}
			buf, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			rm := pdf.NewResourceManager(buf)
			_, err := rm.Embed(c.sh)
			if (err == nil) != c.ok {
				t.Errorf("error = %v, want success %v", err, c.ok)
			}
		})
	}
}

// Embedding a connected patch fails where the data it shares with the
// previous patch differs.
func TestPatchEmbedSharedData(t *testing.T) {
	cs := color.SpaceDeviceGray
	decode := []float64{0, 1, 0, 1, 0, 1}
	embed := func(t *testing.T, sh graphics.Shading) error {
		buf, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
		rm := pdf.NewResourceManager(buf)
		_, err := rm.Embed(sh)
		return err
	}

	for flag := uint8(1); flag <= 3; flag++ {
		t.Run(fmt.Sprintf("type6 flag %d", flag), func(t *testing.T) {
			var prev Type6Patch
			for i := range prev.ControlPoints {
				prev.ControlPoints[i] = vec.Vec2{X: float64(i) / 16, Y: 0.5}
			}
			prev.CornerColors = [][]float64{{0.1}, {0.2}, {0.3}, {0.4}}
			conn := edgeConnections[flag]
			next := Type6Patch{Flag: flag, CornerColors: [][]float64{nil, nil, {0}, {0}}}
			for i, src := range conn.ImplicitPoints {
				next.ControlPoints[i] = prev.ControlPoints[src]
			}
			for i, src := range conn.ImplicitColors {
				next.CornerColors[i] = prev.CornerColors[src]
			}
			sh := &Type6{
				Common: Common{ColorSpace: cs}, BitsPerCoordinate: 16, BitsPerComponent: 8, BitsPerFlag: 8,
				Decode: decode, Patches: []Type6Patch{prev, next},
			}
			if err := embed(t, sh); err != nil {
				t.Fatalf("consistent patches rejected: %v", err)
			}
			next.ControlPoints[2].Y = 0.25
			sh.Patches[1] = next
			if embed(t, sh) == nil {
				t.Error("different control point accepted")
			}
			next.ControlPoints[2] = prev.ControlPoints[conn.ImplicitPoints[2]]
			next.CornerColors[1] = []float64{0.9}
			sh.Patches[1] = next
			if embed(t, sh) == nil {
				t.Error("different corner color accepted")
			}
		})
		t.Run(fmt.Sprintf("type7 flag %d", flag), func(t *testing.T) {
			var prev Type7Patch
			for i := range prev.ControlPoints {
				prev.ControlPoints[i] = vec.Vec2{X: float64(i) / 16, Y: 0.5}
			}
			prev.CornerColors = [][]float64{{0.1}, {0.2}, {0.3}, {0.4}}
			conn := type7EdgeConnections[flag]
			next := Type7Patch{Flag: flag, CornerColors: [][]float64{nil, nil, {0}, {0}}}
			for i, src := range conn.ImplicitStreamIndices {
				next.ControlPoints[i] = prev.ControlPoints[src]
			}
			for i, src := range conn.ImplicitColorIndices {
				next.CornerColors[i] = prev.CornerColors[src]
			}
			sh := &Type7{
				Common: Common{ColorSpace: cs}, BitsPerCoordinate: 16, BitsPerComponent: 8, BitsPerFlag: 8,
				Decode: decode, Patches: []Type7Patch{prev, next},
			}
			if err := embed(t, sh); err != nil {
				t.Fatalf("consistent patches rejected: %v", err)
			}
			next.ControlPoints[3].X = 0.75
			sh.Patches[1] = next
			if embed(t, sh) == nil {
				t.Error("different control point accepted")
			}
			next.ControlPoints[3] = prev.ControlPoints[conn.ImplicitStreamIndices[3]]
			next.CornerColors[0] = []float64{0.9}
			sh.Patches[1] = next
			if embed(t, sh) == nil {
				t.Error("different corner color accepted")
			}
		})
	}
}

// Reading a Type 5 shading ignores an incomplete last row, and fails where
// fewer than two whole rows remain.
func TestType5ReadPartialRow(t *testing.T) {
	s := &Type5{
		Common: Common{ColorSpace: color.SpaceDeviceGray}, BitsPerCoordinate: 16, BitsPerComponent: 8, VerticesPerRow: 2,
		Decode: []float64{0, 1, 0, 1, 0, 1},
		Vertices: []Type5Vertex{
			{Color: []float64{0}}, {X: 1, Color: []float64{0}},
			{Y: 1, Color: []float64{1}}, {X: 1, Y: 1, Color: []float64{1}},
		},
	}
	data := meshStreamData(t, s)
	const vertexBytes = 5

	got, err := parseType5Vertices(append(slices.Clone(data), make([]byte, vertexBytes)...), s, membudget.New(1<<20))
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(s.Vertices, got); diff != "" {
		t.Errorf("vertices differ (-want +got):\n%s", diff)
	}

	if _, err := parseType5Vertices(data[:3*vertexBytes], s, membudget.New(1<<20)); err == nil {
		t.Error("one and a half rows accepted")
	}
}
