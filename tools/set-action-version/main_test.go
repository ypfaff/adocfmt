package main

import "testing"

func TestSetDefault(t *testing.T) {
	const in = "  version:\n    default: '0.0.0' # adocfmt-version-marker\n  args:\n    default: '--check .'\n"
	const want = "  version:\n    default: '1.2.3' # adocfmt-version-marker\n  args:\n    default: '--check .'\n"

	got, err := setDefault([]byte(in), "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("setDefault() = %q, want %q", got, want)
	}
}

func TestSetDefaultWithoutMarker(t *testing.T) {
	if _, err := setDefault([]byte("    default: '0.0.0'\n"), "1.2.3"); err == nil {
		t.Error("setDefault() succeeded without the marker")
	}
}
