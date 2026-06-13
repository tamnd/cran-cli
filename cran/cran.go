// Package cran is the library behind the cran command: the HTTP client,
// request shaping, and the typed data models for the CRAN R package repository.
//
// The crandb API at crandb.r-pkg.org is open and requires no key. It provides
// structured JSON for package metadata, search, and listings.
package cran

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const defaultBaseURL = "https://crandb.r-pkg.org"

// DefaultUserAgent identifies the client to CRAN.
const DefaultUserAgent = "cran/dev (+https://github.com/tamnd/cran-cli)"

// ErrNotFound is returned when the API returns a 404.
var ErrNotFound = errors.New("not found")

// Config holds constructor parameters.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   defaultBaseURL,
		UserAgent: DefaultUserAgent,
		Rate:      200 * time.Millisecond,
		Retries:   5,
		Timeout:   30 * time.Second,
	}
}

// Client talks to the CRAN crandb API.
type Client struct {
	httpClient *http.Client
	userAgent  string
	baseURL    string
	rate       time.Duration
	retries    int
	mu         sync.Mutex
	last       time.Time
}

// NewClient returns a Client configured from cfg.
func NewClient(cfg Config) *Client {
	base := cfg.BaseURL
	if base == "" {
		base = defaultBaseURL
	}
	return &Client{
		httpClient: &http.Client{Timeout: cfg.Timeout},
		userAgent:  cfg.UserAgent,
		baseURL:    strings.TrimRight(base, "/"),
		rate:       cfg.Rate,
		retries:    cfg.Retries,
	}
}

// get fetches a URL with pacing and retries.
func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, false, ErrNotFound
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rate <= 0 {
		return
	}
	if wait := c.rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

func (c *Client) getJSON(ctx context.Context, rawURL string, v any) error {
	body, err := c.get(ctx, rawURL)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("decode %s: %w", rawURL, err)
	}
	return nil
}

// Package fetches full metadata for a single CRAN package.
func (c *Client) Package(ctx context.Context, name string) (Package, error) {
	var raw cranPackage
	u := fmt.Sprintf("%s/%s", c.baseURL, name)
	if err := c.getJSON(ctx, u, &raw); err != nil {
		return Package{}, err
	}
	return rawToPackage(raw), nil
}

// descEntry is one entry from the /-/desc listing.
type descEntry struct {
	Version string `json:"version"`
	Title   string `json:"title"`
}

// desc fetches the /-/desc listing: a map of package name to {version, title}.
func (c *Client) desc(ctx context.Context) (map[string]descEntry, error) {
	u := c.baseURL + "/-/desc"
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var raw map[string]descEntry
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode /-/desc: %w", err)
	}
	return raw, nil
}

// Latest returns the first limit entries from the /-/desc listing.
func (c *Client) Latest(ctx context.Context, limit int) ([]LatestEntry, error) {
	raw, err := c.desc(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]LatestEntry, 0, limit)
	for name, e := range raw {
		out = append(out, LatestEntry{Name: name, Version: e.Version})
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// Search fetches /-/desc and does a case-insensitive substring match
// on the package name and title. Returns up to limit results.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	raw, err := c.desc(ctx)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(query)
	var out []SearchResult
	for name, e := range raw {
		if strings.Contains(strings.ToLower(name), q) || strings.Contains(strings.ToLower(e.Title), q) {
			out = append(out, SearchResult{Name: name, Title: e.Title})
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

// Deps fetches the direct dependencies of a package (Imports + Depends + Suggests).
func (c *Client) Deps(ctx context.Context, name string) ([]Dep, error) {
	var raw cranPackage
	u := fmt.Sprintf("%s/%s", c.baseURL, name)
	if err := c.getJSON(ctx, u, &raw); err != nil {
		return nil, err
	}
	var out []Dep
	out = append(out, mapToDeps(raw.Imports, "Imports")...)
	out = append(out, mapToDeps(raw.Depends, "Depends")...)
	out = append(out, mapToDeps(raw.Suggests, "Suggests")...)
	return out, nil
}

// mapToDeps converts the crandb dependency map {"pkgname":"version"} to Dep records,
// skipping the bare "R" entry which is not a real package.
func mapToDeps(m map[string]string, depType string) []Dep {
	if len(m) == 0 {
		return nil
	}
	out := make([]Dep, 0, len(m))
	for pkg, ver := range m {
		if strings.EqualFold(pkg, "R") {
			continue
		}
		v := ver
		if v == "*" {
			v = ""
		}
		out = append(out, Dep{Name: pkg, Type: depType, Version: v})
	}
	return out
}

func rawToPackage(r cranPackage) Package {
	desc := strings.TrimSpace(r.Description)
	desc = strings.ReplaceAll(desc, "\n", " ")
	return Package{
		Name:        r.Package,
		Title:       r.Title,
		Version:     r.Version,
		Description: desc,
		License:     r.License,
		URL:         r.URL,
		Author:      r.Author,
		Maintainer:  r.Maintainer,
	}
}
