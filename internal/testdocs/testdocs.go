// Package testdocs lists the AsciiDoc documents the tests read: every
// Asciidoctor case and the input of every golden case.
//
// See docs/contributing/testing-strategy.adoc for what each set covers.
package testdocs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ypfaff/adocfmt/internal/asciidoctorcases"
	"github.com/ypfaff/adocfmt/internal/golden"
	"github.com/ypfaff/adocfmt/internal/repo"
)

// Document is an AsciiDoc source and the name its subtest runs under.
type Document struct {
	Name string
	Src  []byte
}

// Documents returns every Asciidoctor case, then the input of every golden
// case, each in path order.
func Documents(tb testing.TB) []Document {
	tb.Helper()

	root, err := repo.Root()
	if err != nil {
		tb.Fatal(err)
	}
	asciidoctorDir := filepath.Join(root, asciidoctorcases.Dir)
	asciidoctorFiles, err := asciidoctorcases.Files(asciidoctorDir)
	if err != nil {
		tb.Fatal(err)
	}
	goldenDir := filepath.Join(root, golden.Dir)
	goldenCases, err := golden.Cases(goldenDir)
	if err != nil {
		tb.Fatal(err)
	}

	// An Asciidoctor case is a single file. A golden case is a directory,
	// and only its input file is read.
	var docs []Document
	for _, caseFile := range asciidoctorFiles {
		docs = append(docs, read(tb, name(asciidoctorDir, caseFile), caseFile))
	}
	for _, caseDir := range goldenCases {
		docs = append(docs, read(tb, name(goldenDir, caseDir), filepath.Join(caseDir, golden.InputFile)))
	}
	return docs
}

// name is the path of a case below root, without the extension, so it reads
// the same on every platform.
func name(root, path string) string {
	rel := strings.TrimPrefix(filepath.ToSlash(path), filepath.ToSlash(root)+"/")
	return strings.TrimSuffix(rel, ".adoc")
}

func read(tb testing.TB, name, path string) Document {
	tb.Helper()

	src, err := os.ReadFile(path)
	if err != nil {
		tb.Fatal(err)
	}
	return Document{Name: name, Src: src}
}
