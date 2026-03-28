package cmd

import (
	"context"
	"sort"

	"github.com/ScaleCommerce-DEV/scdev/internal/config"
	"github.com/ScaleCommerce-DEV/scdev/internal/project"
	"github.com/ScaleCommerce-DEV/scdev/internal/services"
	"github.com/ScaleCommerce-DEV/scdev/internal/state"
	"github.com/spf13/cobra"
)

var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Open documentation page",
	Long:  `Open the scdev documentation page in your default browser. This page provides quick reference and links to all shared services.`,
	RunE:  runDocs,
}

func init() {
	rootCmd.AddCommand(docsCmd)
}

func runDocs(cmd *cobra.Command, args []string) error {
	return openSharedServiceURL("router", "docs.shared",
		func(ctx context.Context, mgr *services.Manager) (*services.ServiceStatus, error) {
			return mgr.RouterStatus(ctx)
		},
	)
}

// updateDocsWithProjects updates the docs page with current project information
// This should be called after any project state change (start, stop, down)
func updateDocsWithProjects(ctx context.Context) {
	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		return
	}

	stateMgr, err := state.DefaultManager()
	if err != nil {
		return
	}

	projects, err := stateMgr.ListProjects()
	if err != nil {
		return
	}

	var projectInfos []config.ProjectInfo

	names := make([]string, 0, len(projects))
	for name := range projects {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		entry := projects[name]

		proj, err := project.LoadFromDir(entry.Path)
		if err != nil {
			continue
		}

		running := isProjectRunning(ctx, proj)

		projectInfos = append(projectInfos, config.ProjectInfo{
			Name:    name,
			Domain:  proj.Config.Domain,
			Path:    entry.Path,
			Running: running,
		})
	}

	_ = config.UpdateDocsProjects(cfg.Domain, cfg.SSL.Enabled, projectInfos)
}

// isProjectRunning checks if any container for the project is running
func isProjectRunning(ctx context.Context, proj *project.Project) bool {
	for serviceName := range proj.Config.Services {
		containerName := proj.ContainerName(serviceName)
		running, err := proj.Runtime.IsContainerRunning(ctx, containerName)
		if err == nil && running {
			return true
		}
	}
	return false
}
