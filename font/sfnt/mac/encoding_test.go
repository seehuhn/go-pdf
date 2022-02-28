package mac

import "testing"

func TestEncoding(t *testing.T) {
	for i := 0; i < 256; i++ {
		s := Decode([]byte{byte(i)})
		cc := Encode(s)
		if len(cc) != 1 || cc[0] != byte(i) {
			t.Errorf("%d: %q -> %q", i, s, cc)
		}
	}
}
