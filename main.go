package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/pflag"

	"github.com/Mscotello/docker-agent-tail/internal/cli"
	"github.com/Mscotello/docker-agent-tail/internal/docker"
	"github.com/Mscotello/docker-agent-tail/internal/session"
)

var Version = "dev"

func main() {
	// Define flags using spf13/pflag
	var (
		names   = pflag.StringSliceP("names", "n", nil, "explicit container names")
		compose = pflag.BoolP("compose", "c", false, "auto-discover from compose project")
		follow  = pflag.BoolP("follow", "f", true, "reattach on restart")
		output  = pflag.StringP("output", "o", "./logs", "output directory")
		since   = pflag.StringP("since", "s", "", "start from N ago (e.g. \"5m\")")
		version = pflag.BoolP("version", "v", false, "show version and exit")
	)

	// Unused flags reserved for Phase 4
	_ = pflag.BoolP("all", "a", false, "tail all running containers")
	_ = pflag.StringSliceP("exclude", "e", nil, "regex patterns to exclude")
	_ = pflag.StringSliceP("mute", "m", nil, "hide from terminal, still write to file")
	_ = pflag.BoolP("json", "j", false, "JSON lines output")
	_ = pflag.Bool("no-color", false, "disable terminal colors")

	// Add help flag
	pflag.BoolP("help", "h", false, "show help message")

	// Parse flags
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	// Handle version
	if *version {
		fmt.Printf("docker-agent-tail %s\n", Version)
		os.Exit(0)
	}

	// Handle help
	if help, _ := pflag.CommandLine.GetBool("help"); help {
		printUsage()
		os.Exit(0)
	}

	// Get positional arguments (glob patterns)
	args := pflag.Args()

	// If no flags or args provided, show usage
	all, _ := pflag.CommandLine.GetBool("all")
	if !all && len(*names) == 0 && !*compose && len(args) == 0 {
		printUsage()
		os.Exit(0)
	}

	// Handle subcommands
	if len(args) > 0 {
		switch args[0] {
		case "init":
			if err := cli.RunInit(*output); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		case "agent-help":
			fmt.Print(cli.AgentHelp())
			os.Exit(0)
		case "lnav-install":
			if err := cli.RunLnavInstall(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		case "clean":
			retain := 5
			if len(args) > 1 {
				for i := 1; i < len(args); i++ {
					if args[i] == "--retain" && i+1 < len(args) {
						n, err := strconv.Atoi(args[i+1])
						if err != nil || n < 0 {
							fmt.Fprintf(os.Stderr, "Error: --retain must be a non-negative integer\n")
							os.Exit(1)
						}
						retain = n
					}
				}
			}
			deleted, err := session.CleanSessions(*output, retain)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			remaining := 0
			entries, _ := os.ReadDir(*output)
			for _, e := range entries {
				if e.IsDir() && e.Name() != "latest" {
					remaining++
				}
			}
			fmt.Printf("Removed %d sessions, kept %d\n", len(deleted), remaining)
			os.Exit(0)
		}
	}

	// Setup context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle SIGINT and SIGTERM
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	// Create Docker client
	dc, err := docker.NewClient(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: creating Docker client: %v\n", err)
		os.Exit(1)
	}
	defer dc.Close()

	// Determine compose project
	var composeProject string
	if *compose {
		composeProject, err = detectComposeProject(ctx, dc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: detecting compose project: %v\n", err)
		}
	}

	// Discover initial containers
	containers, err := discoverInitialContainers(ctx, dc, args, *names, composeProject)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: discovering containers: %v\n", err)
		os.Exit(1)
	}

	if len(containers) == 0 {
		fmt.Fprintf(os.Stderr, "No containers found matching the criteria\n")
		os.Exit(1)
	}

	// Create session
	containerNames := make([]string, len(containers))
	for i, c := range containers {
		containerNames[i] = c.Name
	}

	sess, err := session.NewSession(*output, os.Args[0]+" "+strings.Join(os.Args[1:], " "), containerNames)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: creating session: %v\n", err)
		os.Exit(1)
	}

	// Create log writer
	writer, err := session.NewLogWriter(sess.Dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: creating log writer: %v\n", err)
		os.Exit(1)
	}
	defer writer.Close()

	// Parse since duration
	var sinceTime time.Time
	if *since != "" {
		d, err := time.ParseDuration(*since)
		if err == nil {
			sinceTime = time.Now().Add(-d)
		}
	}

	// Start streaming logs from initial containers
	var wg sync.WaitGroup
	streamMap := make(map[string]context.CancelFunc)
	streamMu := sync.Mutex{}

	for _, c := range containers {
		containerID := c.ID
		containerName := c.Name
		wg.Add(1)
		go func() {
			defer wg.Done()
			streamContainerLogs(ctx, dc, containerID, containerName, sinceTime, *follow, writer, &streamMap, &streamMu)
		}()
	}

	// Start event watcher
	watchOpts := docker.WatchEventsOpts{
		GlobPattern:    strings.Join(args, "*"),
		ComposeProject: composeProject,
	}

	eventCh, errCh := docker.WatchEvents(ctx, dc, watchOpts)

	// Process events
	wg.Add(1)
	go func() {
		defer wg.Done()
		processEvents(ctx, dc, eventCh, errCh, writer, *follow, &streamMap, &streamMu, &wg, sess)
	}()

	// Wait for context to be done
	<-ctx.Done()
	wg.Wait()

	// Print shutdown summary
	fmt.Printf("\nSession closed: %s\n", sess.Dir)
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: docker-agent-tail [FLAGS] [PATTERN...]

Auto-discover Docker containers and tail their logs.

Commands:
  init          Set up AI agent config files (.claude, .cursor, .windsurf)
  agent-help    Print usage guide for AI coding agents
  clean         Remove old log sessions (--retain N, default 5)
  lnav-install  Install lnav format for viewing logs with lnav

Flags:
`)
	pflag.PrintDefaults()
}

// containerRef holds a reference to a discovered container
type containerRef struct {
	ID   string
	Name string
}

// detectComposeProject detects the Docker Compose project from environment or running containers
func detectComposeProject(ctx context.Context, c docker.DockerClient) (string, error) {
	// Try reading docker-compose.yml from current directory
	composeFiles := []string{"docker-compose.yml", "compose.yml"}
	for _, file := range composeFiles {
		if _, err := os.Stat(file); err == nil {
			// Found compose file - extract project name from directory
			wd, err := os.Getwd()
			if err == nil {
				return filepath.Base(wd), nil
			}
		}
	}

	// Try reading from environment
	if project := os.Getenv("COMPOSE_PROJECT_NAME"); project != "" {
		return project, nil
	}

	// No project found
	return "", fmt.Errorf("no docker-compose.yml found and COMPOSE_PROJECT_NAME not set")
}

// discoverInitialContainers discovers containers based on filters
func discoverInitialContainers(ctx context.Context, c docker.DockerClient, patterns, names []string, composeProject string) ([]containerRef, error) {
	var opts docker.DiscoverOpts

	if composeProject != "" {
		opts.ComposeProject = composeProject
	}

	var result []containerRef

	// If names are specified, discover each explicitly
	if len(names) > 0 {
		for _, name := range names {
			opts.GlobPattern = name
			containers, err := docker.DiscoverContainers(ctx, c, opts)
			if err != nil {
				return nil, err
			}
			for _, ci := range containers {
				result = append(result, containerRef{ID: ci.ID, Name: ci.Name})
			}
		}
		return result, nil
	}

	// If patterns are specified, discover matching containers
	if len(patterns) > 0 {
		for _, pattern := range patterns {
			opts.GlobPattern = pattern
			containers, err := docker.DiscoverContainers(ctx, c, opts)
			if err != nil {
				return nil, err
			}
			for _, ci := range containers {
				result = append(result, containerRef{ID: ci.ID, Name: ci.Name})
			}
		}
		return result, nil
	}

	// Discover all containers if no filters specified
	opts.GlobPattern = ""
	containers, err := docker.DiscoverContainers(ctx, c, opts)
	if err != nil {
		return nil, err
	}
	for _, ci := range containers {
		result = append(result, containerRef{ID: ci.ID, Name: ci.Name})
	}
	return result, nil
}

// streamContainerLogs streams logs from a single container
func streamContainerLogs(ctx context.Context, c docker.DockerClient, containerID, containerName string, sinceTime time.Time, follow bool, w *session.LogWriter, streamMap *map[string]context.CancelFunc, mu *sync.Mutex) {
	logCtx, logCancel := context.WithCancel(ctx)
	mu.Lock()
	(*streamMap)[containerID] = logCancel
	mu.Unlock()
	defer func() {
		logCancel()
		mu.Lock()
		delete(*streamMap, containerID)
		mu.Unlock()
	}()

	opts := docker.StreamLogsOpts{
		Follow:        follow,
		Since:         sinceTime,
		ContainerName: containerName,
	}

	logCh, errCh := docker.StreamLogs(logCtx, c, containerID, opts)

	for {
		select {
		case <-logCtx.Done():
			return
		case line := <-logCh:
			if line.Content == "" && line.ContainerName == "" {
				return
			}
			w.Write(line)
		case err := <-errCh:
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading logs from %s: %v\n", containerName, err)
			}
			return
		}
	}
}

// processEvents handles Docker container lifecycle events
func processEvents(ctx context.Context, c docker.DockerClient, eventCh <-chan docker.ContainerEvent, errCh <-chan error, w *session.LogWriter, follow bool, streamMap *map[string]context.CancelFunc, mu *sync.Mutex, wg *sync.WaitGroup, sess *session.Session) {
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errCh:
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error watching events: %v\n", err)
			}
			return
		case event := <-eventCh:
			switch event.Type {
			case docker.EventStart:
				// New container started - attach log stream
				wg.Add(1)
				go func(e docker.ContainerEvent) {
					defer wg.Done()
					streamContainerLogs(ctx, c, e.ContainerID, e.ContainerName, time.Now(), follow, w, streamMap, mu)
				}(event)

				// Update session metadata
				sess.Containers = append(sess.Containers, event.ContainerName)
				updateSessionMetadata(sess)

			case docker.EventStop, docker.EventDie:
				// Container stopped - cancel its log stream
				mu.Lock()
				if cancel, ok := (*streamMap)[event.ContainerID]; ok {
					cancel()
				}
				mu.Unlock()

				// Write marker to log file
				w.Write(docker.LogLine{
					Timestamp:     event.Time,
					Stream:        "info",
					Content:       "[CONTAINER STOPPED]",
					ContainerName: event.ContainerName,
				})

			case docker.EventRestart:
				if follow {
					// Reattach log stream from current timestamp
					mu.Lock()
					if cancel, ok := (*streamMap)[event.ContainerID]; ok {
						cancel()
					}
					mu.Unlock()

					wg.Add(1)
					go func(e docker.ContainerEvent) {
						defer wg.Done()
						time.Sleep(100 * time.Millisecond) // Wait for container to fully start
						streamContainerLogs(ctx, c, e.ContainerID, e.ContainerName, time.Now(), follow, w, streamMap, mu)
					}(event)
				}
			}
		}
	}
}

// updateSessionMetadata updates the session metadata.json file
func updateSessionMetadata(sess *session.Session) {
	meta := session.Metadata{
		StartTime:  sess.StartTime,
		Command:    sess.Command,
		Containers: sess.Containers,
	}
	metadataPath := filepath.Join(sess.Dir, "metadata.json")
	data, err := json.MarshalIndent(meta, "", "  ")
	if err == nil {
		_ = os.WriteFile(metadataPath, data, 0644)
	}
}
