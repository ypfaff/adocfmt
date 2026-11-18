package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCheck(t *testing.T) {
	now := time.Date(2026, time.November, 18, 16, 8, 0, 0, time.UTC)
	const url = "https://github.com/ypfaff/adocfmt/issues/55"
	tests := []struct {
		name      string
		updatedAt time.Time
		want      string
	}{
		{name: "changed just now", updatedAt: now},
		{name: "the longest healthy gap", updatedAt: now.Add(-35 * time.Hour)},
		{name: "exactly the maximum age", updatedAt: now.Add(-maxAge)},
		{
			name:      "just over the maximum age",
			updatedAt: now.Add(-maxAge - time.Second),
			want: "the Dependency Dashboard has not changed for 5 days, so Renovate has stopped running; " +
				"tick the manual job checkbox at the bottom of " + url,
		},
		{
			name:      "the outage of November 2026",
			updatedAt: time.Date(2026, time.October, 31, 9, 0, 0, 0, time.UTC),
			want: "the Dependency Dashboard has not changed for 18 days, so Renovate has stopped running; " +
				"tick the manual job checkbox at the bottom of " + url,
		},
	}
	for _, tt := range tests {
		err := check(dashboard{UpdatedAt: tt.updatedAt, URL: url}, now, maxAge)
		got := ""
		if err != nil {
			got = err.Error()
		}
		if got != tt.want {
			t.Errorf("%s: check() = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestFetchDashboard(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			http.Error(w, "bad credentials", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"updated_at": "2026-10-31T09:00:00Z", "html_url": "https://example.com/issues/55"}`))
	}))
	defer server.Close()

	got, err := fetchDashboard(t.Context(), server.URL, "secret")
	want := dashboard{
		UpdatedAt: time.Date(2026, time.October, 31, 9, 0, 0, 0, time.UTC),
		URL:       "https://example.com/issues/55",
	}
	if err != nil || got != want {
		t.Errorf("fetchDashboard() = %v, %v, want %v", got, err, want)
	}

	// An error from the API must fail the run, not pass as a fresh dashboard.
	if _, err := fetchDashboard(t.Context(), server.URL, "wrong"); err == nil {
		t.Error("fetchDashboard() with a rejected token returned no error")
	}
}
