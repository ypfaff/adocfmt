// Command calculate-version prints the version the next release gets, without
// the leading v, derived from the newest v* tag or given explicitly:
//
//	go run ./tools/calculate-version --bump major|minor|patch
//	go run ./tools/calculate-version --explicit X.Y.Z
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

var semver = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

func main() {
	log.SetFlags(0)

	bump := flag.String("bump", "", "raise the newest tag by this part: major, minor or patch")
	explicit := flag.String("explicit", "", "use this version instead of raising the newest tag")
	flag.Parse()

	if err := run(*bump, *explicit); err != nil {
		log.Fatalln("error:", err)
	}
}

func run(bump, explicit string) error {
	if (bump == "") == (explicit == "") {
		return errors.New("exactly one of --bump and --explicit is required")
	}

	version := strings.TrimPrefix(explicit, "v")
	if bump != "" {
		tag, err := latestTag()
		if err != nil {
			return err
		}
		if tag == "" {
			log.Print("no v* tag yet, counting from v0.0.0")
			tag = "v0.0.0"
		}
		log.Printf("newest tag is %s", tag)
		if version, err = raise(strings.TrimPrefix(tag, "v"), bump); err != nil {
			return err
		}
	}
	if err := check(version); err != nil {
		return err
	}

	// A tag that exists would make the release overwrite a published one.
	exists, err := tagExists("v" + version)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("tag v%s exists already", version)
	}

	fmt.Println(version)
	return nil
}

func latestTag() (string, error) {
	cmd := exec.Command("git", "tag", "--list", "v[0-9]*", "--sort=-version:refname")
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("listing the tags: %w", err)
	}
	first, _, _ := strings.Cut(string(out), "\n")
	return first, nil
}

func tagExists(tag string) (bool, error) {
	cmd := exec.Command("git", "tag", "--list", tag)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("looking up %s: %w", tag, err)
	}
	return len(out) > 0, nil
}

func raise(version, part string) (string, error) {
	if err := check(version); err != nil {
		return "", err
	}
	var major, minor, patch int
	if _, err := fmt.Sscanf(version, "%d.%d.%d", &major, &minor, &patch); err != nil {
		return "", err
	}

	switch part {
	case "major":
		return fmt.Sprintf("%d.0.0", major+1), nil
	case "minor":
		return fmt.Sprintf("%d.%d.0", major, minor+1), nil
	case "patch":
		return fmt.Sprintf("%d.%d.%d", major, minor, patch+1), nil
	}
	return "", fmt.Errorf("unknown bump type %s, expected major, minor or patch", part)
}

func check(version string) error {
	if !semver.MatchString(version) {
		return fmt.Errorf("%s is no X.Y.Z version", version)
	}
	return nil
}
