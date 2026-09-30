// Command build-website builds the website from docs/ into website/public:
//
//	go run ./tools/build-website
//
// The --serve flag serves it afterwards at http://localhost:8080/adocfmt/, the
// path GitHub Pages serves it under.
//
// Asciidoctor has to be on the PATH; see
// docs/contributing/development-setup.adoc.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ypfaff/adocfmt/internal/repo"
)

const (
	siteDir = "website"
	// The site sits one directory below the output root, as it does on GitHub
	// Pages, so its root-relative links resolve when it is served or checked.
	outDir   = "website/public"
	basePath = "adocfmt"
	address  = "localhost:8080"
)

func main() {
	log.SetFlags(0)

	serve := flag.Bool("serve", false, "serve the site after building it")
	flag.Parse()

	if err := run(*serve); err != nil {
		log.Fatalln("error:", err)
	}
}

func run(serve bool) error {
	root, err := repo.Root()
	if err != nil {
		return err
	}
	if _, err := exec.LookPath("asciidoctor"); err != nil {
		return errors.New("asciidoctor is not on the PATH; see docs/contributing/development-setup.adoc")
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	cacheDir = filepath.Join(cacheDir, "adocfmt-website")
	packages, err := readLockfile(filepath.Join(root, lockfile))
	if err != nil {
		return err
	}

	out := filepath.Join(root, outDir)
	if err := os.RemoveAll(out); err != nil {
		return err
	}

	// A warning fails the build, so a missing layout or a broken page cannot
	// slip through to the deployed site. Not --quiet: it drops the errors too.
	args := []string{"tool", "-modfile=tools/hugo/go.mod", "hugo", "build",
		"--source", siteDir, "--destination", filepath.Join(out, basePath),
		"--minify", "--panicOnWarning"}
	if serve {
		args = append(args, "--baseURL", fmt.Sprintf("http://%s/%s/", address, basePath))
	}
	if err := runIn(root, "go", args...); err != nil {
		return fmt.Errorf("building with Hugo: %w", err)
	}

	// Pagefind indexes the built pages and writes its search bundle next to
	// them. It reads pagefind.yml from the website directory.
	pagefindBin, err := pagefind(cacheDir, packages)
	if err != nil {
		return err
	}
	if err := runIn(filepath.Join(root, siteDir), pagefindBin, "--site", filepath.Join(out, basePath)); err != nil {
		return fmt.Errorf("indexing with Pagefind: %w", err)
	}

	if !serve {
		log.Printf("built %s", outDir)
		return nil
	}
	log.Printf("serving http://%s/%s/", address, basePath)
	return http.ListenAndServe(address, http.FileServer(http.Dir(out)))
}

func runIn(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
