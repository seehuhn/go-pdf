package tounicode

import (
	"testing"
)

func TestOptimize(t *testing.T) {
	entries := []Single{
		{Code: 251, UTF16: []uint16{53}},
		{Code: 252, UTF16: []uint16{54}},
		{Code: 253, UTF16: []uint16{55}},
		{Code: 254, UTF16: []uint16{56}},
		{Code: 255, UTF16: []uint16{57}},
		{Code: 256, UTF16: []uint16{58}},
	}
	info := FromMappings(entries)
	if len(info.Ranges) != 2 {
		t.Errorf("expected 2 ranges, got %d", len(info.Ranges))
	}
}
