package graph

import (
	"encoding/json"
	"fmt"

	"github.com/blang/semver/v4"
	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/property"
)

const TypeMaxOpenShiftVersion = "olm.maxOpenShiftVersion"

type PackageDep struct {
	PackageName  string `json:"packageName"`
	VersionRange string `json:"versionRange"`
}

type GVKDep struct {
	Group   string `json:"group"`
	Kind    string `json:"kind"`
	Version string `json:"version"`
}

func ExtractVersion(props []property.Property) string {
	for _, p := range props {
		if p.Type == property.TypePackage {
			var pkg struct {
				Version string `json:"version"`
			}
			if err := json.Unmarshal(p.Value, &pkg); err == nil {
				return pkg.Version
			}
		}
	}
	return ""
}

func ExtractMaxOpenShiftVersion(props []property.Property) string {
	for _, p := range props {
		if p.Type == TypeMaxOpenShiftVersion {
			var v string
			if err := json.Unmarshal(p.Value, &v); err == nil {
				return v
			}
		}
	}
	return ""
}

func ExtractPackageDeps(props []property.Property) []PackageDep {
	var deps []PackageDep
	for _, p := range props {
		if p.Type == property.TypePackageRequired {
			var dep PackageDep
			if err := json.Unmarshal(p.Value, &dep); err == nil {
				deps = append(deps, dep)
			}
		}
	}
	return deps
}

func ExtractGVKDeps(props []property.Property) []GVKDep {
	var deps []GVKDep
	for _, p := range props {
		if p.Type == property.TypeGVKRequired {
			var dep GVKDep
			if err := json.Unmarshal(p.Value, &dep); err == nil {
				deps = append(deps, dep)
			}
		}
	}
	return deps
}

func buildVersionMap(cfg *declcfg.DeclarativeConfig) map[string]semver.Version {
	versions := make(map[string]semver.Version)
	for i := range cfg.Bundles {
		b := &cfg.Bundles[i]
		if _, ok := versions[b.Name]; ok {
			continue
		}
		props, err := property.Parse(b.Properties)
		if err != nil || len(props.Packages) != 1 {
			continue
		}
		v, err := semver.Parse(props.Packages[0].Version)
		if err != nil {
			fmt.Printf("warning: bundle %q has unparseable version %q\n", b.Name, props.Packages[0].Version)
			continue
		}
		versions[b.Name] = v
	}
	return versions
}
