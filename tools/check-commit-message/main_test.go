package main

import (
	"slices"
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	tests := []struct {
		message string
		want    []string
	}{
		{message: "feat: add a rule"},
		{message: "fix(block)!: keep the blank line\n"},
		{message: "chore(deps): update module github.com/gohugoio/hugo to v0.168.0"},
		{message: "feat: `--check` reads stdin"},
		{message: "revert: drop the website\n\nRefs: 676104e\n"},
		{message: "docs: say why\n\nThe body.\n\nSigned-off-by: A <a@example.com>\n"},
		{message: "feat: add a rule\n# Please enter the commit message.\n"},
		{message: "feat: add a rule\n" + scissors + "\ndiff --git a/x b/x\n"},
		{message: "feat: add a rule\r\n\r\nThe body.\r\n"},
		{message: "feat: add a rule\n  \nThe body.\n"},
		{message: "feat: " + strings.Repeat("a", 66)},
		{
			message: "feat: " + strings.Repeat("a", 67),
			want:    []string{"the subject has 73 characters, more than 72"},
		},
		{
			message: "Fix the parser",
			want: []string{
				`the subject "Fix the parser" is not of the form type(scope)!: description, in lowercase`,
			},
		},
		{
			message: "feat(Block): add a rule",
			want: []string{
				`the subject "feat(Block): add a rule" is not of the form type(scope)!: description, in lowercase`,
			},
		},
		{
			message: "feat:  Add a rule",
			want: []string{
				`the subject "feat:  Add a rule" is not of the form type(scope)!: description, in lowercase`,
			},
		},
		{
			message: "fixup! feat: add a rule",
			want: []string{
				`the subject "fixup! feat: add a rule" is not of the form type(scope)!: description, in lowercase`,
			},
		},
		{
			message: "feature: add a rule",
			want: []string{
				`the type "feature" is none of build, chore, ci, docs, feat, fix, perf, refactor, revert, style, test`,
			},
		},
		{
			message: "feat: Add a rule",
			want:    []string{"the description starts with an uppercase letter"},
		},
		{
			message: "feat: add a rule\nThe body.",
			want:    []string{"a blank line must separate the subject from the body"},
		},
		{
			message: "",
			want:    []string{"the message is empty"},
		},
	}
	for _, tt := range tests {
		if got := check(tt.message); !slices.Equal(got, tt.want) {
			t.Errorf("check(%q) = %q, want %q", tt.message, got, tt.want)
		}
	}
}
