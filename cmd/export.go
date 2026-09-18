package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/operator-framework/splat-viewer/pkg/graph"
	"github.com/operator-framework/splat-viewer/web"
)

var outputDir string

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export graph as self-contained HTML viewer",
	Long: `Export the catalog graph as a self-contained HTML file that can be
opened directly in a browser without a server. The graph data is embedded
inline in the HTML.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		name, catalogs, err := loadCatalogs(ctx)
		if err != nil {
			return err
		}

		g := graph.BuildGraph(name, catalogs, packageName, graphOpts())
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Graph built: %d nodes, %d links\n", len(g.Nodes), len(g.Links))

		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return fmt.Errorf("creating output directory: %w", err)
		}

		// Also write standalone JSON for programmatic use
		graphPath := filepath.Join(outputDir, "graph.json")
		graphFile, err := os.Create(graphPath)
		if err != nil {
			return fmt.Errorf("creating graph.json: %w", err)
		}
		defer func() { _ = graphFile.Close() }()
		enc := json.NewEncoder(graphFile)
		enc.SetIndent("", "  ")
		if err := enc.Encode(g); err != nil {
			return fmt.Errorf("encoding graph: %w", err)
		}

		// Embed JSON data into the HTML viewer
		jsonData, err := json.Marshal(g)
		if err != nil {
			return fmt.Errorf("marshaling graph: %w", err)
		}

		html := bytes.Replace(
			web.ViewerHTML,
			[]byte(`<div id="graph"></div>`),
			[]byte(fmt.Sprintf("<script id=\"graph-data\" type=\"application/json\">\n%s\n</script>\n<div id=\"graph\"></div>", jsonData)),
			1,
		)

		viewerPath := filepath.Join(outputDir, "viewer.html")
		if err := os.WriteFile(viewerPath, html, 0o644); err != nil {
			return fmt.Errorf("writing viewer.html: %w", err)
		}

		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Exported to %s:\n  %s (self-contained, open in browser)\n  %s (raw data)\n", outputDir, viewerPath, graphPath)
		return nil
	},
}

func init() {
	exportCmd.Flags().StringVar(&outputDir, "output-dir", "./splat-output", "output directory for exported files")
	rootCmd.AddCommand(exportCmd)
}
