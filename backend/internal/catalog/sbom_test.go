package catalog

import (
	"strings"
	"testing"
)

const fixtureSBOM = `{
  "bomFormat": "CycloneDX",
  "specVersion": "1.5",
  "version": 1,
  "components": [
    {
      "type": "library",
      "name": "jackson-databind",
      "purl": "pkg:maven/com.fasterxml.jackson.core/jackson-databind@2.15.0"
    },
    {
      "type": "library",
      "name": "no-purl-component"
    },
    {
      "type": "library",
      "name": "left-pad",
      "purl": "pkg:npm/left-pad@1.3.0"
    }
  ]
}`

func TestParseCycloneDXSBOM_SplitsParsedAndSkipped(t *testing.T) {
	libs, skipped, err := ParseCycloneDXSBOM(strings.NewReader(fixtureSBOM))
	if err != nil {
		t.Fatalf("ParseCycloneDXSBOM failed: %v", err)
	}

	if len(libs) != 1 {
		t.Fatalf("expected 1 parsed library, got %d: %+v", len(libs), libs)
	}
	if libs[0].Purl != "pkg:maven/com.fasterxml.jackson.core/jackson-databind@2.15.0" {
		t.Errorf("unexpected parsed purl: %q", libs[0].Purl)
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
}

func TestParseCycloneDXSBOM_MalformedJSONReturnsError(t *testing.T) {
	_, _, err := ParseCycloneDXSBOM(strings.NewReader("{not valid json"))
	if err == nil {
		t.Fatal("expected an error for malformed JSON, got nil")
	}
}

func TestParseCycloneDXSBOM_NoBOMStructureReturnsError(t *testing.T) {
	_, _, err := ParseCycloneDXSBOM(strings.NewReader("{}"))
	if err == nil {
		t.Fatal("expected an error for a file with no BOM structure, got nil")
	}
}
