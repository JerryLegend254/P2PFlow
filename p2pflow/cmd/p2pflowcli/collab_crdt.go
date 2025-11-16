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

	"github.com/JerryLegend254/p2pflow/internal/crdt"
	"github.com/JerryLegend254/p2pflow/internal/network"
	"github.com/JerryLegend254/p2pflow/internal/watcher"
	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// newCollabCRDTServeCommand creates a CRDT-based serve command
func (app *application) newCollabCRDTServeCommand() *cobra.Command {
	var filePath string
	var port int

	cmd := &cobra.Command{
		Use:   "serve [file]",
		Short: "Start a CRDT-based collaboration session",
		Long:  "Create a new CRDT collaboration session with eventual consistency guarantees",
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

			// Generate agent ID
			agentID := generateAgentID()

			// Get username from config
			cfg, _ := app.loadAuth()
			agentName := "Anonymous"
			if cfg != nil && cfg.Auth.Username != "" {
				agentName = cfg.Auth.Username
			}

			cyan := color.New(color.FgCyan).SprintFunc()
			green := color.New(color.FgGreen).SprintFunc()
			yellow := color.New(color.FgYellow).SprintFunc()

			app.console.Infof("🚀 Starting CRDT collaboration session...")
			app.console.Infof("Session ID: %s", cyan(sessionID))
			app.console.Infof("Path: %s", yellow(filePath))
			app.console.Infof("Agent: %s", green(agentName))
			app.console.Infof("Mode: %s", green("CRDT (Eventual Consistency)"))

			// Create context
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Create CRDT engine
			crdtEngine := crdt.NewCRDTEngine()

			// Create CRDT session
			session := crdtEngine.CreateSession(sessionID, filePath, agentID)

			// Create P2P node with CRDT support
			node, err := network.NewCRDTNode(ctx, port, agentID)
			if err != nil {
				return fmt.Errorf("failed to create CRDT node: %w", err)
			}
			defer node.Close()

			// Join the session
			if err := node.JoinSession(sessionID, agentID, agentName); err != nil {
				return fmt.Errorf("failed to join session: %w", err)
			}

			// Initialize files in the session
			if info.IsDir() {
				s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
				s.Suffix = " Scanning directory..."
				s.Start()

				fileCount := 0
				var totalSize int64

				err = filepath.Walk(filePath, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}

					// Skip directories
					if info.IsDir() {
						return nil
					}

					// Read file content
					content, err := os.ReadFile(path)
					if err != nil {
						return nil // Skip files we can't read
					}

					// Convert to relative path
					relPath, err := filepath.Rel(filePath, path)
					if err != nil {
						relPath = path
					}

					// Initialize document in CRDT engine
					if err := crdtEngine.InitializeDocument(sessionID, relPath, agentID, string(content)); err != nil {
						log.Printf("Warning: Failed to initialize %s: %v", relPath, err)
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

				fmt.Printf("✓ Added %s to session (%s)\n", green(fmt.Sprintf("%d files", fileCount)), cyan(formatBytes(totalSize)))

			} else {
				// Single file mode
				content, err := os.ReadFile(filePath)
				if err != nil {
					return fmt.Errorf("failed to read file: %w", err)
				}

				if err := crdtEngine.InitializeDocument(sessionID, filePath, agentID, string(content)); err != nil {
					return fmt.Errorf("failed to initialize document: %w", err)
				}

				fmt.Printf("✓ Added %s to session (%s)\n", green("1 file"), cyan(formatBytes(int64(len(content)))))
			}

			app.console.Infof("Session created: %s", session.ID)
			app.console.Infof("Listening on port: %d", port)
			app.console.Infof("Node ID: %s", node.GetNodeID())

			// Print connection info
			app.console.Infof("\n📡 Peers can join using:")
			app.console.Infof("  p2pflow collab-crdt join %s --port <port>", cyan(sessionID))

			// Set up CRDT watcher
			crdtWatcher, err := watcher.NewCRDTWatcher(filePath, crdtEngine, sessionID, agentID)
			if err != nil {
				return fmt.Errorf("failed to create CRDT watcher: %w", err)
			}
			defer crdtWatcher.Close()

			// Load ignore patterns
			useDefaults := viper.GetBool("ignore.use_defaults")
			useP2PIgnore := viper.GetBool("ignore.use_p2pignore")
			customPatterns := viper.GetStringSlice("ignore.patterns")
			crdtWatcher.LoadIgnorePatterns(useDefaults, useP2PIgnore, customPatterns)

			// Set up watcher callback to broadcast operations
			crdtWatcher.OnChange = func(filePath string, op *crdt.Operation) {
				if err := node.BroadcastOperation(filePath, op); err != nil {
					log.Printf("Failed to broadcast operation: %v", err)
				} else {
					app.console.Infof("📤 Broadcasted change to %s", filePath)
				}
			}

			// Set up node callback to apply remote operations
			node.SetOnOperation(func(msg *network.CRDTOperationMessage) {
				app.console.Infof("📥 Received change to %s from remote peer", msg.FilePath)

				// ApplyRemoteOperation handles incoming write tracking internally
				if err := crdtWatcher.ApplyRemoteOperation(msg.FilePath, msg.Operation); err != nil {
					log.Printf("Failed to apply remote operation: %v", err)
				}
			})

			// Set up peer connection handlers
			node.SetOnPeerConnected(func(peerID peer.ID) {
				app.console.Infof("✓ Peer connected: %s", green(peerID.String()))
			})

			node.SetOnPeerDisconnected(func(peerID peer.ID) {
				app.console.Infof("✗ Peer disconnected: %s", yellow(peerID.String()))
			})

			// Start watcher
			errCh := make(chan error)
			if err := crdtWatcher.Start(errCh); err != nil {
				return fmt.Errorf("failed to start watcher: %w", err)
			}

			go func() {
				for err := range errCh {
					log.Printf("Watcher error: %v", err)
				}
			}()

			app.console.Infof("👁  File watcher started for: %s", filePath)
			app.console.Infof("\n✨ CRDT collaboration session is running!")
			app.console.Infof("   Press Ctrl+C to stop\n")

			// Periodic stats display
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()

			go func() {
				for range ticker.C {
					stats, err := crdtEngine.GetStats(sessionID)
					if err == nil {
						app.console.Infof("📊 Stats: %d documents, %d agents, %d operations, %d tombstones",
							stats.DocumentCount, stats.AgentCount, stats.OperationCount, stats.TotalTombstones)
					}
				}
			}()

			// Handle shutdown
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

			<-sigCh
			app.console.Infof("\n👋 Shutting down...")
			cancel()

			// Save session state
			persistence := crdt.NewSessionPersistence(".")
			if err := persistence.SaveSession(session); err != nil {
				app.console.Errorf("Failed to save session: %v", err)
			} else {
				app.console.Infof("✓ Session state saved")
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "File or directory path to serve")
	cmd.Flags().IntVarP(&port, "port", "p", 0, "Port to listen on (0 = random)")

	return cmd
}

// newCollabCRDTJoinCommand creates a CRDT-based join command
func (app *application) newCollabCRDTJoinCommand() *cobra.Command {
	var filePath string
	var port int

	cmd := &cobra.Command{
		Use:   "join <session-id>",
		Short: "Join a CRDT-based collaboration session",
		Long:  "Connect to an existing CRDT collaboration session with eventual consistency",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID := args[0]

			if filePath == "" {
				// Default to current directory
				filePath = "."
			}

			// Check if path exists, create if necessary
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				if err := os.MkdirAll(filePath, 0755); err != nil {
					return fmt.Errorf("failed to create directory: %w", err)
				}
			}

			// Generate agent ID
			agentID := generateAgentID()

			// Get username from config
			cfg, _ := app.loadAuth()
			agentName := "Anonymous"
			if cfg != nil && cfg.Auth.Username != "" {
				agentName = cfg.Auth.Username
			}

			cyan := color.New(color.FgCyan).SprintFunc()
			green := color.New(color.FgGreen).SprintFunc()
			yellow := color.New(color.FgYellow).SprintFunc()

			app.console.Infof("🔗 Joining CRDT collaboration session...")
			app.console.Infof("Session ID: %s", cyan(sessionID))
			app.console.Infof("Path: %s", yellow(filePath))
			app.console.Infof("Agent: %s", green(agentName))

			// Create context
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Create CRDT engine
			crdtEngine := crdt.NewCRDTEngine()

			// Try to load existing session
			persistence := crdt.NewSessionPersistence(".")
			existingSession, err := persistence.LoadSession(sessionID)
			if err == nil {
				app.console.Infof("✓ Loaded existing session from disk")
				crdtEngine.ImportSession(existingSession)
			}

			// Create P2P node
			node, err := network.NewCRDTNode(ctx, port, agentID)
			if err != nil {
				return fmt.Errorf("failed to create CRDT node: %w", err)
			}
			defer node.Close()

			// Join session
			if err := node.JoinSession(sessionID, agentID, agentName); err != nil {
				return fmt.Errorf("failed to join session: %w", err)
			}

			// Also join/create in CRDT engine
			if _, err := crdtEngine.JoinSession(sessionID, agentID, agentName); err != nil {
				// Session doesn't exist yet, create it
				crdtEngine.CreateSession(sessionID, filePath, agentID)
				crdtEngine.JoinSession(sessionID, agentID, agentName)
			}

			app.console.Infof("Node ID: %s", node.GetNodeID())
			app.console.Infof("✓ Joined session: %s", sessionID)

			// Set up CRDT watcher
			crdtWatcher, err := watcher.NewCRDTWatcher(filePath, crdtEngine, sessionID, agentID)
			if err != nil {
				return fmt.Errorf("failed to create CRDT watcher: %w", err)
			}
			defer crdtWatcher.Close()

			// Load ignore patterns
			useDefaults := viper.GetBool("ignore.use_defaults")
			useP2PIgnore := viper.GetBool("ignore.use_p2pignore")
			customPatterns := viper.GetStringSlice("ignore.patterns")
			crdtWatcher.LoadIgnorePatterns(useDefaults, useP2PIgnore, customPatterns)

			// Set up watcher callback to broadcast operations
			crdtWatcher.OnChange = func(filePath string, op *crdt.Operation) {
				if err := node.BroadcastOperation(filePath, op); err != nil {
					log.Printf("Failed to broadcast operation: %v", err)
				} else {
					app.console.Infof("📤 Broadcasted change to %s", filePath)
				}
			}

			// Set up callback for when full session state is received
			receivedFiles := make(map[string]bool)
			node.SetOnSessionReceived(func(session *crdt.CRDTSession) {
				app.console.Infof("📦 Received session state with %d documents", len(session.Documents))

				// Write all documents to files
				for docPath := range session.Documents {
					// Get document content from CRDT engine
					content, err := crdtEngine.GetDocumentContent(sessionID, docPath)
					if err != nil {
						log.Printf("Failed to get document content for %s: %v", docPath, err)
						continue
					}

					// Full path on local filesystem
					fullPath := filepath.Join(filePath, docPath)

					// Ensure directory exists
					dir := filepath.Dir(fullPath)
					if err := os.MkdirAll(dir, 0755); err != nil {
						log.Printf("Failed to create directory: %v", err)
						continue
					}

					// Write file
					if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
						log.Printf("Failed to write file %s: %v", docPath, err)
					} else {
						app.console.Infof("✓ Synced file: %s (%d bytes)", docPath, len(content))
						receivedFiles[docPath] = true

						// Initialize watcher state for this file
						if err := crdtWatcher.InitializeFile(fullPath); err != nil {
							log.Printf("Failed to initialize watcher for %s: %v", fullPath, err)
						}
					}
				}

				app.console.Infof("✅ Initial file synchronization complete")
			})

			// Set up node callback to apply remote operations
			node.SetOnOperation(func(msg *network.CRDTOperationMessage) {
				app.console.Infof("📥 Received change to %s", msg.FilePath)

				// For new files, ensure they exist
				if !receivedFiles[msg.FilePath] {
					fullPath := filepath.Join(filePath, msg.FilePath)

					// Ensure directory exists
					dir := filepath.Dir(fullPath)
					if err := os.MkdirAll(dir, 0755); err != nil {
						log.Printf("Failed to create directory: %v", err)
						return
					}

					// Get current content from CRDT engine
					content, err := crdtEngine.GetDocumentContent(sessionID, msg.FilePath)
					if err == nil && content != "" {
						// Initialize local file with CRDT content
						if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
							log.Printf("Failed to create file: %v", err)
						} else {
							app.console.Infof("✓ Created file: %s", msg.FilePath)
						}
					}

					receivedFiles[msg.FilePath] = true
				}

				if err := crdtWatcher.ApplyRemoteOperation(msg.FilePath, msg.Operation); err != nil {
					log.Printf("Failed to apply remote operation: %v", err)
				}
			})

			// Set up peer connection handlers
			node.SetOnPeerConnected(func(peerID peer.ID) {
				app.console.Infof("✓ Peer connected: %s", green(peerID.String()))

				// Request sync when a peer connects
				session, _ := crdtEngine.GetSession(sessionID)
				if session != nil {
					if err := node.RequestSync(session.VectorClock); err != nil {
						log.Printf("Failed to request sync: %v", err)
					} else {
						app.console.Infof("📡 Requested sync from peer")
					}
				}
			})

			node.SetOnPeerDisconnected(func(peerID peer.ID) {
				app.console.Infof("✗ Peer disconnected: %s", yellow(peerID.String()))
			})

			// Start watcher
			errCh := make(chan error)
			if err := crdtWatcher.Start(errCh); err != nil {
				return fmt.Errorf("failed to start watcher: %w", err)
			}

			go func() {
				for err := range errCh {
					log.Printf("Watcher error: %v", err)
				}
			}()

			app.console.Infof("👁  File watcher started for: %s", filePath)
			app.console.Infof("\n✨ Connected to CRDT collaboration session!")
			app.console.Infof("   Waiting for file synchronization...")
			app.console.Infof("   Press Ctrl+C to stop\n")

			// Periodic stats display
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()

			go func() {
				for range ticker.C {
					stats, err := crdtEngine.GetStats(sessionID)
					if err == nil {
						app.console.Infof("📊 Stats: %d documents, %d agents, %d operations, %d tombstones",
							stats.DocumentCount, stats.AgentCount, stats.OperationCount, stats.TotalTombstones)
					}
				}
			}()

			// Handle shutdown
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

			<-sigCh
			app.console.Infof("\n👋 Shutting down...")
			cancel()

			// Save session state
			session, _ := crdtEngine.GetSession(sessionID)
			if session != nil {
				if err := persistence.SaveSession(session); err != nil {
					app.console.Errorf("Failed to save session: %v", err)
				} else {
					app.console.Infof("✓ Session state saved")
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Local directory path to sync files to")
	cmd.Flags().IntVarP(&port, "port", "p", 0, "Port to listen on (0 = random)")

	return cmd
}

// Add CRDT collab command to main collab command
func (app *application) newCollabCRDTCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "collab-crdt",
		Short: "CRDT-based peer-to-peer collaboration (eventual consistency)",
		Long:  "Commands for creating and joining CRDT collaboration sessions with eventual consistency guarantees",
	}

	cmd.AddCommand(app.newCollabCRDTServeCommand())
	cmd.AddCommand(app.newCollabCRDTJoinCommand())

	return cmd
}

// Utility function for CRDT agent ID
func generateAgentID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
