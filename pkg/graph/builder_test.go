package graph

import (
	"encoding/json"
	"testing"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/property"
)

func bundleProps(pkg, version string) []property.Property {
	return []property.Property{
		{Type: "olm.package", Value: json.RawMessage(`{"packageName":"` + pkg + `","version":"` + version + `"}`)},
	}
}

func testCatalog() *declcfg.DeclarativeConfig {
	return &declcfg.DeclarativeConfig{
		Packages: []declcfg.Package{
			{Schema: "olm.package", Name: "alpha-op", DefaultChannel: "stable"},
			{Schema: "olm.package", Name: "beta-op", DefaultChannel: "fast"},
		},
		Channels: []declcfg.Channel{
			{
				Schema:  "olm.channel",
				Name:    "stable",
				Package: "alpha-op",
				Entries: []declcfg.ChannelEntry{
					{Name: "alpha-op.v1.0.0"},
					{Name: "alpha-op.v1.1.0", Replaces: "alpha-op.v1.0.0"},
					{Name: "alpha-op.v2.0.0", Replaces: "alpha-op.v1.1.0", Skips: []string{"alpha-op.v1.0.0"}},
				},
			},
			{
				Schema:  "olm.channel",
				Name:    "fast",
				Package: "beta-op",
				Entries: []declcfg.ChannelEntry{
					{Name: "beta-op.v0.1.0"},
					{Name: "beta-op.v0.2.0", Replaces: "beta-op.v0.1.0"},
				},
			},
		},
		Bundles: []declcfg.Bundle{
			{Schema: "olm.bundle", Name: "alpha-op.v1.0.0", Package: "alpha-op", Properties: bundleProps("alpha-op", "1.0.0")},
			{Schema: "olm.bundle", Name: "alpha-op.v1.1.0", Package: "alpha-op", Properties: bundleProps("alpha-op", "1.1.0")},
			{Schema: "olm.bundle", Name: "alpha-op.v2.0.0", Package: "alpha-op", Properties: bundleProps("alpha-op", "2.0.0")},
			{Schema: "olm.bundle", Name: "beta-op.v0.1.0", Package: "beta-op", Properties: bundleProps("beta-op", "0.1.0")},
			{Schema: "olm.bundle", Name: "beta-op.v0.2.0", Package: "beta-op", Properties: bundleProps("beta-op", "0.2.0")},
		},
	}
}

func TestBuildGraphNodeCount(t *testing.T) {
	catalogs := map[string]*declcfg.DeclarativeConfig{"4.17": testCatalog()}
	g := BuildGraph("test", catalogs, "", BuildOptions{})
	if len(g.Nodes) != 5 {
		t.Errorf("got %d nodes, want 5", len(g.Nodes))
	}
}

func TestBuildGraphPackageFilter(t *testing.T) {
	catalogs := map[string]*declcfg.DeclarativeConfig{"4.17": testCatalog()}
	g := BuildGraph("test", catalogs, "alpha-op", BuildOptions{})
	for _, n := range g.Nodes {
		if n.Package != "alpha-op" {
			t.Errorf("node %q has package %q, want alpha-op", n.ID, n.Package)
		}
	}
	if len(g.Nodes) != 3 {
		t.Errorf("got %d nodes, want 3", len(g.Nodes))
	}
}

func TestBuildGraphChannelHeads(t *testing.T) {
	catalogs := map[string]*declcfg.DeclarativeConfig{"4.17": testCatalog()}
	g := BuildGraph("test", catalogs, "", BuildOptions{})
	heads := map[string]bool{}
	for _, n := range g.Nodes {
		if n.IsChannelHead {
			heads[n.BundleName] = true
		}
	}
	if !heads["alpha-op.v2.0.0"] {
		t.Error("alpha-op.v2.0.0 should be channel head")
	}
	if !heads["beta-op.v0.2.0"] {
		t.Error("beta-op.v0.2.0 should be channel head")
	}
	if len(heads) != 2 {
		t.Errorf("got %d heads, want 2", len(heads))
	}
}

func TestBuildGraphSkippedStatus(t *testing.T) {
	catalogs := map[string]*declcfg.DeclarativeConfig{"4.17": testCatalog()}
	g := BuildGraph("test", catalogs, "", BuildOptions{})
	for _, n := range g.Nodes {
		if n.BundleName == "alpha-op.v1.0.0" && !n.Skipped {
			t.Error("alpha-op.v1.0.0 should be marked skipped (in v2.0.0's skips list)")
		}
	}
}

func TestBuildGraphLinkTypes(t *testing.T) {
	catalogs := map[string]*declcfg.DeclarativeConfig{"4.17": testCatalog()}
	g := BuildGraph("test", catalogs, "", BuildOptions{})
	types := map[string]int{}
	for _, l := range g.Links {
		types[l.Type]++
	}
	if types["replaces"] != 3 {
		t.Errorf("got %d replaces links, want 3", types["replaces"])
	}
	if types["skips"] != 1 {
		t.Errorf("got %d skips links, want 1", types["skips"])
	}
}

func TestComputeLayoutYConsistency(t *testing.T) {
	cfg := testCatalog()
	catalogs := map[string]*declcfg.DeclarativeConfig{"4.17": cfg, "4.20": cfg}
	g := BuildGraph("test", catalogs, "", BuildOptions{})

	yByPkg := map[string]float64{}
	for _, n := range g.Nodes {
		if prev, ok := yByPkg[n.Package]; ok {
			if n.FY != prev {
				t.Errorf("package %q has inconsistent Y: %f vs %f", n.Package, n.FY, prev)
			}
		}
		yByPkg[n.Package] = n.FY
	}

	if yByPkg["alpha-op"] == yByPkg["beta-op"] {
		t.Error("different packages should have different Y positions")
	}
}

func TestComputeLayoutXOrdering(t *testing.T) {
	catalogs := map[string]*declcfg.DeclarativeConfig{"4.17": testCatalog()}
	g := BuildGraph("test", catalogs, "alpha-op", BuildOptions{})

	nodeByName := map[string]Node{}
	for _, n := range g.Nodes {
		nodeByName[n.BundleName] = n
	}

	v1 := nodeByName["alpha-op.v1.0.0"]
	v11 := nodeByName["alpha-op.v1.1.0"]
	v2 := nodeByName["alpha-op.v2.0.0"]

	if v1.FX >= v11.FX {
		t.Errorf("v1.0.0 (fx=%f) should be left of v1.1.0 (fx=%f)", v1.FX, v11.FX)
	}
	if v11.FX >= v2.FX {
		t.Errorf("v1.1.0 (fx=%f) should be left of v2.0.0 (fx=%f)", v11.FX, v2.FX)
	}
}
