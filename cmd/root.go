package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/splat-viewer/pkg/catalog"
	"github.com/operator-framework/splat-viewer/pkg/graph"
)

var (
	catalogDir         string
	catalogName        string
	releases           []string
	refs               []string
	packageName        string
	skipTLSVerify      bool
	skipRangeEdges     bool
)

var rootCmd = &cobra.Command{
	Use:   "splat-viewer",
	Short: "Interactive 3D visualization of OpenShift FBC catalogs",
	Long: `splat-viewer renders OpenShift File-Based Catalog (FBC) data as an
interactive 3D graph in your browser. The Z-axis represents OpenShift
release versions, showing how packages and their upgrade graphs evolve
across releases.

Input can be OCI pullspecs, local directories, or local YAML files.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&catalogDir, "catalog-dir", "", "base directory containing catalog data (used with --catalog/--releases)")
	rootCmd.PersistentFlags().StringVar(&catalogName, "catalog", "", "catalog name (used with --catalog-dir and --releases)")
	rootCmd.PersistentFlags().StringSliceVar(&releases, "releases", nil, "comma-separated catalog revisions (e.g., 4.17,5.0,stable)")
	rootCmd.PersistentFlags().StringSliceVar(&refs, "refs", nil, "OCI pullspecs, directories, or YAML files")
	rootCmd.PersistentFlags().StringVar(&packageName, "package", "", "filter to a single package across releases")
	rootCmd.PersistentFlags().BoolVar(&skipTLSVerify, "skip-tls-verify", false, "skip TLS verification for OCI registries")
	rootCmd.PersistentFlags().BoolVar(&skipRangeEdges, "skip-range-edges", false, "include skipRange edges (can be very large for full catalogs)")
}

func graphOpts() graph.BuildOptions {
	return graph.BuildOptions{
		IncludeSkipRangeEdges: skipRangeEdges,
	}
}

func loadCatalogs(ctx context.Context) (string, map[string]*declcfg.DeclarativeConfig, error) {
	if len(refs) > 0 {
		catalogs, err := catalog.LoadRefs(ctx, refs, skipTLSVerify)
		if err != nil {
			return "", nil, err
		}
		return inferCatalogName(refs), catalogs, nil
	}

	if catalogDir == "" || catalogName == "" {
		return "", nil, fmt.Errorf("specify input with --refs, or with --catalog-dir and --catalog (and optionally --releases)")
	}

	resolvedReleases, err := resolveReleases(catalogDir, catalogName)
	if err != nil {
		return "", nil, err
	}
	catalogs, err := catalog.LoadCatalogReleases(catalogDir, catalogName, resolvedReleases)
	if err != nil {
		return "", nil, err
	}
	return catalogName, catalogs, nil
}

func inferCatalogName(refs []string) string {
	if len(refs) == 0 {
		return "catalog"
	}
	ref := refs[0]
	// Extract image name from pullspec like registry.redhat.io/redhat/redhat-operator-index:v4.17
	parts := strings.Split(ref, "/")
	last := parts[len(parts)-1]
	if idx := strings.Index(last, ":"); idx > 0 {
		return last[:idx]
	}
	if idx := strings.Index(last, "@"); idx > 0 {
		return last[:idx]
	}
	return filepath.Base(ref)
}

func resolveReleases(baseDir, cat string) ([]string, error) {
	if len(releases) > 0 {
		return releases, nil
	}
	releaseDir := filepath.Join(baseDir, cat)
	entries, err := os.ReadDir(releaseDir)
	if err != nil {
		return nil, fmt.Errorf("reading catalog directory %s: %w", releaseDir, err)
	}
	var found []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// Local catalog caches use the immediate parent directory name as
		// the revision label. Verify the expected file before treating a
		// directory as a catalog revision, so unrelated directories are ignored.
		catalogFile := filepath.Join(releaseDir, e.Name(), fmt.Sprintf("%s-v%s.yaml", cat, e.Name()))
		if info, statErr := os.Stat(catalogFile); statErr == nil && !info.IsDir() {
			found = append(found, e.Name())
		}
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("no catalog revisions found in %s", releaseDir)
	}
	return found, nil
}
