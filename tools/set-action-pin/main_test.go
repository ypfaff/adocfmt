package main

import "testing"

func TestSetPin(t *testing.T) {
	const in = "    - uses: ypfaff/adocfmt@d97e6c7945053ce3b1748e73784bac5c9c78eb66 # v0.3.0\n      with:\n"
	const want = "    - uses: ypfaff/adocfmt@1b2b1786ab0c3b9e1c1e4f1a7f4c3d8e9a0b1c2d # v0.3.1\n      with:\n"

	got, err := setPin([]byte(in), "1b2b1786ab0c3b9e1c1e4f1a7f4c3d8e9a0b1c2d", "0.3.1")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("setPin() = %q, want %q", got, want)
	}
}

func TestSetPinWithoutPin(t *testing.T) {
	if _, err := setPin([]byte("    - uses: ypfaff/adocfmt@v0\n"), "1b2b1786ab0c3b9e1c1e4f1a7f4c3d8e9a0b1c2d", "0.3.1"); err == nil {
		t.Error("setPin() succeeded without a pinned line")
	}
}
