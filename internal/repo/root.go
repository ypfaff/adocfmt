// Package repo locates this repository on disk, so tools reach the same files
// from any working directory.
package repo

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Root returns the directory holding this module's go.mod.
func Root() (string, error) {
	out, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		return "", fmt.Errorf("locating the module: %w", err)
	}

	// An empty path means GOPATH mode, os.DevNull module-aware mode without a
	// go.mod. Either way there is no module to write into.
	goMod := strings.TrimSpace(string(out))
	if goMod == "" || goMod == os.DevNull {
		return "", errors.New("run this from inside the repository")
	}
	return filepath.Dir(goMod), nil
}
