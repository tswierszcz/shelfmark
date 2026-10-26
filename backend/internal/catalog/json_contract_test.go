package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"testing"

	"github.com/tswierszcz/shelfmark/backend/internal/auth"
)

// assertNoNullArrays checks the raw JSON contract of a response body: the
// top level must not be null, and no object field anywhere in the document
// may be null unless its name is listed in nullable. Decoding into any (not
// into Go structs) is the point: encoding/json decodes null and [] into the
// same empty slice, so struct-based assertions cannot tell them apart.
func assertNoNullArrays(t *testing.T, body []byte, nullable ...string) {
	t.Helper()

	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("response is not valid JSON: %v: %s", err, body)
	}
	if doc == nil {
		t.Fatalf("expected a non-null JSON document, got %s", body)
	}

	var walk func(path string, v any)
	walk = func(path string, v any) {
		switch val := v.(type) {
		case map[string]any:
			for key, field := range val {
				fieldPath := path + "." + key
				if field == nil {
					if !slices.Contains(nullable, key) {
						t.Errorf("field %s is null (expected [] or a value): %s", fieldPath, body)
					}
					continue
				}
				walk(fieldPath, field)
			}
		case []any:
			for i, item := range val {
				walk(path+"["+strconv.Itoa(i)+"]", item)
			}
		}
	}
	walk("$", doc)
}

// assertEmptyArrayField fails unless body is a JSON object whose field is
// present and is exactly an empty array.
func assertEmptyArrayField(t *testing.T, body []byte, field string) {
	t.Helper()

	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err != nil {
		t.Fatalf("response is not a JSON object: %v: %s", err, body)
	}
	raw, ok := obj[field]
	if !ok {
		t.Fatalf("expected field %q in response, got %s", field, body)
	}
	arr, ok := raw.([]any)
	if !ok {
		t.Fatalf("expected field %q to be an array, got %#v: %s", field, raw, body)
	}
	if len(arr) != 0 {
		t.Fatalf("expected field %q to be empty, got %s", field, body)
	}
}

// createUncategorizedLibrary inserts one library with no categories and no
// strategic marks.
func createUncategorizedLibrary(t *testing.T, store *Store) {
	t.Helper()

	lib := NewLibrary{Purl: "pkg:maven/com.example/foo@1.0.0", Name: "foo", Ecosystem: "maven"}
	if err := store.CreateLibraries(context.Background(), "11111111-1111-1111-1111-111111111111", []NewLibrary{lib}); err != nil {
		t.Fatalf("CreateLibraries failed: %v", err)
	}
}

// assertLibraryListContract runs the two list-library scenarios (empty DB,
// one uncategorized library) against handler.
func assertLibraryListContract(t *testing.T, newHandler func(*Store) http.HandlerFunc, path string) {
	t.Helper()

	get := func(t *testing.T, store *Store) []byte {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		newHandler(store)(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		return rec.Body.Bytes()
	}

	t.Run("empty", func(t *testing.T) {
		store := NewStore(startTestPostgres(t))

		body := get(t, store)
		if string(body) != "[]\n" {
			t.Fatalf("expected body %q, got %q", "[]\n", string(body))
		}
	})

	t.Run("uncategorized", func(t *testing.T) {
		store := NewStore(startTestPostgres(t))
		createUncategorizedLibrary(t, store)

		body := get(t, store)
		assertNoNullArrays(t, body, "github_repo")

		var items []map[string]any
		if err := json.Unmarshal(body, &items); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("expected 1 library, got %s", body)
		}
		itemBody, _ := json.Marshal(items[0])
		assertEmptyArrayField(t, itemBody, "categories")
		assertEmptyArrayField(t, itemBody, "strategic")
	})
}

func TestListLibrariesHandler_EmptyAndUncategorized_NoNullArrays(t *testing.T) {
	assertLibraryListContract(t, ListLibrariesHandler, "/api/catalog/libraries")
}

func TestListOrganizationLibrariesHandler_EmptyAndUncategorized_NoNullArrays(t *testing.T) {
	assertLibraryListContract(t, ListOrganizationLibrariesHandler, "/api/organization/libraries")
}

func TestBatchProgressHandler_UnknownBatch_ReturnsEmptyObject(t *testing.T) {
	store := NewStore(startTestPostgres(t))

	const batchID = "00000000-0000-0000-0000-000000000000"
	req := httptest.NewRequest(http.MethodGet, "/api/catalog/batches/"+batchID, nil)
	req.SetPathValue("batchID", batchID)
	rec := httptest.NewRecorder()
	BatchProgressHandler(store)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "{}\n" {
		t.Fatalf("expected body %q, got %q", "{}\n", rec.Body.String())
	}
}

func TestUploadHandler_NothingSkipped_ReturnsEmptySkippedArray(t *testing.T) {
	cases := []struct {
		name     string
		kind     string
		filename string
		content  string
	}{
		{"csv", "csv", "purls.csv", "purl\npkg:maven/com.google.guava/guava@32.0.0\n"},
		{"sbom", "sbom", "sbom.json", projectFixtureSBOMSingleDirect},
		{"sbom without components", "sbom", "sbom.json", `{"bomFormat":"CycloneDX","specVersion":"1.5","version":1}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := NewStore(startTestPostgres(t))

			req := newMultipartUploadRequest(t, tc.kind, tc.filename, []byte(tc.content))
			rec := httptest.NewRecorder()
			UploadHandler(store, &recordingEnqueuer{})(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
			}
			assertNoNullArrays(t, rec.Body.Bytes())
			assertEmptyArrayField(t, rec.Body.Bytes(), "skipped")
		})
	}
}

func TestProjectUploadHandler_NothingSkipped_ReturnsEmptySkippedArray(t *testing.T) {
	store := NewStore(startTestPostgres(t))

	t.Setenv("SESSION_SECRET", "test-secret")
	cookie := sessionCookie(t, "architect@example.com", auth.RoleArchitect)

	handler := auth.RequireRole(auth.RoleArchitect)(ProjectUploadHandler(store, &recordingEnqueuer{}))
	req := newProjectUploadRequest(t, "My Project", "sbom.json", []byte(projectFixtureSBOM))
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	assertNoNullArrays(t, rec.Body.Bytes())
	assertEmptyArrayField(t, rec.Body.Bytes(), "skipped")
}
