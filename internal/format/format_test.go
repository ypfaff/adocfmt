package format

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// TestFormatRefuses pins that a document the scanner could not read safely
// comes back as an error naming every finding, and never as output.
func TestFormatRefuses(t *testing.T) {
	t.Parallel()

	out, err := Format([]byte("ifdef::extra[]\n----\nendif::[]\ncode\n"))
	if out != nil {
		t.Errorf("got output %q, want none", out)
	}
	want := "line 2: block has no closing delimiter\nline 2: delimiter opens and closes in different conditional regions"
	if err == nil || err.Error() != want {
		t.Errorf("got error %v, want %q", err, want)
	}
}

// TestFormatRefusalCarriesFindings pins that a caller reaches the line of every
// finding, which is what lets the command write path:line: message.
func TestFormatRefusalCarriesFindings(t *testing.T) {
	t.Parallel()

	_, err := Format([]byte("ifdef::extra[]\n\n----\ncode\n"))

	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("got error %v, want a *Refusal", err)
	}
	want := []Finding{
		{Line: 1, Message: "conditional region has no endif"},
		{Line: 3, Message: "block has no closing delimiter"},
	}
	if !slices.Equal(refusal.Findings, want) {
		t.Errorf("got %+v, want %+v", refusal.Findings, want)
	}
}

// TestFormatRejects pins that source the scanner must not repair silently is
// refused with a reason, and that it is no Refusal: it names no line, so the
// command has nothing to place it on.
func TestFormatRejects(t *testing.T) {
	t.Parallel()

	_, err := Format([]byte("a\r\nb\n"))
	if err == nil || !strings.Contains(err.Error(), "mixed line endings") {
		t.Errorf("got error %v, want one about mixed line endings", err)
	}

	var refusal *Refusal
	if errors.As(err, &refusal) {
		t.Errorf("got a *Refusal, want a plain error")
	}
}

// TestRefusalWithoutFindings pins that the exported type answers rather than
// panics when it holds nothing, which errors.As lets a caller reach.
func TestRefusalWithoutFindings(t *testing.T) {
	t.Parallel()

	for name, refusal := range map[string]*Refusal{"nil": nil, "empty": {}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got, want := refusal.Error(), "refused without a finding"; got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}
