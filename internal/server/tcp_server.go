package server

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
	
	"github.com/sjohri/dcache/internal/cache"
)

// Server represents the TCP cache server
type Server struct {
	address  string
	listener net.Listener
	cache    *cache.Store
	wg       sync.WaitGroup
	shutdown chan struct{}
	
	// Connection tracking
	connections sync.Map // map[net.Conn]bool
	connCount   int32
}

// NewServer creates a new TCP server
func NewServer(address string, cacheStore *cache.Store) *Server {
	return &Server{
		address:  address,
		cache:    cacheStore,
		shutdown: make(chan struct{}),
	}
}

// Start begins listening for connections
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.address, err)
	}
	
	s.listener = listener
	log.Printf("TCP server listening on %s", s.address)
	
	// Accept connections in a goroutine
	go s.acceptConnections()
	
	return nil
}

// Stop gracefully shuts down the server
func (s *Server) Stop() error {
	log.Println("Shutting down server...")
	
	// Signal shutdown
	close(s.shutdown)
	
	// Stop accepting new connections
	if s.listener != nil {
		s.listener.Close()
	}
	
	// Close all existing connections
	s.connections.Range(func(key, value interface{}) bool {
		if conn, ok := key.(net.Conn); ok {
			conn.Close()
		}
		return true
	})
	
	// Wait for all handlers to finish
	s.wg.Wait()
	
	log.Println("Server shutdown complete")
	return nil
}

func (s *Server) acceptConnections() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.shutdown:
				return
			default:
				log.Printf("Error accepting connection: %v", err)
				continue
			}
		}
		
		// Track connection
		s.connections.Store(conn, true)
		
		// Handle connection in goroutine
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer func() {
		conn.Close()
		s.connections.Delete(conn)
		s.wg.Done()
	}()
	
	log.Printf("New connection from %s", conn.RemoteAddr())
	
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	
	// Send welcome message
	writer.WriteString("+OK DCache Server Ready\r\n")
	writer.Flush()
	
	for {
		// Set read timeout
		conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		
		// Read command line
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading from %s: %v", conn.RemoteAddr(), err)
			}
			return
		}
		
		// Process command
		response := s.processCommand(line, reader)
		
		// Send response
		writer.WriteString(response)
		writer.Flush()
		
		// Check for shutdown
		select {
		case <-s.shutdown:
			return
		default:
		}
	}
}

func (s *Server) processCommand(line string, reader *bufio.Reader) string {
	line = strings.TrimSpace(line)
	parts := strings.Fields(line)
	
	if len(parts) == 0 {
		return "-ERR empty command\r\n"
	}
	
	command := strings.ToUpper(parts[0])
	
	switch command {
	case "GET":
		if len(parts) != 2 {
			return "-ERR GET requires exactly 1 argument\r\n"
		}
		return s.handleGet(parts[1])
		
	case "SET":
		if len(parts) < 3 {
			return "-ERR SET requires at least 2 arguments\r\n"
		}
		key := parts[1]
		size, err := strconv.Atoi(parts[2])
		if err != nil {
			return "-ERR invalid size\r\n"
		}
		
		var ttl time.Duration
		if len(parts) >= 4 {
			ttlSec, err := strconv.Atoi(parts[3])
			if err != nil {
				return "-ERR invalid TTL\r\n"
			}
			ttl = time.Duration(ttlSec) * time.Second
		}
		
		return s.handleSet(key, size, ttl, reader)
		
	case "DELETE", "DEL":
		if len(parts) != 2 {
			return "-ERR DELETE requires exactly 1 argument\r\n"
		}
		return s.handleDelete(parts[1])
		
	case "EXISTS":
		if len(parts) != 2 {
			return "-ERR EXISTS requires exactly 1 argument\r\n"
		}
		return s.handleExists(parts[1])
		
	case "MGET":
		if len(parts) < 2 {
			return "-ERR MGET requires at least 1 key\r\n"
		}
		return s.handleMGet(parts[1:])
		
	case "KEYS":
		return s.handleKeys()
		
	case "STATS":
		return s.handleStats()
		
	case "CLEAR", "FLUSH":
		return s.handleClear()
		
	case "PING":
		return "+PONG\r\n"
		
	case "QUIT", "EXIT":
		return "+OK bye\r\n"
		
	default:
		return fmt.Sprintf("-ERR unknown command '%s'\r\n", command)
	}
}

func (s *Server) handleGet(key string) string {
	value, found := s.cache.Get(key)
	if !found {
		return "$-1\r\n" // Null bulk string (not found)
	}
	
	// Return bulk string format
	return fmt.Sprintf("$%d\r\n%s\r\n", len(value), string(value))
}

func (s *Server) handleSet(key string, size int, ttl time.Duration, reader *bufio.Reader) string {
	// Read the data
	data := make([]byte, size)
	n, err := io.ReadFull(reader, data)
	if err != nil || n != size {
		return "-ERR failed to read data\r\n"
	}
	
	// Read the trailing \r\n
	reader.ReadString('\n')
	
	// Store in cache
	if ttl > 0 {
		err = s.cache.SetWithTTL(key, data, ttl)
	} else {
		err = s.cache.Set(key, data)
	}
	
	if err != nil {
		return fmt.Sprintf("-ERR %v\r\n", err)
	}
	
	return "+OK\r\n"
}

func (s *Server) handleDelete(key string) string {
	if s.cache.Delete(key) {
		return "+OK\r\n"
	}
	return "+NOT_FOUND\r\n"
}

func (s *Server) handleExists(key string) string {
	_, found := s.cache.Get(key)
	if found {
		return ":1\r\n"
	}
	return ":0\r\n"
}

func (s *Server) handleMGet(keys []string) string {
	values := s.cache.GetMulti(keys)
	
	// Return array format
	response := fmt.Sprintf("*%d\r\n", len(keys))
	for _, key := range keys {
		if value, found := values[key]; found {
			response += fmt.Sprintf("$%d\r\n%s\r\n", len(value), string(value))
		} else {
			response += "$-1\r\n" // Null for not found
		}
	}
	
	return response
}

func (s *Server) handleKeys() string {
	keys := s.cache.Keys()
	
	// Return array format
	response := fmt.Sprintf("*%d\r\n", len(keys))
	for _, key := range keys {
		response += fmt.Sprintf("$%d\r\n%s\r\n", len(key), key)
	}
	
	return response
}

func (s *Server) handleStats() string {
	stats := s.cache.Stats()
	
	// Format stats as a simple string response
	response := fmt.Sprintf("*%d\r\n", len(stats)*2)
	for key, value := range stats {
		// Key
		response += fmt.Sprintf("$%d\r\n%s\r\n", len(key), key)
		// Value
		valStr := fmt.Sprintf("%v", value)
		response += fmt.Sprintf("$%d\r\n%s\r\n", len(valStr), valStr)
	}
	
	return response
}

func (s *Server) handleClear() string {
	s.cache.Clear()
	return "+OK\r\n"
}
