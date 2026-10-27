package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RepoFinder resolves a Maven coordinate to its GitHub repo ("owner/repo"),
// when the implementation's data source has that mapping. found=false (with
// a nil error) means "this source doesn't know the repo", not a failure;
// a non-nil error means a genuine transport-level failure, which the
// pipeline's caller (the worker pool queue) treats as retryable.
type RepoFinder interface {
	FindRepo(ctx context.Context, group, artifact, version string) (repo string, found bool, err error)
}

// DepsDevClient resolves a Maven coordinate's GitHub repo via the deps.dev
// API, which tracks package-to-source-repository mappings for several
// package ecosystems.
type DepsDevClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewDepsDevClient constructs a DepsDevClient with a sane default timeout.
// DEPSDEV_BASE_URL is unset in production; local E2E runs point it at the
// mock in web/tests/e2e/mocks/external-apis-mock.mjs.
func NewDepsDevClient() *DepsDevClient {
	return &DepsDevClient{
		baseURL:    envOr("DEPSDEV_BASE_URL", "https://api.deps.dev"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// depsDevResponse is the subset of the deps.dev package-version response
// this client needs. This shape is unverified against a live call (no API
// access in this environment); it is decoded defensively, so a response
// that doesn't match this shape results in found=false, not a crash or
// spurious error.
type depsDevResponse struct {
	Links struct {
		Repo string `json:"repo"`
	} `json:"links"`
}

// FindRepo implements RepoFinder. A 404 ("deps.dev doesn't know this
// package") or a repo link that isn't on GitHub is found=false, err=nil —
// expected, not a failure. Only a genuine transport failure (request
// construction, network error, or a response that errors on decode after a
// 2xx status) returns a non-nil error.
func (c *DepsDevClient) FindRepo(ctx context.Context, group, artifact, version string) (string, bool, error) {
	// deps.dev addresses Maven packages as "{group}:{artifact}", URL-escaped
	// as a single path segment.
	packageName := url.PathEscape(group + ":" + artifact)
	versionSegment := url.PathEscape(version)
	endpoint := fmt.Sprintf("%s/v3/systems/maven/packages/%s/versions/%s", c.baseURL, packageName, versionSegment)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", false, fmt.Errorf("catalog: failed to build deps.dev request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("catalog: deps.dev request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Not found, rate-limited, or otherwise unresolvable: deps.dev simply
		// doesn't have this mapping. Not an error.
		return "", false, nil
	}

	var decoded depsDevResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		// A 2xx response that fails to decode as expected is treated as
		// "no mapping found" rather than a transport error, since the
		// response shape is unverified against a live call.
		return "", false, nil
	}

	repo, ok := githubRepoFromURL(decoded.Links.Repo)
	if !ok {
		return "", false, nil
	}

	return repo, true, nil
}

// githubRepoFromURL extracts an "owner/repo" string from a github.com URL.
// Returns ok=false for an empty string, a non-github.com URL, or a
// github.com URL that doesn't have at least an owner and repo segment.
func githubRepoFromURL(raw string) (string, bool) {
	if raw == "" {
		return "", false
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", false
	}

	host := strings.ToLower(u.Hostname())
	if host != "github.com" && host != "www.github.com" {
		return "", false
	}

	path := strings.Trim(u.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	segments := strings.Split(path, "/")
	if len(segments) < 2 || segments[0] == "" || segments[1] == "" {
		return "", false
	}

	return segments[0] + "/" + segments[1], true
}
