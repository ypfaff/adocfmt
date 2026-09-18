package golden

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestCases(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		want  []string
	}{
		{
			name:  "one case per directory, nested directories included",
			files: []string{"simple/input.adoc", "simple/golden.adoc", "lists/nested/input.adoc", "lists/nested/golden.adoc"},
			want:  []string{"lists/nested", "simple"},
		},
		{
			name:  "a half written case still counts",
			files: []string{"simple/golden.adoc"},
			want:  []string{"simple"},
		},
		{
			name:  "other files are ignored",
			files: []string{"simple/input.adoc", "simple/notes.adoc", "README.adoc"},
			want:  []string{"simple"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			root := writeFiles(t, test.files)
			got, err := Cases(root)
			if err != nil {
				t.Fatal(err)
			}

			want := make([]string, len(test.want))
			for i, dir := range test.want {
				want[i] = filepath.Join(root, dir)
			}
			if !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

func TestCasesWithoutAnyCase(t *testing.T) {
	t.Parallel()

	if _, err := Cases(writeFiles(t, []string{"README.adoc"})); err == nil {
		t.Error("got no error, want one")
	}
}

func writeFiles(t *testing.T, paths []string) string {
	t.Helper()

	root := t.TempDir()
	for _, path := range paths {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
