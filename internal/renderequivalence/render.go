package renderequivalence

import (
	"bytes"
	"fmt"
	"os/exec"
)

// render converts src to embedded HTML with Asciidoctor.
//
// Warnings go to stderr and are ignored: malformed input is a legitimate test
// case, and the same warning appears on both sides of every comparison.
func render(src []byte) (string, error) {
	cmd := exec.Command("asciidoctor", "--embedded", "--safe-mode", "safe", "--out-file", "-", "-")
	cmd.Stdin = bytes.NewReader(src)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("asciidoctor: %w: %s", err, stderr.String())
	}
	return stdout.String(), nil
}
