package catalog

import (
	"strings"
	"testing"
)

const fixtureProjectSBOMFullGraph = `{
  "bomFormat": "CycloneDX",
  "specVersion": "1.5",
  "version": 1,
  "metadata": {
    "component": {
      "type": "application",
      "bom-ref": "root-app",
      "name": "my-app",
      "version": "1.0.0"
    }
  },
  "components": [
    {
      "type": "library",
      "bom-ref": "direct-1",
      "name": "jackson-databind",
      "purl": "pkg:maven/com.fasterxml.jackson.core/jackson-databind@2.15.0"
    },
    {
      "type": "library",
      "bom-ref": "direct-2",
      "name": "guava",
      "purl": "pkg:maven/com.google.guava/guava@32.0.0"
    },
    {
      "type": "library",
      "bom-ref": "transitive-1",
      "name": "jackson-core",
      "purl": "pkg:maven/com.fasterxml.jackson.core/jackson-core@2.15.0"
    }
  ],
  "dependencies": [
    {
      "ref": "root-app",
      "dependsOn": ["direct-1", "direct-2"]
    },
    {
      "ref": "direct-1",
      "dependsOn": ["transitive-1"]
    },
    {
      "ref": "direct-2",
      "dependsOn": []
    },
    {
      "ref": "transitive-1",
      "dependsOn": []
    }
  ]
}`

const fixtureProjectSBOMNoDependencies = `{
  "bomFormat": "CycloneDX",
  "specVersion": "1.5",
  "version": 1,
  "metadata": {
    "component": {
      "type": "application",
      "bom-ref": "root-app",
      "name": "my-app",
      "version": "1.0.0"
    }
  },
  "components": [
    {
      "type": "library",
      "bom-ref": "comp-1",
      "name": "jackson-databind",
      "purl": "pkg:maven/com.fasterxml.jackson.core/jackson-databind@2.15.0"
    },
    {
      "type": "library",
      "bom-ref": "comp-2",
      "name": "guava",
      "purl": "pkg:maven/com.google.guava/guava@32.0.0"
    }
  ]
}`

const fixtureProjectSBOMUnresolvableRoot = `{
  "bomFormat": "CycloneDX",
  "specVersion": "1.5",
  "version": 1,
  "metadata": {
    "component": {
      "type": "application",
      "bom-ref": "root-app",
      "name": "my-app",
      "version": "1.0.0"
    }
  },
  "components": [
    {
      "type": "library",
      "bom-ref": "comp-1",
      "name": "jackson-databind",
      "purl": "pkg:maven/com.fasterxml.jackson.core/jackson-databind@2.15.0"
    },
    {
      "type": "library",
      "bom-ref": "comp-2",
      "name": "guava",
      "purl": "pkg:maven/com.google.guava/guava@32.0.0"
    }
  ],
  "dependencies": [
    {
      "ref": "some-other-ref",
      "dependsOn": ["comp-1"]
    }
  ]
}`

const fixtureProjectSBOMSkipRows = `{
  "bomFormat": "CycloneDX",
  "specVersion": "1.5",
  "version": 1,
  "components": [
    {
      "type": "library",
      "bom-ref": "comp-1",
      "name": "jackson-databind",
      "purl": "pkg:maven/com.fasterxml.jackson.core/jackson-databind@2.15.0"
    },
    {
      "type": "library",
      "name": "no-purl-component"
    },
    {
      "type": "library",
      "bom-ref": "comp-2",
      "name": "left-pad",
      "purl": "pkg:npm/left-pad@1.3.0"
    }
  ]
}`

func TestParseProjectSBOM(t *testing.T) {
	t.Run("full dependency graph splits direct and transitive", func(t *testing.T) {
		libs, isDirect, skipped, err := ParseProjectSBOM(strings.NewReader(fixtureProjectSBOMFullGraph))
		if err != nil {
			t.Fatalf("ParseProjectSBOM failed: %v", err)
		}
		if len(skipped) != 0 {
			t.Fatalf("expected no skipped rows, got %+v", skipped)
		}
		if len(libs) != 3 {
			t.Fatalf("expected 3 parsed libraries, got %d: %+v", len(libs), libs)
		}

		wantDirect := map[string]bool{
			"pkg:maven/com.fasterxml.jackson.core/jackson-databind@2.15.0": true,
			"pkg:maven/com.google.guava/guava@32.0.0":                      true,
			"pkg:maven/com.fasterxml.jackson.core/jackson-core@2.15.0":     false,
		}
		for purl, want := range wantDirect {
			got, ok := isDirect[purl]
			if !ok {
				t.Errorf("missing IsDirect entry for %q", purl)
				continue
			}
			if got != want {
				t.Errorf("IsDirect[%q] = %v, want %v", purl, got, want)
			}
		}
	})

	t.Run("no dependencies field falls back to all-direct", func(t *testing.T) {
		libs, isDirect, skipped, err := ParseProjectSBOM(strings.NewReader(fixtureProjectSBOMNoDependencies))
		if err != nil {
			t.Fatalf("ParseProjectSBOM failed: %v", err)
		}
		if len(skipped) != 0 {
			t.Fatalf("expected no skipped rows, got %+v", skipped)
		}
		if len(libs) != 2 {
			t.Fatalf("expected 2 parsed libraries, got %d: %+v", len(libs), libs)
		}
		for _, lib := range libs {
			if !isDirect[lib.Purl] {
				t.Errorf("expected IsDirect[%q] = true (no dependency graph), got false", lib.Purl)
			}
		}
	})

	t.Run("unresolvable root falls back to all-direct", func(t *testing.T) {
		libs, isDirect, skipped, err := ParseProjectSBOM(strings.NewReader(fixtureProjectSBOMUnresolvableRoot))
		if err != nil {
			t.Fatalf("ParseProjectSBOM failed: %v", err)
		}
		if len(skipped) != 0 {
			t.Fatalf("expected no skipped rows, got %+v", skipped)
		}
		if len(libs) != 2 {
			t.Fatalf("expected 2 parsed libraries, got %d: %+v", len(libs), libs)
		}
		for _, lib := range libs {
			if !isDirect[lib.Purl] {
				t.Errorf("expected IsDirect[%q] = true (unresolvable root), got false", lib.Purl)
			}
		}
	})

	t.Run("skips rows with no purl or unsupported purl type", func(t *testing.T) {
		libs, isDirect, skipped, err := ParseProjectSBOM(strings.NewReader(fixtureProjectSBOMSkipRows))
		if err != nil {
			t.Fatalf("ParseProjectSBOM failed: %v", err)
		}
		if len(libs) != 1 {
			t.Fatalf("expected 1 parsed library, got %d: %+v", len(libs), libs)
		}
		if libs[0].Purl != "pkg:maven/com.fasterxml.jackson.core/jackson-databind@2.15.0" {
			t.Errorf("unexpected parsed purl: %q", libs[0].Purl)
		}
		if !isDirect[libs[0].Purl] {
			t.Errorf("expected IsDirect[%q] = true (no dependency graph), got false", libs[0].Purl)
		}

		if len(skipped) != 2 {
			t.Fatalf("expected 2 skipped rows, got %d: %+v", len(skipped), skipped)
		}
		if skipped[0].Index != 1 {
			t.Errorf("expected first skip at index 1, got %d", skipped[0].Index)
		}
		if skipped[1].Index != 2 {
			t.Errorf("expected second skip at index 2, got %d", skipped[1].Index)
		}
	})
}
