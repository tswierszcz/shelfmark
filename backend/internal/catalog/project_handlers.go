package catalog

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/tswierszcz/shelfmark/backend/internal/auth"
	"github.com/tswierszcz/shelfmark/backend/internal/obs"
)

type projectUploadResponse struct {
	ProjectID   string       `json:"project_id"`
	ProjectName string       `json:"project_name"`
	BatchID     string       `json:"batch_id"`
	Created     int          `json:"created"`
	Skipped     []SkippedRow `json:"skipped"`
}

// ProjectUploadHandler returns a handler for POST /api/projects/upload: it
// accepts a multipart file upload of a project name plus a CycloneDX SBOM,
// auto-ingests any purl the catalog doesn't already know (enqueueing only
// the newly-created, still-"pending" ones for background categorization so
// an already-categorized or manually-corrected library is never silently
// re-processed), and replaces the project's recorded library membership
// (direct/transitive) with the SBOM's current contents.
func ProjectUploadHandler(store *Store, queue Enqueuer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)

		session, ok := auth.SessionFromContext(r.Context())
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Parse the multipart form explicitly so an oversized body is
		// caught here with a specific message, rather than silently
		// discarded by FormValue (which swallows parse errors and would
		// otherwise surface as a misleading "missing project_name" 400).
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				writeCatalogError(w, r, http.StatusBadRequest, "uploaded file exceeds the 10MB limit", nil)
				return
			}
		}

		projectName := strings.TrimSpace(r.FormValue("project_name"))
		if projectName == "" {
			writeCatalogError(w, r, http.StatusBadRequest, "missing project_name", nil)
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

		libs, isDirect, skipped, err := ParseProjectSBOM(file)
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

		purls := make([]string, len(libs))
		for i, lib := range libs {
			purls[i] = lib.Purl
		}
		resolved, err := store.LibrariesByPurls(ctx, purls)
		if err != nil {
			writeCatalogError(w, r, http.StatusInternalServerError, "failed to resolve libraries", err, "batch_id", batchID)
			return
		}

		// Enqueue only the freshly-inserted (still "pending") libraries.
		// Queue.process unconditionally reruns the full categorization
		// cascade for anything enqueued, and AttachCategory upserts, so
		// enqueueing an already-categorized or manually-corrected library
		// would silently overwrite its library_categories.source.
		for _, lib := range resolved {
			if lib.Status == "pending" {
				queue.Enqueue(lib)
			}
		}

		project, err := store.FindOrCreateProject(ctx, projectName, session.Email)
		if err != nil {
			writeCatalogError(w, r, http.StatusInternalServerError, "failed to find or create project", err, "batch_id", batchID)
			return
		}

		entries := make([]ProjectLibraryEntry, len(resolved))
		for i, lib := range resolved {
			entries[i] = ProjectLibraryEntry{LibraryID: lib.ID, IsDirect: isDirect[lib.Purl]}
		}
		if err := store.ReplaceProjectLibraries(ctx, project.ID, entries); err != nil {
			writeCatalogError(w, r, http.StatusInternalServerError, "failed to record project libraries", err, "batch_id", batchID, "project_id", project.ID)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(projectUploadResponse{
			ProjectID:   project.ID,
			ProjectName: project.Name,
			BatchID:     batchID,
			Created:     len(resolved),
			Skipped:     skipped,
		})
	}
}

// projectReportCategory is one category a project report library is
// attached to, mirroring libraryResponse's shape for consistency with
// ListOrganizationLibrariesHandler.
type projectReportCategory struct {
	CategoryID      string `json:"category_id"`
	CategoryName    string `json:"category_name"`
	Strategic       bool   `json:"strategic"`
	RetireCandidate bool   `json:"retire_candidate"`
}

// projectReportLibrary is one library of a project's redundancy report,
// grouping its ProjectReportRow entries (one per category) under a single
// library record.
type projectReportLibrary struct {
	Library
	IsDirect   bool                    `json:"is_direct"`
	Categories []projectReportCategory `json:"categories"`
}

// ProjectReportHandler returns a handler for GET
// /api/projects/{projectID}/report: it computes the project's redundancy
// report and groups it by library, each annotated with its categories and
// their strategic/retire-candidate status. It responds 404 if projectID
// does not name an existing project (a report for a nonexistent project is
// a real error, unlike BatchProgressHandler's empty-map-on-unknown-batch
// behavior, which is legitimate for a batch that simply has no libraries
// yet).
func ProjectReportHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := r.PathValue("projectID")

		ctx := r.Context()
		exists, err := store.projectExists(ctx, projectID)
		if err != nil {
			writeCatalogError(w, r, http.StatusInternalServerError, "failed to check project existence", err, "project_id", projectID)
			return
		}
		if !exists {
			writeCatalogError(w, r, http.StatusNotFound, "project not found", nil)
			return
		}

		rows, err := store.ProjectReport(ctx, projectID)
		if err != nil {
			writeCatalogError(w, r, http.StatusInternalServerError, "failed to compute project report", err, "project_id", projectID)
			return
		}

		var (
			order []string
			byID  = map[string]*projectReportLibrary{}
		)
		for _, row := range rows {
			lib, ok := byID[row.LibraryID]
			if !ok {
				lib = &projectReportLibrary{
					Library: Library{
						ID:         row.LibraryID,
						Purl:       row.Purl,
						Name:       row.Name,
						Ecosystem:  row.Ecosystem,
						GithubRepo: row.GithubRepo,
						Status:     row.Status,
					},
					IsDirect:   row.IsDirect,
					Categories: []projectReportCategory{},
				}
				byID[row.LibraryID] = lib
				order = append(order, row.LibraryID)
			}
			lib.Categories = append(lib.Categories, projectReportCategory{
				CategoryID:      row.CategoryID,
				CategoryName:    row.CategoryName,
				Strategic:       row.Strategic,
				RetireCandidate: row.RetireCandidate,
			})
		}

		resp := make([]projectReportLibrary, len(order))
		for i, id := range order {
			resp[i] = *byID[id]
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			obs.LogWarn(r, "failed to encode response", err)
		}
	}
}

// ListProjectsHandler returns a handler for GET /api/projects: it lists
// every uploaded project for the frontend's project picker.
func ListProjectsHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projects, err := store.ListProjects(r.Context())
		if err != nil {
			writeCatalogError(w, r, http.StatusInternalServerError, "failed to list projects", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(projects); err != nil {
			obs.LogWarn(r, "failed to encode response", err)
		}
	}
}
