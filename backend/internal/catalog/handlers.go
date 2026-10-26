package catalog

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/tswierszcz/shelfmark/backend/internal/auth"
	"github.com/tswierszcz/shelfmark/backend/internal/obs"
)

// Enqueuer accepts a library for background categorization processing. The
// concrete implementation (a worker-pool queue) is added in a later phase;
// this interface lets the upload handler depend on that behavior without
// depending on the concrete type.
type Enqueuer interface {
	Enqueue(lib Library)
}

// noopEnqueuer is a placeholder Enqueuer until the real worker-pool queue
// (a later phase) exists. It intentionally does nothing.
type noopEnqueuer struct{}

func (noopEnqueuer) Enqueue(lib Library) {}

// NewNoopEnqueuer returns an Enqueuer that discards every library handed to
// it. Used to wire UploadHandler before the real queue exists.
func NewNoopEnqueuer() Enqueuer {
	return noopEnqueuer{}
}

// maxUploadBytes caps the request body read for the multipart upload
// handlers (UploadHandler, ProjectUploadHandler), mirroring the
// http.MaxBytesReader pattern used for login in auth/handlers.go. An
// oversized SBOM/CSV upload is rejected with a clean 400 instead of being
// read to completion unbounded.
const maxUploadBytes = 10 << 20 // 10MB

type uploadResponse struct {
	BatchID string       `json:"batch_id"`
	Created int          `json:"created"`
	Skipped []SkippedRow `json:"skipped"`
}

type catalogErrorResponse struct {
	Error string `json:"error"`
}

// writeCatalogError writes msg as the JSON error body with the given status.
// A 5xx also logs msg, err and attrs together with the request's attributes,
// so the cause of a server error is recorded while the client still receives
// only msg. 4xx responses are client mistakes and are not logged; callers
// pass err when they have one for symmetry.
func writeCatalogError(w http.ResponseWriter, r *http.Request, status int, msg string, err error, attrs ...any) {
	if status >= http.StatusInternalServerError {
		obs.LogError(r, msg, err, attrs...)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(catalogErrorResponse{Error: msg})
}

// UploadHandler returns a handler for POST /api/catalog/upload: it accepts a
// multipart file upload (either a CycloneDX SBOM or a CSV purl list,
// distinguished by the required "kind" form field), parses it, creates the
// resulting library rows under a fresh upload batch, and enqueues each
// newly-created library for background categorization.
func UploadHandler(store *Store, queue Enqueuer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)

		// Parse the multipart form explicitly so an oversized body is
		// caught here with a specific message, rather than silently
		// discarded by FormValue (which swallows parse errors and would
		// otherwise surface as a misleading "kind must be..." 400).
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				writeCatalogError(w, r, http.StatusBadRequest, "uploaded file exceeds the 10MB limit", nil)
				return
			}
		}

		kind := r.FormValue("kind")
		if kind != "sbom" && kind != "csv" {
			writeCatalogError(w, r, http.StatusBadRequest, `kind must be "sbom" or "csv"`, nil)
			return
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				writeCatalogError(w, r, http.StatusBadRequest, "uploaded file exceeds the 10MB limit", nil)
				return
			}
			writeCatalogError(w, r, http.StatusBadRequest, "missing file", err)
			return
		}
		defer file.Close()

		var (
			libs    []NewLibrary
			skipped []SkippedRow
		)
		switch kind {
		case "sbom":
			libs, skipped, err = ParseCycloneDXSBOM(file)
		case "csv":
			libs, skipped, err = ParsePurlCSV(file)
		}
		if err != nil {
			writeCatalogError(w, r, http.StatusBadRequest, err.Error(), err)
			return
		}

		batchID, err := newBatchID()
		if err != nil {
			writeCatalogError(w, r, http.StatusInternalServerError, "failed to generate batch id", err)
			return
		}

		ctx := r.Context()
		if err := store.CreateLibraries(ctx, batchID, libs); err != nil {
			writeCatalogError(w, r, http.StatusInternalServerError, "failed to create libraries", err, "batch_id", batchID)
			return
		}

		created, err := store.ListLibraries(ctx, LibraryFilter{BatchID: batchID})
		if err != nil {
			writeCatalogError(w, r, http.StatusInternalServerError, "failed to list created libraries", err, "batch_id", batchID)
			return
		}

		for _, lib := range created {
			queue.Enqueue(lib)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(uploadResponse{
			BatchID: batchID,
			Created: len(created),
			Skipped: skipped,
		})
	}
}

// BatchProgressHandler returns a handler for GET
// /api/catalog/batches/{batchID}: it reports the count of libraries in each
// status for the given batch. A batch ID with zero matching libraries (e.g.
// a typo'd ID) returns 200 with an empty progress map, not a 404.
func BatchProgressHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		batchID := r.PathValue("batchID")

		progress, err := store.BatchProgress(r.Context(), batchID)
		if err != nil {
			writeCatalogError(w, r, http.StatusInternalServerError, "failed to get batch progress", err, "batch_id", batchID)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(progress)
	}
}

// libraryResponse is a Library plus the names of its attached categories.
// The plan's contract for ListLibrariesHandler didn't spell out this exact
// response shape; folding categories in here (rather than a separate
// per-library endpoint) is the simplest way to let the catalog page show
// "which libraries are uncategorized" without N follow-up requests.
type libraryResponse struct {
	Library
	Categories []string        `json:"categories"`
	Strategic  []StrategicMark `json:"strategic"`
}

// ListLibrariesHandler returns a handler for GET /api/catalog/libraries: it
// lists libraries matching the optional status/category/ecosystem query
// params, each annotated with the names of its attached categories.
func ListLibrariesHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter := LibraryFilter{
			Status:    r.URL.Query().Get("status"),
			Category:  r.URL.Query().Get("category"),
			Ecosystem: r.URL.Query().Get("ecosystem"),
		}

		ctx := r.Context()
		libraries, err := store.ListLibraries(ctx, filter)
		if err != nil {
			writeCatalogError(w, r, http.StatusInternalServerError, "failed to list libraries", err, "query", "list_libraries")
			return
		}

		ids := make([]string, len(libraries))
		for i, l := range libraries {
			ids[i] = l.ID
		}
		categoryNames, err := store.CategoryNamesByLibrary(ctx, ids)
		if err != nil {
			writeCatalogError(w, r, http.StatusInternalServerError, "failed to list library categories", err, "query", "category_names_by_library")
			return
		}

		resp := make([]libraryResponse, len(libraries))
		for i, l := range libraries {
			names := categoryNames[l.ID]
			if names == nil {
				names = []string{}
			}
			resp[i] = libraryResponse{Library: l, Categories: names, Strategic: []StrategicMark{}}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			obs.LogWarn(r, "failed to encode response", err)
		}
	}
}

// ListOrganizationLibrariesHandler returns a handler for GET
// /api/organization/libraries: it lists libraries matching the same
// status/category/ecosystem filters as ListLibrariesHandler, plus an
// optional "strategic" filter ("true"/"false"; absent means no filter), each
// annotated with its attached category names and its active strategic
// marks.
func ListOrganizationLibrariesHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter := LibraryFilter{
			Status:    r.URL.Query().Get("status"),
			Category:  r.URL.Query().Get("category"),
			Ecosystem: r.URL.Query().Get("ecosystem"),
		}

		switch strategicParam := r.URL.Query().Get("strategic"); strategicParam {
		case "":
			// No filter.
		case "true":
			v := true
			filter.Strategic = &v
		case "false":
			v := false
			filter.Strategic = &v
		default:
			writeCatalogError(w, r, http.StatusBadRequest, `strategic must be "true" or "false"`, nil)
			return
		}

		ctx := r.Context()
		libraries, err := store.ListLibraries(ctx, filter)
		if err != nil {
			writeCatalogError(w, r, http.StatusInternalServerError, "failed to list libraries", err, "query", "list_libraries")
			return
		}

		ids := make([]string, len(libraries))
		for i, l := range libraries {
			ids[i] = l.ID
		}
		categoryNames, err := store.CategoryNamesByLibrary(ctx, ids)
		if err != nil {
			writeCatalogError(w, r, http.StatusInternalServerError, "failed to list library categories", err, "query", "category_names_by_library")
			return
		}
		strategicMarks, err := store.StrategicMarksByLibrary(ctx, ids)
		if err != nil {
			writeCatalogError(w, r, http.StatusInternalServerError, "failed to list strategic marks", err, "query", "strategic_marks_by_library")
			return
		}

		resp := make([]libraryResponse, len(libraries))
		for i, l := range libraries {
			names := categoryNames[l.ID]
			if names == nil {
				names = []string{}
			}
			marks := strategicMarks[l.ID]
			if marks == nil {
				marks = []StrategicMark{}
			}
			// error_detail is an admin-only pipeline diagnostic (raw upstream
			// error chains); Architect/Viewer don't get it.
			l.ErrorDetail = nil
			resp[i] = libraryResponse{Library: l, Categories: names, Strategic: marks}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			obs.LogWarn(r, "failed to encode response", err)
		}
	}
}

// ListTaxonomyHandler returns a handler for GET /api/catalog/categories: it
// reports the full seeded taxonomy as a JSON array.
func ListTaxonomyHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		categories, err := store.ListTaxonomy(r.Context())
		if err != nil {
			writeCatalogError(w, r, http.StatusInternalServerError, "failed to list taxonomy", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(categories); err != nil {
			obs.LogWarn(r, "failed to encode response", err)
		}
	}
}

type attachCategoryRequest struct {
	CategoryID string `json:"category_id"`
}

// AttachCategoryHandler returns a handler for POST
// /api/catalog/libraries/{libraryID}/categories: it records a manual
// (library, category) attachment, overwriting whatever source produced an
// existing attachment for the same pair (AttachCategory's upsert
// semantics).
func AttachCategoryHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		libraryID := r.PathValue("libraryID")

		var req attachCategoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CategoryID == "" {
			writeCatalogError(w, r, http.StatusBadRequest, "missing or malformed category_id", nil)
			return
		}

		if err := store.AttachCategory(r.Context(), libraryID, req.CategoryID, "manual"); err != nil {
			writeCatalogError(w, r, http.StatusInternalServerError, "failed to attach category", err, "library_id", libraryID, "category_id", req.CategoryID)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

type strategicMarkRequest struct {
	Justification string `json:"justification"`
}

// MarkStrategicHandler returns a handler for POST
// /api/organization/libraries/{libraryID}/categories/{categoryID}/strategic:
// it records the library as a strategic choice within the category, with
// the acting Architect's session email as the attribution ("marked_by").
func MarkStrategicHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		libraryID := r.PathValue("libraryID")
		categoryID := r.PathValue("categoryID")

		var req strategicMarkRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Justification) == "" {
			writeCatalogError(w, r, http.StatusBadRequest, "missing or blank justification", nil)
			return
		}

		session, ok := auth.SessionFromContext(r.Context())
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if err := store.MarkStrategic(r.Context(), libraryID, categoryID, req.Justification, session.Email); err != nil {
			if errors.Is(err, ErrInvalidReference) {
				writeCatalogError(w, r, http.StatusBadRequest, "library or category does not exist", err)
				return
			}
			if errors.Is(err, ErrStrategicCapReached) {
				writeCatalogError(w, r, http.StatusConflict, "already 2 strategic libraries marked for this category and ecosystem", err)
				return
			}
			writeCatalogError(w, r, http.StatusInternalServerError, "failed to mark strategic", err, "library_id", libraryID, "category_id", categoryID)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// UnmarkStrategicHandler returns a handler for DELETE
// /api/organization/libraries/{libraryID}/categories/{categoryID}/strategic:
// it deactivates an existing strategic mark for the (library, category)
// pair, recording the reason and the acting Architect via the same body
// contract as MarkStrategicHandler. It responds 404 when the pair is not
// currently marked strategic.
func UnmarkStrategicHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		libraryID := r.PathValue("libraryID")
		categoryID := r.PathValue("categoryID")

		var req strategicMarkRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Justification) == "" {
			writeCatalogError(w, r, http.StatusBadRequest, "missing or blank justification", nil)
			return
		}

		session, ok := auth.SessionFromContext(r.Context())
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if err := store.UnmarkStrategic(r.Context(), libraryID, categoryID, req.Justification, session.Email); err != nil {
			if errors.Is(err, ErrStrategicMarkNotFound) {
				writeCatalogError(w, r, http.StatusNotFound, "no active strategic mark found for this library and category", err)
				return
			}
			writeCatalogError(w, r, http.StatusInternalServerError, "failed to unmark strategic", err, "library_id", libraryID, "category_id", categoryID)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// newBatchID generates a fresh v4-shaped UUID string. There is no uuid
// package dependency in this module; this is the well-known ~10-line
// crypto/rand pattern rather than a new dependency for it.
func newBatchID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("catalog: failed to generate batch id: %w", err)
	}

	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
