package tounicode

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestIsValidVCString(t *testing.T) {
	testCases := []struct {
		s      string
		expect bool
	}{
		{"a", true},
		{"abc", true},
		{"a1_b2", true},
		{"a b", false},
		{"aβc", false},
		{"a\tb", false},
		{"0\n", false},
	}
	for _, tc := range testCases {
		if isValidVCString(tc.s) != tc.expect {
			t.Errorf("isValidVCString(%q) = %v, want %v", tc.s, !tc.expect, tc.expect)
		}
	}
}

func FuzzToUnicode(f *testing.F) {
	f.Add([]byte(`/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CMapType 2 def
1 begincodespacerange
<00><ff>
endcodespacerange
1 beginbfrange
<21><29><1078>
endbfrange
endcmap
CMapName currentdict /CMap defineresource pop
end end`))
	f.Add([]byte(`/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CMapType 2 def
1 begincodespacerange
<00><ffff>
endcodespacerange
1 beginbfrange
<0020> <0020>
endbfrange
endcmap
CMapName currentdict /CMap defineresource pop
end end`))
	f.Add([]byte(`/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CMapName /Jochen-Chaotic-UCS2 def
/CMapType 2 def
/CIDSystemInfo <<
  /Registry (Jochen)
  /Ordering (Chaotic)
  /Supplement 12
>> def
1 begincodespacerange
<00> <FF>
endcodespacerange
2 beginbfchar
<20> <006C 006F 0074 0027 0073 0020 006F 0066 0020 0073 0070 0061 0063 0065>
<21> <>
endbfchar
2 beginbfrange
<41> <5A> <0041>
<64> <66> [<0066 0069> <0066 006C> <0066 0066 006C>]
endbfrange
endcmap
CMapName currentdict /CMap defineresource pop
end
end`))

	f.Fuzz(func(t *testing.T, data []byte) {
		info, err := Read(bytes.NewReader(data))
		if err != nil {
			return
		}

		buf := &bytes.Buffer{}
		err = info.Write(buf)
		if err != nil {
			t.Fatal(err)
		}

		info2, err := Read(buf)
		if err != nil {
			t.Fatal(err)
		}

		if d := cmp.Diff(info, info2); d != "" {
			t.Fatal(d)
		}
	})
}
