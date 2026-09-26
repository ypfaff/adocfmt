// Command generate-formula writes the Homebrew formula for a release from the
// checksums GoReleaser left in dist/:
//
//	go run ./tools/generate-formula --version X.Y.Z --checksums dist/checksums.txt --output HomebrewFormula/adocfmt.rb
package main

import (
	"bytes"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

const repository = "ypfaff/adocfmt"

// Homebrew runs on these; the release also builds Windows, which has no formula.
var platforms = []string{"darwin_arm64", "darwin_amd64", "linux_arm64", "linux_amd64"}

//go:embed formula.rb.tmpl
var formulaTemplate string

var tmpl = template.Must(template.New("formula").Parse(formulaTemplate))

type archive struct {
	URL, SHA256 string
}

func main() {
	log.SetFlags(0)

	version := flag.String("version", "", "the released version")
	checksums := flag.String("checksums", "", "GoReleaser's checksums.txt")
	output := flag.String("output", "", "where to write the formula")
	flag.Parse()

	if err := run(strings.TrimPrefix(*version, "v"), *checksums, *output); err != nil {
		log.Fatalln("error:", err)
	}
}

func run(version, checksums, output string) error {
	if version == "" || checksums == "" || output == "" {
		return errors.New("--version, --checksums and --output are required")
	}

	sums, err := os.ReadFile(checksums)
	if err != nil {
		return err
	}
	content, err := formula(version, sums)
	if err != nil {
		return fmt.Errorf("%s: %w", checksums, err)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(output, content, 0o644); err != nil {
		return err
	}
	log.Printf("wrote %s for v%s", output, version)
	return nil
}

func formula(version string, checksums []byte) ([]byte, error) {
	sums := map[string]string{}
	for line := range strings.Lines(string(checksums)) {
		if fields := strings.Fields(line); len(fields) == 2 {
			sums[fields[1]] = fields[0]
		}
	}

	archives := map[string]archive{}
	for _, platform := range platforms {
		name := fmt.Sprintf("adocfmt_%s_%s.tar.gz", version, platform)
		sha, ok := sums[name]
		if !ok {
			return nil, fmt.Errorf("no checksum for %s", name)
		}
		archives[platform] = archive{
			URL:    fmt.Sprintf("https://github.com/%s/releases/download/v%s/%s", repository, version, name),
			SHA256: sha,
		}
	}

	var out bytes.Buffer
	data := struct {
		Repository string
		Archives   map[string]archive
	}{repository, archives}
	if err := tmpl.Execute(&out, data); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
