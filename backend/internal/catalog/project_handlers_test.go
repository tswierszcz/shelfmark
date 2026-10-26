package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tswierszcz/shelfmark/backend/internal/auth"
)

// projectFixtureSBOM is a project SBOM with a resolvable dependency graph:
// two direct libraries (jackson-databind, guava) and one transitive library
// (jackson-core), mirroring project_sbom_test.go's
// fixtureProjectSBOMFullGraph.
const projectFixtureSBOM = `{
  "bomFormat": "CycloneDX",
  "specVersion": "1.5",
  "version": 1,
  "metadata": {
    "component": {
      "type": "application",
      "bom-ref": "root-app",
      "name": "my-app",
      "version": "1.0.0"
    }
  },
  "components": [
    {
      "type": "library",
      "bom-ref": "direct-1",
      "name": "jackson-databind",
      "purl": "pkg:maven/com.fasterxml.jackson.core/jackson-databind@2.15.0"
    },
    {
      "type": "library",
      "bom-ref": "direct-2",
      "name": "guava",
      "purl": "pkg:maven/com.google.guava/guava@32.0.0"
    },
    {
      "type": "library",
      "bom-ref": "transitive-1",
      "name": "jackson-core",
      "purl": "pkg:maven/com.fasterxml.jackson.core/jackson-core@2.15.0"
    }
  ],
  "dependencies": [
    {
      "ref": "root-app",
      "dependsOn": ["direct-1", "direct-2"]
    },
    {
      "ref": "direct-1",
      "dependsOn": ["transitive-1"]
    },
    {
      "ref": "direct-2",
      "dependsOn": []
    },
    {
      "ref": "transitive-1",
      "dependsOn": []
    }
  ]
}`

// projectFixtureSBOMSingleDirect is a smaller project SBOM: a single direct
// library, no dependency graph (so it falls back to all-direct). Used by
// tests that need a re-upload with a shrunk library set.
const projectFixtureSBOMSingleDirect = `{
  "bomFormat": "CycloneDX",
  "specVersion": "1.5",
  "version": 1,
  "components": [
    {
      "type": "library",
      "name": "guava",
      "purl": "pkg:maven/com.google.guava/guava@32.0.0"
    }
  ]
}`

func newProjectUploadRequest(t *testing.T, projectName, filename string, content []byte) *http.Request {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	if projectName != "" {
		if err := writer.WriteField("project_name", projectName); err != nil {
			t.Fatalf("failed to write project_name field: %v", err)
		}
	}

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("failed to write file content: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/projects/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

// doProjectUpload issues an authenticated project-upload request and
// decodes its response, failing the test on a non-200 status.
func doProjectUpload(t *testing.T, store *Store, queue Enqueuer, cookie *http.Cookie, projectName, filename string, content []byte) projectUploadResponse {
	t.Helper()

	handler := auth.RequireRole(auth.RoleArchitect)(ProjectUploadHandler(store, queue))

	req := newProjectUploadRequest(t, projectName, filename, content)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp projectUploadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return resp
}

func TestProjectUploadHandler_NewProjectIngestsAndRecordsMembership(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	queue := &recordingEnqueuer{}

	t.Setenv("SESSION_SECRET", "test-secret")
	cookie := sessionCookie(t, "architect@example.com", auth.RoleArchitect)

	resp := doProjectUpload(t, store, queue, cookie, "My Project", "sbom.json", []byte(projectFixtureSBOM))

	if resp.ProjectID == "" {
		t.Error("expected a non-empty project_id")
	}
	if resp.ProjectName != "My Project" {
		t.Errorf("expected project_name %q, got %q", "My Project", resp.ProjectName)
	}
	if resp.BatchID == "" {
		t.Error("expected a non-empty batch_id")
	}
	if resp.Created != 3 {
		t.Errorf("expected 3 created libraries, got %d", resp.Created)
	}
	if len(resp.Skipped) != 0 {
		t.Errorf("expected no skipped rows, got %+v", resp.Skipped)
	}

	// All three libraries are brand new ("pending"), so all three should
	// have been enqueued.
	if len(queue.enqueued) != 3 {
		t.Fatalf("expected 3 libraries enqueued, got %d", len(queue.enqueued))
	}

	ctx := context.Background()
	report, err := store.ProjectReport(ctx, resp.ProjectID)
	if err != nil {
		t.Fatalf("ProjectReport failed: %v", err)
	}
	// No categories attached yet (categorization hasn't run), so the report
	// has no rows even though the project has library membership recorded.
	if len(report) != 0 {
		t.Fatalf("expected no report rows before categorization, got %+v", report)
	}

	projects, err := store.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}
	found := false
	for _, p := range projects {
		if p.ID == resp.ProjectID {
			found = true
			if p.CreatedBy != "architect@example.com" {
				t.Errorf("expected created_by %q, got %q", "architect@example.com", p.CreatedBy)
			}
		}
	}
	if !found {
		t.Fatalf("expected project %s in ListProjects, got %+v", resp.ProjectID, projects)
	}
}

func TestProjectUploadHandler_OversizedFileReturns400(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	queue := &recordingEnqueuer{}

	t.Setenv("SESSION_SECRET", "test-secret")
	cookie := sessionCookie(t, "architect@example.com", auth.RoleArchitect)

	handler := auth.RequireRole(auth.RoleArchitect)(ProjectUploadHandler(store, queue))

	oversized := bytes.Repeat([]byte("a"), maxUploadBytes+1)
	req := newProjectUploadRequest(t, "Oversized Project", "sbom.json", oversized)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	if len(queue.enqueued) != 0 {
		t.Fatalf("expected no libraries enqueued, got %+v", queue.enqueued)
	}

	ctx := context.Background()
	libs, err := store.ListLibraries(ctx, LibraryFilter{})
	if err != nil {
		t.Fatalf("ListLibraries failed: %v", err)
	}
	if len(libs) != 0 {
		t.Fatalf("expected no libraries persisted, got %+v", libs)
	}

	projects, err := store.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}
	if len(projects) != 0 {
		t.Fatalf("expected no projects persisted, got %+v", projects)
	}
}

func TestProjectUploadHandler_MissingProjectNameReturns400(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	queue := &recordingEnqueuer{}

	t.Setenv("SESSION_SECRET", "test-secret")
	cookie := sessionCookie(t, "architect@example.com", auth.RoleArchitect)

	handler := auth.RequireRole(auth.RoleArchitect)(ProjectUploadHandler(store, queue))

	req := newProjectUploadRequest(t, "", "sbom.json", []byte(projectFixtureSBOM))
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestProjectUploadHandler_ReuploadReplacesPriorMembership(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	queue := &recordingEnqueuer{}

	t.Setenv("SESSION_SECRET", "test-secret")
	cookie := sessionCookie(t, "architect@example.com", auth.RoleArchitect)

	first := doProjectUpload(t, store, queue, cookie, "Reupload Project", "sbom.json", []byte(projectFixtureSBOM))

	// Attach a category to all libraries so the report has rows to compare
	// membership against, and mark the project's libraries categorized so
	// the second upload doesn't re-enqueue them (which is covered by its
	// own test).
	ctx := context.Background()
	categories, err := store.ListTaxonomy(ctx)
	if err != nil || len(categories) == 0 {
		t.Fatalf("ListTaxonomy failed: err=%v categories=%+v", err, categories)
	}
	libsBefore, err := store.ListLibraries(ctx, LibraryFilter{BatchID: first.BatchID})
	if err != nil {
		t.Fatalf("ListLibraries failed: %v", err)
	}
	for _, lib := range libsBefore {
		if err := store.AttachCategory(ctx, lib.ID, categories[0].ID, "manual"); err != nil {
			t.Fatalf("AttachCategory failed: %v", err)
		}
	}

	reportBefore, err := store.ProjectReport(ctx, first.ProjectID)
	if err != nil {
		t.Fatalf("ProjectReport failed: %v", err)
	}
	if len(reportBefore) != 3 {
		t.Fatalf("expected 3 report rows before re-upload, got %d: %+v", len(reportBefore), reportBefore)
	}

	// Re-upload under the same project name with a smaller SBOM: only
	// guava remains.
	second := doProjectUpload(t, store, queue, cookie, "Reupload Project", "sbom2.json", []byte(projectFixtureSBOMSingleDirect))

	if second.ProjectID != first.ProjectID {
		t.Fatalf("expected re-upload to resolve to the same project, got %q vs %q", second.ProjectID, first.ProjectID)
	}

	// guava already carries categories[0] from the "attach category to all
	// libsBefore" step above (categories live on the library, not the
	// per-project membership), so the report after re-upload already shows
	// exactly that one row — confirming the re-upload left only guava
	// recorded against the project.
	reportAfter, err := store.ProjectReport(ctx, first.ProjectID)
	if err != nil {
		t.Fatalf("ProjectReport failed: %v", err)
	}
	if len(reportAfter) != 1 || reportAfter[0].Purl != "pkg:maven/com.google.guava/guava@32.0.0" {
		t.Fatalf("expected re-upload to leave only guava recorded against the project, got %+v", reportAfter)
	}

	// guava already existed under the first upload's batch (CreateLibraries
	// is a no-op on conflict), so it's resolved by purl here rather than by
	// second.BatchID, which only covers libraries newly inserted by this
	// particular upload.
	libsAfter, err := store.LibrariesByPurls(ctx, []string{"pkg:maven/com.google.guava/guava@32.0.0"})
	if err != nil {
		t.Fatalf("LibrariesByPurls failed: %v", err)
	}
	if len(libsAfter) != 1 {
		t.Fatalf("expected 1 resolved library for guava, got %d", len(libsAfter))
	}
}

func TestProjectUploadHandler_PurlAlreadyInCatalogUnderDifferentBatchIsReused(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	queue := &recordingEnqueuer{}
	ctx := context.Background()

	// Seed guava under an unrelated batch, already categorized, before any
	// project upload sees it.
	existing := seedOneLibrary(t, store, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1",
		NewLibrary{Purl: "pkg:maven/com.google.guava/guava@32.0.0", Name: "guava", Ecosystem: "maven"})
	categories, err := store.ListTaxonomy(ctx)
	if err != nil || len(categories) == 0 {
		t.Fatalf("ListTaxonomy failed: err=%v categories=%+v", err, categories)
	}
	if err := store.AttachCategory(ctx, existing.ID, categories[0].ID, "manual"); err != nil {
		t.Fatalf("AttachCategory failed: %v", err)
	}
	if err := store.UpdateLibraryStatus(ctx, existing.ID, "categorized", nil); err != nil {
		t.Fatalf("UpdateLibraryStatus failed: %v", err)
	}

	t.Setenv("SESSION_SECRET", "test-secret")
	cookie := sessionCookie(t, "architect@example.com", auth.RoleArchitect)

	resp := doProjectUpload(t, store, queue, cookie, "Reuse Project", "sbom.json", []byte(projectFixtureSBOMSingleDirect))

	// The purl already existed (CreateLibraries is a no-op on conflict), so
	// the resolved library set is still 1, but it must resolve to the
	// pre-existing row (already-categorized), not a new "pending" one, and
	// must not be re-enqueued.
	if resp.Created != 1 {
		t.Fatalf("expected 1 resolved library, got %d", resp.Created)
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("expected the already-categorized library not to be re-enqueued, got %+v", queue.enqueued)
	}

	report, err := store.ProjectReport(ctx, resp.ProjectID)
	if err != nil {
		t.Fatalf("ProjectReport failed: %v", err)
	}
	if len(report) != 1 || report[0].LibraryID != existing.ID {
		t.Fatalf("expected the project to record membership against the pre-existing library %s, got %+v", existing.ID, report)
	}
}

func TestProjectUploadHandler_UnknownPurlTriggersEnqueue(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	queue := &recordingEnqueuer{}

	t.Setenv("SESSION_SECRET", "test-secret")
	cookie := sessionCookie(t, "architect@example.com", auth.RoleArchitect)

	doProjectUpload(t, store, queue, cookie, "Unknown Purl Project", "sbom.json", []byte(projectFixtureSBOMSingleDirect))

	if len(queue.enqueued) != 1 {
		t.Fatalf("expected the brand-new purl to be enqueued, got %d: %+v", len(queue.enqueued), queue.enqueued)
	}
	if queue.enqueued[0].Purl != "pkg:maven/com.google.guava/guava@32.0.0" {
		t.Fatalf("expected guava to be enqueued, got %+v", queue.enqueued[0])
	}
	if queue.enqueued[0].Status != "pending" {
		t.Fatalf("expected enqueued library to be pending, got %q", queue.enqueued[0].Status)
	}
}

// setUpProjectReportFixture uploads a two-direct/one-transitive project SBOM
// and returns the resulting project ID plus the three created libraries
// (direct1, direct2, transitive), for report tests to attach
// categories/strategic marks against.
func setUpProjectReportFixture(t *testing.T, store *Store) (projectID string, direct1, direct2, transitive Library) {
	t.Helper()

	t.Setenv("SESSION_SECRET", "test-secret")
	cookie := sessionCookie(t, "architect@example.com", auth.RoleArchitect)
	queue := &recordingEnqueuer{}

	resp := doProjectUpload(t, store, queue, cookie, "Report Fixture Project", "sbom.json", []byte(projectFixtureSBOM))

	ctx := context.Background()
	libs, err := store.ListLibraries(ctx, LibraryFilter{BatchID: resp.BatchID})
	if err != nil {
		t.Fatalf("ListLibraries failed: %v", err)
	}
	byPurl := map[string]Library{}
	for _, l := range libs {
		byPurl[l.Purl] = l
	}

	return resp.ProjectID,
		byPurl["pkg:maven/com.fasterxml.jackson.core/jackson-databind@2.15.0"],
		byPurl["pkg:maven/com.google.guava/guava@32.0.0"],
		byPurl["pkg:maven/com.fasterxml.jackson.core/jackson-core@2.15.0"]
}

func TestProjectReportHandler_ReturnsReportRowsWithRetireCandidateFlags(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	projectID, direct1, direct2, transitive := setUpProjectReportFixture(t, store)

	categories, err := store.ListTaxonomy(ctx)
	if err != nil || len(categories) < 2 {
		t.Fatalf("ListTaxonomy failed: err=%v categories=%+v", err, categories)
	}
	reviewedCategory := categories[0].ID   // will get a strategic mark
	unreviewedCategory := categories[1].ID // never marked strategic

	// direct1: direct + strategic in reviewedCategory -> not a retire
	// candidate (it's the strategic pick itself).
	if err := store.AttachCategory(ctx, direct1.ID, reviewedCategory, "manual"); err != nil {
		t.Fatalf("AttachCategory(direct1) failed: %v", err)
	}
	if err := store.MarkStrategic(ctx, direct1.ID, reviewedCategory, "core dependency", "architect@example.com"); err != nil {
		t.Fatalf("MarkStrategic(direct1) failed: %v", err)
	}

	// direct2: direct, non-strategic, in the same reviewed category -> a
	// retire candidate (lost out to direct1's strategic pick).
	if err := store.AttachCategory(ctx, direct2.ID, reviewedCategory, "manual"); err != nil {
		t.Fatalf("AttachCategory(direct2) failed: %v", err)
	}

	// transitive: attached to the reviewed category too, but never a
	// retire candidate regardless of strategic status, since it isn't
	// direct.
	if err := store.AttachCategory(ctx, transitive.ID, reviewedCategory, "manual"); err != nil {
		t.Fatalf("AttachCategory(transitive) failed: %v", err)
	}

	// direct2 also attached to a never-reviewed category -> not a retire
	// candidate there (no library has been reviewed for that category).
	if err := store.AttachCategory(ctx, direct2.ID, unreviewedCategory, "manual"); err != nil {
		t.Fatalf("AttachCategory(direct2, unreviewed) failed: %v", err)
	}

	handler := ProjectReportHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/report", nil)
	req.SetPathValue("projectID", projectID)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp []projectReportLibrary
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	byID := map[string]projectReportLibrary{}
	for _, l := range resp {
		byID[l.ID] = l
	}

	// direct1 + strategic -> not flagged.
	d1, ok := byID[direct1.ID]
	if !ok {
		t.Fatalf("expected direct1 in report, got %+v", resp)
	}
	categoryByID := map[string]projectReportCategory{}
	for _, c := range d1.Categories {
		categoryByID[c.CategoryID] = c
	}
	if c, ok := categoryByID[reviewedCategory]; !ok || !c.Strategic || c.RetireCandidate {
		t.Fatalf("expected direct1's reviewed-category entry to be strategic and not a retire candidate, got %+v", categoryByID[reviewedCategory])
	}

	// direct2 + non-strategic in a reviewed category -> flagged.
	d2, ok := byID[direct2.ID]
	if !ok {
		t.Fatalf("expected direct2 in report, got %+v", resp)
	}
	categoryByID = map[string]projectReportCategory{}
	for _, c := range d2.Categories {
		categoryByID[c.CategoryID] = c
	}
	if c, ok := categoryByID[reviewedCategory]; !ok || c.Strategic || !c.RetireCandidate {
		t.Fatalf("expected direct2's reviewed-category entry to be non-strategic and a retire candidate, got %+v", categoryByID[reviewedCategory])
	}
	// direct2 in a never-reviewed category -> not flagged.
	if c, ok := categoryByID[unreviewedCategory]; !ok || c.Strategic || c.RetireCandidate {
		t.Fatalf("expected direct2's unreviewed-category entry to not be a retire candidate, got %+v", categoryByID[unreviewedCategory])
	}

	// transitive, attached to the reviewed category, non-strategic -> never
	// flagged (not direct).
	tr, ok := byID[transitive.ID]
	if !ok {
		t.Fatalf("expected transitive in report, got %+v", resp)
	}
	categoryByID = map[string]projectReportCategory{}
	for _, c := range tr.Categories {
		categoryByID[c.CategoryID] = c
	}
	if c, ok := categoryByID[reviewedCategory]; !ok || c.RetireCandidate {
		t.Fatalf("expected transitive's reviewed-category entry to never be a retire candidate, got %+v", categoryByID[reviewedCategory])
	}
	if tr.IsDirect {
		t.Fatalf("expected transitive library to be recorded as not direct, got IsDirect=true")
	}
}

func TestProjectReportHandler_UnknownProjectReturns404(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)

	handler := ProjectReportHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/api/projects/00000000-0000-0000-0000-000000000000/report", nil)
	req.SetPathValue("projectID", "00000000-0000-0000-0000-000000000000")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListProjectsHandler_ReturnsUploadedProjects(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	queue := &recordingEnqueuer{}

	t.Setenv("SESSION_SECRET", "test-secret")
	cookie := sessionCookie(t, "architect@example.com", auth.RoleArchitect)

	resp := doProjectUpload(t, store, queue, cookie, "List Me Project", "sbom.json", []byte(projectFixtureSBOMSingleDirect))

	handler := ListProjectsHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var projects []Project
	if err := json.Unmarshal(rec.Body.Bytes(), &projects); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	found := false
	for _, p := range projects {
		if p.ID == resp.ProjectID {
			found = true
			if p.Name != "List Me Project" {
				t.Errorf("expected project name %q, got %q", "List Me Project", p.Name)
			}
		}
	}
	if !found {
		t.Fatalf("expected project %s in ListProjects response, got %+v", resp.ProjectID, projects)
	}
}

func TestListProjectsHandler_NoProjectsReturnsEmptyArrayNotNull(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)

	handler := ListProjectsHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// A nil slice would marshal to the literal "null\n" instead, which a
	// frontend .map() over the decoded response would crash on.
	if rec.Body.String() != "[]\n" {
		t.Fatalf("expected body %q, got %q", "[]\n", rec.Body.String())
	}
}

func TestProjectReportHandler_NoLibrariesReturnsEmptyArrayNotNull(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	project, err := store.FindOrCreateProject(ctx, "Empty Project", "test@example.com")
	if err != nil {
		t.Fatalf("FindOrCreateProject failed: %v", err)
	}

	handler := ProjectReportHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/api/projects/"+project.ID+"/report", nil)
	req.SetPathValue("projectID", project.ID)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// A nil slice would marshal to the literal "null\n" instead, which a
	// frontend .map() over the decoded response would crash on.
	if rec.Body.String() != "[]\n" {
		t.Fatalf("expected body %q, got %q", "[]\n", rec.Body.String())
	}
}
