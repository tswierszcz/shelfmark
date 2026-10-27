package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// TopicsFetcher fetches the GitHub topics configured on a repo, once the
// repo's "owner/repo" is known (from a RepoFinder).
type TopicsFetcher interface {
	Topics(ctx context.Context, repo string) ([]string, error)
}

// GitHubClient implements TopicsFetcher via the GitHub REST API.
type GitHubClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewGitHubClient constructs a GitHubClient. GITHUB_TOKEN is optional —
// its absence is not an error, since unauthenticated calls to the GitHub
// API still work, just at a lower rate limit. GITHUB_API_BASE_URL is unset in
// production; local E2E runs point it at the mock.
func NewGitHubClient() *GitHubClient {
	return &GitHubClient{
		baseURL:    envOr("GITHUB_API_BASE_URL", "https://api.github.com"),
		token:      os.Getenv("GITHUB_TOKEN"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

type githubRepoResponse struct {
	Topics []string `json:"topics"`
}

// Topics implements TopicsFetcher. repo is "owner/repo"; it's split on "/"
// to build the request path.
func (c *GitHubClient) Topics(ctx context.Context, repo string) ([]string, error) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("catalog: invalid github repo %q (expected \"owner/repo\")", repo)
	}
	owner, name := parts[0], parts[1]

	endpoint := fmt.Sprintf("%s/repos/%s/%s", c.baseURL, owner, name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("catalog: failed to build github request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("catalog: github request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("catalog: github returned status %d for repo %q", resp.StatusCode, repo)
	}

	var decoded githubRepoResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("catalog: failed to decode github response: %w", err)
	}

	return decoded.Topics, nil
}

// envOr returns the environment variable key, or fallback when it is unset.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
