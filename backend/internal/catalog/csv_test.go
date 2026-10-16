package catalog

import (
	"strings"
	"testing"
)

func TestParsePurlCSV_WithHeaderRow(t *testing.T) {
	csvContent := "purl\npkg:maven/com.fasterxml.jackson.core/jackson-databind@2.15.0\npkg:npm/left-pad@1.3.0\n"

	libs, skipped, err := ParsePurlCSV(strings.NewReader(csvContent))
	if err != nil {
		t.Fatalf("ParsePurlCSV failed: %v", err)
	}

	if len(libs) != 1 {
		t.Fatalf("expected 1 parsed library, got %d: %+v", len(libs), libs)
	}
	if len(skipped) != 1 {
		t.Fatalf("expected 1 skipped row, got %d: %+v", len(skipped), skipped)
	}
	// Row 0 is the header (skipped silently), row 1 is the valid purl, row 2
	// is the rejected non-Maven purl at index 2.
	if skipped[0].Index != 2 {
		t.Errorf("expected skip at index 2, got %d", skipped[0].Index)
	}
}

func TestParsePurlCSV_WithoutHeaderRow(t *testing.T) {
	csvContent := "pkg:maven/com.example/foo@1.0.0\npkg:maven/com.example/bar@2.0.0\n"

	libs, skipped, err := ParsePurlCSV(strings.NewReader(csvContent))
	if err != nil {
		t.Fatalf("ParsePurlCSV failed: %v", err)
	}

	if len(libs) != 2 {
		t.Fatalf("expected 2 parsed libraries, got %d: %+v", len(libs), libs)
	}
	if len(skipped) != 0 {
		t.Fatalf("expected 0 skipped rows, got %d: %+v", len(skipped), skipped)
	}
}
