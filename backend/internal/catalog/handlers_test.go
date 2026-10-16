package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
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
