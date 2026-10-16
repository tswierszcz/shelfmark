package catalog

import (
	"fmt"
	"io"

	cyclonedx "github.com/CycloneDX/cyclonedx-go"
)

// SkippedRow records a per-component (or per-CSV-row) failure that did not
// abort the whole upload: either the row had no purl, or its purl failed
// ParsePurl (e.g. a non-Maven purl type).
type SkippedRow struct {
	Index  int
	Reason string
}

// ParseCycloneDXSBOM extracts purls from an uploaded CycloneDX SBOM's
// components. A component with no PackageURL, or one that fails ParsePurl,
// is skipped (collected in the returned []SkippedRow) rather than failing
// the whole upload. A fundamentally malformed file - not valid JSON, or
// lacking any recognizable CycloneDX BOM structure - returns a non-nil
// error, since there is nothing to salvage per-component in that case.
func ParseCycloneDXSBOM(r io.Reader) ([]NewLibrary, []SkippedRow, error) {
	var bom cyclonedx.BOM
	if err := cyclonedx.NewBOMDecoder(r, cyclonedx.BOMFileFormatJSON).Decode(&bom); err != nil {
		return nil, nil, fmt.Errorf("catalog: failed to decode CycloneDX SBOM: %w", err)
	}

	if bom.BOMFormat != "CycloneDX" {
		return nil, nil, fmt.Errorf("catalog: not a CycloneDX BOM (missing or unexpected bomFormat %q)", bom.BOMFormat)
	}

	var (
		libraries []NewLibrary
		skipped   []SkippedRow
	)

	if bom.Components == nil {
		return libraries, skipped, nil
	}

	for i, component := range *bom.Components {
		if component.PackageURL == "" {
			skipped = append(skipped, SkippedRow{Index: i, Reason: "component has no purl"})
			continue
		}

		lib, err := ParsePurl(component.PackageURL)
		if err != nil {
			skipped = append(skipped, SkippedRow{Index: i, Reason: err.Error()})
			continue
		}

		libraries = append(libraries, lib)
	}

	return libraries, skipped, nil
}
