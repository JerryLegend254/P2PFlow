package network

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/JerryLegend254/p2pflow/internal/crdt"
	"github.com/JerryLegend254/p2pflow/internal/ignore"
	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/multiformats/go-multiaddr"
)

const (
	// CRDTProtocolID is used to identify the CRDT protocol
	CRDTProtocolID = "/p2pflow/crdt/1.0.0"
	// AntiEntropyInterval is how often we run anti-entropy protocol
	AntiEntropyInterval = 30 * time.Second
)

// CRDTNode represents a CRDT-aware P2P node
type CRDTNode struct {
	// libp2p components
	host      host.Host
	pubsub    *pubsub.PubSub
	topic     *pubsub.Topic
	sub       *pubsub.Subscription
	discovery mdns.Service

	// CRDT components
	crdtEngine *crdt.CRDTEngine

	// Node state
	nodeID     string
	sessionID  string
	peers      map[peer.ID]*PeerInfo
	peersMutex sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc

	// Operation buffering for out-of-order delivery
	operationBuffers map[string]*crdt.OperationBuffer // Map of file path -> buffer
	bufferMutex      sync.RWMutex

	// Callbacks
	onPeerConnected       func(peer.ID)
	onPeerDisconnected    func(peer.ID)
	onOperation           func(*CRDTOperationMessage)
	onFileListReceived    func(filePaths []string)
	onFileContentReceived func(filePath, content string)
	onSessionReceived     func(*crdt.CRDTSession)

	// File filtering
	ignoreMatcher *ignore.IgnoreMatcher

	// Anti-entropy ticker
	antiEntropyTicker *time.Ticker
}

// NewCRDTNode creates a new CRDT-aware P2P node
func NewCRDTNode(ctx context.Context, listenPort int, agentID string, crdtEngine *crdt.CRDTEngine) (*CRDTNode, error) {
	// Create context with cancellation
	nodeCtx, cancel := context.WithCancel(ctx)

	// Generate a random node ID
	nodeID := generateCRDTNodeID()

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

	// Create GossipSub pubsub system
	ps, err := pubsub.NewGossipSub(nodeCtx, h)
	if err != nil {
		cancel()
		h.Close()
		return nil, fmt.Errorf("failed to create pubsub: %w", err)
	}

	// Use the provided CRDT engine instead of creating a new one

	node := &CRDTNode{
		host:             h,
		pubsub:           ps,
		crdtEngine:       crdtEngine,
		nodeID:           nodeID,
		peers:            make(map[peer.ID]*PeerInfo),
		ctx:              nodeCtx,
		cancel:           cancel,
		operationBuffers: make(map[string]*crdt.OperationBuffer),
	}

	log.Printf("CRDT Node created with ID: %s", nodeID)
	log.Printf("Listening on: %s", h.Addrs())

	return node, nil
}

// JoinSession joins a CRDT collaboration session
func (n *CRDTNode) JoinSession(sessionID, agentID, agentName string) error {
	n.sessionID = sessionID

	// Join pubsub topic for this session
	topic, err := n.pubsub.Join(fmt.Sprintf("p2pflow-crdt-%s", sessionID))
	if err != nil {
		return fmt.Errorf("failed to join topic: %w", err)
	}
	n.topic = topic

	// Subscribe to the topic
	sub, err := topic.Subscribe()
	if err != nil {
		return fmt.Errorf("failed to subscribe to topic: %w", err)
	}
	n.sub = sub

	// Start message handler
	go n.handleMessages()

	// Setup mDNS discovery
	if err := n.setupDiscovery(sessionID); err != nil {
		return fmt.Errorf("failed to setup discovery: %w", err)
	}

	// Start anti-entropy protocol
	n.startAntiEntropy()

	// Give pubsub time to propagate subscription (increased for reliability)
	time.Sleep(2 * time.Second)

	// Broadcast join message
	joinMsg := CRDTJoinRequest{
		SessionID: sessionID,
		AgentID:   agentID,
		AgentName: agentName,
	}

	if err := n.broadcastCRDTMessage(CRDTMessageTypeJoin, joinMsg); err != nil {
		return fmt.Errorf("failed to broadcast join message: %w", err)
	}

	log.Printf("Joined CRDT session %s as agent %s", sessionID, agentID)
	return nil
}

// BroadcastOperation broadcasts a CRDT operation to all peers
func (n *CRDTNode) BroadcastOperation(filePath string, op *crdt.Operation) error {
	opMsg := CRDTOperationMessage{
		FilePath:  filePath,
		Operation: op,
	}

	return n.broadcastCRDTMessage(CRDTMessageTypeOperation, opMsg)
}

// RequestSync requests missing operations from peers
func (n *CRDTNode) RequestSync(vectorClock *crdt.VectorClock) error {
	if n.sessionID == "" {
		return fmt.Errorf("not joined to any session")
	}

	syncReq := CRDTSyncRequest{
		SessionID:   n.sessionID,
		AgentID:     n.nodeID,
		VectorClock: vectorClock,
	}

	return n.broadcastCRDTMessage(CRDTMessageTypeSync, syncReq)
}

// handleMessages handles incoming CRDT messages from peers
func (n *CRDTNode) handleMessages() {
	for {
		msg, err := n.sub.Next(n.ctx)
		if err != nil {
			if n.ctx.Err() != nil {
				return // Context cancelled, shutting down
			}
			log.Printf("Error reading message: %v", err)
			continue
		}

		// Ignore messages from self
		if msg.ReceivedFrom == n.host.ID() {
			continue
		}

		// Deserialize message
		var crdtMsg CRDTMessage
		if err := json.Unmarshal(msg.Data, &crdtMsg); err != nil {
			log.Printf("Failed to unmarshal CRDT message: %v", err)
			continue
		}

		// Handle message based on type
		switch crdtMsg.Type {
		case CRDTMessageTypeJoin:
			n.handleJoinMessage(&crdtMsg)
		case CRDTMessageTypeOperation:
			n.handleOperationMessage(&crdtMsg)
		case CRDTMessageTypeSync:
			n.handleSyncRequest(&crdtMsg)
		case CRDTMessageTypeSyncResponse:
			n.handleSyncResponse(&crdtMsg)
		case CRDTMessageTypeAntiEntropy:
			n.handleAntiEntropyMessage(&crdtMsg)
		case CRDTMessageTypePing:
			// Update peer last seen
			n.updatePeerLastSeen(msg.ReceivedFrom)
		case CRDTMessageTypeFileList:
			n.handleFileList(&crdtMsg)
		case CRDTMessageTypeFileContent:
			n.handleFileContent(&crdtMsg)
		default:
			log.Printf("Unknown CRDT message type: %d", crdtMsg.Type)
		}
	}
}

// handleJoinMessage handles a join message from a new peer
func (n *CRDTNode) handleJoinMessage(msg *CRDTMessage) {
	var joinReq CRDTJoinRequest
	if err := json.Unmarshal(msg.Payload, &joinReq); err != nil {
		log.Printf("Failed to unmarshal join request: %v", err)
		return
	}

	log.Printf("Peer %s joined session %s", joinReq.AgentID, joinReq.SessionID)

	// Join the session in CRDT engine
	if _, err := n.crdtEngine.JoinSession(joinReq.SessionID, joinReq.AgentID, joinReq.AgentName); err != nil {
		log.Printf("Failed to join session: %v", err)
		return
	}

	// Send current session state to the new peer
	session, err := n.crdtEngine.GetSession(joinReq.SessionID)
	if err != nil {
		log.Printf("Failed to get session: %v", err)
		return
	}

	// Broadcast session state
	joinResp := CRDTJoinResponse{
		Session: session,
	}

	if err := n.broadcastCRDTMessage(CRDTMessageTypeSyncResponse, joinResp); err != nil {
		log.Printf("Failed to send join response: %v", err)
	}

	// Trigger callback
	if n.onPeerConnected != nil {
		// We need to map agent ID to peer ID somehow - for now, skip
		// In production, maintain a mapping
	}
}

// handleOperationMessage handles a CRDT operation from a peer
func (n *CRDTNode) handleOperationMessage(msg *CRDTMessage) {
	var opMsg CRDTOperationMessage
	if err := json.Unmarshal(msg.Payload, &opMsg); err != nil {
		log.Printf("Failed to unmarshal operation message: %v", err)
		return
	}

	log.Printf("Received CRDT operation for file %s from %s", opMsg.FilePath, msg.AgentID)

	// Apply operation to CRDT engine
	if err := n.crdtEngine.ApplyOperation(msg.SessionID, opMsg.FilePath, opMsg.Operation); err != nil {
		log.Printf("Failed to apply operation: %v", err)

		// Buffer the operation if it can't be applied yet (missing dependencies)
		n.bufferOperation(opMsg.FilePath, opMsg.Operation)
		return
	}

	// Try to apply buffered operations
	n.tryApplyBufferedOperations(msg.SessionID, opMsg.FilePath)

	// Trigger callback
	if n.onOperation != nil {
		n.onOperation(&opMsg)
	}
}

// handleSyncRequest handles a sync request from a peer
func (n *CRDTNode) handleSyncRequest(msg *CRDTMessage) {
	var syncReq CRDTSyncRequest
	if err := json.Unmarshal(msg.Payload, &syncReq); err != nil {
		log.Printf("Failed to unmarshal sync request: %v", err)
		return
	}

	log.Printf("Received sync request from %s", syncReq.AgentID)

	// Get missing operations
	missingOps, err := n.crdtEngine.SyncOperations(syncReq.SessionID, syncReq.VectorClock)
	if err != nil {
		log.Printf("Failed to get sync operations: %v", err)
		return
	}

	// Group operations by file path
	opsByFile := make(map[string][]*crdt.Operation)
	for _, op := range missingOps {
		// We need to track which file each operation belongs to
		// This requires extending the operation log to include file paths
		// For now, we'll send all operations
		opsByFile[""] = append(opsByFile[""], op)
	}

	// Send sync response
	syncResp := CRDTSyncResponse{
		SessionID:  syncReq.SessionID,
		Operations: opsByFile,
	}

	if err := n.broadcastCRDTMessage(CRDTMessageTypeSyncResponse, syncResp); err != nil {
		log.Printf("Failed to send sync response: %v", err)
	}
}

// handleSyncResponse handles a sync response from a peer
func (n *CRDTNode) handleSyncResponse(msg *CRDTMessage) {
	// First, try to parse as CRDTJoinResponse (contains full session)
	var joinResp CRDTJoinResponse
	if err := json.Unmarshal(msg.Payload, &joinResp); err == nil && joinResp.Session != nil {
		log.Printf("Received join response with full session state")

		// Import the session into our CRDT engine
		n.crdtEngine.ImportSession(joinResp.Session)

		// Trigger callback for session received (so CLI can write files)
		if n.onSessionReceived != nil {
			n.onSessionReceived(joinResp.Session)
		}
		return
	}

	// Otherwise, try to parse as sync operations response
	var syncResp CRDTSyncResponse
	if err := json.Unmarshal(msg.Payload, &syncResp); err != nil {
		log.Printf("Failed to unmarshal sync response: %v", err)
		return
	}

	log.Printf("Received sync response with %d files", len(syncResp.Operations))

	// Apply all operations
	for filePath, ops := range syncResp.Operations {
		if err := n.crdtEngine.ApplyOperations(syncResp.SessionID, filePath, ops); err != nil {
			log.Printf("Failed to apply sync operations for %s: %v", filePath, err)
		}
	}
}

// handleAntiEntropyMessage handles anti-entropy full state sync
func (n *CRDTNode) handleAntiEntropyMessage(msg *CRDTMessage) {
	var aeMsg CRDTAntiEntropyMessage
	if err := json.Unmarshal(msg.Payload, &aeMsg); err != nil {
		log.Printf("Failed to unmarshal anti-entropy message: %v", err)
		return
	}

	log.Printf("Received anti-entropy message for session %s", aeMsg.SessionID)

	// Compare vector clocks to determine if we need to update
	session, err := n.crdtEngine.GetSession(aeMsg.SessionID)
	if err != nil {
		log.Printf("Failed to get session: %v", err)
		return
	}

	// Merge documents if remote is ahead
	for filePath, remoteDoc := range aeMsg.Documents {
		_, err := n.crdtEngine.GetOrCreateDocument(aeMsg.SessionID, filePath, n.nodeID)
		if err != nil {
			log.Printf("Failed to get local document: %v", err)
			continue
		}

		// Compare clocks and merge if needed
		if session.VectorClock.HappenedBefore(aeMsg.VectorClock) {
			// Remote is ahead, merge state
			// This is a simplified merge - production should be more sophisticated
			session.Documents[filePath] = remoteDoc
		}
	}

	// Merge vector clocks
	session.VectorClock.Merge(aeMsg.VectorClock)
}

// startAntiEntropy starts periodic anti-entropy protocol
func (n *CRDTNode) startAntiEntropy() {
	n.antiEntropyTicker = time.NewTicker(AntiEntropyInterval)

	go func() {
		for {
			select {
			case <-n.ctx.Done():
				n.antiEntropyTicker.Stop()
				return
			case <-n.antiEntropyTicker.C:
				n.runAntiEntropy()
			}
		}
	}()
}

// runAntiEntropy runs the anti-entropy protocol
func (n *CRDTNode) runAntiEntropy() {
	if n.sessionID == "" {
		return
	}

	session, err := n.crdtEngine.GetSession(n.sessionID)
	if err != nil {
		log.Printf("Failed to get session for anti-entropy: %v", err)
		return
	}

	aeMsg := CRDTAntiEntropyMessage{
		SessionID:   n.sessionID,
		Documents:   session.Documents,
		VectorClock: session.VectorClock,
	}

	if err := n.broadcastCRDTMessage(CRDTMessageTypeAntiEntropy, aeMsg); err != nil {
		log.Printf("Failed to broadcast anti-entropy message: %v", err)
	}
}

// bufferOperation buffers an operation that can't be applied yet
func (n *CRDTNode) bufferOperation(filePath string, op *crdt.Operation) {
	n.bufferMutex.Lock()
	defer n.bufferMutex.Unlock()

	if _, exists := n.operationBuffers[filePath]; !exists {
		n.operationBuffers[filePath] = crdt.NewOperationBuffer()
	}

	n.operationBuffers[filePath].Add(op)
	log.Printf("Buffered operation for %s (buffer size: %d)", filePath, n.operationBuffers[filePath].Len())
}

// tryApplyBufferedOperations tries to apply buffered operations
func (n *CRDTNode) tryApplyBufferedOperations(sessionID, filePath string) {
	n.bufferMutex.Lock()
	defer n.bufferMutex.Unlock()

	buffer, exists := n.operationBuffers[filePath]
	if !exists || buffer.Len() == 0 {
		return
	}

	// Get operations in order
	ops := buffer.GetOrdered()

	// Try to apply each operation
	applied := 0
	for _, op := range ops {
		if err := n.crdtEngine.ApplyOperation(sessionID, filePath, op); err == nil {
			applied++
		}
	}

	if applied > 0 {
		log.Printf("Applied %d buffered operations for %s", applied, filePath)
		// Clear buffer after successful application
		buffer.Clear()
	}
}

// broadcastCRDTMessage broadcasts a CRDT message to all peers
func (n *CRDTNode) broadcastCRDTMessage(msgType CRDTMessageType, payload interface{}) error {
	if n.topic == nil {
		return fmt.Errorf("not joined to any session")
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	msg := CRDTMessage{
		Type:      msgType,
		SessionID: n.sessionID,
		AgentID:   n.nodeID,
		Payload:   payloadBytes,
		Timestamp: time.Now().UnixNano(),
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	if err := n.topic.Publish(n.ctx, msgBytes); err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

// setupDiscovery sets up mDNS peer discovery
func (n *CRDTNode) setupDiscovery(sessionID string) error {
	s := mdns.NewMdnsService(n.host, fmt.Sprintf("p2pflow-crdt-%s", sessionID), &discoveryNotifee{
		node: n,
	})

	if err := s.Start(); err != nil {
		return err
	}

	n.discovery = s
	return nil
}

// updatePeerLastSeen updates the last seen time for a peer
func (n *CRDTNode) updatePeerLastSeen(peerID peer.ID) {
	n.peersMutex.Lock()
	defer n.peersMutex.Unlock()

	if peer, exists := n.peers[peerID]; exists {
		// Update last seen (simplified - need proper tracking)
		_ = peer
	}
}

// Close closes the CRDT node
func (n *CRDTNode) Close() error {
	n.cancel()

	if n.antiEntropyTicker != nil {
		n.antiEntropyTicker.Stop()
	}

	if n.sub != nil {
		n.sub.Cancel()
	}

	if n.topic != nil {
		n.topic.Close()
	}

	if n.discovery != nil {
		n.discovery.Close()
	}

	return n.host.Close()
}

// SetOnPeerConnected sets callback for peer connection
func (n *CRDTNode) SetOnPeerConnected(callback func(peer.ID)) {
	n.onPeerConnected = callback
}

// SetOnPeerDisconnected sets callback for peer disconnection
func (n *CRDTNode) SetOnPeerDisconnected(callback func(peer.ID)) {
	n.onPeerDisconnected = callback
}

// SetOnOperation sets callback for receiving operations
func (n *CRDTNode) SetOnOperation(callback func(*CRDTOperationMessage)) {
	n.onOperation = callback
}

// GetCRDTEngine returns the CRDT engine
func (n *CRDTNode) GetCRDTEngine() *crdt.CRDTEngine {
	return n.crdtEngine
}

// GetNodeID returns the node ID
func (n *CRDTNode) GetNodeID() string {
	return n.nodeID
}

// generateCRDTNodeID generates a random node ID for CRDT nodes
func generateCRDTNodeID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// discoveryNotifee handles peer discovery notifications
type discoveryNotifee struct {
	node *CRDTNode
}

func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	log.Printf("Discovered peer: %s", pi.ID)

	if pi.ID == n.node.host.ID() {
		return
	}

	// Connect to the peer
	if err := n.node.host.Connect(n.node.ctx, pi); err != nil {
		log.Printf("Failed to connect to peer %s: %v", pi.ID, err)
		return
	}

	log.Printf("Connected to peer: %s", pi.ID)

	// Add to peers list
	n.node.peersMutex.Lock()
	n.node.peers[pi.ID] = &PeerInfo{
		ID:        pi.ID,
		Connected: true,
	}
	n.node.peersMutex.Unlock()

	// Trigger callback
	if n.node.onPeerConnected != nil {
		n.node.onPeerConnected(pi.ID)
	}
}

// handleFileList handles a file list message from a peer
func (n *CRDTNode) handleFileList(msg *CRDTMessage) {
	var fileListMsg CRDTFileListMessage
	if err := json.Unmarshal(msg.Payload, &fileListMsg); err != nil {
		log.Printf("Failed to unmarshal file list message: %v", err)
		return
	}

	log.Printf("Received file list with %d files", len(fileListMsg.FilePaths))

	// Trigger callback if set (will be used by CLI to request file content)
	if n.onFileListReceived != nil {
		n.onFileListReceived(fileListMsg.FilePaths)
	}
}

// handleFileContent handles a file content message from a peer
func (n *CRDTNode) handleFileContent(msg *CRDTMessage) {
	var fileContentMsg CRDTFileContentMessage
	if err := json.Unmarshal(msg.Payload, &fileContentMsg); err != nil {
		log.Printf("Failed to unmarshal file content message: %v", err)
		return
	}

	log.Printf("Received file content for: %s (%d bytes)", fileContentMsg.FilePath, len(fileContentMsg.Content))

	// Trigger callback if set (will be used by CLI to save file)
	if n.onFileContentReceived != nil {
		n.onFileContentReceived(fileContentMsg.FilePath, fileContentMsg.Content)
	}
}

// SendFileList broadcasts the list of files in the session
func (n *CRDTNode) SendFileList(sessionID string, filePaths []string) error {
	fileListMsg := CRDTFileListMessage{
		SessionID: sessionID,
		FilePaths: filePaths,
	}

	return n.broadcastCRDTMessage(CRDTMessageTypeFileList, fileListMsg)
}

// SendFileContent sends the content of a specific file
func (n *CRDTNode) SendFileContent(sessionID, filePath, content string) error {
	fileContentMsg := CRDTFileContentMessage{
		SessionID: sessionID,
		FilePath:  filePath,
		Content:   content,
	}

	return n.broadcastCRDTMessage(CRDTMessageTypeFileContent, fileContentMsg)
}

// SetOnFileListReceived sets callback for when file list is received
func (n *CRDTNode) SetOnFileListReceived(callback func(filePaths []string)) {
	n.onFileListReceived = callback
}

// SetOnFileContentReceived sets callback for when file content is received
func (n *CRDTNode) SetOnFileContentReceived(callback func(filePath, content string)) {
	n.onFileContentReceived = callback
}

// SetOnSessionReceived sets callback for when full session state is received
func (n *CRDTNode) SetOnSessionReceived(callback func(*crdt.CRDTSession)) {
	n.onSessionReceived = callback
}
