package format

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ypfaff/adocfmt/internal/golden"
	"github.com/ypfaff/adocfmt/internal/renderequivalence"
)

// casesDir is relative to this package.
const casesDir = "../../" + golden.Dir

func TestGoldenCases(t *testing.T) {
	for _, dir := range goldenCases(t) {
		t.Run(caseName(dir), func(t *testing.T) {
			t.Parallel()

			got, err := Format(readCase(t, dir, golden.InputFile))
			if err != nil {
				t.Fatal(err)
			}
			if want := readCase(t, dir, golden.GoldenFile); !bytes.Equal(got, want) {
				t.Errorf("output differs from %s at %s\nrun go run ./tools/update-golden to rewrite the golden files",
					golden.GoldenFile, firstDifferingLine(want, got))
			}
		})
	}
}

// TestGoldenCasesAreFormatted is the idempotence check on the cases a rule is
// written against: the golden file is what the formatter produces, so
// formatting it has to hand it back unchanged.
func TestGoldenCasesAreFormatted(t *testing.T) {
	for _, dir := range goldenCases(t) {
		t.Run(caseName(dir), func(t *testing.T) {
			t.Parallel()

			want := readCase(t, dir, golden.GoldenFile)
			got, err := Format(want)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("formatting %s changes it at %s", golden.GoldenFile, firstDifferingLine(want, got))
			}
		})
	}
}

// TestGoldenCasesRenderEquivalent guards the golden files themselves: the update
// tool freezes whatever the formatter produces, so without this a rule that
// changes meaning would be recorded as the expectation.
func TestGoldenCasesRenderEquivalent(t *testing.T) {
	if testing.Short() {
		t.Skip("rendering shells out to Asciidoctor")
	}
	if _, err := exec.LookPath("asciidoctor"); err != nil {
		t.Fatal("asciidoctor is not on the PATH")
	}

	for _, dir := range goldenCases(t) {
		t.Run(caseName(dir), func(t *testing.T) {
			t.Parallel()

			findings, err := renderequivalence.Differences(
				readCase(t, dir, golden.InputFile),
				readCase(t, dir, golden.GoldenFile),
			)
			if err != nil {
				t.Fatal(err)
			}
			for _, finding := range findings {
				t.Error(finding)
			}
		})
	}
}

func goldenCases(t *testing.T) []string {
	t.Helper()

	cases, err := golden.Cases(casesDir)
	if err != nil {
		t.Fatal(err)
	}
	return cases
}

func caseName(dir string) string {
	return strings.TrimPrefix(filepath.ToSlash(dir), casesDir+"/")
}

func readCase(t *testing.T, dir, name string) []byte {
	t.Helper()

	src, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return src
}

// firstDifferingLine locates the first differing line, because formatter output
// is read line by line.
func firstDifferingLine(want, got []byte) string {
	wantLines := strings.Split(string(want), "\n")
	gotLines := strings.Split(string(got), "\n")

	for at := range max(len(wantLines), len(gotLines)) {
		wantLine, gotLine := quoteLine(wantLines, at), quoteLine(gotLines, at)
		if wantLine != gotLine {
			return fmt.Sprintf("line %d:\nwant: %s\ngot:  %s", at+1, wantLine, gotLine)
		}
	}
	return "no line: the two are equal"
}

// quoteLine quotes a line so trailing whitespace and a missing final newline
// stay visible, which is what a formatter changes.
func quoteLine(lines []string, at int) string {
	if at >= len(lines) {
		return "<past the end>"
	}
	return strconv.Quote(lines[at])
}

func TestFirstDifferingLine(t *testing.T) {
	tests := []struct {
		name string
		want string
		got  string
		diff string
	}{
		{
			name: "a changed line",
			want: "a\nb\nc",
			got:  "a\nB\nc",
			diff: "line 2:\nwant: \"b\"\ngot:  \"B\"",
		},
		{
			name: "trailing whitespace",
			want: "a",
			got:  "a ",
			diff: "line 1:\nwant: \"a\"\ngot:  \"a \"",
		},
		{
			name: "missing final newline",
			want: "a\n",
			got:  "a",
			diff: "line 2:\nwant: \"\"\ngot:  <past the end>",
		},
		{
			name: "equal",
			want: "a\nb",
			got:  "a\nb",
			diff: "no line: the two are equal",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := firstDifferingLine([]byte(test.want), []byte(test.got)); got != test.diff {
				t.Errorf("got %q, want %q", got, test.diff)
			}
		})
	}
}
