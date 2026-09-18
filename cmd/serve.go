package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/operator-framework/splat-viewer/pkg/graph"
	"github.com/operator-framework/splat-viewer/pkg/server"
)

var (
	port      int
	noBrowser bool
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Build graph and serve interactive 3D viewer",
	Long: `Load FBC catalog data for the specified releases, build a cross-release
upgrade graph, and serve an interactive 3D visualization in your browser.`,
	Example: `  # Local catalog files by convention
  splat-viewer serve --catalog redhat-operator-index --releases 4.17,4.18,4.19,4.20

  # Single package detail view
  splat-viewer serve --catalog redhat-operator-index --releases 4.17,4.20 --package 3scale-operator

  # OCI pullspecs
  splat-viewer serve --refs registry.redhat.io/redhat/redhat-operator-index:v4.17,registry.redhat.io/redhat/redhat-operator-index:v4.18

  # Mix of local files and directories
  splat-viewer serve --refs /path/to/catalog.yaml,/path/to/catalog-dir`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		name, catalogs, err := loadCatalogs(ctx)
		if err != nil {
			return err
		}

		g := graph.BuildGraph(name, catalogs, packageName, graphOpts())
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Graph built: %d nodes, %d links\n", len(g.Nodes), len(g.Links))

		return server.Serve(g, port, !noBrowser)
	},
}

func init() {
	serveCmd.Flags().IntVar(&port, "port", 8080, "port to serve on")
	serveCmd.Flags().BoolVar(&noBrowser, "no-browser", false, "don't open browser automatically")
	rootCmd.AddCommand(serveCmd)
}
