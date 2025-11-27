package network

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/JerryLegend254/p2pflow/internal/analytics"
	"github.com/JerryLegend254/p2pflow/internal/collab"
	"github.com/JerryLegend254/p2pflow/internal/ignore"
	"github.com/JerryLegend254/p2pflow/internal/modes"
	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/multiformats/go-multiaddr"
	"github.com/schollz/progressbar/v3"
)

const (
	// ProtocolID is used to identify our protocol in the libp2p network
	ProtocolID = "/p2pflow/collab/1.0.0"
	// DiscoveryInterval is how often we search for peers
	DiscoveryInterval = 10
	// SyncTimeout is how long to wait for a sync response
	SyncTimeout = 10 * time.Second
)

// P2PNode represents a P2P networking node
type P2PNode struct {
	// libp2p components
	host      host.Host
	pubsub    *pubsub.PubSub
	topic     *pubsub.Topic
	sub       *pubsub.Subscription
	discovery mdns.Service

	// Collaboration components
	collabEngine   *collab.CollaborationEngine
	sessionManager *collab.SessionManager

	// Node state
	nodeID                   string
	sessionID                string
	peers                    map[peer.ID]*PeerInfo
	peersMutex               sync.RWMutex
	ctx                      context.Context
	cancel                   context.CancelFunc
	syncResponseChan         chan *SyncResponse
	fileManifestResponseChan chan *FileManifestResponse
	fileTransferChan         chan *FileTransfer

	// File write tracking to prevent infinite loops
	incomingWrites      map[string]bool
	incomingWritesMutex sync.RWMutex

	// Callbacks
	onPeerConnected    func(peer.ID)
	onPeerDisconnected func(peer.ID)
	onMessage          func(*Message)

	// File filtering
	ignoreMatcher *ignore.IgnoreMatcher

	// Analytics engine
	analyticsEngine *analytics.AnalyticsEngine

	// Mode configuration
	modeConfig *modes.ModeConfig

	// Debounce timer for batch mode
	debounceTimer *time.Timer
	debounceMutex sync.Mutex
}

// PeerInfo contains information about a connected peer
type PeerInfo struct {
	ID        peer.ID
	Connected bool
	AgentID   string
	Name      string
}

// NewP2PNode creates a new P2P node
func NewP2PNode(ctx context.Context, listenPort int, agentID string) (*P2PNode, error) {
	// Use default realtime mode
	defaultMode, _ := modes.GetModeConfig(modes.RealtimeMode)
	return NewP2PNodeWithMode(ctx, listenPort, agentID, defaultMode)
}

// NewP2PNodeWithMode creates a new P2P node with a specific mode
func NewP2PNodeWithMode(ctx context.Context, listenPort int, agentID string, modeConfig modes.ModeConfig) (*P2PNode, error) {
	// Create context with cancellation
	nodeCtx, cancel := context.WithCancel(ctx)

	// Generate a random node ID
	nodeID := generateNodeID()

	// Create libp2p host
	listenAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", listenPort))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create listen address: %w", err)
	}

	h, err := libp2p.New(
		libp2p.ListenAddrs(listenAddr),
		libp2p.DefaultTransports,
		libp2p.DefaultMuxers,
		libp2p.DefaultSecurity,
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	// Initialize collaboration engine
	collabEngine := collab.NewCollaborationEngine()
	sessionManager := collab.NewSessionManager(".")

	// Initialize analytics engine
	analyticsConfig := analytics.DefaultConfig()
	analyticsEngine, err := analytics.NewAnalyticsEngine(analyticsConfig)
	if err != nil {
		cancel()
		h.Close()
		return nil, fmt.Errorf("failed to create analytics engine: %w", err)
	}

	// Try to load existing analytics data
	if err := analyticsEngine.Load(); err != nil {
		log.Printf("Note: Could not load existing analytics data: %v", err)
	}

	node := &P2PNode{
		host:            h,
		collabEngine:    collabEngine,
		sessionManager:  sessionManager,
		nodeID:          nodeID,
		peers:           make(map[peer.ID]*PeerInfo),
		ctx:             nodeCtx,
		cancel:          cancel,
		incomingWrites:  make(map[string]bool),
		ignoreMatcher:   nil, // Will be set via SetIgnoreMatcher
		analyticsEngine: analyticsEngine,
		modeConfig:      &modeConfig,
	}

	// Set up libp2p host handlers
	h.SetStreamHandler(ProtocolID, node.handleStream)
	//	// Set up MDNS discovery with notifee
	//	notifee := &nodeNotifee{node: node}
	//	discovery := mdns.NewMdnsService(h, "p2pflow", notifee)
	//
	//	node.discovery = discovery
	//
	//	// Set up mDNS notifee
	//	discovery.RegisterNotifee(&nodeNotifee{node: node})
	notifee := &nodeNotifee{node: node}
	discovery := mdns.NewMdnsService(h, "p2pflow", notifee)

	node.discovery = discovery
	// Initialize pubsub
	ps, err := pubsub.NewGossipSub(nodeCtx, h)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create pubsub: %w", err)
	}

	node.pubsub = ps

	log.Printf("P2P Node created with ID: %s", h.ID())
	log.Printf("Listening on: %s", h.Addrs())

	return node, nil
}

// Start starts the P2P node
func (n *P2PNode) Start() error {
	log.Println("Starting P2P node...")
	return nil
}

// Stop stops the P2P node
func (n *P2PNode) Stop() error {
	log.Println("Stopping P2P node...")

	// Save analytics data before stopping
	if n.analyticsEngine != nil {
		if err := n.analyticsEngine.Save(); err != nil {
			log.Printf("Warning: Failed to save analytics data: %v", err)
		}
		if err := n.analyticsEngine.Close(); err != nil {
			log.Printf("Warning: Failed to close analytics engine: %v", err)
		}
	}

	if n.cancel != nil {
		n.cancel()
	}

	if n.sub != nil {
		n.sub.Cancel()
	}

	if n.topic != nil {
		n.topic.Close()
	}

	if n.host != nil {
		return n.host.Close()
	}

	return nil
}

// CreateSession creates a new collaboration session
func (n *P2PNode) CreateSession(sessionID, filePath, content string) (*collab.Session, error) {
	n.sessionID = sessionID

	// Create session in collaboration engine
	session := n.collabEngine.CreateSession(sessionID, filePath, content)

	// Add ourselves as an agent in the session
	_, err := n.collabEngine.JoinSession(sessionID, n.nodeID, n.nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to join session as creator: %w", err)
	}

	// Save session
	if err := n.sessionManager.SaveSession(session); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	// Join the session topic (using session ID as topic)
	topic, err := n.pubsub.Join(fmt.Sprintf("p2pflow-session-%s", sessionID))
	if err != nil {
		return nil, fmt.Errorf("failed to join topic: %w", err)
	}

	n.topic = topic

	// Subscribe to the topic
	sub, err := n.topic.Subscribe()
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to topic: %w", err)
	}

	n.sub = sub

	// Start message handler
	go n.handleMessages()

	log.Printf("Created session: %s", sessionID)

	return session, nil
}

// JoinSession joins an existing collaboration session
func (n *P2PNode) JoinSession(sessionID, agentName string, content string) error {
	n.sessionID = sessionID

	// Join the session topic
	topic, err := n.pubsub.Join(fmt.Sprintf("p2pflow-session-%s", sessionID))
	if err != nil {
		return fmt.Errorf("failed to join topic: %w", err)
	}

	n.topic = topic

	// Subscribe to the topic
	sub, err := n.topic.Subscribe()
	if err != nil {
		return fmt.Errorf("failed to subscribe to topic: %w", err)
	}

	n.sub = sub

	// Create a channel to receive sync response
	n.syncResponseChan = make(chan *SyncResponse, 1)

	// Start message handler
	go n.handleMessages()

	// Try to load existing session locally first
	session, err := n.sessionManager.LoadSession(sessionID)
	if err != nil {
		// Session doesn't exist locally, request it from peers
		log.Printf("Session not found locally, requesting sync from peers...")

		// Wait a bit for peer discovery to complete and peers to connect
		log.Printf("Waiting for peer discovery...")
		time.Sleep(2 * time.Second)

		// Check libp2p host level connections
		hostPeers := n.host.Network().Peers()
		log.Printf("Host-level connected peers: %d", len(hostPeers))
		for i, p := range hostPeers {
			log.Printf("  Peer %d: %s", i+1, p)
		}

		// Check if we have any peers in the topic
		topicPeers := n.topic.ListPeers()
		log.Printf("Topic-level peers: %d", len(topicPeers))
		for i, p := range topicPeers {
			log.Printf("  Topic peer %d: %s", i+1, p)
		}

		if len(topicPeers) == 0 {
			log.Printf("No peers in topic yet, waiting longer for GossipSub mesh formation...")
			// Wait a bit more for mDNS to discover peers and GossipSub mesh to form
			time.Sleep(3 * time.Second)

			hostPeers = n.host.Network().Peers()
			log.Printf("After additional wait - Host-level peers: %d", len(hostPeers))

			topicPeers = n.topic.ListPeers()
			log.Printf("After additional wait - Topic-level peers: %d", len(topicPeers))
		}

		// Send sync request (will be retried)
		maxRetries := 3
		retryDelay := 3 * time.Second

		for attempt := 1; attempt <= maxRetries; attempt++ {
			log.Printf("Sending sync request (attempt %d/%d)...", attempt, maxRetries)

			if err := n.requestSessionSync(sessionID, agentName); err != nil {
				return fmt.Errorf("failed to request session sync: %w", err)
			}

			// Wait for sync response with timeout
			select {
			case syncResp := <-n.syncResponseChan:
				if syncResp == nil || syncResp.Session == nil {
					log.Printf("Received invalid sync response, retrying...")
					if attempt < maxRetries {
						time.Sleep(retryDelay)
						continue
					}
					return fmt.Errorf("received invalid sync response")
				}

				log.Printf("Received session state from peer")

				// Create session from the received state
				session = syncResp.Session
				n.collabEngine.ImportSession(session)

				// Save session locally
				if err := n.sessionManager.SaveSession(session); err != nil {
					log.Printf("Warning: failed to save session locally: %v", err)
				}

				// Success! Break out of retry loop
				goto sessionSynced

			case <-time.After(SyncTimeout):
				if attempt < maxRetries {
					log.Printf("Sync request timed out, retrying...")
					hostPeers := n.host.Network().Peers()
					topicPeers = n.topic.ListPeers()
					log.Printf("Current host-level peers: %d, topic-level peers: %d", len(hostPeers), len(topicPeers))
					time.Sleep(retryDelay)
					continue
				}
				return fmt.Errorf("timeout waiting for session sync response after %d attempts - no peers available or session not found", maxRetries)

			case <-n.ctx.Done():
				return fmt.Errorf("context cancelled while waiting for sync")
			}
		}
	} else {
		log.Printf("Loaded existing session from local storage: %s", session.ID)
		// Import the loaded session into the collaboration engine
		n.collabEngine.ImportSession(session)
	}

sessionSynced:
	// Now sync files if we don't have them locally
	log.Printf("Starting file synchronization...")

	// Create channels for file manifest and transfer responses
	n.fileManifestResponseChan = make(chan *FileManifestResponse, 1)
	n.fileTransferChan = make(chan *FileTransfer, 10) // Buffer for multiple files

	// Request file manifest from peers
	maxRetries := 3
	retryDelay := 3 * time.Second
	var fileManifest *FileManifestResponse

	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Printf("Requesting file manifest (attempt %d/%d)...", attempt, maxRetries)

		if err := n.requestFileManifest(sessionID); err != nil {
			log.Printf("Failed to send file manifest request: %v", err)
			if attempt < maxRetries {
				time.Sleep(retryDelay)
				continue
			}
			return fmt.Errorf("failed to request file manifest: %w", err)
		}

		// Wait for file manifest response
		select {
		case manifest := <-n.fileManifestResponseChan:
			log.Printf("Received file manifest with %d files", len(manifest.Files))
			fileManifest = manifest
			goto filesReceived

		case <-time.After(SyncTimeout):
			if attempt < maxRetries {
				log.Printf("File manifest request timed out, retrying...")
				time.Sleep(retryDelay)
				continue
			}
			log.Printf("Warning: Could not get file manifest from peers, continuing without files")
			goto skipFileSync

		case <-n.ctx.Done():
			return fmt.Errorf("context cancelled while waiting for file manifest")
		}
	}

filesReceived:
	// Download each file
	if fileManifest != nil && len(fileManifest.Files) > 0 {
		totalFiles := len(fileManifest.Files)
		fmt.Printf("\nDownloading %d files...\n\n", totalFiles)

		// Create progress bar
		bar := progressbar.NewOptions(totalFiles,
			progressbar.OptionEnableColorCodes(true),
			progressbar.OptionShowBytes(false),
			progressbar.OptionSetWidth(40),
			progressbar.OptionSetDescription("[cyan]Syncing files...[reset]"),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        "[green]=[reset]",
				SaucerHead:    "[green]>[reset]",
				SaucerPadding: " ",
				BarStart:      "[",
				BarEnd:        "]",
			}),
			progressbar.OptionShowCount(),
			progressbar.OptionOnCompletion(func() {
				fmt.Println("")
			}),
		)

		successCount := 0
		failedFiles := []string{}
		var totalBytes int64

		for filePath := range fileManifest.Files {
			if err := n.requestFile(sessionID, filePath); err != nil {
				log.Printf("Failed to request file %s: %v", filePath, err)
				failedFiles = append(failedFiles, filePath)
				bar.Add(1)
				continue
			}

			// Wait for file transfer
			select {
			case transfer := <-n.fileTransferChan:
				// Add file to the session
				if err := n.collabEngine.AddFile(sessionID, transfer.FilePath, transfer.Content); err != nil {
					log.Printf("Failed to add file %s to session: %v", transfer.FilePath, err)
					failedFiles = append(failedFiles, filePath)
					bar.Add(1)
					continue
				}

				// Ensure directory exists
				dir := filepath.Dir(transfer.FilePath)
				if dir != "." && dir != "" {
					if err := os.MkdirAll(dir, 0755); err != nil {
						log.Printf("Failed to create directory %s: %v", dir, err)
						failedFiles = append(failedFiles, filePath)
						bar.Add(1)
						continue
					}
				}

				// Save file to local filesystem
				if err := os.WriteFile(transfer.FilePath, []byte(transfer.Content), 0644); err != nil {
					log.Printf("Failed to write file %s: %v", transfer.FilePath, err)
					failedFiles = append(failedFiles, filePath)
					bar.Add(1)
					continue
				}

				successCount++
				totalBytes += transfer.Size
				bar.Add(1)

			case <-time.After(SyncTimeout):
				log.Printf("Timeout waiting for file %s, skipping", filePath)
				failedFiles = append(failedFiles, filePath)
				bar.Add(1)
				continue

			case <-n.ctx.Done():
				return fmt.Errorf("context cancelled while downloading files")
			}
		}

		// Print summary
		fmt.Printf("\n✅ File synchronization complete!\n")
		fmt.Printf("   📊 Summary:\n")
		fmt.Printf("      • Total files: %d\n", totalFiles)
		fmt.Printf("      • Successfully synced: [green]%d[reset]\n", successCount)
		if len(failedFiles) > 0 {
			fmt.Printf("      • Failed: [red]%d[reset]\n", len(failedFiles))
		}
		fmt.Printf("      • Total size: %s\n", formatBytes(totalBytes))

		if len(failedFiles) > 0 {
			fmt.Printf("\n   Failed files:\n")
			for _, f := range failedFiles {
				fmt.Printf("      - %s\n", f)
			}
		}
		fmt.Println()
	}

skipFileSync:
	// Add ourselves to the session
	_, err = n.collabEngine.JoinSession(sessionID, n.nodeID, agentName)
	if err != nil {
		return fmt.Errorf("failed to join session: %w", err)
	}

	// Send join message
	if err := n.broadcastJoin(); err != nil {
		return fmt.Errorf("failed to broadcast join: %w", err)
	}

	log.Printf("Joined session: %s", sessionID)

	return nil
}

// BroadcastChange broadcasts a file change to all peers
func (n *P2PNode) BroadcastChange(sessionID, agentID string, event *collab.ChangeEvent) error {
	// Check if we can send changes based on mode
	if n.modeConfig != nil && !n.modeConfig.CanSendChanges {
		log.Printf("Mode %s does not allow sending changes", n.modeConfig.Mode)
		return nil
	}

	// Check if we're in read-only mode
	if n.modeConfig != nil && n.modeConfig.ReadOnly {
		log.Printf("Read-only mode active, skipping broadcast")
		return nil
	}

	// Handle debouncing for batch mode
	if n.modeConfig != nil && n.modeConfig.DebounceInterval > 0 {
		return n.debouncedBroadcast(sessionID, agentID, event)
	}

	// Immediate broadcast for realtime mode
	return n.sendChangeImmediate(sessionID, agentID, event)
}

// sendChangeImmediate sends a change immediately
func (n *P2PNode) sendChangeImmediate(sessionID, agentID string, event *collab.ChangeEvent) error {
	// Serialize the change event
	payload, err := event.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize change event: %w", err)
	}

	// Create message
	msg := &Message{
		Type:      MessageTypeChange,
		SessionID: sessionID,
		AgentID:   agentID,
		Payload:   payload,
	}

	// Send message
	return n.sendMessage(msg)
}

// debouncedBroadcast handles debounced broadcasting for batch mode
func (n *P2PNode) debouncedBroadcast(sessionID, agentID string, event *collab.ChangeEvent) error {
	n.debounceMutex.Lock()
	defer n.debounceMutex.Unlock()

	// Cancel existing timer if any
	if n.debounceTimer != nil {
		n.debounceTimer.Stop()
	}

	// Create new timer
	n.debounceTimer = time.AfterFunc(n.modeConfig.DebounceInterval, func() {
		if err := n.sendChangeImmediate(sessionID, agentID, event); err != nil {
			log.Printf("Failed to send debounced change: %v", err)
		}
	})

	return nil
}

// SetOnPeerConnected sets the callback for when a peer connects
func (n *P2PNode) SetOnPeerConnected(callback func(peer.ID)) {
	n.onPeerConnected = callback
}

// SetOnPeerDisconnected sets the callback for when a peer disconnects
func (n *P2PNode) SetOnPeerDisconnected(callback func(peer.ID)) {
	n.onPeerDisconnected = callback
}

// SetOnMessage sets the callback for when a message is received
func (n *P2PNode) SetOnMessage(callback func(*Message)) {
	n.onMessage = callback
}

// GetHost returns the libp2p host
func (n *P2PNode) GetHost() host.Host {
	return n.host
}

// GetNodeID returns the node ID
func (n *P2PNode) GetNodeID() string {
	return n.nodeID
}

// GetCollabEngine returns the collaboration engine
func (n *P2PNode) GetCollabEngine() *collab.CollaborationEngine {
	return n.collabEngine
}

// GetPeers returns the list of connected peers
func (n *P2PNode) GetPeers() []*PeerInfo {
	n.peersMutex.RLock()
	defer n.peersMutex.RUnlock()

	peers := make([]*PeerInfo, 0, len(n.peers))
	for _, peer := range n.peers {
		peers = append(peers, peer)
	}

	return peers
}

// GetSessionID returns the current session ID
func (n *P2PNode) GetSessionID() string {
	return n.sessionID
}

// ConnectToPeer manually connects to a peer using its multiaddress
func (n *P2PNode) ConnectToPeer(peerAddr string) error {
	// Parse the multiaddress
	maddr, err := multiaddr.NewMultiaddr(peerAddr)
	if err != nil {
		return fmt.Errorf("invalid multiaddress: %w", err)
	}

	// Extract peer info
	peerInfo, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return fmt.Errorf("failed to extract peer info: %w", err)
	}

	log.Printf("Connecting to peer %s at %v", peerInfo.ID, peerInfo.Addrs)

	// Connect to the peer
	if err := n.host.Connect(n.ctx, *peerInfo); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	log.Printf("Successfully connected to peer %s", peerInfo.ID)

	// Store peer info
	n.peersMutex.Lock()
	n.peers[peerInfo.ID] = &PeerInfo{
		ID:        peerInfo.ID,
		Connected: true,
	}
	n.peersMutex.Unlock()

	// Open a stream to establish the connection
	stream, err := n.host.NewStream(n.ctx, peerInfo.ID, ProtocolID)
	if err != nil {
		log.Printf("Warning: failed to open stream to peer %s: %v", peerInfo.ID, err)
		// Connection is still valid even if stream fails
	} else {
		log.Printf("Opened stream to peer %s", peerInfo.ID)
		// Keep the stream open briefly then close
		go func() {
			time.Sleep(100 * time.Millisecond)
			stream.Close()
		}()
	}

	return nil
}

// private methods

func (n *P2PNode) handleStream(s network.Stream) {
	defer s.Close()

	log.Printf("New stream from peer: %s", s.Conn().RemotePeer())

	// Add peer to our list
	n.peersMutex.Lock()
	n.peers[s.Conn().RemotePeer()] = &PeerInfo{
		ID:        s.Conn().RemotePeer(),
		Connected: true,
	}
	n.peersMutex.Unlock()

	if n.onPeerConnected != nil {
		n.onPeerConnected(s.Conn().RemotePeer())
	}
}

func (n *P2PNode) handleMessages() {
	log.Printf("Message handler started, waiting for messages...")
	for {
		msg, err := n.sub.Next(n.ctx)
		if err != nil {
			if err == context.Canceled {
				log.Printf("Message handler stopped (context cancelled)")
				return
			}
			log.Printf("Error receiving message: %v", err)
			continue
		}

		// Ignore messages from ourselves
		if msg.GetFrom() == n.host.ID() {
			log.Printf("Ignoring message from self")
			continue
		}

		var message Message
		if err := json.Unmarshal(msg.Data, &message); err != nil {
			log.Printf("Error unmarshaling message: %v", err)
			continue
		}

		messageTypeNames := map[MessageType]string{
			MessageTypeJoin:                 "JOIN",
			MessageTypeChange:               "CHANGE",
			MessageTypeSync:                 "SYNC",
			MessageTypeSyncResponse:         "SYNC_RESPONSE",
			MessageTypePing:                 "PING",
			MessageTypeFileManifestRequest:  "FILE_MANIFEST_REQUEST",
			MessageTypeFileManifestResponse: "FILE_MANIFEST_RESPONSE",
			MessageTypeFileRequest:          "FILE_REQUEST",
			MessageTypeFileTransfer:         "FILE_TRANSFER",
		}
		typeName := messageTypeNames[message.Type]
		if typeName == "" {
			typeName = fmt.Sprintf("UNKNOWN(%d)", message.Type)
		}

		log.Printf("Received %s message from peer %s (session: %s)", typeName, msg.GetFrom(), message.SessionID)

		// Handle the message
		if err := n.handleMessage(&message); err != nil {
			log.Printf("Error handling %s message: %v", typeName, err)
		}

		// Call callback if set
		if n.onMessage != nil {
			n.onMessage(&message)
		}
	}
}

func (n *P2PNode) handleMessage(msg *Message) error {
	switch msg.Type {
	case MessageTypeJoin:
		return n.handleJoin(msg)
	case MessageTypeChange:
		return n.handleChange(msg)
	case MessageTypeSync:
		return n.handleSync(msg)
	case MessageTypeSyncResponse:
		return n.handleSyncResponse(msg)
	case MessageTypeFileManifestRequest:
		return n.handleFileManifestRequest(msg)
	case MessageTypeFileManifestResponse:
		return n.handleFileManifestResponse(msg)
	case MessageTypeFileRequest:
		return n.handleFileRequest(msg)
	case MessageTypeFileTransfer:
		return n.handleFileTransfer(msg)
	default:
		return fmt.Errorf("unknown message type: %d", msg.Type)
	}
}

func (n *P2PNode) handleJoin(msg *Message) error {
	log.Printf("Peer %s joined session %s", msg.AgentID, msg.SessionID)

	// Add the peer to our local session state
	// This ensures we can accept changes from this peer
	_, err := n.collabEngine.JoinSession(msg.SessionID, msg.AgentID, "Remote Peer")
	if err != nil {
		// If agent already exists, that's fine
		log.Printf("Note: Could not add peer to session: %v", err)
	} else {
		log.Printf("Added peer %s to local session state", msg.AgentID)
	}

	return nil
}

func (n *P2PNode) handleChange(msg *Message) error {
	// Check if we can receive changes based on mode
	if n.modeConfig != nil && !n.modeConfig.CanReceiveChanges {
		log.Printf("Mode %s does not allow receiving changes", n.modeConfig.Mode)
		return nil
	}

	// Deserialize change event
	var event collab.ChangeEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		return fmt.Errorf("failed to unmarshal change event: %w", err)
	}

	log.Printf("Received change for file: %s from peer %s", event.FilePath, msg.AgentID)

	// If in review mode, queue for approval instead of applying immediately
	if n.modeConfig != nil && n.modeConfig.RequireApproval {
		log.Printf("Review mode active - change queued for approval")
		// TODO: Implement approval queue
		return nil
	}

	// Record analytics: file change from peer
	if n.analyticsEngine != nil && event.FilePath != "" {
		// Estimate size from patch length (rough approximation)
		patchSize := int64(len(event.Patch))
		n.analyticsEngine.RecordFileChange(event.FilePath, patchSize, msg.AgentID)
		n.analyticsEngine.RecordFileAccess(event.FilePath, analytics.AccessTypeSync)
	}

	// Apply change to collaboration engine
	session, err := n.collabEngine.ApplyChange(&event)
	if err != nil {
		return fmt.Errorf("failed to apply change: %w", err)
	}

	log.Printf("Applied change from peer %s to session", msg.AgentID)

	// Write the updated content to the filesystem
	if event.FilePath != "" {
		file, err := n.collabEngine.GetFile(event.SessionID, event.FilePath)
		if err != nil {
			log.Printf("Warning: Could not get file %s: %v", event.FilePath, err)
			return nil
		}

		// Convert relative path to absolute path based on session root
		targetPath := event.FilePath
		if session.RootPath != "" && !filepath.IsAbs(event.FilePath) {
			targetPath = filepath.Join(session.RootPath, event.FilePath)
		}

		log.Printf("Writing updated content to file: %s (%d bytes)", targetPath, len(file.Content))

		// Mark this as an incoming write to prevent watcher loop
		n.markIncomingWrite(targetPath)

		// Ensure directory exists
		dir := filepath.Dir(targetPath)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				log.Printf("Warning: Failed to create directory %s: %v", dir, err)
			}
		}

		// Write file
		if err := os.WriteFile(targetPath, []byte(file.Content), 0644); err != nil {
			log.Printf("Warning: Failed to write file %s: %v", targetPath, err)
			return nil
		}

		log.Printf("Successfully wrote file: %s", targetPath)
	} else {
		// Backward compatibility: write session.Content to session.FilePath
		if session.FilePath != "" && session.Content != "" {
			if err := os.WriteFile(session.FilePath, []byte(session.Content), 0644); err != nil {
				log.Printf("Warning: Failed to write file %s: %v", session.FilePath, err)
			}
		}
	}

	return nil
}

func (n *P2PNode) handleSync(msg *Message) error {
	// Deserialize sync request
	var req SyncRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return fmt.Errorf("failed to unmarshal sync request: %w", err)
	}

	log.Printf("Received sync request for session %s from agent %s", req.SessionID, req.AgentID)

	// Get the session from collaboration engine
	session, err := n.collabEngine.GetSession(req.SessionID)
	if err != nil {
		log.Printf("Session %s not found, cannot respond to sync request", req.SessionID)
		return nil // Don't propagate error, just don't respond
	}

	// Create sync response
	syncResp := &SyncResponse{
		Session: session,
	}

	// Serialize the response
	payload, err := json.Marshal(syncResp)
	if err != nil {
		return fmt.Errorf("failed to marshal sync response: %w", err)
	}

	// Create response message
	responseMsg := &Message{
		Type:      MessageTypeSyncResponse,
		SessionID: req.SessionID,
		AgentID:   n.nodeID,
		Payload:   payload,
	}

	// Send the response
	if err := n.sendMessage(responseMsg); err != nil {
		return fmt.Errorf("failed to send sync response: %w", err)
	}

	log.Printf("Sent session state to peer %s", req.AgentID)
	return nil
}

func (n *P2PNode) handleSyncResponse(msg *Message) error {
	// Deserialize sync response
	var resp SyncResponse
	if err := json.Unmarshal(msg.Payload, &resp); err != nil {
		return fmt.Errorf("failed to unmarshal sync response: %w", err)
	}

	log.Printf("Received sync response from peer %s", msg.AgentID)

	// Send the response through the channel if it's being waited on
	if n.syncResponseChan != nil {
		select {
		case n.syncResponseChan <- &resp:
			log.Printf("Delivered sync response to waiting goroutine")
		default:
			log.Printf("No goroutine waiting for sync response")
		}
	}

	return nil
}

func (n *P2PNode) broadcastJoin() error {
	msg := &Message{
		Type:      MessageTypeJoin,
		SessionID: n.sessionID,
		AgentID:   n.nodeID,
	}

	return n.sendMessage(msg)
}

func (n *P2PNode) requestSessionSync(sessionID, agentName string) error {
	// Create sync request
	req := &SyncRequest{
		SessionID: sessionID,
		AgentID:   n.nodeID,
		AgentName: agentName,
	}

	// Serialize the request
	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal sync request: %w", err)
	}

	// Create message
	msg := &Message{
		Type:      MessageTypeSync,
		SessionID: sessionID,
		AgentID:   n.nodeID,
		Payload:   payload,
	}

	// Broadcast the sync request
	if err := n.sendMessage(msg); err != nil {
		return fmt.Errorf("failed to send sync request: %w", err)
	}

	log.Printf("Sent sync request for session %s", sessionID)
	return nil
}

func (n *P2PNode) sendMessage(msg *Message) error {
	if n.topic == nil {
		return fmt.Errorf("not subscribed to any topic")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return n.topic.Publish(n.ctx, data)
}

// nodeNotifee handles MDNS discovery events
type nodeNotifee struct {
	node *P2PNode
}

func (nn *nodeNotifee) HandlePeerFound(pi peer.AddrInfo) {
	log.Printf("mDNS: Found peer %s with %d addresses", pi.ID, len(pi.Addrs))

	// Check if already connected
	if nn.node.host.Network().Connectedness(pi.ID) == network.Connected {
		log.Printf("mDNS: Already connected to peer %s", pi.ID)

		// Store peer info if not already stored
		nn.node.peersMutex.Lock()
		if _, exists := nn.node.peers[pi.ID]; !exists {
			nn.node.peers[pi.ID] = &PeerInfo{
				ID:        pi.ID,
				Connected: true,
			}
		}
		nn.node.peersMutex.Unlock()
		return
	}

	// Connect to the peer
	log.Printf("mDNS: Connecting to peer %s...", pi.ID)
	err := nn.node.host.Connect(nn.node.ctx, pi)
	if err != nil {
		log.Printf("mDNS: Failed to connect to peer %s: %v", pi.ID, err)
		return
	}

	log.Printf("mDNS: Successfully connected to peer: %s", pi.ID)

	// Open a stream
	stream, err := nn.node.host.NewStream(nn.node.ctx, pi.ID, ProtocolID)
	if err != nil {
		log.Printf("mDNS: Failed to open stream to peer %s: %v", pi.ID, err)
		// Connection is still valid even if stream fails
	} else {
		log.Printf("mDNS: Opened stream to peer %s", pi.ID)
		// Keep the stream open briefly then close
		go func() {
			time.Sleep(100 * time.Millisecond)
			stream.Close()
		}()
	}

	// Store peer info
	nn.node.peersMutex.Lock()
	nn.node.peers[pi.ID] = &PeerInfo{
		ID:        pi.ID,
		Connected: true,
	}
	nn.node.peersMutex.Unlock()

	log.Printf("mDNS: Stored peer info for %s. Total peers: %d", pi.ID, len(nn.node.peers))

	if nn.node.onPeerConnected != nil {
		nn.node.onPeerConnected(pi.ID)
	}
}

// Utility functions

func generateNodeID() string {
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

// ValidateMessage validates a pubsub message before it's processed
func ValidateMessage(msg *pubsub_pb.Message) error {
	return nil // Add validation logic if needed
}

// handleFileManifestRequest responds with the list of files in the session
func (n *P2PNode) handleFileManifestRequest(msg *Message) error {
	var req FileManifestRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return fmt.Errorf("failed to unmarshal file manifest request: %w", err)
	}

	log.Printf("Received file manifest request for session %s from agent %s", req.SessionID, req.AgentID)

	// Get the session files
	files, err := n.collabEngine.ListFiles(req.SessionID)
	if err != nil {
		log.Printf("Session %s not found, cannot respond to file manifest request", req.SessionID)
		return nil // Don't propagate error
	}

	// Filter out ignored files
	filteredFiles := make(map[string]*collab.FileInfo)
	for path, fileInfo := range files {
		if n.ignoreMatcher != nil {
			// Check if file should be ignored
			info, err := os.Stat(path)
			isDir := err == nil && info.IsDir()
			if n.ignoreMatcher.ShouldIgnore(path, isDir) {
				log.Printf("Filtering out ignored file from manifest: %s", path)
				continue
			}
		}
		filteredFiles[path] = fileInfo
	}

	// Create response
	resp := &FileManifestResponse{
		SessionID: req.SessionID,
		Files:     filteredFiles,
	}

	// Serialize the response
	payload, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal file manifest response: %w", err)
	}

	// Create response message
	responseMsg := &Message{
		Type:      MessageTypeFileManifestResponse,
		SessionID: req.SessionID,
		AgentID:   n.nodeID,
		Payload:   payload,
	}

	// Send the response
	if err := n.sendMessage(responseMsg); err != nil {
		return fmt.Errorf("failed to send file manifest response: %w", err)
	}

	log.Printf("Sent file manifest to peer %s (%d files, %d filtered)", req.AgentID, len(filteredFiles), len(files)-len(filteredFiles))
	return nil
}

// handleFileManifestResponse processes the file manifest from a peer
func (n *P2PNode) handleFileManifestResponse(msg *Message) error {
	var resp FileManifestResponse
	if err := json.Unmarshal(msg.Payload, &resp); err != nil {
		return fmt.Errorf("failed to unmarshal file manifest response: %w", err)
	}

	log.Printf("Received file manifest from peer %s (%d files)", msg.AgentID, len(resp.Files))

	// Send the response through the channel if it's being waited on
	if n.fileManifestResponseChan != nil {
		select {
		case n.fileManifestResponseChan <- &resp:
			log.Printf("Delivered file manifest to waiting goroutine")
		default:
			log.Printf("No goroutine waiting for file manifest")
		}
	}

	return nil
}

// handleFileRequest sends the requested file content to a peer
func (n *P2PNode) handleFileRequest(msg *Message) error {
	var req FileRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return fmt.Errorf("failed to unmarshal file request: %w", err)
	}

	log.Printf("Received file request for %s from agent %s", req.FilePath, req.AgentID)

	// Get the file from the session
	file, err := n.collabEngine.GetFile(req.SessionID, req.FilePath)
	if err != nil {
		log.Printf("File %s not found in session %s", req.FilePath, req.SessionID)
		return nil // Don't propagate error
	}

	// Create file transfer
	transfer := &FileTransfer{
		SessionID: req.SessionID,
		FilePath:  req.FilePath,
		Content:   file.Content,
		Hash:      file.Hash,
		Size:      file.Size,
	}

	// Serialize the transfer
	payload, err := json.Marshal(transfer)
	if err != nil {
		return fmt.Errorf("failed to marshal file transfer: %w", err)
	}

	// Create response message
	responseMsg := &Message{
		Type:      MessageTypeFileTransfer,
		SessionID: req.SessionID,
		AgentID:   n.nodeID,
		Payload:   payload,
	}

	// Send the response
	if err := n.sendMessage(responseMsg); err != nil {
		return fmt.Errorf("failed to send file transfer: %w", err)
	}

	log.Printf("Sent file %s to peer %s (%d bytes)", req.FilePath, req.AgentID, file.Size)
	return nil
}

// handleFileTransfer processes a file transfer from a peer
func (n *P2PNode) handleFileTransfer(msg *Message) error {
	var transfer FileTransfer
	if err := json.Unmarshal(msg.Payload, &transfer); err != nil {
		return fmt.Errorf("failed to unmarshal file transfer: %w", err)
	}

	log.Printf("Received file transfer for %s from peer %s (%d bytes)", transfer.FilePath, msg.AgentID, transfer.Size)

	// Send the transfer through the channel if it's being waited on
	if n.fileTransferChan != nil {
		select {
		case n.fileTransferChan <- &transfer:
			log.Printf("Delivered file transfer to waiting goroutine")
		default:
			log.Printf("No goroutine waiting for file transfer")
		}
	}

	return nil
}

// requestFileManifest requests the list of files from peers
func (n *P2PNode) requestFileManifest(sessionID string) error {
	req := &FileManifestRequest{
		SessionID: sessionID,
		AgentID:   n.nodeID,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal file manifest request: %w", err)
	}

	msg := &Message{
		Type:      MessageTypeFileManifestRequest,
		SessionID: sessionID,
		AgentID:   n.nodeID,
		Payload:   payload,
	}

	if err := n.sendMessage(msg); err != nil {
		return fmt.Errorf("failed to send file manifest request: %w", err)
	}

	log.Printf("Sent file manifest request for session %s", sessionID)
	return nil
}

// requestFile requests a specific file from peers
func (n *P2PNode) requestFile(sessionID, filePath string) error {
	req := &FileRequest{
		SessionID: sessionID,
		AgentID:   n.nodeID,
		FilePath:  filePath,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal file request: %w", err)
	}

	msg := &Message{
		Type:      MessageTypeFileRequest,
		SessionID: sessionID,
		AgentID:   n.nodeID,
		Payload:   payload,
	}

	if err := n.sendMessage(msg); err != nil {
		return fmt.Errorf("failed to send file request: %w", err)
	}

	log.Printf("Sent file request for %s in session %s", filePath, sessionID)
	return nil
}

// markIncomingWrite marks a file as being written from an incoming change
// This prevents the watcher from detecting it and creating a loop
func (n *P2PNode) markIncomingWrite(filePath string) {
	n.incomingWritesMutex.Lock()
	defer n.incomingWritesMutex.Unlock()
	n.incomingWrites[filePath] = true

	// Auto-clear after 1 second to prevent stale entries
	go func() {
		time.Sleep(1 * time.Second)
		n.incomingWritesMutex.Lock()
		delete(n.incomingWrites, filePath)
		n.incomingWritesMutex.Unlock()
	}()
}

// isIncomingWrite checks if a file is currently being written from an incoming change
func (n *P2PNode) IsIncomingWrite(filePath string) bool {
	n.incomingWritesMutex.RLock()
	defer n.incomingWritesMutex.RUnlock()
	return n.incomingWrites[filePath]
}

// GetSessionRootPath returns the root path of the current session
func (n *P2PNode) GetSessionRootPath() (string, error) {
	if n.sessionID == "" {
		return "", fmt.Errorf("no active session")
	}

	session, err := n.collabEngine.GetSession(n.sessionID)
	if err != nil {
		return "", err
	}

	return session.RootPath, nil
}

// SetIgnoreMatcher sets the ignore matcher for file filtering
func (n *P2PNode) SetIgnoreMatcher(matcher *ignore.IgnoreMatcher) {
	n.ignoreMatcher = matcher
}

// GetAnalyticsEngine returns the analytics engine
func (n *P2PNode) GetAnalyticsEngine() *analytics.AnalyticsEngine {
	return n.analyticsEngine
}

// GetPrefetchSuggestions returns intelligent prefetch suggestions
func (n *P2PNode) GetPrefetchSuggestions(currentFiles []string, maxSuggestions int) []analytics.PrefetchSuggestion {
	if n.analyticsEngine == nil {
		return nil
	}
	return n.analyticsEngine.GetPrefetchSuggestions(currentFiles, maxSuggestions)
}

// GetFileImportance returns the importance score for a file
func (n *P2PNode) GetFileImportance(filePath string) float64 {
	if n.analyticsEngine == nil {
		return 0.5
	}
	return n.analyticsEngine.GetFileImportance(filePath)
}

// DetectAnomalies checks for unusual sync patterns
func (n *P2PNode) DetectAnomalies() []analytics.Anomaly {
	if n.analyticsEngine == nil {
		return nil
	}
	return n.analyticsEngine.DetectAnomalies()
}

// GetModeConfig returns the current mode configuration
func (n *P2PNode) GetModeConfig() *modes.ModeConfig {
	return n.modeConfig
}

// SetModeConfig updates the mode configuration
func (n *P2PNode) SetModeConfig(config modes.ModeConfig) error {
	if err := config.ValidateConfig(); err != nil {
		return fmt.Errorf("invalid mode config: %w", err)
	}
	n.modeConfig = &config
	log.Printf("Updated mode to: %s", config.Mode)
	return nil
}

// ShouldSyncFile checks if a file should be synced based on mode configuration
func (n *P2PNode) ShouldSyncFile(filePath string) bool {
	if n.modeConfig == nil {
		return true
	}

	// Clean the file path
	cleanPath := filepath.Clean(filePath)

	// Check selective paths
	if n.modeConfig.Mode == modes.SelectiveMode && len(n.modeConfig.SelectivePaths) > 0 {
		for _, path := range n.modeConfig.SelectivePaths {
			cleanSelectivePath := filepath.Clean(path)
			// Check if the file is within the selective path
			rel, err := filepath.Rel(cleanSelectivePath, cleanPath)
			if err == nil && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, "..") {
				return true
			}
		}
		return false
	}

	// Check exclude paths
	for _, path := range n.modeConfig.ExcludePaths {
		cleanExcludePath := filepath.Clean(path)
		// Check if the file is within the exclude path
		rel, err := filepath.Rel(cleanExcludePath, cleanPath)
		if err == nil && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, "..") {
			return false
		}
	}

	return true
}
