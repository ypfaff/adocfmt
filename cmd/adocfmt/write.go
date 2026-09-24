package main

import (
	"io/fs"
	"os"
	"path/filepath"
)

// replace puts out in the file at path.
//
// The result goes to a temporary file beside the target and is renamed onto it,
// so a run cut short leaves the original whole rather than half rewritten. The
// document therefore cannot be lost; a hard link, extended attributes and the
// owner can, because the name comes to point at a file of its own.
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
		return err
	}
	// A no-op once the rename has moved it away.
	defer func() { _ = os.Remove(temp.Name()) }()

	if _, err := temp.Write(out); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	// os.CreateTemp makes a file only its owner may read, and that mode would
	// travel with the rename. Mode carries the file type as well, so the mask
	// names what chmod is allowed to set.
	mode := info.Mode() & (fs.ModePerm | fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky)
	if err := os.Chmod(temp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(temp.Name(), target)
}
