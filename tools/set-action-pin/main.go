// Command set-action-pin rewrites the pinned uses: ypfaff/adocfmt@<sha> # vX.Y.Z
// line in a workflow or action file, the same way Renovate updates it:
//
//	go run ./tools/set-action-pin --sha SHA --version X.Y.Z --file .github/actions/check-docs/action.yml
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
)

var pinLine = regexp.MustCompile(`uses: ypfaff/adocfmt@[0-9a-f]{40} # v\d+\.\d+\.\d+`)

func main() {
	log.SetFlags(0)

	sha := flag.String("sha", "", "the full commit SHA of the release tag")
	version := flag.String("version", "", "the version to write, without the leading v")
	file := flag.String("file", "", "the file to rewrite")
	flag.Parse()

	if err := run(*sha, *version, *file); err != nil {
		log.Fatalln("error:", err)
	}
}

func run(sha, version, file string) error {
	if sha == "" || version == "" || file == "" {
		return errors.New("--sha, --version and --file are required")
	}

	content, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	rewritten, err := setPin(content, sha, version)
	if err != nil {
		return fmt.Errorf("%s: %w", file, err)
	}
	if err := os.WriteFile(file, rewritten, 0o644); err != nil {
		return err
	}
	log.Printf("pinned ypfaff/adocfmt to %s (v%s)", sha, version)
	return nil
}

func setPin(content []byte, sha, version string) ([]byte, error) {
	if !pinLine.Match(content) {
		return nil, errors.New("no line pins ypfaff/adocfmt to a commit SHA")
	}
	pin := fmt.Sprintf("uses: ypfaff/adocfmt@%s # v%s", sha, version)
	return pinLine.ReplaceAllLiteral(content, []byte(pin)), nil
}
