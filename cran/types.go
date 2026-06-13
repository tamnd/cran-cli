package cran

// cranPackage is the raw JSON shape returned by crandb.r-pkg.org/{name}.
// Depends/Imports/Suggests are JSON objects: {"pkgname": "version-constraint"}.
type cranPackage struct {
	Package     string            `json:"Package"`
	Title       string            `json:"Title"`
	Version     string            `json:"Version"`
	Description string            `json:"Description"`
	License     string            `json:"License"`
	URL         string            `json:"URL"`
	Author      string            `json:"Author"`
	Maintainer  string            `json:"Maintainer"`
	Imports     map[string]string `json:"Imports"`
	Depends     map[string]string `json:"Depends"`
	Suggests    map[string]string `json:"Suggests"`
}

// Package is the output record for a CRAN package.
type Package struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description"`
	License     string `json:"license"`
	URL         string `json:"url"`
	Author      string `json:"author"`
	Maintainer  string `json:"maintainer"`
}

// LatestEntry is one entry in the /-/latest listing.
type LatestEntry struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// SearchResult is one match from a package search.
type SearchResult struct {
	Name  string `json:"name"`
	Title string `json:"title"`
}

// Dep is a direct dependency of a package.
type Dep struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Version string `json:"version"`
}
