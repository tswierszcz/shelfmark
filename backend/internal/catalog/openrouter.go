package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// defaultOpenRouterModel is used when OPENROUTER_MODEL is unset. This is a
// chosen default, not a hard requirement — operators can override it per
// deployment.
const defaultOpenRouterModel = "glm-5.3"

// OpenRouterClient is an LLM-backed fallback used in two places in the
// categorization pipeline: resolving a library's GitHub repo when deps.dev
// doesn't know it, and validating/extending GitHub-topic-derived categories
// against the curated taxonomy.
type OpenRouterClient struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewOpenRouterClient reads OPENROUTER_API_KEY (required, fail-fast — same
// convention as auth.LoadAccounts) and OPENROUTER_MODEL (optional, defaults
// to defaultOpenRouterModel) from the environment.
func NewOpenRouterClient() (*OpenRouterClient, error) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("catalog: missing required env var OPENROUTER_API_KEY")
	}

	model := os.Getenv("OPENROUTER_MODEL")
	if model == "" {
		model = defaultOpenRouterModel
	}

	return &OpenRouterClient{
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

const openRouterChatEndpoint = "https://openrouter.ai/api/v1/chat/completions"

// chatMessage and chatCompletionRequest/Response model the subset of the
// OpenAI-compatible chat completions shape OpenRouter expects/returns.
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// githubRepoPattern extracts a "github.com/owner/repo"-shaped substring
// from free-form model text.
var githubRepoPattern = regexp.MustCompile(`github\.com/([A-Za-z0-9._-]+)/([A-Za-z0-9._-]+)`)

// complete sends a single-turn chat completion request and returns the
// first choice's message content.
func (c *OpenRouterClient) complete(ctx context.Context, prompt string) (string, error) {
	reqBody := chatCompletionRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "user", Content: prompt},
		},
	}
	encoded, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("catalog: failed to encode openrouter request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterChatEndpoint, bytes.NewReader(encoded))
	if err != nil {
		return "", fmt.Errorf("catalog: failed to build openrouter request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("catalog: openrouter request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("catalog: openrouter returned status %d", resp.StatusCode)
	}

	var decoded chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", fmt.Errorf("catalog: failed to decode openrouter response: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return "", fmt.Errorf("catalog: openrouter response had no choices")
	}

	return decoded.Choices[0].Message.Content, nil
}

// sanitizeForPrompt neutralizes attacker-controlled strings (e.g.
// purl-derived group/artifact/version/purl fields) before they're
// interpolated into an LLM prompt. It replaces every Unicode control
// (Cc: newlines, carriage returns, tabs, etc.) and format (Cf: bidi
// overrides, zero-width characters) rune with a single space, collapses repeated whitespace, trims the result, and
// truncates to 200 runes (appending "..." when truncated) so a
// maliciously huge field can't balloon the prompt.
func sanitizeForPrompt(s string) string {
	replaced := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return ' '
		}
		return r
	}, s)

	collapsed := strings.Join(strings.Fields(replaced), " ")

	runes := []rune(collapsed)
	if len(runes) > 200 {
		return string(runes[:200]) + "..."
	}
	return collapsed
}

// maxPromptTopics caps how many GitHub topics reach the Categorize prompt.
const maxPromptTopics = 20

// sanitizeTopicsForPrompt applies sanitizeForPrompt to each GitHub topic
// (third-party controlled) and caps the count, returning a new slice.
func sanitizeTopicsForPrompt(topics []string) []string {
	if len(topics) > maxPromptTopics {
		topics = topics[:maxPromptTopics]
	}
	sanitized := make([]string, 0, len(topics))
	for _, t := range topics {
		sanitized = append(sanitized, sanitizeForPrompt(t))
	}
	return sanitized
}

// FindRepo implements RepoFinder as an LLM-backed fallback for when
// deps.dev has no mapping for the coordinate. found=false (err=nil) when
// the model answers "unknown" or its answer contains no
// github.com/owner/repo-shaped substring.
func (c *OpenRouterClient) FindRepo(ctx context.Context, group, artifact, version string) (string, bool, error) {
	group = sanitizeForPrompt(group)
	artifact = sanitizeForPrompt(artifact)
	version = sanitizeForPrompt(version)

	prompt := fmt.Sprintf(
		"What is the GitHub repository URL for the Maven library with group ID %q, artifact ID %q, version %q? "+
			"Respond with ONLY the GitHub URL (e.g. https://github.com/owner/repo), or the literal word \"unknown\" if you don't know.",
		group, artifact, version,
	)

	content, err := c.complete(ctx, prompt)
	if err != nil {
		return "", false, err
	}

	if strings.TrimSpace(strings.ToLower(content)) == "unknown" {
		return "", false, nil
	}

	match := githubRepoPattern.FindStringSubmatch(content)
	if match == nil {
		return "", false, nil
	}

	owner := strings.TrimSuffix(match[1], ".git")
	repoName := strings.TrimSuffix(match[2], ".git")

	return owner + "/" + repoName, true, nil
}

// Categorize asks the model to pick, from taxonomy, the category names it's
// confident apply to a library given its GitHub topics. confident is true
// iff the model returned at least one category. A response that isn't
// valid JSON is treated as confident=false with an empty slice — a
// confused model response is not a transport failure.
func (c *OpenRouterClient) Categorize(ctx context.Context, purl string, githubTopics []string, taxonomy []string) ([]string, bool, error) {
	purl = sanitizeForPrompt(purl)
	githubTopics = sanitizeTopicsForPrompt(githubTopics)

	prompt := fmt.Sprintf(
		"A software library has the package URL %q and GitHub topics %v.\n"+
			"Given this taxonomy of categories: %v\n"+
			"Respond with ONLY a JSON array of the taxonomy category names you are confident apply to this library "+
			"(e.g. [\"http-client\", \"json-processing\"]), or an empty JSON array [] if none apply confidently. "+
			"Do not include any category name that is not in the taxonomy list.",
		purl, githubTopics, taxonomy,
	)

	content, err := c.complete(ctx, prompt)
	if err != nil {
		return nil, false, err
	}

	var categories []string
	if err := json.Unmarshal([]byte(extractJSONArray(content)), &categories); err != nil {
		return []string{}, false, nil
	}

	return categories, len(categories) > 0, nil
}

// extractJSONArray trims model chatter around a JSON array, returning the
// substring from the first '[' to the last ']' inclusive. If no bracket
// pair is found, the original string is returned unchanged (and will fail
// to parse as JSON, which callers treat as "not confident").
func extractJSONArray(s string) string {
	start := strings.Index(s, "[")
	end := strings.LastIndex(s, "]")
	if start == -1 || end == -1 || end < start {
		return s
	}
	return s[start : end+1]
}
