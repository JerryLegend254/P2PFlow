package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/JerryLegend254/p2pflow/internal/collab"
	"github.com/JerryLegend254/p2pflow/internal/network"
	"github.com/JerryLegend254/p2pflow/internal/watcher"
	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func (app *application) newCollabCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "collab",
		Short: "Peer-to-peer collaboration commands",
		Long:  "Commands for creating and joining collaboration sessions",
	}

	cmd.AddCommand(app.newCollabServeCommand())
	cmd.AddCommand(app.newCollabJoinCommand())

	return cmd
}

func (app *application) newCollabServeCommand() *cobra.Command {
	var filePath string
	var port int

	cmd := &cobra.Command{
		Use:   "serve [file]",
		Short: "Start a collaboration session for a file",
		Long:  "Create a new collaboration session and start serving the file to peers",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				filePath = args[0]
			}

			if filePath == "" {
				return fmt.Errorf("file path is required")
			}

			// Check if path exists
			info, err := os.Stat(filePath)
			if os.IsNotExist(err) {
				return fmt.Errorf("path does not exist: %s", filePath)
			}
			if err != nil {
				return fmt.Errorf("failed to stat path: %w", err)
			}

			// Generate session ID
			sessionID := generateSessionID()

			// Get username from config
			cfg, _ := app.loadAuth()
			agentName := "Anonymous"
			if cfg != nil && cfg.Auth.Username != "" {
				agentName = cfg.Auth.Username
			}

			app.console.Infof("Starting collaboration session...")
			app.console.Infof("Session ID: %s", sessionID)
			app.console.Infof("Path: %s", filePath)
			app.console.Infof("Agent: %s", agentName)

			// Create context
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Create P2P node
			node, err := network.NewP2PNode(ctx, port, agentName)
			if err != nil {
				return fmt.Errorf("failed to create P2P node: %w", err)
			}
			defer node.Stop()

			// Start the node
			if err := node.Start(); err != nil {
				return fmt.Errorf("failed to start P2P node: %w", err)
			}

			var session *collab.Session

			if info.IsDir() {
				// Directory mode - scan all files
				cyan := color.New(color.FgCyan).SprintFunc()
				green := color.New(color.FgGreen).SprintFunc()

				s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
				s.Suffix = " Scanning directory..."
				s.Start()

				// Create session with empty content for backward compatibility
				session, err = node.CreateSession(sessionID, "", "")
				if err != nil {
					s.Stop()
					return fmt.Errorf("failed to create session: %w", err)
				}

				// Set the root path for the session
				session.RootPath = filePath

				// Scan and add all files
				fileCount := 0
				var totalSize int64
				err = filepath.Walk(filePath, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}

					// Skip directories and hidden files
					if info.IsDir() || info.Name()[0] == '.' {
						return nil
					}

					// Read file content
					content, err := os.ReadFile(path)
					if err != nil {
						return nil
					}

					// Convert to relative path from session root
					relPath, err := filepath.Rel(filePath, path)
					if err != nil {
						relPath = path // Fallback to absolute if rel fails
					}

					// Add file to session with relative path
					if err := node.GetCollabEngine().AddFile(sessionID, relPath, string(content)); err != nil {
						return nil
					}

					fileCount++
					totalSize += int64(len(content))
					s.Suffix = fmt.Sprintf(" Scanning directory... (%d files, %s)", fileCount, formatBytes(totalSize))
					return nil
				})

				s.Stop()

				if err != nil {
					return fmt.Errorf("failed to scan directory: %w", err)
				}

				fmt.Printf("Added %s to session (%s)\n", green(fmt.Sprintf("%d files", fileCount)), cyan(formatBytes(totalSize)))

			} else {
				// Single file mode (backward compatibility)
				content, err := os.ReadFile(filePath)
				if err != nil {
					return fmt.Errorf("failed to read file: %w", err)
				}

				session, err = node.CreateSession(sessionID, filePath, string(content))
				if err != nil {
					return fmt.Errorf("failed to create session: %w", err)
				}
			}

			app.console.Infof("Session created: %s", session.ID)
			app.console.Infof("Listening on port: %d", port)
			app.console.Infof("Node ID: %s", node.GetHost().ID())

			// Print multiaddress for manual connection
			addrs := node.GetHost().Addrs()
			if len(addrs) > 0 {
				app.console.Infof("\nNode multiaddresses:")
				for _, addr := range addrs {
					fullAddr := fmt.Sprintf("%s/p2p/%s", addr, node.GetHost().ID())
					app.console.Infof("  %s", fullAddr)
				}
			}

			app.console.Infof("\nPeers can join using:")
			app.console.Infof("  p2pflow collab join %s --port 4002", sessionID)
			if len(addrs) > 0 {
				// Show example with first address
				firstAddr := fmt.Sprintf("%s/p2p/%s", addrs[0], node.GetHost().ID())
				app.console.Infof("\nOr with explicit peer connection:")
				app.console.Infof("  p2pflow collab join %s --port 4002 --peer %s", sessionID, firstAddr)
			}

			// Set up peer connection handler
			node.SetOnPeerConnected(func(peerID peer.ID) {
				app.console.Infof("Peer connected: %s", peerID)
			})

			// Set up file watcher (use node ID as agent ID to match the session)
			fileWatcher, err := createFileWatcher(node, filePath, sessionID, node.GetNodeID())
			if err != nil {
				app.console.Errorf("Failed to create file watcher: %v", err)
			} else {
				errCh := make(chan error)
				if err := fileWatcher.Start(errCh); err != nil {
					return fmt.Errorf("failed to start watcher: %w", err)
				}

				go func() {
					for err := range errCh {
						log.Printf("Watcher error: %v", err)
					}
				}()

				app.console.Infof("File watcher started for: %s", filePath)
			}

			// Handle shutdown
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

			// Block until we send signal
			<-sigCh
			app.console.Infof("\nShutting down...")
			cancel()

			return nil
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "File path to serve")
	cmd.Flags().IntVarP(&port, "port", "p", 0, "Port to listen on (0 = random)")

	return cmd
}

func (app *application) newCollabJoinCommand() *cobra.Command {
	var filePath string
	var port int
	var bootstrapPeer string

	cmd := &cobra.Command{
		Use:   "join <session-id>",
		Short: "Join an existing collaboration session",
		Long:  "Connect to an existing collaboration session and receive file changes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID := args[0]

			if sessionID == "" {
				return fmt.Errorf("session ID is required")
			}

			// Get username from config
			cfg, _ := app.loadAuth()
			agentName := "Anonymous"
			if cfg != nil && cfg.Auth.Username != "" {
				agentName = cfg.Auth.Username
			}

			app.console.Infof("Joining collaboration session...")
			app.console.Infof("Session ID: %s", sessionID)
			app.console.Infof("Agent: %s", agentName)

			// Create context
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Create P2P node
			node, err := network.NewP2PNode(ctx, port, agentName)
			if err != nil {
				return fmt.Errorf("failed to create P2P node: %w", err)
			}
			defer node.Stop()

			// Start the node
			if err := node.Start(); err != nil {
				return fmt.Errorf("failed to start P2P node: %w", err)
			}

			// Connect to bootstrap peer if provided
			if bootstrapPeer != "" {
				green := color.New(color.FgGreen).SprintFunc()

				s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
				s.Suffix = " Connecting to peer..."
				s.Start()

				if err := node.ConnectToPeer(bootstrapPeer); err != nil {
					s.Stop()
					app.console.Warnf("Failed to connect to bootstrap peer: %v", err)
				} else {
					s.Stop()
					fmt.Printf("%s to peer\n", green("Connected"))
				}
			}

			// Prepare initial content
			initialContent := ""
			if filePath != "" {
				if _, err := os.Stat(filePath); err == nil {
					content, err := os.ReadFile(filePath)
					if err == nil {
						initialContent = string(content)
						app.console.Infof("Loaded initial content from: %s", filePath)
					}
				}
			}

			// Join session
			if err := node.JoinSession(sessionID, agentName, initialContent); err != nil {
				return fmt.Errorf("failed to join session: %w", err)
			}

			app.console.Infof("Joined session: %s", sessionID)
			app.console.Infof("Node ID: %s", node.GetHost().ID())
			app.console.Infof("Waiting for file changes...")

			// Set up peer connection handler
			node.SetOnPeerConnected(func(peerID peer.ID) {
				app.console.Infof("Peer connected: %s", peerID)
			})

			// Set up message handler
			node.SetOnMessage(func(msg *network.Message) {
				if msg.Type == network.MessageTypeChange {
					app.console.Infof("Received file change from peer %s", msg.AgentID)
					// TODO: Apply change to local file
				}
			})

			// Set up file watcher - watch the session directory for bidirectional sync
			// If --file flag is provided, use that path; otherwise use current directory
			watchPath := filePath
			if watchPath == "" {
				// Get session root path from node
				rootPath, err := node.GetSessionRootPath()
				if err == nil && rootPath != "" {
					watchPath = rootPath
				} else {
					// Fall back to current working directory
					watchPath, _ = os.Getwd()
				}
			}

			if watchPath != "" {
				fileWatcher, err := createFileWatcher(node, watchPath, sessionID, node.GetNodeID())
				if err != nil {
					app.console.Errorf("Failed to create file watcher: %v", err)
				} else {
					errCh := make(chan error)
					if err := fileWatcher.Start(errCh); err != nil {
						return fmt.Errorf("failed to start watcher: %w", err)
					}

					go func() {
						for err := range errCh {
							log.Printf("Watcher error: %v", err)
						}
					}()

					app.console.Infof("File watcher started for: %s", watchPath)
				}
			}

			// Handle shutdown
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

			<-sigCh
			app.console.Infof("\nShutting down...")
			cancel()

			return nil
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Local file path to sync")
	cmd.Flags().IntVarP(&port, "port", "p", 0, "Port to listen on (0 = random)")
	cmd.Flags().StringVarP(&bootstrapPeer, "peer", "b", "", "Bootstrap peer multiaddress (e.g., /ip4/127.0.0.1/tcp/4001/p2p/12D3...)")

	return cmd
}

// Helper function to create a file watcher with P2P integration
func createFileWatcher(node *network.P2PNode, filePath, sessionID, agentName string) (*watcher.Watcher, error) {
	// Create watcher
	w, err := watcher.NewWatcher(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	// Override the session ID and replace the collaboration engine with the node's engine
	w.SessionID = sessionID
	w.AgentID = agentName
	w.CollabEngine = node.GetCollabEngine()

	// Load ignore patterns from configuration
	cfg := loadConfigForIgnore()
	w.LoadIgnorePatterns(cfg.Ignore.UseDefaults, cfg.Ignore.UseP2PIgnore, cfg.Ignore.Patterns)

	// Share the ignore matcher with the node
	node.SetIgnoreMatcher(w.IgnoreMatcher)

	// Share the analytics engine with the watcher
	w.AnalyticsEngine = node.GetAnalyticsEngine()

	// Set up incoming write check to prevent loops
	w.IsIncomingWrite = node.IsIncomingWrite

	// Set up change handler
	w.OnChange = func(patch string, changedFilePath string) {
		// Get the session to apply changes
		session, err := w.CollabEngine.GetSession(sessionID)
		if err != nil {
			log.Printf("Failed to get session: %v", err)
			return
		}

		// Convert absolute path to relative path from session root
		relPath := changedFilePath
		if session.RootPath != "" && filepath.IsAbs(changedFilePath) {
			rel, err := filepath.Rel(session.RootPath, changedFilePath)
			if err == nil {
				relPath = rel
			}
		}

		// Get the current file version to use as base
		baseVersion := 0
		if file, err := w.CollabEngine.GetFile(sessionID, relPath); err == nil {
			baseVersion = file.Version
		}

		// Create change event with relative file path and version info
		changeEvent := &collab.ChangeEvent{
			SessionID:   sessionID,
			AgentID:     agentName,
			FilePath:    relPath, // Use relative path
			Patch:       patch,
			Version:     session.Version,
			BaseVersion: baseVersion, // Version patch was created from
		}

		// Apply change locally
		_, err = w.CollabEngine.ApplyChange(changeEvent)
		if err != nil {
			log.Printf("Failed to apply change: %v", err)
			return
		}

		// Broadcast change to peers
		if err := node.BroadcastChange(sessionID, agentName, changeEvent); err != nil {
			log.Printf("Failed to broadcast change: %v", err)
		}
	}

	return w, nil
}

func generateSessionID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// formatBytes converts bytes to human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// loadConfigForIgnore loads configuration specifically for ignore patterns
func loadConfigForIgnore() *appConfig {
	cfg := &appConfig{}

	// Set defaults
	cfg.Ignore.UseDefaults = viper.GetBool("ignore.use_defaults")
	cfg.Ignore.UseP2PIgnore = viper.GetBool("ignore.use_p2pignore")
	cfg.Ignore.Patterns = viper.GetStringSlice("ignore.patterns")

	// Apply defaults if not set
	if !viper.IsSet("ignore.use_defaults") {
		cfg.Ignore.UseDefaults = true
	}
	if !viper.IsSet("ignore.use_p2pignore") {
		cfg.Ignore.UseP2PIgnore = true
	}

	return cfg
}
