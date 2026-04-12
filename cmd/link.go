package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ScaleCommerce-DEV/scdev/internal/config"
	"github.com/ScaleCommerce-DEV/scdev/internal/project"
	runtimePkg "github.com/ScaleCommerce-DEV/scdev/internal/runtime"
	"github.com/ScaleCommerce-DEV/scdev/internal/state"
	"github.com/spf13/cobra"
)

var linkCmd = &cobra.Command{
	Use:   "link",
	Short: "Manage link networks between projects",
	Long:  `Create named link networks to enable direct container-to-container communication between projects.`,
}

var linkCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a named link network",
	Args:  cobra.ExactArgs(1),
	RunE:  runLinkCreate,
}

var linkDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a link network and disconnect all members",
	Args:  cobra.ExactArgs(1),
	RunE:  runLinkDelete,
}

var linkJoinCmd = &cobra.Command{
	Use:   "join <name> <member> [<member>...]",
	Short: "Add members to a link network",
	Long: `Add projects or individual services to a link network.

Members can be:
  myproject       - all services from a project
  myproject.app   - only the "app" service from a project`,
	Args: cobra.MinimumNArgs(2),
	RunE: runLinkJoin,
}

var linkLeaveCmd = &cobra.Command{
	Use:   "leave <name> <member> [<member>...]",
	Short: "Remove members from a link network",
	Args:  cobra.MinimumNArgs(2),
	RunE:  runLinkLeave,
}

var linkLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List all link networks",
	Args:  cobra.NoArgs,
	RunE:  runLinkLs,
}

var linkStatusCmd = &cobra.Command{
	Use:   "status <name>",
	Short: "Show members and connection state of a link network",
	Args:  cobra.ExactArgs(1),
	RunE:  runLinkStatus,
}

func init() {
	linkCmd.AddCommand(linkCreateCmd)
	linkCmd.AddCommand(linkDeleteCmd)
	linkCmd.AddCommand(linkJoinCmd)
	linkCmd.AddCommand(linkLeaveCmd)
	linkCmd.AddCommand(linkLsCmd)
	linkCmd.AddCommand(linkStatusCmd)
	rootCmd.AddCommand(linkCmd)
}

func runLinkCreate(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := requireDocker(ctx); err != nil {
		return err
	}

	name := args[0]

	if err := state.ValidateLinkName(name); err != nil {
		return err
	}

	stateMgr, err := state.DefaultManager()
	if err != nil {
		return err
	}

	if err := stateMgr.CreateLink(name); err != nil {
		return err
	}

	// Create the Docker network (check if it already exists first)
	docker := runtimePkg.NewDockerCLI()
	networkName := state.LinkNetworkName(name)
	networkExists, err := docker.NetworkExists(ctx, networkName)
	if err != nil {
		_ = stateMgr.DeleteLink(name)
		return fmt.Errorf("failed to check network %s: %w", networkName, err)
	}

	if !networkExists {
		if err := docker.CreateNetwork(ctx, networkName); err != nil {
			_ = stateMgr.DeleteLink(name)
			return fmt.Errorf("failed to create network %s: %w", networkName, err)
		}
	}

	fmt.Printf("Created link %q (network: %s)\n", name, networkName)
	return nil
}

func runLinkDelete(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := requireDocker(ctx); err != nil {
		return err
	}

	name := args[0]

	stateMgr, err := state.DefaultManager()
	if err != nil {
		return err
	}

	link, err := stateMgr.GetLink(name)
	if err != nil {
		return err
	}
	if link == nil {
		return fmt.Errorf("link %q does not exist", name)
	}

	docker := runtimePkg.NewDockerCLI()

	// Disconnect all members before removing the network
	disconnectLinkMembers(ctx, docker, link, stateMgr)

	// Remove the Docker network
	networkExists, err := docker.NetworkExists(ctx, link.Network)
	if err == nil && networkExists {
		if err := docker.RemoveNetwork(ctx, link.Network); err != nil {
			fmt.Printf("Warning: failed to remove network %s: %v\n", link.Network, err)
		}
	}

	if err := stateMgr.DeleteLink(name); err != nil {
		return err
	}

	fmt.Printf("Deleted link %q\n", name)
	return nil
}

func runLinkJoin(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := requireDocker(ctx); err != nil {
		return err
	}

	name := args[0]
	memberArgs := args[1:]

	stateMgr, err := state.DefaultManager()
	if err != nil {
		return err
	}

	link, err := stateMgr.GetLink(name)
	if err != nil {
		return err
	}
	if link == nil {
		return fmt.Errorf("link %q does not exist - create it first with: scdev link create %s", name, name)
	}

	// Parse and validate members
	members := make([]state.LinkMember, 0, len(memberArgs))
	for _, arg := range memberArgs {
		member := state.ParseMember(arg)

		// Validate project exists in state
		proj, err := stateMgr.GetProject(member.Project)
		if err != nil {
			return err
		}
		if proj == nil {
			return fmt.Errorf("project %q is not registered - start it first with: scdev start", member.Project)
		}

		// Validate service exists if specified
		if member.Service != "" {
			projCfg, err := config.LoadProject(proj.Path)
			if err != nil {
				return fmt.Errorf("failed to load config for project %q: %w", member.Project, err)
			}
			if _, ok := projCfg.Services[member.Service]; !ok {
				available := make([]string, 0, len(projCfg.Services))
				for svcName := range projCfg.Services {
					available = append(available, svcName)
				}
				sort.Strings(available)
				return fmt.Errorf("service %q not found in project %q (available: %s)", member.Service, member.Project, strings.Join(available, ", "))
			}
		}

		members = append(members, member)
	}

	// Add to state
	if err := stateMgr.AddLinkMembers(name, members); err != nil {
		return err
	}

	// Connect containers to the link network
	docker := runtimePkg.NewDockerCLI()
	for _, member := range members {
		containers, err := resolveMemberContainers(stateMgr, member)
		if err != nil {
			fmt.Printf("Warning: %v\n", err)
			continue
		}

		for _, containerName := range containers {
			running, err := docker.IsContainerRunning(ctx, containerName)
			if err != nil || !running {
				fmt.Printf("  %s (not running, will connect on next start)\n", containerName)
				continue
			}

			if err := docker.NetworkConnect(ctx, link.Network, containerName); err != nil {
				errStr := strings.ToLower(err.Error())
				if strings.Contains(errStr, "already exists") || strings.Contains(errStr, "already connected") {
					fmt.Printf("  %s (already connected)\n", containerName)
				} else {
					fmt.Printf("  Warning: failed to connect %s: %v\n", containerName, err)
				}
			} else {
				fmt.Printf("  %s connected\n", containerName)
			}
		}
	}

	fmt.Printf("Members joined link %q\n", name)
	return nil
}

func runLinkLeave(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := requireDocker(ctx); err != nil {
		return err
	}

	name := args[0]
	memberArgs := args[1:]

	stateMgr, err := state.DefaultManager()
	if err != nil {
		return err
	}

	link, err := stateMgr.GetLink(name)
	if err != nil {
		return err
	}
	if link == nil {
		return fmt.Errorf("link %q does not exist", name)
	}

	members := make([]state.LinkMember, 0, len(memberArgs))
	for _, arg := range memberArgs {
		members = append(members, state.ParseMember(arg))
	}

	// Disconnect containers from the link network
	docker := runtimePkg.NewDockerCLI()
	for _, member := range members {
		containers, err := resolveMemberContainers(stateMgr, member)
		if err != nil {
			continue
		}
		for _, containerName := range containers {
			if err := docker.NetworkDisconnect(ctx, link.Network, containerName); err == nil {
				fmt.Printf("  %s disconnected\n", containerName)
			}
		}
	}

	if err := stateMgr.RemoveLinkMembers(name, members); err != nil {
		return err
	}

	fmt.Printf("Members left link %q\n", name)
	return nil
}

func runLinkLs(cmd *cobra.Command, args []string) error {
	stateMgr, err := state.DefaultManager()
	if err != nil {
		return err
	}

	links, err := stateMgr.ListLinks()
	if err != nil {
		return err
	}

	if len(links) == 0 {
		fmt.Println("No links defined")
		return nil
	}

	// Sort link names for stable output
	names := make([]string, 0, len(links))
	for name := range links {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		entry := links[name]
		memberStrs := make([]string, len(entry.Members))
		for i, m := range entry.Members {
			memberStrs[i] = m.String()
		}
		if len(memberStrs) == 0 {
			fmt.Printf("%s  (no members)\n", name)
		} else {
			fmt.Printf("%s  %s\n", name, strings.Join(memberStrs, ", "))
		}
	}

	return nil
}

func runLinkStatus(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := requireDocker(ctx); err != nil {
		return err
	}

	name := args[0]

	stateMgr, err := state.DefaultManager()
	if err != nil {
		return err
	}

	link, err := stateMgr.GetLink(name)
	if err != nil {
		return err
	}
	if link == nil {
		return fmt.Errorf("link %q does not exist", name)
	}

	docker := runtimePkg.NewDockerCLI()

	networkExists, _ := docker.NetworkExists(ctx, link.Network)

	netStatus := "exists"
	if !networkExists {
		netStatus = "missing"
	}
	fmt.Printf("Link: %s\n", name)
	fmt.Printf("Network: %s (%s)\n", link.Network, netStatus)
	fmt.Println()

	if len(link.Members) == 0 {
		fmt.Println("No members")
		return nil
	}

	fmt.Println("Members:")
	for _, member := range link.Members {
		containers, err := resolveMemberContainers(stateMgr, member)
		if err != nil {
			fmt.Printf("  %s - %v\n", member.String(), err)
			continue
		}
		for _, containerName := range containers {
			status := "not running"
			running, err := docker.IsContainerRunning(ctx, containerName)
			if err == nil && running {
				status = "running"
			}
			fmt.Printf("  %s  %s\n", containerName, status)
		}
	}

	return nil
}

// resolveMemberContainers returns container names for a link member.
// For a whole-project member, it returns all service containers.
// For a service-specific member, it returns just that one container.
func resolveMemberContainers(stateMgr *state.Manager, member state.LinkMember) ([]string, error) {
	proj, err := stateMgr.GetProject(member.Project)
	if err != nil {
		return nil, fmt.Errorf("failed to look up project %q: %w", member.Project, err)
	}
	if proj == nil {
		return nil, fmt.Errorf("project %q is not registered", member.Project)
	}

	if member.Service != "" {
		return []string{project.ContainerNameFor(member.Service, member.Project)}, nil
	}

	// Whole project: load config to get all service names
	projCfg, err := config.LoadProject(proj.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to load config for project %q: %w", member.Project, err)
	}

	containers := make([]string, 0, len(projCfg.Services))
	for svcName := range projCfg.Services {
		containers = append(containers, project.ContainerNameFor(svcName, member.Project))
	}
	sort.Strings(containers)
	return containers, nil
}

// disconnectLinkMembers disconnects all member containers from a link network
func disconnectLinkMembers(ctx context.Context, docker *runtimePkg.DockerCLI, link *state.LinkEntry, stateMgr *state.Manager) {
	seen := make(map[string]bool)
	for _, member := range link.Members {
		if !seen[member.Project] {
			seen[member.Project] = true
			project.DisconnectProjectFromLinks(ctx, docker, link, member.Project, stateMgr)
		}
	}
}
