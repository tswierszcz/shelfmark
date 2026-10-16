package catalog

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

// ParsePurlCSV extracts purls from an uploaded CSV file: one purl per line,
// with an optional "purl" header row (case-insensitive). Same skip-not-fail
// behavior as ParseCycloneDXSBOM for rows whose purl fails to parse.
func ParsePurlCSV(r io.Reader) ([]NewLibrary, []SkippedRow, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("catalog: failed to parse CSV: %w", err)
	}

	var (
		libraries []NewLibrary
		skipped   []SkippedRow
	)

	for i, record := range records {
		if len(record) == 0 {
			continue
		}
		raw := strings.TrimSpace(record[0])
		if raw == "" {
			continue
		}
		if i == 0 && strings.EqualFold(raw, "purl") {
			continue
		}

		lib, err := ParsePurl(raw)
		if err != nil {
			skipped = append(skipped, SkippedRow{Index: i, Reason: err.Error()})
			continue
		}

		libraries = append(libraries, lib)
	}

	return libraries, skipped, nil
}
