// Command set-action-version rewrites the version input's default in
// action.yml, so uses: ypfaff/adocfmt@vX.Y.Z installs that exact release
// without the version input:
//
//	go run ./tools/set-action-version --version X.Y.Z --file action.yml
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"slices"
)

var defaultLine = regexp.MustCompile(`default: '([^']*)' # adocfmt-version-marker`)

func main() {
	log.SetFlags(0)

	version := flag.String("version", "", "the version to write, without the leading v")
	file := flag.String("file", "", "the action.yml to rewrite")
	flag.Parse()

	if err := run(*version, *file); err != nil {
		log.Fatalln("error:", err)
	}
}

func run(version, file string) error {
	if version == "" || file == "" {
		return errors.New("--version and --file are required")
	}

	content, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	rewritten, err := setDefault(content, version)
	if err != nil {
		return fmt.Errorf("%s: %w", file, err)
	}
	if err := os.WriteFile(file, rewritten, 0o644); err != nil {
		return err
	}
	log.Printf("set the action's version default to %s", version)
	return nil
}

func setDefault(content []byte, version string) ([]byte, error) {
	loc := defaultLine.FindSubmatchIndex(content)
	if loc == nil {
		return nil, errors.New("no default line carries the version marker")
	}
	return slices.Concat(content[:loc[2]], []byte(version), content[loc[3]:]), nil
}
