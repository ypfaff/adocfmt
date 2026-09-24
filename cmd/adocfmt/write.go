package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ypfaff/adocfmt/internal/format"
)

// writeAll leaves a document the scanner refuses as it is, and still writes the
// surrounding files.
func writeAll(paths []string, stderr io.Writer) int {
	return eachFile(paths, stderr, func(src []byte, path string) int {
		out, err := format.Format(src)
		if err != nil {
			report(stderr, path, err)
			return exitError
		}
		if bytes.Equal(src, out) {
			return exitOK
		}
		if err := replace(path, out); err != nil {
			return fail(stderr, "%v", err)
		}
		return exitOK
	})
}

// replace puts out in the file at path.
//
// The result goes to a temporary file beside the target and is renamed onto it,
// so a killed process leaves the original whole rather than half rewritten. A
// hard link, extended attributes and the owner do not survive it, because the
// name comes to point at a file of its own.
func replace(path string, out []byte) error {
	// A link renamed onto would become a regular file, and the document behind
	// it would never be written.
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	info, err := os.Stat(target)
	if err != nil {
		return err
	}

	temp, err := os.CreateTemp(filepath.Dir(target), filepath.Base(target)+".")
	if err != nil {
		return writeError(path, err)
	}
	// A no-op once the rename has moved it away.
	defer func() { _ = os.Remove(temp.Name()) }()

	if _, err := temp.Write(out); err != nil {
		_ = temp.Close()
		return writeError(path, err)
	}
	if err := temp.Close(); err != nil {
		return writeError(path, err)
	}
	// os.CreateTemp makes a file only its owner may read, and that mode would
	// travel with the rename. Mode carries the file type as well, so the mask
	// names what chmod is allowed to set.
	mode := info.Mode() & (fs.ModePerm | fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky)
	if err := os.Chmod(temp.Name(), mode); err != nil {
		return writeError(path, err)
	}
	if err := os.Rename(temp.Name(), target); err != nil {
		return writeError(path, err)
	}
	return nil
}

// writeError names the file the command was asked to write, because the errors
// under it name the temporary file beside it, whose random name stands for
// nothing a reader can look up. EvalSymlinks and Stat name a file that is
// really there, so they are left as they are.
func writeError(path string, err error) error {
	return fmt.Errorf("writing %s: %w", path, err)
}
