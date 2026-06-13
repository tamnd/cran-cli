package cran_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tamnd/cran-cli/cran"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) (*cran.Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	cfg := cran.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	return cran.NewClient(cfg), srv
}

// cranPackageFixture mirrors the real crandb JSON shape for test payloads.
type cranPackageFixture struct {
	Package     string            `json:"Package"`
	Title       string            `json:"Title"`
	Version     string            `json:"Version"`
	Description string            `json:"Description"`
	License     string            `json:"License"`
	URL         string            `json:"URL"`
	Author      string            `json:"Author"`
	Maintainer  string            `json:"Maintainer"`
	Imports     map[string]string `json:"Imports,omitempty"`
	Depends     map[string]string `json:"Depends,omitempty"`
	Suggests    map[string]string `json:"Suggests,omitempty"`
}

func TestGetSendsUserAgent(t *testing.T) {
	c, srv := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(cranPackageFixture{
			Package: "testpkg",
			Title:   "Test",
			Version: "1.0",
		})
	}))
	defer srv.Close()

	_, err := c.Package(context.Background(), "testpkg")
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(cranPackageFixture{Package: "pkg", Title: "T", Version: "1.0"})
	}))
	defer srv.Close()

	cfg := cran.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 5
	c := cran.NewClient(cfg)

	start := time.Now()
	_, err := c.Package(context.Background(), "pkg")
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

func TestPackage(t *testing.T) {
	payload := cranPackageFixture{
		Package:     "ggplot2",
		Title:       "Create Elegant Data Visualisations",
		Version:     "3.5.1",
		Description: "A plotting system.",
		License:     "MIT + file LICENSE",
		URL:         "https://ggplot2.tidyverse.org",
		Author:      "Hadley Wickham",
		Maintainer:  "Hadley Wickham",
		Imports:     map[string]string{"cli": "*", "glue": "*"},
	}
	c, srv := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	pkg, err := c.Package(context.Background(), "ggplot2")
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Name != "ggplot2" {
		t.Errorf("Name = %q, want %q", pkg.Name, "ggplot2")
	}
	if pkg.Version != "3.5.1" {
		t.Errorf("Version = %q, want %q", pkg.Version, "3.5.1")
	}
}

// descEntryFixture mirrors the real /-/desc JSON shape for test payloads.
type descEntryFixture struct {
	Version string `json:"version"`
	Title   string `json:"title"`
}

func TestSearch(t *testing.T) {
	payload := map[string]descEntryFixture{
		"ggplot2":   {Version: "4.0.0", Title: "Create Elegant Data Visualisations"},
		"neuralnet": {Version: "1.44.2", Title: "Training of Neural Networks"},
		"caret":     {Version: "6.0-94", Title: "Classification and Regression Training"},
		"deepnet":   {Version: "0.2", Title: "Deep Learning Toolkit"},
	}
	c, srv := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	results, err := c.Search(context.Background(), "neural", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results, got none")
	}
	for _, r := range results {
		if r.Name == "" {
			t.Error("result has empty Name")
		}
	}
}

func TestLatest(t *testing.T) {
	payload := map[string]descEntryFixture{
		"ggplot2": {Version: "3.5.1", Title: "Create Elegant Data Visualisations"},
		"dplyr":   {Version: "1.1.4", Title: "A Grammar of Data Manipulation"},
		"tidyr":   {Version: "1.3.1", Title: "Tidy Messy Data"},
	}
	c, srv := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	entries, err := c.Latest(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("got %d entries, want 2", len(entries))
	}
}

func TestDeps(t *testing.T) {
	payload := cranPackageFixture{
		Package: "ggplot2",
		Title:   "Elegant Data Visualisations",
		Version: "3.5.1",
		Imports: map[string]string{"cli": "*", "glue": "*", "rlang": ">= 1.1.0"},
		Depends: map[string]string{"R": ">= 3.3"},
	}
	c, srv := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	deps, err := c.Deps(context.Background(), "ggplot2")
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) == 0 {
		t.Fatal("expected deps, got none")
	}
	names := map[string]bool{}
	for _, d := range deps {
		names[d.Name] = true
	}
	if !names["cli"] {
		t.Error("expected cli in imports")
	}
	if !names["rlang"] {
		t.Error("expected rlang in imports")
	}
	// R should be filtered out
	if names["R"] {
		t.Error("R should not appear in deps")
	}
}

func TestNotFound(t *testing.T) {
	c, srv := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := c.Package(context.Background(), "nonexistent-pkg-xyz")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}
