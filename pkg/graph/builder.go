package graph

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/blang/semver/v4"
	"github.com/operator-framework/operator-registry/alpha/declcfg"
)

const zSpacing = 200.0

func BuildGraph(catalogName string, catalogs map[string]*declcfg.DeclarativeConfig, packageFilter string, opts BuildOptions) *Graph {
	releases := make([]string, 0, len(catalogs))
	for r := range catalogs {
		releases = append(releases, r)
	}
	sort.Strings(releases)

	g := &Graph{
		Catalog:   catalogName,
		Releases:  releases,
		Generated: time.Now().UTC().Format(time.RFC3339),
	}

	for i, release := range releases {
		cfg := catalogs[release]
		zPos := float64(i) * zSpacing
		addRelease(g, cfg, release, zPos, packageFilter, opts)
	}

	// Add cross-release continuity edges: connect the same package across
	// adjacent Z-planes. For each package present in consecutive releases,
	// link the channel head of release N to the channel head of release N+1.
	if len(releases) > 1 {
		addCrossReleaseEdges(g, catalogs, releases, packageFilter)
	}

	// Create phantom nodes for missing upgrade edge endpoints. In upgrade
	// links (replaces/skips/skipRange), a missing node means the bundle
	// existed in a prior catalog but isn't in this one — create a phantom.
	// For non-upgrade links (packageDep/gvkDep), drop if either end is missing.
	nodeIDs := make(map[string]bool, len(g.Nodes))
	for _, n := range g.Nodes {
		nodeIDs[n.ID] = true
	}

	isUpgradeEdge := func(linkType string) bool {
		return linkType == "replaces" || linkType == "skips" || linkType == "skipRange" || linkType == "crossRelease"
	}

	phantoms := make(map[string]bool)
	createPhantom := func(missingID, peerID string) {
		if phantoms[missingID] {
			return
		}
		phantoms[missingID] = true
		bundleName, release := parseNodeID(missingID)
		pkg := ""
		fz := 0.0
		for _, n := range g.Nodes {
			if n.ID == peerID {
				pkg = n.Package
				fz = n.FZ
				break
			}
		}
		g.Nodes = append(g.Nodes, Node{
			ID:         missingID,
			Package:    pkg,
			BundleName: bundleName,
			Release:    release,
			Group:      pkg,
			FZ:         fz,
			Phantom:    true,
		})
		nodeIDs[missingID] = true
	}

	filtered := g.Links[:0]
	for _, l := range g.Links {
		srcExists := nodeIDs[l.Source]
		tgtExists := nodeIDs[l.Target]

		if srcExists && tgtExists {
			filtered = append(filtered, l)
			continue
		}

		if !isUpgradeEdge(l.Type) {
			continue
		}

		if !srcExists && tgtExists {
			createPhantom(l.Source, l.Target)
		} else if srcExists && !tgtExists {
			createPhantom(l.Target, l.Source)
		} else {
			continue
		}
		filtered = append(filtered, l)
	}
	g.Links = filtered

	computeLayout(g)

	return g
}

const (
	xSpacing = 30.0
	ySpacing = 40.0
)

func computeLayout(g *Graph) {
	// 1. Assign Y: each unique package gets a fixed row, sorted alphabetically
	pkgSet := make(map[string]bool)
	for i := range g.Nodes {
		pkgSet[g.Nodes[i].Package] = true
	}
	pkgNames := make([]string, 0, len(pkgSet))
	for p := range pkgSet {
		pkgNames = append(pkgNames, p)
	}
	sort.Strings(pkgNames)

	packageY := make(map[string]float64, len(pkgNames))
	for i, p := range pkgNames {
		packageY[p] = float64(i) * ySpacing
	}

	// 2. Assign X: within each (package, release), sort bundles by semver, oldest left
	type prKey struct{ pkg, release string }
	groups := make(map[prKey][]*Node)
	for i := range g.Nodes {
		n := &g.Nodes[i]
		k := prKey{n.Package, n.Release}
		groups[k] = append(groups[k], n)
	}

	versionMap := buildVersionMapFromNodes(g.Nodes)

	for _, nodes := range groups {
		sort.Slice(nodes, func(i, j int) bool {
			vi, oki := versionMap[nodes[i].BundleName]
			vj, okj := versionMap[nodes[j].BundleName]
			if oki && okj {
				return vi.LT(vj)
			}
			return nodes[i].BundleName < nodes[j].BundleName
		})

		for i, n := range nodes {
			if n.Phantom {
				n.FX = -xSpacing
			} else {
				n.FX = float64(i) * xSpacing
			}
		}
	}

	// 3. Set Y from package map
	for i := range g.Nodes {
		g.Nodes[i].FY = packageY[g.Nodes[i].Package]
	}
}

func buildVersionMapFromNodes(nodes []Node) map[string]semver.Version {
	versions := make(map[string]semver.Version)
	for _, n := range nodes {
		if n.Version == "" {
			continue
		}
		v, err := semver.Parse(n.Version)
		if err != nil {
			continue
		}
		versions[n.BundleName] = v
	}
	return versions
}

func addRelease(g *Graph, cfg *declcfg.DeclarativeConfig, release string, zPos float64, packageFilter string, opts BuildOptions) {
	deprecatedBundles, deprecatedPackages, deprecationMessages := buildDeprecationIndex(cfg)
	bundleChannels := buildBundleChannelIndex(cfg)
	channelHeads := buildChannelHeadIndex(cfg)
	versionMap := buildVersionMap(cfg)

	// Track which bundles are "skipped" (v0 semantics: superseded by skip/skipRange)
	skippedBundles := make(map[string]bool)
	for _, ch := range cfg.Channels {
		if packageFilter != "" && ch.Package != packageFilter {
			continue
		}
		for _, entry := range ch.Entries {
			for _, skip := range entry.Skips {
				skippedBundles[skip] = true
			}
			if entry.SkipRange != "" {
				skipRange, err := semver.ParseRange(entry.SkipRange)
				if err != nil {
					continue
				}
				for _, other := range ch.Entries {
					if other.Name == entry.Name {
						continue
					}
					if v, ok := versionMap[other.Name]; ok && skipRange(v) {
						skippedBundles[other.Name] = true
					}
				}
			}
		}
	}

	for _, bundle := range cfg.Bundles {
		if packageFilter != "" && bundle.Package != packageFilter {
			continue
		}

		nid := nodeID(bundle.Name, release)
		version := ExtractVersion(bundle.Properties)
		maxOCP := ExtractMaxOpenShiftVersion(bundle.Properties)

		channelKeys := bundleChannels[bundle.Name]
		isHead := false
		var channelNames []string
		for _, chKey := range channelKeys {
			if channelHeads[chKey] == bundle.Name {
				isHead = true
			}
			if idx := len(bundle.Package) + 1; idx < len(chKey) {
				channelNames = append(channelNames, chKey[idx:])
			} else {
				channelNames = append(channelNames, chKey)
			}
		}

		deprecated := deprecatedBundles[bundle.Name] || deprecatedPackages[bundle.Package]
		depMsg := deprecationMessages[bundle.Name]
		if depMsg == "" {
			depMsg = deprecationMessages[bundle.Package]
		}

		node := Node{
			ID:                  nid,
			Package:             bundle.Package,
			BundleName:          bundle.Name,
			Version:             version,
			Release:             release,
			Channels:            channelNames,
			IsChannelHead:       isHead,
			Skipped:             skippedBundles[bundle.Name],
			Deprecated:          deprecated,
			DeprecationMessage:  depMsg,
			MaxOpenShiftVersion: maxOCP,
			Group:               bundle.Package,
			FZ:                  zPos,
		}
		g.Nodes = append(g.Nodes, node)

		for _, dep := range ExtractPackageDeps(bundle.Properties) {
			if packageFilter != "" && dep.PackageName != packageFilter && bundle.Package != packageFilter {
				continue
			}
			// Find the channel head of the target package in this release
			// to use as the link target
			depTarget := findPackageHead(cfg, channelHeads, dep.PackageName, release)
			if depTarget == "" {
				continue
			}
			g.Links = append(g.Links, Link{
				Source: nid,
				Target: depTarget,
				Type:   "packageDep",
				Label:  fmt.Sprintf("requires %s %s", dep.PackageName, dep.VersionRange),
			})
		}
	}

	// Build upgrade edges per channel, following opm's processing order:
	// skips first, then skipRange, then replaces last.
	for _, ch := range cfg.Channels {
		if packageFilter != "" && ch.Package != packageFilter {
			continue
		}

		// Sort entries by decreasing version (matching opm behavior)
		sortedEntries := make([]declcfg.ChannelEntry, len(ch.Entries))
		copy(sortedEntries, ch.Entries)
		sort.Slice(sortedEntries, func(i, j int) bool {
			vi, oki := versionMap[sortedEntries[i].Name]
			vj, okj := versionMap[sortedEntries[j].Name]
			if !oki || !okj {
				return sortedEntries[i].Name > sortedEntries[j].Name
			}
			return vi.GT(vj)
		})

		for _, entry := range sortedEntries {
			src := nodeID(entry.Name, release)

			// 1. Skips
			for _, skip := range entry.Skips {
				g.Links = append(g.Links, Link{
					Source:  src,
					Target:  nodeID(skip, release),
					Type:    "skips",
					Channel: ch.Name,
				})
			}

			// 2. SkipRange — resolve to concrete edges (optional, can be very large)
			if opts.IncludeSkipRangeEdges && entry.SkipRange != "" {
				skipRange, err := semver.ParseRange(entry.SkipRange)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: invalid skipRange for %s/%s: %v\n", ch.Package, entry.Name, err)
					continue
				}
				for _, other := range ch.Entries {
					if other.Name == entry.Name {
						continue
					}
					if v, ok := versionMap[other.Name]; ok && skipRange(v) {
						g.Links = append(g.Links, Link{
							Source:  nodeID(other.Name, release),
							Target:  src,
							Type:    "skipRange",
							Channel: ch.Name,
							Label:   entry.SkipRange,
						})
					}
				}
			}

			// 3. Replaces (last, because applicability can be impacted by skips)
			if entry.Replaces != "" {
				g.Links = append(g.Links, Link{
					Source:  src,
					Target:  nodeID(entry.Replaces, release),
					Type:    "replaces",
					Channel: ch.Name,
				})
			}
		}
	}
}

func addCrossReleaseEdges(g *Graph, catalogs map[string]*declcfg.DeclarativeConfig, releases []string, packageFilter string) {
	for i := 0; i < len(releases)-1; i++ {
		thisRelease := releases[i]
		nextRelease := releases[i+1]
		thisCfg := catalogs[thisRelease]
		nextCfg := catalogs[nextRelease]

		thisHeads := buildChannelHeadIndex(thisCfg)
		nextHeads := buildChannelHeadIndex(nextCfg)

		// Connect channel heads from the same channel across releases
		for chKey, thisHead := range thisHeads {
			pkg, _ := parseChannelKey(chKey)
			if packageFilter != "" && pkg != packageFilter {
				continue
			}
			if nextHead, ok := nextHeads[chKey]; ok {
				g.Links = append(g.Links, Link{
					Source: nodeID(thisHead, thisRelease),
					Target: nodeID(nextHead, nextRelease),
					Type:   "crossRelease",
					Label:  fmt.Sprintf("%s → %s", thisRelease, nextRelease),
				})
			}
		}
	}
}

func nodeID(bundleName, release string) string {
	return bundleName + "@" + release
}

func parseNodeID(id string) (bundleName, release string) {
	for i := len(id) - 1; i >= 0; i-- {
		if id[i] == '@' {
			return id[:i], id[i+1:]
		}
	}
	return id, ""
}

func findPackageHead(cfg *declcfg.DeclarativeConfig, channelHeads map[string]string, pkgName, release string) string {
	// Find the default channel for the package
	var defaultChannel string
	for _, p := range cfg.Packages {
		if p.Name == pkgName {
			defaultChannel = p.DefaultChannel
			break
		}
	}
	if defaultChannel != "" {
		key := channelKey(pkgName, defaultChannel)
		if head, ok := channelHeads[key]; ok {
			return nodeID(head, release)
		}
	}
	// Fall back to any channel head for this package
	for key, head := range channelHeads {
		pkg, _ := parseChannelKey(key)
		if pkg == pkgName {
			return nodeID(head, release)
		}
	}
	return ""
}

func parseChannelKey(key string) (pkg, chName string) {
	for i := range key {
		if key[i] == '/' {
			return key[:i], key[i+1:]
		}
	}
	return key, ""
}

func buildDeprecationIndex(cfg *declcfg.DeclarativeConfig) (bundles map[string]bool, packages map[string]bool, messages map[string]string) {
	bundles = make(map[string]bool)
	packages = make(map[string]bool)
	messages = make(map[string]string)

	for _, dep := range cfg.Deprecations {
		for _, entry := range dep.Entries {
			switch entry.Reference.Schema {
			case declcfg.SchemaPackage:
				packages[dep.Package] = true
				messages[dep.Package] = entry.Message
			case declcfg.SchemaBundle:
				bundles[entry.Reference.Name] = true
				messages[entry.Reference.Name] = entry.Message
			}
		}
	}
	return
}

func channelKey(pkg, chName string) string {
	return pkg + "/" + chName
}

func buildBundleChannelIndex(cfg *declcfg.DeclarativeConfig) map[string][]string {
	index := make(map[string][]string)
	for _, ch := range cfg.Channels {
		for _, entry := range ch.Entries {
			index[entry.Name] = append(index[entry.Name], channelKey(ch.Package, ch.Name))
		}
	}
	return index
}

func buildChannelHeadIndex(cfg *declcfg.DeclarativeConfig) map[string]string {
	heads := make(map[string]string)
	for _, ch := range cfg.Channels {
		superseded := make(map[string]bool)
		for _, entry := range ch.Entries {
			if entry.Replaces != "" {
				superseded[entry.Replaces] = true
			}
			for _, skip := range entry.Skips {
				superseded[skip] = true
			}
		}
		key := channelKey(ch.Package, ch.Name)
		for _, entry := range ch.Entries {
			if !superseded[entry.Name] {
				heads[key] = entry.Name
				break
			}
		}
	}
	return heads
}
