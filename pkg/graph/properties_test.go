package graph

import (
	"encoding/json"
	"testing"

	"github.com/operator-framework/operator-registry/alpha/property"
)

func TestExtractVersion(t *testing.T) {
	props := []property.Property{
		{Type: "olm.gvk", Value: json.RawMessage(`{"group":"apps","version":"v1","kind":"Foo"}`)},
		{Type: "olm.package", Value: json.RawMessage(`{"packageName":"test-operator","version":"1.2.3"}`)},
	}
	got := ExtractVersion(props)
	if got != "1.2.3" {
		t.Errorf("ExtractVersion = %q, want %q", got, "1.2.3")
	}
}

func TestExtractVersionMissing(t *testing.T) {
	props := []property.Property{
		{Type: "olm.gvk", Value: json.RawMessage(`{"group":"apps","version":"v1","kind":"Foo"}`)},
	}
	got := ExtractVersion(props)
	if got != "" {
		t.Errorf("ExtractVersion = %q, want empty", got)
	}
}

func TestExtractMaxOpenShiftVersion(t *testing.T) {
	props := []property.Property{
		{Type: "olm.maxOpenShiftVersion", Value: json.RawMessage(`"4.17"`)},
	}
	got := ExtractMaxOpenShiftVersion(props)
	if got != "4.17" {
		t.Errorf("ExtractMaxOpenShiftVersion = %q, want %q", got, "4.17")
	}
}

func TestExtractPackageDeps(t *testing.T) {
	props := []property.Property{
		{Type: "olm.package.required", Value: json.RawMessage(`{"packageName":"dep-operator","versionRange":">=1.0.0"}`)},
		{Type: "olm.package.required", Value: json.RawMessage(`{"packageName":"other-dep","versionRange":">=2.0.0"}`)},
	}
	deps := ExtractPackageDeps(props)
	if len(deps) != 2 {
		t.Fatalf("ExtractPackageDeps returned %d deps, want 2", len(deps))
	}
	if deps[0].PackageName != "dep-operator" {
		t.Errorf("deps[0].PackageName = %q, want %q", deps[0].PackageName, "dep-operator")
	}
}
