package catalog

import (
	"fmt"
	"io"

	cyclonedx "github.com/CycloneDX/cyclonedx-go"
)

// ParseProjectSBOM extracts purls from an uploaded CycloneDX SBOM's
// components, exactly as ParseCycloneDXSBOM does (same skip semantics for a
// missing or unparseable purl, same error handling for a malformed or
// non-CycloneDX file), but additionally classifies each parsed purl as
// direct or transitive by walking the BOM's dependency graph.
//
// The returned map is keyed by purl and reports IsDirect. When the BOM
// carries no dependency graph (no "dependencies" field) or its root
// component (metadata.component) cannot be resolved against that graph -
// either because metadata.component has no bom-ref or because no
// Dependency entry's Ref matches it - every parsed purl is reported as
// direct. This is a deliberate fallback: it avoids presenting a
// direct/transitive distinction the input doesn't actually support, rather
// than guessing. The same conservative default applies per-component: a
// component with no bom-ref, or whose bom-ref doesn't appear in the root's
// dependsOn set, is reported as direct rather than transitive.
func ParseProjectSBOM(r io.Reader) ([]NewLibrary, map[string]bool, []SkippedRow, error) {
	var bom cyclonedx.BOM
	if err := cyclonedx.NewBOMDecoder(r, cyclonedx.BOMFileFormatJSON).Decode(&bom); err != nil {
		return nil, nil, nil, fmt.Errorf("catalog: failed to decode CycloneDX SBOM: %w", err)
	}

	if bom.BOMFormat != "CycloneDX" {
		return nil, nil, nil, fmt.Errorf("catalog: not a CycloneDX BOM (missing or unexpected bomFormat %q)", bom.BOMFormat)
	}

	// skipped is non-nil so a clean upload marshals "skipped": [] rather
	// than null (lessons.md: Go nil slices marshal to JSON null).
	var libraries []NewLibrary
	skipped := []SkippedRow{}

	isDirect := map[string]bool{}

	if bom.Components == nil {
		return libraries, isDirect, skipped, nil
	}

	// Resolve the root component's direct dependencies, if the BOM carries
	// enough structure to do so. directRefs stays nil (not just empty) when
	// unresolved, so the per-component loop below can fall back to "every
	// purl is direct" by checking for a nil map.
	var rootRef string
	if bom.Metadata != nil && bom.Metadata.Component != nil {
		rootRef = bom.Metadata.Component.BOMRef
	}

	var directRefs map[string]bool
	if bom.Dependencies != nil && rootRef != "" {
		for _, dep := range *bom.Dependencies {
			if dep.Ref != rootRef {
				continue
			}
			directRefs = map[string]bool{}
			if dep.Dependencies != nil {
				for _, ref := range *dep.Dependencies {
					directRefs[ref] = true
				}
			}
			break
		}
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

		// Conservative default: no resolvable dependency graph, no bom-ref
		// on this component, or a bom-ref absent from the root's dependsOn
		// set all mean "direct" rather than "transitive".
		direct := true
		if directRefs != nil && component.BOMRef != "" {
			direct = directRefs[component.BOMRef]
		}
		isDirect[lib.Purl] = direct
	}

	return libraries, isDirect, skipped, nil
}
