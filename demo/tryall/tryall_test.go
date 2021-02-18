package main

import "testing"

func TestOne(t *testing.T) {
	err := doOneFile("/Users/voss/Sync/manuals/F-140R_e01_W.pdf")
	if err != nil {
		t.Error(err)
	}
}
