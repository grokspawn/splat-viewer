package catalog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/operator-framework/operator-registry/alpha/action"
	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/pkg/image"
	"github.com/operator-framework/operator-registry/pkg/image/containersimageregistry"
)

func LoadCatalog(path string) (*declcfg.DeclarativeConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening catalog file %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	cfg, err := declcfg.LoadReader(f)
	if err != nil {
		return nil, fmt.Errorf("parsing catalog %s: %w", path, err)
	}
	return cfg, nil
}

func LoadCatalogReleases(baseDir, catalogName string, releases []string) (map[string]*declcfg.DeclarativeConfig, error) {
	sort.Strings(releases)

	type result struct {
		release string
		cfg     *declcfg.DeclarativeConfig
		err     error
	}

	results := make(chan result, len(releases))
	var wg sync.WaitGroup

	for _, release := range releases {
		wg.Add(1)
		go func(rel string) {
			defer wg.Done()
			filename := fmt.Sprintf("%s-v%s.yaml", catalogName, rel)
			path := filepath.Join(baseDir, catalogName, rel, filename)
			cfg, err := LoadCatalog(path)
			if err != nil {
				results <- result{rel, nil, fmt.Errorf("loading release %s: %w", rel, err)}
				return
			}
			fmt.Fprintf(os.Stderr, "Loaded %s: %d packages, %d channels, %d bundles\n",
				rel, len(cfg.Packages), len(cfg.Channels), len(cfg.Bundles))
			results <- result{rel, cfg, nil}
		}(release)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	catalogs := make(map[string]*declcfg.DeclarativeConfig, len(releases))
	for r := range results {
		if r.err != nil {
			return nil, r.err
		}
		catalogs[r.release] = r.cfg
	}
	return catalogs, nil
}

func LoadRef(ctx context.Context, ref string, skipTLSVerify bool) (*declcfg.DeclarativeConfig, error) {
	if stat, err := os.Stat(ref); err == nil && !stat.IsDir() {
		return LoadCatalog(ref)
	}

	reg, err := containersimageregistry.New(
		containersimageregistry.DefaultSystemContext,
		containersimageregistry.WithInsecureSkipTLSVerify(skipTLSVerify),
	)
	if err != nil {
		return nil, fmt.Errorf("creating image registry: %w", err)
	}
	defer func() { _ = reg.Destroy() }()

	return renderRef(ctx, ref, reg)
}

func renderRef(ctx context.Context, ref string, reg image.Registry) (*declcfg.DeclarativeConfig, error) {
	render := action.Render{
		Refs:           []string{ref},
		Registry:       reg,
		AllowedRefMask: action.RefDCImage | action.RefDCDir,
	}

	cfg, err := render.Run(ctx)
	if err != nil {
		return nil, fmt.Errorf("rendering ref %q: %w", ref, err)
	}

	fmt.Fprintf(os.Stderr, "Loaded %s: %d packages, %d channels, %d bundles\n",
		ref, len(cfg.Packages), len(cfg.Channels), len(cfg.Bundles))
	return cfg, nil
}

func LoadRefs(ctx context.Context, refs []string, skipTLSVerify bool) (map[string]*declcfg.DeclarativeConfig, error) {
	// Check if any refs need OCI pulling
	needsRegistry := false
	for _, ref := range refs {
		if stat, err := os.Stat(ref); err != nil || stat.IsDir() {
			needsRegistry = true
			break
		}
	}

	// Share a single registry across all OCI refs for layer caching
	var reg image.Registry
	if needsRegistry {
		var err error
		reg, err = containersimageregistry.New(
			containersimageregistry.DefaultSystemContext,
			containersimageregistry.WithInsecureSkipTLSVerify(skipTLSVerify),
		)
		if err != nil {
			return nil, fmt.Errorf("creating image registry: %w", err)
		}
		defer func() { _ = reg.Destroy() }()
	}

	result := make(map[string]*declcfg.DeclarativeConfig, len(refs))
	for _, ref := range refs {
		var cfg *declcfg.DeclarativeConfig
		var err error

		if stat, statErr := os.Stat(ref); statErr == nil && !stat.IsDir() {
			cfg, err = LoadCatalog(ref)
		} else {
			cfg, err = renderRef(ctx, ref, reg)
		}
		if err != nil {
			return nil, err
		}
		result[extractLabel(ref)] = cfg
	}
	return result, nil
}

func extractLabel(ref string) string {
	for i := len(ref) - 1; i >= 0; i-- {
		if ref[i] == ':' {
			tag := ref[i+1:]
			if len(tag) > 1 && tag[0] == 'v' {
				return tag[1:]
			}
			return tag
		}
		if ref[i] == '/' || ref[i] == '@' {
			break
		}
	}
	return filepath.Base(ref)
}
