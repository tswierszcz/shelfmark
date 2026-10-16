package catalog

import (
	"fmt"

	"github.com/package-url/packageurl-go"
)

// ParsePurl turns a purl string into the fields a libraries row needs. Per
// the Java/Maven-only NFR, only "maven"-type purls are accepted; any other
// purl type is a validation failure (returned as an error, not a panic),
// since the catalog has no other ecosystem to file it under.
func ParsePurl(raw string) (NewLibrary, error) {
	parsed, err := packageurl.FromString(raw)
	if err != nil {
		return NewLibrary{}, fmt.Errorf("catalog: failed to parse purl %q: %w", raw, err)
	}

	if parsed.Type != packageurl.TypeMaven {
		return NewLibrary{}, fmt.Errorf("catalog: unsupported purl type %q (only %q is supported): %q", parsed.Type, packageurl.TypeMaven, raw)
	}

	name := parsed.Name
	if parsed.Namespace != "" {
		name = parsed.Namespace + "/" + parsed.Name
	}

	return NewLibrary{
		Purl:      raw,
		Name:      name,
		Ecosystem: "maven",
	}, nil
}
