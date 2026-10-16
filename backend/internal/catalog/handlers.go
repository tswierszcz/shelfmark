package catalog

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
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

type uploadResponse struct {
	BatchID string       `json:"batch_id"`
	Created int          `json:"created"`
	Skipped []SkippedRow `json:"skipped"`
}

type catalogErrorResponse struct {
	Error string `json:"error"`
}

func writeCatalogError(w http.ResponseWriter, status int, msg string) {
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
		kind := r.FormValue("kind")
		if kind != "sbom" && kind != "csv" {
			writeCatalogError(w, http.StatusBadRequest, `kind must be "sbom" or "csv"`)
			return
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			writeCatalogError(w, http.StatusBadRequest, "missing file")
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
			writeCatalogError(w, http.StatusBadRequest, err.Error())
			return
		}

		batchID, err := newBatchID()
		if err != nil {
			writeCatalogError(w, http.StatusInternalServerError, "failed to generate batch id")
			return
		}

		ctx := r.Context()
		if err := store.CreateLibraries(ctx, batchID, libs); err != nil {
			writeCatalogError(w, http.StatusInternalServerError, "failed to create libraries")
			return
		}

		created, err := store.ListLibraries(ctx, LibraryFilter{BatchID: batchID})
		if err != nil {
			writeCatalogError(w, http.StatusInternalServerError, "failed to list created libraries")
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
			writeCatalogError(w, http.StatusInternalServerError, "failed to get batch progress")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(progress)
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
