package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/JerryLegend254/p2pflow/internal/collab"
	"github.com/JerryLegend254/p2pflow/internal/network"
	"github.com/JerryLegend254/p2pflow/internal/watcher"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/spf13/cobra"
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

			// Check if file exists
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				return fmt.Errorf("file does not exist: %s", filePath)
			}

			// Read file content
			content, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
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
			app.console.Infof("File: %s", filePath)
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

			// Create session
			session, err := node.CreateSession(sessionID, filePath, string(content))
			if err != nil {
				return fmt.Errorf("failed to create session: %w", err)
			}

			app.console.Infof("Session created: %s", session.ID)
			app.console.Infof("Listening on port: %d", port)
			app.console.Infof("Node ID: %s", node.GetHost().ID())
			app.console.Infof("\nPeers can join using:")
			app.console.Infof("  p2pflow collab join %s", sessionID)

			// Set up peer connection handler
			node.SetOnPeerConnected(func(peerID peer.ID) {
				app.console.Infof("Peer connected: %s", peerID)
			})

			// Set up file watcher
			fileWatcher, err := createFileWatcher(node, filePath, sessionID, agentName)
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

			// Set up file watcher if file path is provided
			if filePath != "" {
				fileWatcher, err := createFileWatcher(node, filePath, sessionID, agentName)
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

	return cmd
}

// Helper function to create a file watcher with P2P integration
func createFileWatcher(node *network.P2PNode, filePath, sessionID, agentName string) (*watcher.Watcher, error) {
	// Create watcher
	w, err := watcher.NewWatcher(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	// Override the session ID
	w.SessionID = sessionID
	w.AgentID = agentName

	// Set up change handler
	w.OnChange = func(patch string) {
		// Get the session to apply changes
		session, err := w.CollabEngine.GetSession(sessionID)
		if err != nil {
			log.Printf("Failed to get session: %v", err)
			return
		}

		// Create change event
		changeEvent := &collab.ChangeEvent{
			SessionID: sessionID,
			AgentID:   agentName,
			Patch:     patch,
			Version:   session.Version,
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
