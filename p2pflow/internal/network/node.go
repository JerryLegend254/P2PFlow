package network

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/JerryLegend254/p2pflow/internal/collab"
	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/multiformats/go-multiaddr"
)

const (
	// ProtocolID is used to identify our protocol in the libp2p network
	ProtocolID = "/p2pflow/collab/1.0.0"
	// DiscoveryInterval is how often we search for peers
	DiscoveryInterval = 10
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
	nodeID     string
	sessionID  string
	peers      map[peer.ID]*PeerInfo
	peersMutex sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc

	// Callbacks
	onPeerConnected    func(peer.ID)
	onPeerDisconnected func(peer.ID)
	onMessage          func(*Message)
}

// PeerInfo contains information about a connected peer
type PeerInfo struct {
	ID        peer.ID
	Connected bool
	AgentID   string
	Name      string
}

// Message represents a message exchanged between peers
type Message struct {
	Type      MessageType     `json:"type"`
	SessionID string          `json:"session_id"`
	AgentID   string          `json:"agent_id"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp int64           `json:"timestamp"`
}

// MessageType represents the type of message
type MessageType int

const (
	// MessageTypeJoin announces joining a session
	MessageTypeJoin MessageType = iota
	// MessageTypeChange represents a file change/diff
	MessageTypeChange
	// MessageTypeSync requests synchronization
	MessageTypeSync
	// MessageTypeSyncResponse responds to sync request
	MessageTypeSyncResponse
	// MessageTypePing keeps the connection alive
	MessageTypePing
)

// NewP2PNode creates a new P2P node
func NewP2PNode(ctx context.Context, listenPort int, agentID string) (*P2PNode, error) {
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

	node := &P2PNode{
		host:           h,
		collabEngine:   collabEngine,
		sessionManager: sessionManager,
		nodeID:         nodeID,
		peers:          make(map[peer.ID]*PeerInfo),
		ctx:            nodeCtx,
		cancel:         cancel,
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

	// Start message handler
	go n.handleMessages()

	// Try to load existing session
	session, err := n.sessionManager.LoadSession(sessionID)
	if err != nil {
		// Session doesn't exist locally, create a new one
		session = n.collabEngine.CreateSession(sessionID, "", content)
		n.sessionManager.SaveSession(session)
	}
	fmt.Printf("session id: %s", session.ID)

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
	for {
		msg, err := n.sub.Next(n.ctx)
		if err != nil {
			if err == context.Canceled {
				return
			}
			log.Printf("Error receiving message: %v", err)
			continue
		}

		// Ignore messages from ourselves
		if msg.GetFrom() == n.host.ID() {
			continue
		}

		var message Message
		if err := json.Unmarshal(msg.Data, &message); err != nil {
			log.Printf("Error unmarshaling message: %v", err)
			continue
		}

		log.Printf("Received message type %d from peer: %s", message.Type, msg.GetFrom())

		// Handle the message
		if err := n.handleMessage(&message); err != nil {
			log.Printf("Error handling message: %v", err)
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
	default:
		return fmt.Errorf("unknown message type: %d", msg.Type)
	}
}

func (n *P2PNode) handleJoin(msg *Message) error {
	log.Printf("Peer %s joined session %s", msg.AgentID, msg.SessionID)
	return nil
}

func (n *P2PNode) handleChange(msg *Message) error {
	// Deserialize change event
	var event collab.ChangeEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		return fmt.Errorf("failed to unmarshal change event: %w", err)
	}

	// Apply change to collaboration engine
	_, err := n.collabEngine.ApplyChange(&event)
	if err != nil {
		return fmt.Errorf("failed to apply change: %w", err)
	}

	log.Printf("Applied change from peer %s", msg.AgentID)
	return nil
}

func (n *P2PNode) handleSync(_ *Message) error {
	// Send current session state
	// TODO: Implement sync response
	return nil
}

func (n *P2PNode) handleSyncResponse(_ *Message) error {
	// Handle sync response
	// TODO: Implement sync response handling
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
	log.Printf("Found peer: %s", pi.ID)

	// Connect to the peer
	err := nn.node.host.Connect(nn.node.ctx, pi)
	if err != nil {
		log.Printf("Failed to connect to peer %s: %v", pi.ID, err)
		return
	}

	log.Printf("Connected to peer: %s", pi.ID)

	// Open a stream
	stream, err := nn.node.host.NewStream(nn.node.ctx, pi.ID, ProtocolID)
	if err != nil {
		log.Printf("Failed to open stream to peer %s: %v", pi.ID, err)
		return
	}

	// Store peer info
	nn.node.peersMutex.Lock()
	nn.node.peers[pi.ID] = &PeerInfo{
		ID:        pi.ID,
		Connected: true,
	}
	nn.node.peersMutex.Unlock()

	if nn.node.onPeerConnected != nil {
		nn.node.onPeerConnected(pi.ID)
	}

	// Keep the stream open
	// In a real implementation, you'd want to handle the stream properly
	defer stream.Close()
}

// Utility functions

func generateNodeID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// ValidateMessage validates a pubsub message before it's processed
func ValidateMessage(msg *pubsub_pb.Message) error {
	return nil // Add validation logic if needed
}
