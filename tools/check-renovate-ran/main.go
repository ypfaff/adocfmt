// Command check-renovate-ran fails when Renovate has stopped running on this
// repository. Renovate rewrites its Dependency Dashboard whenever the content
// changes, which here is every day or two, so a dashboard that stayed the same
// for days means that Renovate did not run:
//
//	GITHUB_REPOSITORY=ypfaff/adocfmt GITHUB_TOKEN="$(gh auth token)" go run ./tools/check-renovate-ran
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

const dashboardIssue = 55

// Far above the longest gap between two changes of a healthy dashboard, which
// was 35 hours.
const maxAge = 5 * 24 * time.Hour

type dashboard struct {
	UpdatedAt time.Time `json:"updated_at"`
	URL       string    `json:"html_url"`
}

func main() {
	log.SetFlags(0)

	if err := run(context.Background()); err != nil {
		// GitHub Actions turns the line into an annotation on the run.
		log.Fatalf("::error::%v", err)
	}
}

func run(ctx context.Context) error {
	repository, token := os.Getenv("GITHUB_REPOSITORY"), os.Getenv("GITHUB_TOKEN")
	if repository == "" || token == "" {
		return errors.New("GITHUB_REPOSITORY and GITHUB_TOKEN are required")
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%d", repository, dashboardIssue)
	d, err := fetchDashboard(ctx, url, token)
	if err != nil {
		return err
	}
	if err := check(d, time.Now(), maxAge); err != nil {
		return err
	}
	log.Printf("Renovate last changed its Dependency Dashboard at %s", d.UpdatedAt.Format(time.RFC3339))
	return nil
}

func fetchDashboard(ctx context.Context, url, token string) (dashboard, error) {
	// A stalled connection would otherwise hang the run until CI kills it.
	client := &http.Client{Timeout: time.Minute}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return dashboard{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return dashboard{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return dashboard{}, fmt.Errorf("fetching %s: %s", url, resp.Status)
	}

	var d dashboard
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return dashboard{}, fmt.Errorf("fetching %s: %w", url, err)
	}
	return d, nil
}

func check(d dashboard, now time.Time, maxAge time.Duration) error {
	age := now.Sub(d.UpdatedAt)
	if age <= maxAge {
		return nil
	}
	return fmt.Errorf(
		"the Dependency Dashboard has not changed for %d days, so Renovate has stopped running; "+
			"tick the manual job checkbox at the bottom of %s",
		int(age.Hours()/24), d.URL,
	)
}
