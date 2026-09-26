package main

import "testing"

func TestRaise(t *testing.T) {
	tests := []struct {
		version, part, want string
		wantErr             bool
	}{
		{version: "1.2.3", part: "major", want: "2.0.0"},
		{version: "1.2.3", part: "minor", want: "1.3.0"},
		{version: "1.2.3", part: "patch", want: "1.2.4"},
		{version: "0.0.0", part: "patch", want: "0.0.1"},
		{version: "1.9.9", part: "patch", want: "1.9.10"},
		{version: "1.2.3", part: "build", wantErr: true},
		{version: "1.2.3-rc.1", part: "patch", wantErr: true},
		{version: "1.2", part: "patch", wantErr: true},
	}
	for _, tt := range tests {
		got, err := raise(tt.version, tt.part)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("raise(%q, %q) = %q, %v, want %q, error %t", tt.version, tt.part, got, err, tt.want, tt.wantErr)
		}
	}
}
