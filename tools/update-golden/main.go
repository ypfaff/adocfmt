// Command update-golden rewrites every golden file from the formatter output:
//
//	go run ./tools/update-golden
//
// Read the diff before committing it: the tool records whatever the formatter
// produces, so an unreviewed update turns a bug into the expectation.
//
// See docs/testing-strategy.adoc for how the cases are used.
package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/ypfaff/adocfmt/internal/format"
	"github.com/ypfaff/adocfmt/internal/golden"
	"github.com/ypfaff/adocfmt/internal/repo"
)

func main() {
	log.SetFlags(0)

	if err := run(); err != nil {
		log.Fatalln("error:", err)
	}
}

func run() error {
	root, err := repo.Root()
	if err != nil {
		return err
	}

	cases, err := golden.Cases(filepath.Join(root, golden.Dir))
	if err != nil {
		return err
	}

	for _, dir := range cases {
		src, err := os.ReadFile(filepath.Join(dir, golden.InputFile))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, golden.GoldenFile), format.Format(src), 0o644); err != nil {
			return err
		}
		log.Println(strings.TrimPrefix(dir, root+string(filepath.Separator)))
	}
	return nil
}
