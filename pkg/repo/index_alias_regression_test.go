package repo

import "testing"

func TestLoadIndexDuplicateDependencyAliasCompatibility(t *testing.T) {
	const indexData = `
apiVersion: v1
entries:
  example:
    - apiVersion: v2
      name: example
      version: 1.0.0
      urls:
        - https://example.invalid/example-1.0.0.tgz
      dependencies:
        - name: first
          version: 1.0.0
          repository: https://example.invalid/charts
          alias: shared
        - name: second
          version: 2.0.0
          repository: https://example.invalid/charts
          alias: shared
    - apiVersion: v2
      name: example
      version: 2.0.0
      urls:
        - https://example.invalid/example-2.0.0.tgz
`

	index, err := loadIndex([]byte(indexData), "compatibility-index")
	if err != nil {
		t.Fatalf("loadIndex returned an unexpected error: %v", err)
	}
	versions := index.Entries["example"]
	if len(versions) != 2 {
		t.Fatalf("expected both repository versions to remain available, got %d versions", len(versions))
	}
	seen := map[string]int{}
	for _, version := range versions {
		seen[version.Version]++
	}
	if seen["1.0.0"] != 1 || seen["2.0.0"] != 1 {
		t.Fatalf("expected versions 1.0.0 and 2.0.0 exactly once, got %#v", seen)
	}
}

func TestLoadIndexContinuesFilteringUnrelatedInvalidMetadata(t *testing.T) {
	const indexData = `
apiVersion: v1
entries:
  mixed:
    - apiVersion: v2
      name: mixed
      version: 1.0.0
      urls:
        - https://example.invalid/mixed-1.0.0.tgz
    - apiVersion: v2
      name: mixed
      version: not-semver
      urls:
        - https://example.invalid/mixed-invalid.tgz
    - apiVersion: v2
      name: mixed
      version: 2.0.0
      urls:
        - https://example.invalid/mixed-2.0.0.tgz
`

	index, err := loadIndex([]byte(indexData), "validation-index")
	if err != nil {
		t.Fatalf("loadIndex returned an unexpected error: %v", err)
	}
	if _, err := index.Get("mixed", "not-semver"); err == nil {
		t.Fatal("expected the unrelated invalid version to remain filtered")
	}
	for _, version := range []string{"1.0.0", "2.0.0"} {
		if _, err := index.Get("mixed", version); err != nil {
			t.Fatalf("expected valid version %s to remain available: %v", version, err)
		}
	}
}

func TestLoadIndexPreservesDependenciesUnderToleratedAliasConflict(t *testing.T) {
	const indexData = `
apiVersion: v1
entries:
  example:
    - apiVersion: v2
      name: example
      version: 1.0.0
      urls:
        - https://example.invalid/example-1.0.0.tgz
      dependencies:
        - name: first
          version: 1.0.0
          repository: https://example.invalid/charts
          alias: shared
        - name: second
          version: 2.0.0
          repository: https://example.invalid/charts
          alias: shared
`

	index, err := loadIndex([]byte(indexData), "deps-index")
	if err != nil {
		t.Fatalf("loadIndex returned an unexpected error: %v", err)
	}
	cv, err := index.Get("example", "1.0.0")
	if err != nil {
		t.Fatalf("expected the tolerated version to remain available: %v", err)
	}
	if len(cv.Metadata.Dependencies) != 2 {
		t.Fatalf("expected both dependencies to be preserved, got %d", len(cv.Metadata.Dependencies))
	}
	for _, want := range []struct{ name, version, alias string }{
		{"first", "1.0.0", "shared"},
		{"second", "2.0.0", "shared"},
	} {
		var found *struct{ name, version, alias string }
		for _, d := range cv.Metadata.Dependencies {
			if d.Name == want.name {
				found = &struct{ name, version, alias string }{d.Name, d.Version, d.Alias}
			}
		}
		if found == nil {
			t.Errorf("expected dependency %q to be preserved, got %#v", want.name, cv.Metadata.Dependencies)
			continue
		}
		if found.version != want.version || found.alias != want.alias {
			t.Errorf("dependency %q: expected version=%q alias=%q, got version=%q alias=%q",
				want.name, want.version, want.alias, found.version, found.alias)
		}
	}
}

func TestLoadIndexStillRejectsInvalidDependencyAliasCharacters(t *testing.T) {
	const indexData = `
apiVersion: v1
entries:
  example:
    - apiVersion: v2
      name: example
      version: 1.0.0
      urls:
        - https://example.invalid/example-1.0.0.tgz
      dependencies:
        - name: bad
          version: 1.0.0
          repository: https://example.invalid/charts
          alias: "illegal alias"
    - apiVersion: v2
      name: example
      version: 2.0.0
      urls:
        - https://example.invalid/example-2.0.0.tgz
`

	index, err := loadIndex([]byte(indexData), "badalias-index")
	if err != nil {
		t.Fatalf("loadIndex returned an unexpected error: %v", err)
	}
	if _, err := index.Get("example", "1.0.0"); err == nil {
		t.Fatal("expected the entry with disallowed alias characters to be filtered, but it was retained")
	}
	if _, err := index.Get("example", "2.0.0"); err != nil {
		t.Fatalf("expected the valid version to remain available: %v", err)
	}
}
