package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"

	"github.com/tswierszcz/shelfmark/backend/internal/auth"
	"github.com/tswierszcz/shelfmark/backend/internal/auth/authtest"
)

// Purls shared by the two multi-project report fixtures below.
const (
	reportPurlA = "pkg:maven/com.example/lib-a@1.0.0"
	reportPurlB = "pkg:maven/com.example/lib-b@1.0.0"
	reportPurlC = "pkg:maven/com.example/lib-c@1.0.0"
	reportPurlD = "pkg:maven/com.example/lib-d@1.0.0"
	reportPurlE = "pkg:maven/com.example/lib-e@1.0.0"
)

// reportAlphaSBOM: lib-a, lib-b and lib-c are direct; lib-d is transitive
// (pulled in by lib-c).
const reportAlphaSBOM = `{
  "bomFormat": "CycloneDX",
  "specVersion": "1.5",
  "version": 1,
  "metadata": {"component": {"type": "application", "bom-ref": "alpha-app", "name": "alpha-app", "version": "1.0.0"}},
  "components": [
    {"type": "library", "bom-ref": "a", "name": "lib-a", "purl": "pkg:maven/com.example/lib-a@1.0.0"},
    {"type": "library", "bom-ref": "b", "name": "lib-b", "purl": "pkg:maven/com.example/lib-b@1.0.0"},
    {"type": "library", "bom-ref": "c", "name": "lib-c", "purl": "pkg:maven/com.example/lib-c@1.0.0"},
    {"type": "library", "bom-ref": "d", "name": "lib-d", "purl": "pkg:maven/com.example/lib-d@1.0.0"}
  ],
  "dependencies": [
    {"ref": "alpha-app", "dependsOn": ["a", "b", "c"]},
    {"ref": "a", "dependsOn": []},
    {"ref": "b", "dependsOn": []},
    {"ref": "c", "dependsOn": ["d"]},
    {"ref": "d", "dependsOn": []}
  ]
}`

// reportBetaSBOM: lib-b, lib-d and lib-e are direct; lib-a is transitive
// (pulled in by lib-b). So lib-a is direct in Alpha but transitive here,
// lib-d is the reverse, and lib-c is absent.
const reportBetaSBOM = `{
  "bomFormat": "CycloneDX",
  "specVersion": "1.5",
  "version": 1,
  "metadata": {"component": {"type": "application", "bom-ref": "beta-app", "name": "beta-app", "version": "1.0.0"}},
  "components": [
    {"type": "library", "bom-ref": "a", "name": "lib-a", "purl": "pkg:maven/com.example/lib-a@1.0.0"},
    {"type": "library", "bom-ref": "b", "name": "lib-b", "purl": "pkg:maven/com.example/lib-b@1.0.0"},
    {"type": "library", "bom-ref": "d", "name": "lib-d", "purl": "pkg:maven/com.example/lib-d@1.0.0"},
    {"type": "library", "bom-ref": "e", "name": "lib-e", "purl": "pkg:maven/com.example/lib-e@1.0.0"}
  ],
  "dependencies": [
    {"ref": "beta-app", "dependsOn": ["b", "d", "e"]},
    {"ref": "a", "dependsOn": []},
    {"ref": "b", "dependsOn": ["a"]},
    {"ref": "d", "dependsOn": []},
    {"ref": "e", "dependsOn": []}
  ]
}`

// reportCategoryFlags is one category entry of a report library, without
// the category ID.
type reportCategoryFlags struct {
	Strategic       bool
	RetireCandidate bool
}

// reportLibrarySummary is a report library reduced to what must not depend
// on upload order: id and upload_batch_id differ between runs, so they are
// left out. Categories are keyed by category name.
type reportLibrarySummary struct {
	IsDirect   bool
	Categories map[string]reportCategoryFlags
}

// getProjectReport calls ProjectReportHandler for projectID and returns
// the report keyed by purl.
func getProjectReport(t *testing.T, store *Store, projectID string) map[string]reportLibrarySummary {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/report", nil)
	req.SetPathValue("projectID", projectID)
	rec := httptest.NewRecorder()
	ProjectReportHandler(store)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp []projectReportLibrary
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	byPurl := map[string]reportLibrarySummary{}
	for _, l := range resp {
		cats := map[string]reportCategoryFlags{}
		for _, c := range l.Categories {
			cats[c.CategoryName] = reportCategoryFlags{Strategic: c.Strategic, RetireCandidate: c.RetireCandidate}
		}
		byPurl[l.Purl] = reportLibrarySummary{IsDirect: l.IsDirect, Categories: cats}
	}
	return byPurl
}

// retireCandidates returns the sorted "purl | category" pairs flagged as
// retire candidates in report.
func retireCandidates(report map[string]reportLibrarySummary) []string {
	out := []string{}
	for purl, lib := range report {
		for name, flags := range lib.Categories {
			if flags.RetireCandidate {
				out = append(out, purl+" | "+name)
			}
		}
	}
	sort.Strings(out)
	return out
}

// pairs builds a sorted "purl | category" list for comparison with
// retireCandidates.
func pairs(items ...[2]string) []string {
	out := []string{}
	for _, it := range items {
		out = append(out, it[0]+" | "+it[1])
	}
	sort.Strings(out)
	return out
}

func assertRetireCandidates(t *testing.T, project string, report map[string]reportLibrarySummary, want []string) {
	t.Helper()
	if got := retireCandidates(report); !reflect.DeepEqual(got, want) {
		t.Errorf("%s: expected retire candidates %v, got %v", project, want, got)
	}
}

// multiProjectReportFixture is the state built by
// setUpMultiProjectReportFixture.
type multiProjectReportFixture struct {
	alphaID, betaID string
	libs            map[string]Library // keyed by purl
	// oneMark has a single strategic mark (lib-a); globalMark is reviewed
	// only through lib-e, which is not in Alpha; unreviewed has no mark.
	oneMark, globalMark, unreviewed Category
}

// setUpMultiProjectReportFixture uploads the Alpha and Beta SBOMs (in the
// given order), then attaches categories and strategic marks:
//
//   - oneMark:    lib-a, lib-b, lib-c, lib-d; lib-a marked strategic
//   - globalMark: lib-c, lib-e; lib-e marked strategic (lib-e is Beta-only)
//   - unreviewed: lib-b, lib-d; no marks
func setUpMultiProjectReportFixture(t *testing.T, store *Store, alphaFirst bool) multiProjectReportFixture {
	t.Helper()
	ctx := context.Background()

	t.Setenv("SESSION_SECRET", "test-secret")
	cookie := authtest.SessionCookie(t, "architect@example.com", auth.RoleArchitect)
	queue := &recordingEnqueuer{}

	var f multiProjectReportFixture
	uploadAlpha := func() {
		f.alphaID = doProjectUpload(t, store, queue, cookie, "Report Alpha", "alpha.json", []byte(reportAlphaSBOM)).ProjectID
	}
	uploadBeta := func() {
		f.betaID = doProjectUpload(t, store, queue, cookie, "Report Beta", "beta.json", []byte(reportBetaSBOM)).ProjectID
	}
	if alphaFirst {
		uploadAlpha()
		uploadBeta()
	} else {
		uploadBeta()
		uploadAlpha()
	}

	// Shared purls land under whichever upload came first, so resolve by
	// purl rather than by batch.
	libs, err := store.LibrariesByPurls(ctx, []string{reportPurlA, reportPurlB, reportPurlC, reportPurlD, reportPurlE})
	if err != nil {
		t.Fatalf("LibrariesByPurls failed: %v", err)
	}
	f.libs = map[string]Library{}
	for _, l := range libs {
		f.libs[l.Purl] = l
	}
	if len(f.libs) != 5 {
		t.Fatalf("expected 5 libraries across both projects, got %+v", libs)
	}

	categories, err := store.ListTaxonomy(ctx)
	if err != nil || len(categories) < 3 {
		t.Fatalf("ListTaxonomy failed: err=%v categories=%+v", err, categories)
	}
	f.oneMark, f.globalMark, f.unreviewed = categories[0], categories[1], categories[2]

	attach := func(purl string, cat Category) {
		t.Helper()
		if err := store.AttachCategory(ctx, f.libs[purl].ID, cat.ID, "manual"); err != nil {
			t.Fatalf("AttachCategory(%s, %s) failed: %v", purl, cat.Name, err)
		}
	}
	mark := func(purl string, cat Category) {
		t.Helper()
		if err := store.MarkStrategic(ctx, f.libs[purl].ID, cat.ID, "strategic pick", "architect@example.com"); err != nil {
			t.Fatalf("MarkStrategic(%s, %s) failed: %v", purl, cat.Name, err)
		}
	}

	for _, purl := range []string{reportPurlA, reportPurlB, reportPurlC, reportPurlD} {
		attach(purl, f.oneMark)
	}
	mark(reportPurlA, f.oneMark)

	attach(reportPurlC, f.globalMark)
	attach(reportPurlE, f.globalMark)
	mark(reportPurlE, f.globalMark)

	attach(reportPurlB, f.unreviewed)
	attach(reportPurlD, f.unreviewed)

	return f
}

// TestProjectReport_MultiProjectRetireCandidates covers test-plan risk #4:
// with several direct libraries in one category, the report flags exactly
// the non-strategic direct ones as retire candidates, in every project and
// regardless of the order the projects were uploaded in.
func TestProjectReport_MultiProjectRetireCandidates(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	type reports struct {
		alpha, beta map[string]reportLibrarySummary
	}
	final := map[string]reports{}

	for _, order := range []struct {
		name       string
		alphaFirst bool
	}{
		{"alpha_then_beta", true},
		{"beta_then_alpha", false},
	} {
		t.Run(order.name, func(t *testing.T) {
			resetCatalogTables(t, pool)
			f := setUpMultiProjectReportFixture(t, store, order.alphaFirst)
			one, global, unreviewed := f.oneMark.Name, f.globalMark.Name, f.unreviewed.Name

			alpha := getProjectReport(t, store, f.alphaID)
			beta := getProjectReport(t, store, f.betaID)

			t.Run("direct_status_per_project", func(t *testing.T) {
				for _, want := range []struct {
					project string
					report  map[string]reportLibrarySummary
					purl    string
					direct  bool
				}{
					{"alpha", alpha, reportPurlA, true},
					{"alpha", alpha, reportPurlB, true},
					{"alpha", alpha, reportPurlC, true},
					{"alpha", alpha, reportPurlD, false},
					{"beta", beta, reportPurlA, false},
					{"beta", beta, reportPurlB, true},
					{"beta", beta, reportPurlD, true},
					{"beta", beta, reportPurlE, true},
				} {
					lib, ok := want.report[want.purl]
					if !ok {
						t.Errorf("%s: expected %s in report", want.project, want.purl)
						continue
					}
					if lib.IsDirect != want.direct {
						t.Errorf("%s: expected %s is_direct=%v, got %v", want.project, want.purl, want.direct, lib.IsDirect)
					}
				}
				if len(alpha) != 4 || len(beta) != 4 {
					t.Errorf("expected 4 libraries per project, got alpha=%d beta=%d", len(alpha), len(beta))
				}
			})

			// Exact candidate set per project, covering all three categories
			// at once: oneMark flags the unmarked direct libraries only,
			// globalMark flags lib-c in Alpha although its only mark is on a
			// library outside Alpha, and unreviewed flags nothing.
			t.Run("candidate_set_per_project", func(t *testing.T) {
				assertRetireCandidates(t, "alpha", alpha, pairs(
					[2]string{reportPurlB, one},
					[2]string{reportPurlC, one},
					[2]string{reportPurlC, global},
				))
				assertRetireCandidates(t, "beta", beta, pairs(
					[2]string{reportPurlB, one},
					[2]string{reportPurlD, one},
				))
			})

			t.Run("strategic_library_transitive_in_other_project", func(t *testing.T) {
				want := reportCategoryFlags{Strategic: true, RetireCandidate: false}
				if got := alpha[reportPurlA].Categories[one]; got != want {
					t.Errorf("alpha: expected direct lib-a in %q to be %+v, got %+v", one, want, got)
				}
				if got := beta[reportPurlA].Categories[one]; got != want {
					t.Errorf("beta: expected transitive lib-a in %q to be %+v, got %+v", one, want, got)
				}
			})

			t.Run("category_reviewed_globally_across_projects", func(t *testing.T) {
				if _, ok := alpha[reportPurlE]; ok {
					t.Fatalf("fixture error: lib-e must not be in alpha")
				}
				want := reportCategoryFlags{Strategic: false, RetireCandidate: true}
				if got := alpha[reportPurlC].Categories[global]; got != want {
					t.Errorf("alpha: expected lib-c in %q to be %+v, got %+v", global, want, got)
				}
				want = reportCategoryFlags{Strategic: true, RetireCandidate: false}
				if got := beta[reportPurlE].Categories[global]; got != want {
					t.Errorf("beta: expected lib-e in %q to be %+v, got %+v", global, want, got)
				}
			})

			t.Run("category_without_marks_has_no_candidates", func(t *testing.T) {
				for project, report := range map[string]map[string]reportLibrarySummary{"alpha": alpha, "beta": beta} {
					for _, purl := range []string{reportPurlB, reportPurlD} {
						got, ok := report[purl].Categories[unreviewed]
						if !ok {
							t.Errorf("%s: expected %s attached to %q", project, purl, unreviewed)
							continue
						}
						if got.RetireCandidate || got.Strategic {
							t.Errorf("%s: expected %s in %q to be neither strategic nor a candidate, got %+v", project, purl, unreviewed, got)
						}
					}
				}
			})

			final[order.name] = reports{alpha: alpha, beta: beta}

			t.Run("unmarking_last_mark_clears_candidates", func(t *testing.T) {
				if err := store.UnmarkStrategic(ctx, f.libs[reportPurlA].ID, f.oneMark.ID, "no longer strategic", "architect@example.com"); err != nil {
					t.Fatalf("UnmarkStrategic failed: %v", err)
				}

				alphaAfter := getProjectReport(t, store, f.alphaID)
				betaAfter := getProjectReport(t, store, f.betaID)

				// Only the globalMark candidate is left; oneMark is no longer
				// reviewed, so none of its libraries are flagged.
				assertRetireCandidates(t, "alpha", alphaAfter, pairs([2]string{reportPurlC, global}))
				assertRetireCandidates(t, "beta", betaAfter, pairs())
				if got := alphaAfter[reportPurlA].Categories[one]; got.Strategic {
					t.Errorf("alpha: expected lib-a in %q to be non-strategic after unmark, got %+v", one, got)
				}
			})
		})
	}

	t.Run("upload_order_does_not_change_reports", func(t *testing.T) {
		ab, ok1 := final["alpha_then_beta"]
		ba, ok2 := final["beta_then_alpha"]
		if !ok1 || !ok2 {
			t.Skipf("needs reports from both upload orders, got %d; run the whole test", len(final))
		}
		if !reflect.DeepEqual(ab.alpha, ba.alpha) {
			t.Errorf("alpha report differs by upload order:\nA->B: %+v\nB->A: %+v", ab.alpha, ba.alpha)
		}
		if !reflect.DeepEqual(ab.beta, ba.beta) {
			t.Errorf("beta report differs by upload order:\nA->B: %+v\nB->A: %+v", ab.beta, ba.beta)
		}
	})
}

// TestProjectReport_ReviewedCategoryIgnoresEcosystem_Characterization pins
// today's behavior, not a requirement: the strategic cap is counted per
// (category, ecosystem), but a category counts as "reviewed" for the report
// when it has an active mark in any ecosystem. So an npm mark makes a Maven
// library in the same category a retire candidate. Whether "reviewed" should
// be per ecosystem is open (K1 / FR-007, see
// context/changes/refactor-opportunities/research.md); deciding it means
// changing this test on purpose.
func TestProjectReport_ReviewedCategoryIgnoresEcosystem_Characterization(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	t.Setenv("SESSION_SECRET", "test-secret")
	cookie := authtest.SessionCookie(t, "architect@example.com", auth.RoleArchitect)
	resp := doProjectUpload(t, store, &recordingEnqueuer{}, cookie, "Ecosystem Project", "sbom.json", []byte(projectFixtureSBOMSingleDirect))

	categories, err := store.ListTaxonomy(ctx)
	if err != nil || len(categories) == 0 {
		t.Fatalf("ListTaxonomy failed: err=%v categories=%+v", err, categories)
	}
	category := categories[0]

	// The npm library bypasses ParsePurl (Maven-only) and goes straight in
	// through CreateLibraries; it belongs to no project.
	npmLib := seedOneLibrary(t, store, "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeee1",
		NewLibrary{Purl: "pkg:npm/left-pad@1.3.0", Name: "left-pad", Ecosystem: "npm"})
	if err := store.AttachCategory(ctx, npmLib.ID, category.ID, "manual"); err != nil {
		t.Fatalf("AttachCategory(npm) failed: %v", err)
	}
	if err := store.MarkStrategic(ctx, npmLib.ID, category.ID, "npm pick", "architect@example.com"); err != nil {
		t.Fatalf("MarkStrategic(npm) failed: %v", err)
	}

	guava, err := store.LibrariesByPurls(ctx, []string{"pkg:maven/com.google.guava/guava@32.0.0"})
	if err != nil || len(guava) != 1 {
		t.Fatalf("LibrariesByPurls failed: err=%v libs=%+v", err, guava)
	}
	if err := store.AttachCategory(ctx, guava[0].ID, category.ID, "manual"); err != nil {
		t.Fatalf("AttachCategory(guava) failed: %v", err)
	}

	report := getProjectReport(t, store, resp.ProjectID)
	lib, ok := report["pkg:maven/com.google.guava/guava@32.0.0"]
	if !ok || !lib.IsDirect {
		t.Fatalf("expected direct guava in report, got %+v", report)
	}
	want := reportCategoryFlags{Strategic: false, RetireCandidate: true}
	if got := lib.Categories[category.Name]; got != want {
		t.Fatalf("expected Maven guava in %q to be %+v (reviewed by an npm mark), got %+v", category.Name, want, got)
	}
}
