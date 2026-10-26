package catalog

import (
	"context"
	"fmt"
	"strings"

	"github.com/package-url/packageurl-go"
)

// Categorizer validates/extends a set of GitHub-topic-derived categories
// against the curated taxonomy via an LLM. This is a small interface
// extracted from *OpenRouterClient's Categorize method (ADAPT: the plan
// types ProcessLibrary's llm parameter as "*OpenRouterClient" directly,
// which blocks substituting a fake in tests; Categorizer keeps the
// production call site identical since *OpenRouterClient already satisfies
// it) so ProcessLibrary can be tested with a fake.
type Categorizer interface {
	Categorize(ctx context.Context, purl string, githubTopics []string, taxonomy []string) (categories []string, confident bool, err error)
}

// ProcessLibrary runs the per-library categorization cascade: find the
// library's GitHub repo (trying each RepoFinder in order until one
// succeeds), fetch its GitHub topics, then ask the LLM to pick taxonomy
// categories that confidently apply. Categories are persisted via
// store.AttachCategory, with source "github" when a GitHub topic
// case-insensitively matches the category name exactly, else "llm".
//
// If no RepoFinder resolves a repo, or the LLM returns zero confident
// categories, the library is marked "needs_manual" and this returns nil —
// that's an expected cascade outcome, not a pipeline error. A non-nil
// return means a transport/transient failure occurred somewhere in the
// cascade; the caller (the worker pool queue) is responsible for retrying.
func ProcessLibrary(ctx context.Context, store *Store, repoFinders []RepoFinder, topics TopicsFetcher, llm Categorizer, lib Library) error {
	parsed, err := packageurl.FromString(lib.Purl)
	if err != nil {
		return fmt.Errorf("catalog: failed to parse purl %q: %w", lib.Purl, err)
	}
	group, artifact, version := parsed.Namespace, parsed.Name, parsed.Version

	repo, found, err := findRepo(ctx, repoFinders, group, artifact, version)
	if err != nil {
		return err
	}
	if !found {
		return store.UpdateLibraryStatus(ctx, lib.ID, "needs_manual", nil, nil)
	}

	githubTopics, err := topics.Topics(ctx, repo)
	if err != nil {
		return fmt.Errorf("catalog: failed to fetch github topics for %q: %w", repo, err)
	}

	taxonomy, err := store.ListTaxonomy(ctx)
	if err != nil {
		return fmt.Errorf("catalog: failed to list taxonomy: %w", err)
	}
	taxonomyNames := make([]string, len(taxonomy))
	for i, c := range taxonomy {
		taxonomyNames[i] = c.Name
	}

	categories, confident, err := llm.Categorize(ctx, lib.Purl, githubTopics, taxonomyNames)
	if err != nil {
		return fmt.Errorf("catalog: failed to categorize %q: %w", lib.Purl, err)
	}
	if !confident {
		return store.UpdateLibraryStatus(ctx, lib.ID, "needs_manual", &repo, nil)
	}

	for _, categoryName := range categories {
		categoryID, ok := taxonomyIDByName(taxonomy, categoryName)
		if !ok {
			// LLM returned a name outside the taxonomy; nothing to attach it to.
			continue
		}

		source := "llm"
		if matchesAnyTopic(categoryName, githubTopics) {
			source = "github"
		}

		if err := store.AttachCategory(ctx, lib.ID, categoryID, source); err != nil {
			return fmt.Errorf("catalog: failed to attach category %q to %q: %w", categoryName, lib.Purl, err)
		}
	}

	return store.UpdateLibraryStatus(ctx, lib.ID, "categorized", &repo, nil)
}

// findRepo tries each finder in order until one reports found=true. A
// finder error is returned immediately (a transient failure anywhere in
// the chain aborts the cascade for this attempt, letting the worker pool's
// retry re-run it from the top).
func findRepo(ctx context.Context, finders []RepoFinder, group, artifact, version string) (string, bool, error) {
	for _, finder := range finders {
		repo, found, err := finder.FindRepo(ctx, group, artifact, version)
		if err != nil {
			return "", false, fmt.Errorf("catalog: repo finder failed: %w", err)
		}
		if found {
			return repo, true, nil
		}
	}
	return "", false, nil
}

func taxonomyIDByName(taxonomy []Category, name string) (string, bool) {
	for _, c := range taxonomy {
		if c.Name == name {
			return c.ID, true
		}
	}
	return "", false
}

func matchesAnyTopic(categoryName string, topics []string) bool {
	for _, topic := range topics {
		if strings.EqualFold(topic, categoryName) {
			return true
		}
	}
	return false
}
