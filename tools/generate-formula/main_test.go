package main

import (
	"os"
	"testing"
)

func TestFormula(t *testing.T) {
	checksums, err := os.ReadFile("testdata/checksums.txt")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/adocfmt.rb")
	if err != nil {
		t.Fatal(err)
	}

	got, err := formula("1.2.3", checksums)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("formula() =\n%s\nwant\n%s", got, want)
	}
}

func TestFormulaWithoutChecksum(t *testing.T) {
	const checksums = "1111  adocfmt_1.2.3_darwin_arm64.tar.gz\n"
	if _, err := formula("1.2.3", []byte(checksums)); err == nil {
		t.Error("formula() succeeded without a checksum for every platform")
	}
}
