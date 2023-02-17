package tounicode

import (
	"fmt"
	"testing"
)

func TestRange(t *testing.T) {
	entries := []Single{
		{Code: 1, UTF16: []uint16{53}},
		{Code: 2, UTF16: []uint16{54}},
		{Code: 3, UTF16: []uint16{55}},
		{Code: 4, UTF16: []uint16{56}},
	}
	info := FromMappings(entries)
	if len(info.Singles) != 0 {
		t.Errorf("expected 0 singles, got %d", len(info.Singles))
	}
	if len(info.Ranges) != 1 {
		t.Errorf("expected 1 ranges, got %d", len(info.Ranges))
	}
}

func TestSingles(t *testing.T) {
	entries := []Single{
		{Code: 1, UTF16: []uint16{55}},
		{Code: 3, UTF16: []uint16{54}},
		{Code: 5, UTF16: []uint16{53}},
		{Code: 7, UTF16: []uint16{52}},
	}
	info := FromMappings(entries)
	if len(info.Singles) != 4 {
		t.Errorf("expected 4 singles, got %d", len(info.Singles))
	}
	if len(info.Ranges) != 0 {
		t.Errorf("expected 0 ranges, got %d", len(info.Ranges))
	}
}

func TestNoOverflow(t *testing.T) {
	entries := []Single{
		{Code: 251, UTF16: []uint16{53}},
		{Code: 252, UTF16: []uint16{54}},
		{Code: 253, UTF16: []uint16{55}},
		{Code: 254, UTF16: []uint16{56}},
		{Code: 255, UTF16: []uint16{57}},
		{Code: 256, UTF16: []uint16{58}},
	}
	info := FromMappings(entries)
	fmt.Println(info)
	if len(info.Ranges) != 2 {
		t.Errorf("expected 2 ranges, got %d", len(info.Ranges))
	}
}

func TestExtendedPlane(t *testing.T) {
	entries := []Single{
		{Code: 18773, UTF16: []uint16{0xD861, 0xDDC8}},
		{Code: 18774, UTF16: []uint16{0xD861, 0xDDC9}},
	}
	info := FromMappings(entries)
	if len(info.Ranges) != 1 {
		t.Errorf("expected 1 ranges, got %d", len(info.Ranges))
	}
}
