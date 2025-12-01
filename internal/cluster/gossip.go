package cluster

import (
	"bytes"
	"encoding/gob"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"
)

// GossipMessage represents a gossip protocol message
type GossipMessage struct {
	Type      string
	NodeID    string
	NodeAddr  string
	Timestamp time.Time
	Data      map[string]string
}

// GossipProtocol implements a simple gossip protocol for node discovery
type GossipProtocol struct {
	nodeManager *NodeManager
	localAddr   string
	peers       []string
	mu          sync.RWMutex
	stopChan    chan struct{}
}

// NewGossipProtocol creates a new gossip protocol instance
func NewGossipProtocol(nodeManager *NodeManager, localAddr string, seedPeers []string) *GossipProtocol {
	return &GossipProtocol{
		nodeManager: nodeManager,
		localAddr:   localAddr,
		peers:       seedPeers,
		stopChan:    make(chan struct{}),
	}
}

// Start begins the gossip protocol
func (g *GossipProtocol) Start() error {
	// Start UDP listener
	go g.listen()
	
	// Start gossip sender
	go g.gossipLoop()
	
	// Announce ourselves to seed peers
	g.announce()
	
	return nil
}

// Stop stops the gossip protocol
func (g *GossipProtocol) Stop() {
	close(g.stopChan)
}

// listen for incoming gossip messages
func (g *GossipProtocol) listen() {
	addr, err := net.ResolveUDPAddr("udp", g.localAddr)
	if err != nil {
		log.Printf("Failed to resolve UDP address: %v", err)
		return
	}
	
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Printf("Failed to listen on UDP: %v", err)
		return
	}
	defer conn.Close()
	
	buffer := make([]byte, 4096)
	
	for {
		select {
		case <-g.stopChan:
			return
		default:
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, _, err := conn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				log.Printf("UDP read error: %v", err)
				continue
			}
			
			// Decode message
			var msg GossipMessage
			decoder := gob.NewDecoder(bytes.NewReader(buffer[:n]))
			if err := decoder.Decode(&msg); err != nil {
				log.Printf("Failed to decode gossip message: %v", err)
				continue
			}
			
			g.handleMessage(&msg)
		}
	}
}

// handleMessage processes incoming gossip messages
func (g *GossipProtocol) handleMessage(msg *GossipMessage) {
	switch msg.Type {
	case "announce":
		// New node announcement
		node := &Node{
			ID:       msg.NodeID,
			Address:  msg.NodeAddr,
			Status:   NodeHealthy,
			LastSeen: time.Now(),
			Metadata: msg.Data,
		}
		
		if err := g.nodeManager.AddNode(node); err != nil {
			log.Printf("Failed to add node %s: %v", msg.NodeID, err)
		}
		
		// Add to peers if not already there
		g.addPeer(msg.NodeAddr)
		
	case "heartbeat":
		// Update node's last seen time
		g.nodeManager.UpdateNodeStatus(msg.NodeID, NodeHealthy)
		
	case "leave":
		// Node is leaving
		g.nodeManager.RemoveNode(msg.NodeID)
		g.removePeer(msg.NodeAddr)
	}
}

// gossipLoop periodically sends gossip messages
func (g *GossipProtocol) gossipLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-g.stopChan:
			return
		case <-ticker.C:
			g.sendHeartbeat()
			g.shareNodeList()
		}
	}
}

// announce sends an announcement message to all peers
func (g *GossipProtocol) announce() {
	msg := &GossipMessage{
		Type:      "announce",
		NodeID:    g.nodeManager.GetLocalNodeID(),
		NodeAddr:  g.nodeManager.nodes[g.nodeManager.GetLocalNodeID()].Address,  // Use HTTP address
		Timestamp: time.Now(),
		Data:      make(map[string]string),
	}
	
	g.broadcast(msg)
}

// sendHeartbeat sends a heartbeat to random peers
func (g *GossipProtocol) sendHeartbeat() {
	msg := &GossipMessage{
		Type:      "heartbeat",
		NodeID:    g.nodeManager.GetLocalNodeID(),
		NodeAddr:  g.localAddr,
		Timestamp: time.Now(),
	}
	
	// Send to 3 random peers
	peers := g.getRandomPeers(3)
	for _, peer := range peers {
		g.sendMessage(peer, msg)
	}
}

// shareNodeList shares known nodes with random peers
func (g *GossipProtocol) shareNodeList() {
	nodes := g.nodeManager.GetHealthyNodes()
	
	for _, node := range nodes {
		if node.ID == g.nodeManager.GetLocalNodeID() {
			continue
		}
		
		msg := &GossipMessage{
			Type:      "announce",
			NodeID:    node.ID,
			NodeAddr:  node.Address,
			Timestamp: time.Now(),
			Data:      node.Metadata,
		}
		
		// Share with 2 random peers
		peers := g.getRandomPeers(2)
		for _, peer := range peers {
			g.sendMessage(peer, msg)
		}
	}
}

// broadcast sends a message to all known peers
func (g *GossipProtocol) broadcast(msg *GossipMessage) {
	g.mu.RLock()
	peers := make([]string, len(g.peers))
	copy(peers, g.peers)
	g.mu.RUnlock()
	
	for _, peer := range peers {
		g.sendMessage(peer, msg)
	}
}

// sendMessage sends a message to a specific peer
func (g *GossipProtocol) sendMessage(peerAddr string, msg *GossipMessage) {
	addr, err := net.ResolveUDPAddr("udp", peerAddr)
	if err != nil {
		log.Printf("Failed to resolve peer address %s: %v", peerAddr, err)
		return
	}
	
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		log.Printf("Failed to dial peer %s: %v", peerAddr, err)
		return
	}
	defer conn.Close()
	
	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)
	if err := encoder.Encode(msg); err != nil {
		log.Printf("Failed to encode message: %v", err)
		return
	}
	
	conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
	if _, err := conn.Write(buffer.Bytes()); err != nil {
		log.Printf("Failed to send message to %s: %v", peerAddr, err)
	}
}

// getRandomPeers returns n random peers
func (g *GossipProtocol) getRandomPeers(n int) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	if len(g.peers) <= n {
		result := make([]string, len(g.peers))
		copy(result, g.peers)
		return result
	}
	
	// Shuffle and take first n
	shuffled := make([]string, len(g.peers))
	copy(shuffled, g.peers)
	
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})
	
	return shuffled[:n]
}

// addPeer adds a peer to the list
func (g *GossipProtocol) addPeer(peerAddr string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	// Check if peer already exists
	for _, p := range g.peers {
		if p == peerAddr {
			return
		}
	}
	
	g.peers = append(g.peers, peerAddr)
}

// removePeer removes a peer from the list
func (g *GossipProtocol) removePeer(peerAddr string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	for i, p := range g.peers {
		if p == peerAddr {
			g.peers = append(g.peers[:i], g.peers[i+1:]...)
			return
		}
	}
}

// Leave announces that this node is leaving
func (g *GossipProtocol) Leave() {
	msg := &GossipMessage{
		Type:      "leave",
		NodeID:    g.nodeManager.GetLocalNodeID(),
		NodeAddr:  g.localAddr,
		Timestamp: time.Now(),
	}
	
	g.broadcast(msg)
}
