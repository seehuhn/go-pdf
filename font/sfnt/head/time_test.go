package head

import (
	"testing"
	"time"
)

func TestTimeEncoding(t *testing.T) {
	for _, z := range []int64{0, 1, 2, 10, 100, 1000, 10000, 100000, 1000000,
		10000000, 100000000, 1000000000} {
		for _, s := range []int64{-1, 1} {
			x := z * s
			if encodeTime(decodeTime(x)) != x {
				t.Errorf("encodeTime(%d) != %d", x, x)
			}

			if x != 0 && x+1 != 0 {
				t1 := decodeTime(x).Unix()
				t2 := decodeTime(x + 1).Unix()
				if t1+1 != t2 {
					t.Errorf("decodeTime(%d+1) != decodeTime(%d)+1", x, x)
				}
			}

		}
	}
}

func TestEpoch(t *testing.T) {
	epoch := time.Date(1904, time.January, 1, 0, 0, 0, 0, time.UTC)
	if zeroTime != epoch.Unix() {
		t.Errorf("zeroTime != %d", epoch.Unix())
	}

	if encodeTime(epoch) != 0 {
		t.Errorf("encodeTime(%s) != 0", epoch)
	}

	if !decodeTime(0).IsZero() {
		t.Error("decodeTime(0) != zero")
	}
}
