package catalog

import "testing"

func TestParsePurl_AcceptsValidMavenPurl(t *testing.T) {
	lib, err := ParsePurl("pkg:maven/com.fasterxml.jackson.core/jackson-databind@2.15.0")
	if err != nil {
		t.Fatalf("ParsePurl failed: %v", err)
	}

	if lib.Purl != "pkg:maven/com.fasterxml.jackson.core/jackson-databind@2.15.0" {
		t.Errorf("unexpected Purl: %q", lib.Purl)
	}
	if lib.Name != "com.fasterxml.jackson.core/jackson-databind" {
		t.Errorf("unexpected Name: %q", lib.Name)
	}
	if lib.Ecosystem != "maven" {
		t.Errorf("unexpected Ecosystem: %q", lib.Ecosystem)
	}
}

func TestParsePurl_RejectsNonMavenPurl(t *testing.T) {
	_, err := ParsePurl("pkg:npm/left-pad@1.3.0")
	if err == nil {
		t.Fatal("expected an error for a non-Maven purl, got nil")
	}
}

func TestParsePurl_RejectsUnparseablePurl(t *testing.T) {
	_, err := ParsePurl("not-a-purl-at-all")
	if err == nil {
		t.Fatal("expected an error for an unparseable purl, got nil")
	}
}
