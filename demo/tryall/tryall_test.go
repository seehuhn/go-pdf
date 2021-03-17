package main

import "testing"

func TestOne(t *testing.T) {
	err := doOneFile("/Users/voss/Google Drive/PastEarth/Feasibility studies/Contracts/Amendment/Louise Sime Extension.pdf")
	if err != nil {
		t.Fatal(err)
	}
}
