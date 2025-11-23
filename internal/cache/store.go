package cache

import (
	"container/list"
	"sync"
	"time"
)

// Item represents a cache entry
type Item struct {
	Key        string
	Value      []byte
	Expiry     time.Time
	AccessTime time.Time
	element    *list.Element // pointer to LRU list element
}

// Store represents the main cache store with LRU eviction
type Store struct {
	mu       sync.RWMutex
	items    map[string]*Item
	lruList  *list.List
	capacity int       // max number of items
	maxSize  int64     // max size in bytes
	currSize int64     // current size in bytes
	ttl      time.Duration
	
	// Metrics
	hits     int64
	misses   int64
	evictions int64
}

// NewStore creates a new cache store
func NewStore(capacity int, maxSizeMB int64, defaultTTL time.Duration) *Store {
	s := &Store{
		items:    make(map[string]*Item),
		lruList:  list.New(),
		capacity: capacity,
		maxSize:  maxSizeMB * 1024 * 1024, // Convert MB to bytes
		ttl:      defaultTTL,
	}
	
	// Start background cleanup goroutine for expired items
	go s.cleanupExpired()
	
	return s
}

// Get retrieves a value from cache
func (s *Store) Get(key string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	item, exists := s.items[key]
	if !exists {
		s.misses++
		return nil, false
	}
	
	// Check if expired
	if !item.Expiry.IsZero() && time.Now().After(item.Expiry) {
		s.removeItem(key)
		s.misses++
		return nil, false
	}
	
	// Move to front (most recently used)
	s.lruList.MoveToFront(item.element)
	item.AccessTime = time.Now()
	
	s.hits++
	return item.Value, true
}

// Set adds or updates a value in cache with default TTL
func (s *Store) Set(key string, value []byte) error {
	return s.SetWithTTL(key, value, s.ttl)
}

// SetWithTTL adds or updates a value with specific TTL
func (s *Store) SetWithTTL(key string, value []byte, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	valueSize := int64(len(value))
	
	// Check if updating existing item
	if existingItem, exists := s.items[key]; exists {
		// Update size tracking
		s.currSize = s.currSize - int64(len(existingItem.Value)) + valueSize
		
		// Update the item
		existingItem.Value = value
		existingItem.AccessTime = time.Now()
		if ttl > 0 {
			existingItem.Expiry = time.Now().Add(ttl)
		} else {
			existingItem.Expiry = time.Time{}
		}
		
		// Move to front
		s.lruList.MoveToFront(existingItem.element)
		return nil
	}
	
	// Check capacity and size limits
	for s.needsEviction(valueSize) {
		s.evictOldest()
	}
	
	// Create new item
	item := &Item{
		Key:        key,
		Value:      value,
		AccessTime: time.Now(),
	}
	
	if ttl > 0 {
		item.Expiry = time.Now().Add(ttl)
	}
	
	// Add to LRU list (at front)
	element := s.lruList.PushFront(key)
	item.element = element
	
	// Add to map
	s.items[key] = item
	s.currSize += valueSize
	
	return nil
}

// Delete removes an item from cache
func (s *Store) Delete(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	return s.removeItem(key)
}

// Clear removes all items from cache
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.items = make(map[string]*Item)
	s.lruList.Init()
	s.currSize = 0
}

// Stats returns cache statistics
func (s *Store) Stats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	total := s.hits + s.misses
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(s.hits) / float64(total)
	}
	
	return map[string]interface{}{
		"items":      len(s.items),
		"size_bytes": s.currSize,
		"size_mb":    float64(s.currSize) / (1024 * 1024),
		"capacity":   s.capacity,
		"max_size_mb": s.maxSize / (1024 * 1024),
		"hits":       s.hits,
		"misses":     s.misses,
		"hit_rate":   hitRate,
		"evictions":  s.evictions,
	}
}

// Internal methods

func (s *Store) needsEviction(incomingSize int64) bool {
	if s.capacity > 0 && len(s.items) >= s.capacity {
		return true
	}
	if s.maxSize > 0 && s.currSize+incomingSize > s.maxSize {
		return true
	}
	return false
}

func (s *Store) evictOldest() {
	element := s.lruList.Back()
	if element == nil {
		return
	}
	
	key := element.Value.(string)
	s.removeItem(key)
	s.evictions++
}

func (s *Store) removeItem(key string) bool {
	item, exists := s.items[key]
	if !exists {
		return false
	}
	
	// Remove from LRU list
	s.lruList.Remove(item.element)
	
	// Update size
	s.currSize -= int64(len(item.Value))
	
	// Remove from map
	delete(s.items, key)
	
	return true
}

// Background cleanup for expired items
func (s *Store) cleanupExpired() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		s.mu.Lock()
		
		// Find and remove expired items
		var toRemove []string
		for key, item := range s.items {
			if !item.Expiry.IsZero() && time.Now().After(item.Expiry) {
				toRemove = append(toRemove, key)
			}
		}
		
		for _, key := range toRemove {
			s.removeItem(key)
		}
		
		s.mu.Unlock()
	}
}

// GetMulti retrieves multiple keys at once
func (s *Store) GetMulti(keys []string) map[string][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	result := make(map[string][]byte)
	for _, key := range keys {
		if item, exists := s.items[key]; exists {
			if item.Expiry.IsZero() || time.Now().Before(item.Expiry) {
				result[key] = item.Value
				s.lruList.MoveToFront(item.element)
				item.AccessTime = time.Now()
				s.hits++
			} else {
				s.removeItem(key)
				s.misses++
			}
		} else {
			s.misses++
		}
	}
	
	return result
}

// Keys returns all keys in cache (for debugging)
func (s *Store) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	keys := make([]string, 0, len(s.items))
	for key := range s.items {
		keys = append(keys, key)
	}
	return keys
}
