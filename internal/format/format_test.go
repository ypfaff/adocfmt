package format

import (
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

// TestFormatRejects pins that source the scanner must not repair silently is
// refused with a reason.
func TestFormatRejects(t *testing.T) {
	t.Parallel()

	_, err := Format([]byte("a\r\nb\n"))
	if err == nil || !strings.Contains(err.Error(), "mixed line endings") {
		t.Errorf("got error %v, want one about mixed line endings", err)
	}
}
