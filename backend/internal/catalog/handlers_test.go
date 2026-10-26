package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tswierszcz/shelfmark/backend/internal/auth"
	"github.com/tswierszcz/shelfmark/backend/internal/obs"
	"github.com/tswierszcz/shelfmark/backend/internal/obs/obstest"
)

const handlerFixtureSBOM = `{
  "bomFormat": "CycloneDX",
  "specVersion": "1.5",
  "version": 1,
  "components": [
    {
      "type": "library",
      "name": "jackson-databind",
      "purl": "pkg:maven/com.fasterxml.jackson.core/jackson-databind@2.15.0"
    },
    {
      "type": "library",
      "name": "guava",
      "purl": "pkg:maven/com.google.guava/guava@32.1.0"
    },
    {
      "type": "library",
      "name": "left-pad",
      "purl": "pkg:npm/left-pad@1.3.0"
    }
  ]
}`

// recordingEnqueuer collects every library handed to Enqueue, for assertions
// in tests.
type recordingEnqueuer struct {
	enqueued []Library
}

func (r *recordingEnqueuer) Enqueue(lib Library) {
	r.enqueued = append(r.enqueued, lib)
}

func newMultipartUploadRequest(t *testing.T, kind, filename string, content []byte) *http.Request {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	if err := writer.WriteField("kind", kind); err != nil {
		t.Fatalf("failed to write kind field: %v", err)
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

	req := httptest.NewRequest(http.MethodPost, "/api/catalog/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestUploadHandler_SBOMCreatesLibrariesAndReturnsSummary(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	queue := &recordingEnqueuer{}

	handler := UploadHandler(store, queue)

	req := newMultipartUploadRequest(t, "sbom", "sbom.json", []byte(handlerFixtureSBOM))
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp uploadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.BatchID == "" {
		t.Error("expected a non-empty batch_id")
	}
	if resp.Created != 2 {
		t.Errorf("expected 2 created libraries, got %d", resp.Created)
	}
	if len(resp.Skipped) != 1 {
		t.Fatalf("expected 1 skipped row, got %d: %+v", len(resp.Skipped), resp.Skipped)
	}

	if len(queue.enqueued) != 2 {
		t.Fatalf("expected 2 libraries enqueued, got %d", len(queue.enqueued))
	}

	libs, err := store.ListLibraries(context.Background(), LibraryFilter{BatchID: resp.BatchID})
	if err != nil {
		t.Fatalf("ListLibraries failed: %v", err)
	}
	if len(libs) != 2 {
		t.Fatalf("expected 2 libraries persisted under the batch, got %d", len(libs))
	}
}

func TestUploadHandler_MissingKindReturns400(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	queue := &recordingEnqueuer{}

	handler := UploadHandler(store, queue)

	req := newMultipartUploadRequest(t, "", "sbom.json", []byte(handlerFixtureSBOM))
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUploadHandler_MalformedSBOMReturns400(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	queue := &recordingEnqueuer{}

	handler := UploadHandler(store, queue)

	req := newMultipartUploadRequest(t, "sbom", "sbom.json", []byte("{not valid json"))
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUploadHandler_OversizedFileReturns400(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	queue := &recordingEnqueuer{}

	handler := UploadHandler(store, queue)

	oversized := bytes.Repeat([]byte("a"), maxUploadBytes+1)
	req := newMultipartUploadRequest(t, "sbom", "sbom.json", oversized)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	if len(queue.enqueued) != 0 {
		t.Fatalf("expected no libraries enqueued, got %+v", queue.enqueued)
	}

	libs, err := store.ListLibraries(context.Background(), LibraryFilter{})
	if err != nil {
		t.Fatalf("ListLibraries failed: %v", err)
	}
	if len(libs) != 0 {
		t.Fatalf("expected no libraries persisted, got %+v", libs)
	}
}

func TestBatchProgressHandler_ReturnsEmptyMapForUnknownBatch(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)

	handler := BatchProgressHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/catalog/batches/00000000-0000-0000-0000-000000000000", nil)
	req.SetPathValue("batchID", "00000000-0000-0000-0000-000000000000")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var progress BatchProgress
	if err := json.Unmarshal(rec.Body.Bytes(), &progress); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(progress) != 0 {
		t.Errorf("expected empty progress map, got %+v", progress)
	}
}

func TestBatchProgressHandler_ReturnsCountsForKnownBatch(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	queue := &recordingEnqueuer{}

	uploadHandler := UploadHandler(store, queue)
	req := newMultipartUploadRequest(t, "sbom", "sbom.json", []byte(handlerFixtureSBOM))
	rec := httptest.NewRecorder()
	uploadHandler(rec, req)

	var uploadResp uploadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &uploadResp); err != nil {
		t.Fatalf("failed to decode upload response: %v", err)
	}

	progressHandler := BatchProgressHandler(store)
	progressReq := httptest.NewRequest(http.MethodGet, "/api/catalog/batches/"+uploadResp.BatchID, nil)
	progressReq.SetPathValue("batchID", uploadResp.BatchID)
	progressRec := httptest.NewRecorder()

	progressHandler(progressRec, progressReq)

	if progressRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", progressRec.Code, progressRec.Body.String())
	}

	var progress BatchProgress
	if err := json.Unmarshal(progressRec.Body.Bytes(), &progress); err != nil {
		t.Fatalf("failed to decode progress response: %v", err)
	}
	if progress["pending"] != 2 {
		t.Errorf("expected 2 pending libraries, got %+v", progress)
	}
}

func TestListTaxonomyHandler_ReturnsSeededCategories(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)

	handler := ListTaxonomyHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/catalog/categories", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var categories []Category
	if err := json.Unmarshal(rec.Body.Bytes(), &categories); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(categories) == 0 {
		t.Fatal("expected at least one seeded category")
	}
	for _, c := range categories {
		if c.ID == "" || c.Name == "" {
			t.Fatalf("expected non-empty id/name, got %+v", c)
		}
	}
}

func TestListTaxonomyHandler_NoCategoriesReturnsEmptyArrayNotNull(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)

	// The categories table is otherwise always migration-seeded, so force
	// the zero-row condition directly; this test's own throwaway container
	// isolates the DELETE from every other test.
	if _, err := pool.Exec(context.Background(), `DELETE FROM categories`); err != nil {
		t.Fatalf("failed to clear categories: %v", err)
	}

	handler := ListTaxonomyHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/api/catalog/categories", nil)
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

func TestListLibrariesHandler_FiltersAndIncludesCategories(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	queue := &recordingEnqueuer{}

	uploadHandler := UploadHandler(store, queue)
	req := newMultipartUploadRequest(t, "sbom", "sbom.json", []byte(handlerFixtureSBOM))
	rec := httptest.NewRecorder()
	uploadHandler(rec, req)

	var uploadResp uploadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &uploadResp); err != nil {
		t.Fatalf("failed to decode upload response: %v", err)
	}

	ctx := context.Background()
	libs, err := store.ListLibraries(ctx, LibraryFilter{BatchID: uploadResp.BatchID})
	if err != nil {
		t.Fatalf("ListLibraries failed: %v", err)
	}
	if len(libs) != 2 {
		t.Fatalf("expected 2 libraries, got %d", len(libs))
	}

	categories, err := store.ListTaxonomy(ctx)
	if err != nil {
		t.Fatalf("ListTaxonomy failed: %v", err)
	}
	if err := store.AttachCategory(ctx, libs[0].ID, categories[0].ID, "manual"); err != nil {
		t.Fatalf("AttachCategory failed: %v", err)
	}

	listHandler := ListLibrariesHandler(store)

	// Unfiltered: both libraries present, only the attached one carries a
	// category name.
	listReq := httptest.NewRequest(http.MethodGet, "/api/catalog/libraries", nil)
	listRec := httptest.NewRecorder()
	listHandler(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}

	var resp []struct {
		Library
		Categories []string `json:"categories"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	byID := map[string][]string{}
	for _, r := range resp {
		byID[r.ID] = r.Categories
	}
	if len(byID[libs[0].ID]) != 1 || byID[libs[0].ID][0] != categories[0].Name {
		t.Fatalf("expected library %s to carry category %q, got %+v", libs[0].ID, categories[0].Name, byID[libs[0].ID])
	}
	if len(byID[libs[1].ID]) != 0 {
		t.Fatalf("expected library %s to carry no categories, got %+v", libs[1].ID, byID[libs[1].ID])
	}

	// Filtered by ecosystem=maven should include only the maven library.
	filterReq := httptest.NewRequest(http.MethodGet, "/api/catalog/libraries?ecosystem=maven", nil)
	filterRec := httptest.NewRecorder()
	listHandler(filterRec, filterReq)

	var filtered []struct {
		Library
		Categories []string `json:"categories"`
	}
	if err := json.Unmarshal(filterRec.Body.Bytes(), &filtered); err != nil {
		t.Fatalf("failed to decode filtered response: %v", err)
	}
	for _, l := range filtered {
		if l.Ecosystem != "maven" {
			t.Fatalf("expected only maven libraries, got %+v", l)
		}
	}
}

func TestAttachCategoryHandler_AttachesAndRejectsMalformedBody(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	queue := &recordingEnqueuer{}

	uploadHandler := UploadHandler(store, queue)
	req := newMultipartUploadRequest(t, "sbom", "sbom.json", []byte(handlerFixtureSBOM))
	rec := httptest.NewRecorder()
	uploadHandler(rec, req)

	var uploadResp uploadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &uploadResp); err != nil {
		t.Fatalf("failed to decode upload response: %v", err)
	}

	ctx := context.Background()
	libs, err := store.ListLibraries(ctx, LibraryFilter{BatchID: uploadResp.BatchID})
	if err != nil || len(libs) == 0 {
		t.Fatalf("expected at least one library, err=%v libs=%+v", err, libs)
	}

	categories, err := store.ListTaxonomy(ctx)
	if err != nil || len(categories) == 0 {
		t.Fatalf("expected at least one category, err=%v categories=%+v", err, categories)
	}

	attachHandler := AttachCategoryHandler(store)

	// Malformed body (missing category_id) returns 400.
	badReq := httptest.NewRequest(http.MethodPost, "/api/catalog/libraries/"+libs[0].ID+"/categories", bytes.NewReader([]byte(`{}`)))
	badReq.SetPathValue("libraryID", libs[0].ID)
	badRec := httptest.NewRecorder()
	attachHandler(badRec, badReq)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing category_id, got %d: %s", badRec.Code, badRec.Body.String())
	}

	// Valid body returns 204 and persists the attachment.
	body, err := json.Marshal(attachCategoryRequest{CategoryID: categories[0].ID})
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}
	okReq := httptest.NewRequest(http.MethodPost, "/api/catalog/libraries/"+libs[0].ID+"/categories", bytes.NewReader(body))
	okReq.SetPathValue("libraryID", libs[0].ID)
	okRec := httptest.NewRecorder()
	attachHandler(okRec, okReq)
	if okRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", okRec.Code, okRec.Body.String())
	}

	categoryNames, err := store.CategoryNamesByLibrary(ctx, []string{libs[0].ID})
	if err != nil {
		t.Fatalf("CategoryNamesByLibrary failed: %v", err)
	}
	if len(categoryNames[libs[0].ID]) != 1 || categoryNames[libs[0].ID][0] != categories[0].Name {
		t.Fatalf("expected attached category %q, got %+v", categories[0].Name, categoryNames[libs[0].ID])
	}
}

// sessionCookie issues a signed session cookie for role via auth.IssueSession,
// following auth/middleware_test.go's issueTestSession pattern, so handler
// tests can exercise auth.RequireRole-wrapped routes without a real login.
func sessionCookie(t *testing.T, email string, role auth.Role) *http.Cookie {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := auth.IssueSession(rec, req, email, role); err != nil {
		t.Fatalf("IssueSession: %v", err)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("IssueSession did not set a cookie")
	}
	return cookies[0]
}

// setUpLibraryWithCategory uploads the fixture SBOM and returns one created
// library plus one seeded category, for tests that need a (library,
// category) pair to mark strategic.
func setUpLibraryWithCategory(t *testing.T, store *Store) (Library, Category) {
	t.Helper()

	queue := &recordingEnqueuer{}
	uploadHandler := UploadHandler(store, queue)
	req := newMultipartUploadRequest(t, "sbom", "sbom.json", []byte(handlerFixtureSBOM))
	rec := httptest.NewRecorder()
	uploadHandler(rec, req)

	var uploadResp uploadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &uploadResp); err != nil {
		t.Fatalf("failed to decode upload response: %v", err)
	}

	ctx := context.Background()
	libs, err := store.ListLibraries(ctx, LibraryFilter{BatchID: uploadResp.BatchID})
	if err != nil || len(libs) == 0 {
		t.Fatalf("expected at least one library, err=%v libs=%+v", err, libs)
	}

	categories, err := store.ListTaxonomy(ctx)
	if err != nil || len(categories) == 0 {
		t.Fatalf("expected at least one category, err=%v categories=%+v", err, categories)
	}

	return libs[0], categories[0]
}

func TestListOrganizationLibrariesHandler_ReflectsStrategicMarks(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)

	lib, category := setUpLibraryWithCategory(t, store)

	ctx := context.Background()
	if err := store.MarkStrategic(ctx, lib.ID, category.ID, "core dependency", "architect@example.com"); err != nil {
		t.Fatalf("MarkStrategic failed: %v", err)
	}

	handler := ListOrganizationLibrariesHandler(store)

	// Unfiltered: the marked library carries its strategic mark.
	req := httptest.NewRequest(http.MethodGet, "/api/organization/libraries", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp []libraryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	byID := map[string]libraryResponse{}
	for _, r := range resp {
		byID[r.ID] = r
	}
	marked, ok := byID[lib.ID]
	if !ok {
		t.Fatalf("expected library %s in response, got %+v", lib.ID, resp)
	}
	if len(marked.Strategic) != 1 || marked.Strategic[0].CategoryID != category.ID || marked.Strategic[0].Justification != "core dependency" {
		t.Fatalf("expected one strategic mark for library %s, got %+v", lib.ID, marked.Strategic)
	}

	// strategic=true filters down to only the marked library.
	filterReq := httptest.NewRequest(http.MethodGet, "/api/organization/libraries?strategic=true", nil)
	filterRec := httptest.NewRecorder()
	handler(filterRec, filterReq)

	var filtered []libraryResponse
	if err := json.Unmarshal(filterRec.Body.Bytes(), &filtered); err != nil {
		t.Fatalf("failed to decode filtered response: %v", err)
	}
	if len(filtered) != 1 || filtered[0].ID != lib.ID {
		t.Fatalf("expected only library %s with strategic=true, got %+v", lib.ID, filtered)
	}

	// strategic=false excludes the marked library.
	excludeReq := httptest.NewRequest(http.MethodGet, "/api/organization/libraries?strategic=false", nil)
	excludeRec := httptest.NewRecorder()
	handler(excludeRec, excludeReq)

	var excluded []libraryResponse
	if err := json.Unmarshal(excludeRec.Body.Bytes(), &excluded); err != nil {
		t.Fatalf("failed to decode excluded response: %v", err)
	}
	for _, l := range excluded {
		if l.ID == lib.ID {
			t.Fatalf("expected library %s to be excluded with strategic=false, got %+v", lib.ID, excluded)
		}
	}
}

func TestMarkAndUnmarkStrategicHandler_RoundTrip(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)

	lib, category := setUpLibraryWithCategory(t, store)

	t.Setenv("SESSION_SECRET", "test-secret")
	cookie := sessionCookie(t, "architect@example.com", auth.RoleArchitect)

	markHandler := auth.RequireRole(auth.RoleArchitect)(MarkStrategicHandler(store))
	unmarkHandler := auth.RequireRole(auth.RoleArchitect)(UnmarkStrategicHandler(store))

	path := "/api/organization/libraries/" + lib.ID + "/categories/" + category.ID + "/strategic"

	// Blank justification is rejected.
	badReq := httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte(`{"justification":"  "}`)))
	badReq.SetPathValue("libraryID", lib.ID)
	badReq.SetPathValue("categoryID", category.ID)
	badReq.AddCookie(cookie)
	badRec := httptest.NewRecorder()
	markHandler(badRec, badReq)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for blank justification, got %d: %s", badRec.Code, badRec.Body.String())
	}

	// Valid mark returns 204 and is reflected in the store.
	markReq := httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte(`{"justification":"core dependency"}`)))
	markReq.SetPathValue("libraryID", lib.ID)
	markReq.SetPathValue("categoryID", category.ID)
	markReq.AddCookie(cookie)
	markRec := httptest.NewRecorder()
	markHandler(markRec, markReq)
	if markRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", markRec.Code, markRec.Body.String())
	}

	ctx := context.Background()
	marks, err := store.StrategicMarksByLibrary(ctx, []string{lib.ID})
	if err != nil {
		t.Fatalf("StrategicMarksByLibrary failed: %v", err)
	}
	if len(marks[lib.ID]) != 1 || marks[lib.ID][0].MarkedBy != "architect@example.com" {
		t.Fatalf("expected one strategic mark attributed to architect@example.com, got %+v", marks[lib.ID])
	}

	// Unmark returns 204 and deactivates the mark.
	unmarkReq := httptest.NewRequest(http.MethodDelete, path, bytes.NewReader([]byte(`{"justification":"no longer strategic"}`)))
	unmarkReq.SetPathValue("libraryID", lib.ID)
	unmarkReq.SetPathValue("categoryID", category.ID)
	unmarkReq.AddCookie(cookie)
	unmarkRec := httptest.NewRecorder()
	unmarkHandler(unmarkRec, unmarkReq)
	if unmarkRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", unmarkRec.Code, unmarkRec.Body.String())
	}

	marksAfter, err := store.StrategicMarksByLibrary(ctx, []string{lib.ID})
	if err != nil {
		t.Fatalf("StrategicMarksByLibrary failed: %v", err)
	}
	if len(marksAfter[lib.ID]) != 0 {
		t.Fatalf("expected no active strategic marks after unmark, got %+v", marksAfter[lib.ID])
	}

	// Unmarking an already-inactive pair returns 404.
	repeatReq := httptest.NewRequest(http.MethodDelete, path, bytes.NewReader([]byte(`{"justification":"still not strategic"}`)))
	repeatReq.SetPathValue("libraryID", lib.ID)
	repeatReq.SetPathValue("categoryID", category.ID)
	repeatReq.AddCookie(cookie)
	repeatRec := httptest.NewRecorder()
	unmarkHandler(repeatRec, repeatReq)
	if repeatRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for repeat unmark, got %d: %s", repeatRec.Code, repeatRec.Body.String())
	}

	// Marking a non-existent library/category pair returns 400, not 500.
	badRefPath := "/api/organization/libraries/00000000-0000-0000-0000-000000000000/categories/00000000-0000-0000-0000-000000000000/strategic"
	badRefReq := httptest.NewRequest(http.MethodPost, badRefPath, bytes.NewReader([]byte(`{"justification":"core dependency"}`)))
	badRefReq.SetPathValue("libraryID", "00000000-0000-0000-0000-000000000000")
	badRefReq.SetPathValue("categoryID", "00000000-0000-0000-0000-000000000000")
	badRefReq.AddCookie(cookie)
	badRefRec := httptest.NewRecorder()
	markHandler(badRefRec, badRefReq)
	if badRefRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-existent library/category, got %d: %s", badRefRec.Code, badRefRec.Body.String())
	}
}

func TestMarkStrategicHandler_RoleGating(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)

	lib, category := setUpLibraryWithCategory(t, store)

	t.Setenv("SESSION_SECRET", "test-secret")

	markHandler := auth.RequireRole(auth.RoleArchitect)(MarkStrategicHandler(store))
	path := "/api/organization/libraries/" + lib.ID + "/categories/" + category.ID + "/strategic"
	body := []byte(`{"justification":"core dependency"}`)

	// A Viewer session is rejected with 403.
	viewerCookie := sessionCookie(t, "viewer@example.com", auth.RoleViewer)
	viewerReq := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	viewerReq.SetPathValue("libraryID", lib.ID)
	viewerReq.SetPathValue("categoryID", category.ID)
	viewerReq.AddCookie(viewerCookie)
	viewerRec := httptest.NewRecorder()
	markHandler(viewerRec, viewerReq)
	if viewerRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for viewer, got %d: %s", viewerRec.Code, viewerRec.Body.String())
	}

	// No session at all is rejected with 401.
	noSessionReq := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	noSessionReq.SetPathValue("libraryID", lib.ID)
	noSessionReq.SetPathValue("categoryID", category.ID)
	noSessionRec := httptest.NewRecorder()
	markHandler(noSessionRec, noSessionReq)
	if noSessionRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with no session, got %d: %s", noSessionRec.Code, noSessionRec.Body.String())
	}

	// Confirm the mark was never actually recorded by either rejected call.
	ctx := context.Background()
	marks, err := store.StrategicMarksByLibrary(ctx, []string{lib.ID})
	if err != nil {
		t.Fatalf("StrategicMarksByLibrary failed: %v", err)
	}
	if len(marks[lib.ID]) != 0 {
		t.Fatalf("expected no strategic marks after rejected requests, got %+v", marks[lib.ID])
	}
}

func TestErrorDetail_VisibleToAdminCatalogButNotOrganizationLibraries(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	lib := seedOneLibrary(t, store, "88888888-8888-8888-8888-888888888881",
		NewLibrary{Purl: "pkg:maven/com.example/broken@1.0.0", Name: "broken", Ecosystem: "maven"})
	reason := "catalog: repo finder failed: dial tcp 10.0.0.1:443: connect: connection refused"
	if err := store.UpdateLibraryStatus(ctx, lib.ID, "failed", nil, &reason); err != nil {
		t.Fatalf("UpdateLibraryStatus failed: %v", err)
	}

	adminRec := httptest.NewRecorder()
	ListLibrariesHandler(store)(adminRec, httptest.NewRequest(http.MethodGet, "/api/catalog/libraries", nil))
	if adminRec.Code != http.StatusOK {
		t.Fatalf("admin catalog: expected 200, got %d: %s", adminRec.Code, adminRec.Body.String())
	}
	if !strings.Contains(adminRec.Body.String(), "connection refused") {
		t.Fatalf("admin catalog response should carry error_detail, got %s", adminRec.Body.String())
	}

	orgRec := httptest.NewRecorder()
	ListOrganizationLibrariesHandler(store)(orgRec, httptest.NewRequest(http.MethodGet, "/api/organization/libraries", nil))
	if orgRec.Code != http.StatusOK {
		t.Fatalf("organization libraries: expected 200, got %d: %s", orgRec.Code, orgRec.Body.String())
	}
	if strings.Contains(orgRec.Body.String(), "error_detail") || strings.Contains(orgRec.Body.String(), "connection refused") {
		t.Fatalf("organization libraries response must not expose error_detail to Architect/Viewer, got %s", orgRec.Body.String())
	}
}

// captureLogs installs a capturing JSON logger for the duration of the test.
func captureLogs(t *testing.T) *obstest.Sink { return obstest.CaptureWith(t, obs.NewLogger) }

func TestCatalogHandler_5xx_LogsCause(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	// A closed pool makes every subsequent store query fail; Close is
	// idempotent, so the helper's own t.Cleanup(pool.Close) is still fine.
	pool.Close()

	sink := captureLogs(t)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/projects/{projectID}/report", ProjectReportHandler(store))
	handler := obs.Middleware(mux)

	const projectID = "00000000-0000-4000-8000-000000000001"
	req := httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/report", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	var body catalogErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error body: %v", err)
	}
	if body.Error != "failed to check project existence" {
		t.Fatalf("expected unchanged client message, got %q", body.Error)
	}

	requestID := rec.Header().Get("X-Request-Id")
	if requestID == "" {
		t.Fatalf("expected X-Request-Id response header")
	}

	lines := sink.Find("failed to check project existence",
		"level", "ERROR",
		"request_id", requestID,
		"project_id", projectID,
		"route", "GET /api/projects/{projectID}/report",
		"method", http.MethodGet,
	)
	if len(lines) != 1 {
		t.Fatalf("expected exactly one Error line for request %s, got %d: %v", requestID, len(lines), sink.Lines())
	}
	errAttr, _ := lines[0]["error"].(string)
	if !strings.Contains(errAttr, "closed pool") {
		t.Fatalf("expected error attr to carry the pgx cause, got %q", errAttr)
	}
}

func TestCatalogHandler_4xx_DoesNotLog(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)

	sink := captureLogs(t)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/catalog/upload", UploadHandler(store, &recordingEnqueuer{}))
	handler := obs.Middleware(mux)

	req := newMultipartUploadRequest(t, "", "sbom.json", []byte(handlerFixtureSBOM))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	requestID := rec.Header().Get("X-Request-Id")
	if lines := sink.Find("", "level", "ERROR", "request_id", requestID); len(lines) != 0 {
		t.Fatalf("expected no Error line for a 4xx, got %v", lines)
	}
}
