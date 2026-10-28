package catalog

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestMarkStrategic_UpsertsWithoutErrorOnRepeat(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	batchID := "55555555-5555-5555-5555-555555555555"
	lib := NewLibrary{Purl: "pkg:maven/com.example/strategic-mark@1.0.0", Name: "strategic-mark", Ecosystem: "maven"}
	if err := store.CreateLibraries(ctx, batchID, []NewLibrary{lib}); err != nil {
		t.Fatalf("CreateLibraries failed: %v", err)
	}

	libs, err := store.ListLibraries(ctx, LibraryFilter{BatchID: batchID})
	if err != nil {
		t.Fatalf("ListLibraries failed: %v", err)
	}
	if len(libs) != 1 {
		t.Fatalf("expected 1 library in batch, got %d", len(libs))
	}
	libraryID := libs[0].ID

	categories, err := store.ListTaxonomy(ctx)
	if err != nil {
		t.Fatalf("ListTaxonomy failed: %v", err)
	}
	categoryID := categories[0].ID

	if err := store.MarkStrategic(ctx, libraryID, categoryID, "critical dependency", "alice"); err != nil {
		t.Fatalf("first MarkStrategic failed: %v", err)
	}
	// Repeat call with different fields must upsert, not error.
	if err := store.MarkStrategic(ctx, libraryID, categoryID, "re-confirmed critical dependency", "bob"); err != nil {
		t.Fatalf("second MarkStrategic (repeat) failed: %v", err)
	}

	marks, err := store.StrategicMarksByLibrary(ctx, []string{libraryID})
	if err != nil {
		t.Fatalf("StrategicMarksByLibrary failed: %v", err)
	}
	got := marks[libraryID]
	if len(got) != 1 {
		t.Fatalf("expected 1 active strategic mark, got %d: %+v", len(got), got)
	}
	if got[0].Justification != "re-confirmed critical dependency" || got[0].MarkedBy != "bob" {
		t.Fatalf("expected upsert to update justification/marked_by, got %+v", got[0])
	}
	if got[0].CategoryID != categoryID || got[0].CategoryName != categories[0].Name {
		t.Fatalf("expected category fields to match %q/%q, got %+v", categoryID, categories[0].Name, got[0])
	}
}

func TestMarkStrategic_EnforcesTwoPerCategoryEcosystemCap(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	batchID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	libs := []NewLibrary{
		{Purl: "pkg:maven/com.example/cap-one@1.0.0", Name: "cap-one", Ecosystem: "maven"},
		{Purl: "pkg:maven/com.example/cap-two@1.0.0", Name: "cap-two", Ecosystem: "maven"},
		{Purl: "pkg:maven/com.example/cap-three@1.0.0", Name: "cap-three", Ecosystem: "maven"},
	}
	if err := store.CreateLibraries(ctx, batchID, libs); err != nil {
		t.Fatalf("CreateLibraries failed: %v", err)
	}

	listed, err := store.ListLibraries(ctx, LibraryFilter{BatchID: batchID})
	if err != nil {
		t.Fatalf("ListLibraries failed: %v", err)
	}
	byPurl := map[string]Library{}
	for _, l := range listed {
		byPurl[l.Purl] = l
	}
	libOne := byPurl["pkg:maven/com.example/cap-one@1.0.0"]
	libTwo := byPurl["pkg:maven/com.example/cap-two@1.0.0"]
	libThree := byPurl["pkg:maven/com.example/cap-three@1.0.0"]

	categories, err := store.ListTaxonomy(ctx)
	if err != nil {
		t.Fatalf("ListTaxonomy failed: %v", err)
	}
	categoryID := categories[0].ID

	if err := store.MarkStrategic(ctx, libOne.ID, categoryID, "critical dependency", "alice"); err != nil {
		t.Fatalf("MarkStrategic(libOne) failed: %v", err)
	}
	if err := store.MarkStrategic(ctx, libTwo.ID, categoryID, "critical dependency", "alice"); err != nil {
		t.Fatalf("MarkStrategic(libTwo) failed: %v", err)
	}

	// A 3rd distinct library in the same category+ecosystem must be rejected.
	if err := store.MarkStrategic(ctx, libThree.ID, categoryID, "critical dependency", "alice"); !errors.Is(err, ErrStrategicCapReached) {
		t.Fatalf("expected MarkStrategic(libThree) to return ErrStrategicCapReached, got %v", err)
	}

	// Re-marking (upserting) one of the original two must still succeed
	// despite the cap being at 2.
	if err := store.MarkStrategic(ctx, libOne.ID, categoryID, "re-confirmed critical dependency", "bob"); err != nil {
		t.Fatalf("expected re-mark of libOne to succeed despite cap, got error: %v", err)
	}
}

func TestUnmarkStrategic_OnNeverMarkedPairReturnsError(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	batchID := "66666666-6666-6666-6666-666666666666"
	lib := NewLibrary{Purl: "pkg:maven/com.example/never-marked@1.0.0", Name: "never-marked", Ecosystem: "maven"}
	if err := store.CreateLibraries(ctx, batchID, []NewLibrary{lib}); err != nil {
		t.Fatalf("CreateLibraries failed: %v", err)
	}

	libs, err := store.ListLibraries(ctx, LibraryFilter{BatchID: batchID})
	if err != nil {
		t.Fatalf("ListLibraries failed: %v", err)
	}
	libraryID := libs[0].ID

	categories, err := store.ListTaxonomy(ctx)
	if err != nil {
		t.Fatalf("ListTaxonomy failed: %v", err)
	}
	categoryID := categories[0].ID

	if err := store.UnmarkStrategic(ctx, libraryID, categoryID, "not actually strategic", "carol"); err == nil {
		t.Fatalf("expected UnmarkStrategic on a never-marked pair to return an error, got nil")
	}
}

func TestUnmarkStrategic_HappyPathDeactivatesMark(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	batchID := "77777777-7777-7777-7777-777777777777"
	lib := NewLibrary{Purl: "pkg:maven/com.example/unmark-me@1.0.0", Name: "unmark-me", Ecosystem: "maven"}
	if err := store.CreateLibraries(ctx, batchID, []NewLibrary{lib}); err != nil {
		t.Fatalf("CreateLibraries failed: %v", err)
	}

	libs, err := store.ListLibraries(ctx, LibraryFilter{BatchID: batchID})
	if err != nil {
		t.Fatalf("ListLibraries failed: %v", err)
	}
	libraryID := libs[0].ID

	categories, err := store.ListTaxonomy(ctx)
	if err != nil {
		t.Fatalf("ListTaxonomy failed: %v", err)
	}
	categoryID := categories[0].ID

	if err := store.MarkStrategic(ctx, libraryID, categoryID, "critical dependency", "alice"); err != nil {
		t.Fatalf("MarkStrategic failed: %v", err)
	}

	if err := store.UnmarkStrategic(ctx, libraryID, categoryID, "no longer critical", "dave"); err != nil {
		t.Fatalf("UnmarkStrategic failed: %v", err)
	}

	marks, err := store.StrategicMarksByLibrary(ctx, []string{libraryID})
	if err != nil {
		t.Fatalf("StrategicMarksByLibrary failed: %v", err)
	}
	if got := marks[libraryID]; len(got) != 0 {
		t.Fatalf("expected no active strategic marks after unmark, got %+v", got)
	}

	// A second unmark on the now-inactive pair must fail (active = true
	// predicate no longer matches).
	if err := store.UnmarkStrategic(ctx, libraryID, categoryID, "still not critical", "dave"); err == nil {
		t.Fatalf("expected UnmarkStrategic on an already-inactive pair to return an error, got nil")
	}
}

func TestStrategicMarksByLibrary_ReflectsOnlyActiveMarks(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	batchID := "88888888-8888-8888-8888-888888888888"
	libs := []NewLibrary{
		{Purl: "pkg:maven/com.example/active-mark@1.0.0", Name: "active-mark", Ecosystem: "maven"},
		{Purl: "pkg:maven/com.example/inactive-mark@1.0.0", Name: "inactive-mark", Ecosystem: "maven"},
		{Purl: "pkg:maven/com.example/no-mark@1.0.0", Name: "no-mark", Ecosystem: "maven"},
	}
	if err := store.CreateLibraries(ctx, batchID, libs); err != nil {
		t.Fatalf("CreateLibraries failed: %v", err)
	}

	listed, err := store.ListLibraries(ctx, LibraryFilter{BatchID: batchID})
	if err != nil {
		t.Fatalf("ListLibraries failed: %v", err)
	}
	if len(listed) != 3 {
		t.Fatalf("expected 3 libraries in batch, got %d", len(listed))
	}
	byPurl := map[string]Library{}
	for _, l := range listed {
		byPurl[l.Purl] = l
	}
	activeLib := byPurl["pkg:maven/com.example/active-mark@1.0.0"]
	inactiveLib := byPurl["pkg:maven/com.example/inactive-mark@1.0.0"]
	noMarkLib := byPurl["pkg:maven/com.example/no-mark@1.0.0"]

	categories, err := store.ListTaxonomy(ctx)
	if err != nil {
		t.Fatalf("ListTaxonomy failed: %v", err)
	}
	categoryID := categories[0].ID

	if err := store.MarkStrategic(ctx, activeLib.ID, categoryID, "critical dependency", "alice"); err != nil {
		t.Fatalf("MarkStrategic(active) failed: %v", err)
	}
	if err := store.MarkStrategic(ctx, inactiveLib.ID, categoryID, "critical dependency", "alice"); err != nil {
		t.Fatalf("MarkStrategic(inactive) failed: %v", err)
	}
	if err := store.UnmarkStrategic(ctx, inactiveLib.ID, categoryID, "no longer critical", "bob"); err != nil {
		t.Fatalf("UnmarkStrategic(inactive) failed: %v", err)
	}

	marks, err := store.StrategicMarksByLibrary(ctx, []string{activeLib.ID, inactiveLib.ID, noMarkLib.ID})
	if err != nil {
		t.Fatalf("StrategicMarksByLibrary failed: %v", err)
	}

	if got := marks[activeLib.ID]; len(got) != 1 {
		t.Fatalf("expected 1 active mark for active-mark library, got %d: %+v", len(got), got)
	}
	if got := marks[inactiveLib.ID]; len(got) != 0 {
		t.Fatalf("expected 0 active marks for unmarked (deactivated) library, got %d: %+v", len(got), got)
	}
	if got := marks[noMarkLib.ID]; len(got) != 0 {
		t.Fatalf("expected 0 active marks for never-marked library, got %d: %+v", len(got), got)
	}
}

func TestMarkStrategic_CapEdges(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	mark := func(t *testing.T, lib Library, categoryID string) error {
		t.Helper()
		return store.MarkStrategic(ctx, lib.ID, categoryID, "critical dependency", "alice")
	}
	mustMark := func(t *testing.T, lib Library, categoryID string) {
		t.Helper()
		if err := mark(t, lib, categoryID); err != nil {
			t.Fatalf("MarkStrategic(%s) failed: %v", lib.Name, err)
		}
	}

	t.Run("other ecosystem in same category at cap is allowed", func(t *testing.T) {
		categoryID := newTestCategory(t, pool, "edge-other-ecosystem")
		maven := newTestLibraries(t, store, pool, "maven", "edge-eco-maven", 2)
		npm := newTestLibraries(t, store, pool, "npm", "edge-eco-npm", 1)
		mustMark(t, maven[0], categoryID)
		mustMark(t, maven[1], categoryID)

		if err := mark(t, npm[0], categoryID); err != nil {
			t.Fatalf("expected npm mark in a maven-capped category to succeed, got %v", err)
		}
	})

	t.Run("unmarking one of two frees a slot for a new library", func(t *testing.T) {
		categoryID := newTestCategory(t, pool, "edge-unmark-frees")
		libs := newTestLibraries(t, store, pool, "maven", "edge-unmark", 3)
		mustMark(t, libs[0], categoryID)
		mustMark(t, libs[1], categoryID)
		if err := store.UnmarkStrategic(ctx, libs[0].ID, categoryID, "no longer critical", "bob"); err != nil {
			t.Fatalf("UnmarkStrategic failed: %v", err)
		}

		if err := mark(t, libs[2], categoryID); err != nil {
			t.Fatalf("expected third library to be allowed after an unmark, got %v", err)
		}
		if got := countActiveStrategicMarks(t, pool, categoryID, "maven"); got != 2 {
			t.Fatalf("expected 2 active marks, got %d", got)
		}
	})

	t.Run("re-activating an unmarked library while two others are active is rejected", func(t *testing.T) {
		categoryID := newTestCategory(t, pool, "edge-reactivate")
		libs := newTestLibraries(t, store, pool, "maven", "edge-reactivate", 3)
		mustMark(t, libs[0], categoryID)
		if err := store.UnmarkStrategic(ctx, libs[0].ID, categoryID, "no longer critical", "bob"); err != nil {
			t.Fatalf("UnmarkStrategic failed: %v", err)
		}
		mustMark(t, libs[1], categoryID)
		mustMark(t, libs[2], categoryID)

		if err := mark(t, libs[0], categoryID); !errors.Is(err, ErrStrategicCapReached) {
			t.Fatalf("expected re-activation at cap to return ErrStrategicCapReached, got %v", err)
		}
		if got := countActiveStrategicMarks(t, pool, categoryID, "maven"); got != 2 {
			t.Fatalf("expected 2 active marks, got %d", got)
		}
	})

	t.Run("same library marked in a second category is allowed", func(t *testing.T) {
		firstCategoryID := newTestCategory(t, pool, "edge-two-categories-a")
		secondCategoryID := newTestCategory(t, pool, "edge-two-categories-b")
		libs := newTestLibraries(t, store, pool, "maven", "edge-two-categories", 2)
		mustMark(t, libs[0], firstCategoryID)
		mustMark(t, libs[1], firstCategoryID)

		if err := mark(t, libs[0], secondCategoryID); err != nil {
			t.Fatalf("expected marking the same library in a second category to succeed, got %v", err)
		}
	})
}

func TestMarkStrategic_InterleavedMarksRespectCap(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	categoryID := newTestCategory(t, pool, "interleaved-cap")
	libs := newTestLibraries(t, store, pool, "maven", "interleaved-cap", 3)
	if err := store.MarkStrategic(ctx, libs[0].ID, categoryID, "critical dependency", "alice"); err != nil {
		t.Fatalf("initial MarkStrategic failed: %v", err)
	}

	// Each transaction that passes the cap check waits here until a second
	// one has also passed it, or 2 s elapse. The timeout matters once the
	// check is serialized: the lock holder must not wait forever for a
	// transaction that is blocked on its lock.
	var (
		mu       sync.Mutex
		arrivals int
	)
	bothArrived := make(chan struct{})
	previous := afterCapCheck
	t.Cleanup(func() { afterCapCheck = previous })
	afterCapCheck = func() {
		mu.Lock()
		arrivals++
		if arrivals == 2 {
			close(bothArrived)
		}
		mu.Unlock()
		select {
		case <-bothArrived:
		case <-time.After(2 * time.Second):
		}
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, lib := range libs[1:] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = store.MarkStrategic(ctx, lib.ID, categoryID, "critical dependency", "bob")
		}()
	}
	wg.Wait()

	var succeeded, capped int
	for i, err := range errs {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrStrategicCapReached):
			capped++
		default:
			t.Fatalf("MarkStrategic(goroutine %d) returned unexpected error: %v", i, err)
		}
	}
	if succeeded != 1 || capped != 1 {
		t.Errorf("expected exactly 1 success and 1 ErrStrategicCapReached, got %d successes and %d rejections", succeeded, capped)
	}
	if got := countActiveStrategicMarks(t, pool, categoryID, "maven"); got != 2 {
		t.Errorf("expected 2 active marks, got %d", got)
	}
}

func TestMarkStrategic_LockTimeoutReturnsBusy(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)

	setStrategicLockTimeout(t, "200ms")

	categoryID := newTestCategory(t, pool, "lock-timeout")
	libs := newTestLibraries(t, store, pool, "maven", "lock-timeout", 1)
	release := holdStrategicLock(t, pool, categoryID, "maven")

	// The context deadline only keeps a regression from hanging the suite;
	// the lock timeout must fire well before it.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	start := time.Now()
	err := store.MarkStrategic(ctx, libs[0].ID, categoryID, "critical dependency", "alice")
	elapsed := time.Since(start)
	if !errors.Is(err, ErrStrategicMarkBusy) {
		t.Fatalf("expected ErrStrategicMarkBusy while the lock is held, got %v (after %s)", err, elapsed)
	}
	if elapsed > time.Second {
		t.Fatalf("expected MarkStrategic to give up within ~1s, took %s", elapsed)
	}

	release()
	if err := store.MarkStrategic(context.Background(), libs[0].ID, categoryID, "critical dependency", "alice"); err != nil {
		t.Fatalf("expected MarkStrategic to succeed once the lock is released, got %v", err)
	}
	if got := countActiveStrategicMarks(t, pool, categoryID, "maven"); got != 1 {
		t.Fatalf("expected 1 active mark, got %d", got)
	}
}

func TestMarkStrategic_ConcurrentMarksNeverExceedCap(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	const (
		iterations = 20
		n          = 10
	)
	for iter := range iterations {
		categoryID := newTestCategory(t, pool, fmt.Sprintf("concurrent-cap-%d", iter))
		libs := newTestLibraries(t, store, pool, "maven", fmt.Sprintf("concurrent-cap-%d", iter), n)

		start := make(chan struct{})
		var wg sync.WaitGroup
		errs := make([]error, n)
		for i, lib := range libs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				errs[i] = store.MarkStrategic(ctx, lib.ID, categoryID, "critical dependency", "alice")
			}()
		}
		close(start)
		wg.Wait()

		capped := 0
		for i, err := range errs {
			switch {
			case err == nil:
			case errors.Is(err, ErrStrategicCapReached):
				capped++
			default:
				t.Fatalf("iteration %d: MarkStrategic(goroutine %d) returned unexpected error: %v", iter, i, err)
			}
		}
		active := countActiveStrategicMarks(t, pool, categoryID, "maven")
		if active > 2 {
			t.Fatalf("iteration %d: %d active marks exceed the cap of 2", iter, active)
		}
		if capped != n-2 {
			t.Fatalf("iteration %d: expected %d ErrStrategicCapReached, got %d (active=%d)", iter, n-2, capped, active)
		}
	}
}

func TestStrategicIndex_RetireCandidateRule(t *testing.T) {
	// libA is the strategic pick in catReviewed; catOpen has no active mark.
	idx := newStrategicIndex([]strategicPair{{libraryID: "libA", categoryID: "catReviewed"}})

	cases := []struct {
		name          string
		idx           strategicIndex
		libraryID     string
		categoryID    *string
		isDirect      bool
		wantStrategic bool
		wantReviewed  bool
		wantRetire    bool
	}{
		{name: "direct, strategic, reviewed", idx: idx, libraryID: "libA", categoryID: indexCategory("catReviewed"), isDirect: true, wantStrategic: true, wantReviewed: true, wantRetire: false},
		{name: "direct, not strategic, reviewed", idx: idx, libraryID: "libB", categoryID: indexCategory("catReviewed"), isDirect: true, wantStrategic: false, wantReviewed: true, wantRetire: true},
		{name: "direct, not strategic, not reviewed", idx: idx, libraryID: "libB", categoryID: indexCategory("catOpen"), isDirect: true, wantStrategic: false, wantReviewed: false, wantRetire: false},
		{name: "direct, strategic elsewhere, not reviewed", idx: idx, libraryID: "libA", categoryID: indexCategory("catOpen"), isDirect: true, wantStrategic: false, wantReviewed: false, wantRetire: false},
		{name: "transitive, strategic, reviewed", idx: idx, libraryID: "libA", categoryID: indexCategory("catReviewed"), isDirect: false, wantStrategic: true, wantReviewed: true, wantRetire: false},
		{name: "transitive, not strategic, reviewed", idx: idx, libraryID: "libB", categoryID: indexCategory("catReviewed"), isDirect: false, wantStrategic: false, wantReviewed: true, wantRetire: false},
		{name: "transitive, not strategic, not reviewed", idx: idx, libraryID: "libB", categoryID: indexCategory("catOpen"), isDirect: false, wantStrategic: false, wantReviewed: false, wantRetire: false},
		{name: "direct, no category", idx: idx, libraryID: "libB", categoryID: nil, isDirect: true, wantStrategic: false, wantReviewed: false, wantRetire: false},
		{name: "empty index, direct", idx: newStrategicIndex(nil), libraryID: "libA", categoryID: indexCategory("catReviewed"), isDirect: true, wantStrategic: false, wantReviewed: false, wantRetire: false},
		{name: "zero-value index, direct", idx: strategicIndex{}, libraryID: "libA", categoryID: indexCategory("catReviewed"), isDirect: true, wantStrategic: false, wantReviewed: false, wantRetire: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Mirror ProjectReport: a category-less row never asks the index.
			var gotStrategic, gotReviewed, gotRetire bool
			if tc.categoryID != nil {
				gotStrategic = tc.idx.isStrategic(tc.libraryID, *tc.categoryID)
				gotReviewed = tc.idx.isReviewed(*tc.categoryID)
				gotRetire = tc.idx.isRetireCandidate(tc.libraryID, *tc.categoryID, tc.isDirect)
			}
			if gotStrategic != tc.wantStrategic {
				t.Errorf("isStrategic = %v, want %v", gotStrategic, tc.wantStrategic)
			}
			if gotReviewed != tc.wantReviewed {
				t.Errorf("isReviewed = %v, want %v", gotReviewed, tc.wantReviewed)
			}
			if gotRetire != tc.wantRetire {
				t.Errorf("isRetireCandidate = %v, want %v", gotRetire, tc.wantRetire)
			}
		})
	}
}

// indexCategory returns a pointer to categoryID, for report-row-shaped cases.
func indexCategory(categoryID string) *string { return &categoryID }
