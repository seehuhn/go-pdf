package head

import "time"

func encodeTime(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix() - zeroTime
}

func decodeTime(t int64) time.Time {
	if t == 0 {
		return time.Time{}
	}
	return time.Unix(zeroTime+t, 0)
}

var zeroTime int64 = -2082844800 // start of January 1904 in GMT/UTC time zone
