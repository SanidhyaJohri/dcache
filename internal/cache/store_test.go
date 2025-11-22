package cache

import (
“fmt”
“sync”
“testing”
“time”
)

func TestNewStore(t *testing.T) {
store := NewStore(100, 10, 5*time.Minute)
if store == nil {
t.Fatal(“Expected store to be created”)
}
if store.capacity != 100 {
t.Errorf(“Expected capacity 100, got %d”, store.capacity)
}
}

func TestSetAndGet(t *testing.T) {
store := NewStore(100, 10, 5*time.Minute)

```
// Test basic set and get
key := "test-key"
value := []byte("test-value")

err := store.Set(key, value)
if err != nil {
	t.Fatalf("Failed to set value: %v", err)
}

retrieved, found := store.Get(key)
if !found {
	t.Fatal("Expected to find key")
}

if string(retrieved) != string(value) {
	t.Errorf("Expected value %s, got %s", value, retrieved)
}

// Test getting non-existent key
_, found = store.Get("non-existent")
if found {
	t.Error("Expected not to find non-existent key")
}
```

}

func TestTTL(t *testing.T) {
store := NewStore(100, 10, 1*time.Second)

```
// Set with short TTL
key := "ttl-key"
value := []byte("ttl-value")

err := store.SetWithTTL(key, value, 100*time.Millisecond)
if err != nil {
	t.Fatalf("Failed to set value with TTL: %v", err)
}

// Should exist immediately
_, found := store.Get(key)
if !found {
	t.Fatal("Expected to find key immediately after setting")
}

// Wait for expiration
time.Sleep(150 * time.Millisecond)

// Should be expired
_, found = store.Get(key)
if found {
	t.Error("Expected key to be expired")
}
```

}

func TestLRUEviction(t *testing.T) {
// Small capacity to test eviction
store := NewStore(3, 10, 5*time.Minute)

```
// Add 3 items (at capacity)
store.Set("key1", []byte("value1"))
store.Set("key2", []byte("value2"))
store.Set("key3", []byte("value3"))

// Access key1 to make it recently used
store.Get("key1")

// Add 4th item, should evict key2 (least recently used)
store.Set("key4", []byte("value4"))

// key1 should still exist (recently accessed)
_, found := store.Get("key1")
if !found {
	t.Error("Expected key1 to exist (recently accessed)")
}

// key2 should be evicted (least recently used)
_, found = store.Get("key2")
if found {
	t.Error("Expected key2 to be evicted")
}

// key3 and key4 should exist
_, found = store.Get("key3")
if !found {
	t.Error("Expected key3 to exist")
}
_, found = store.Get("key4")
if !found {
	t.Error("Expected key4 to exist")
}
```

}

func TestSizeEviction(t *testing.T) {
// 1KB max size
store := NewStore(100, 1.0/1024, 5*time.Minute) // 1KB in MB

```
// Add item that uses most of the space (900 bytes)
largeValue := make([]byte, 900)
store.Set("large", largeValue)

// Add another large item (500 bytes), should evict first
anotherLarge := make([]byte, 500)
store.Set("another", anotherLarge)

// First should be evicted
_, found := store.Get("large")
if found {
	t.Error("Expected large key to be evicted due to size limit")
}

// Second should exist
_, found = store.Get("another")
if !found {
	t.Error("Expected another key to exist")
}
```

}

func TestDelete(t *testing.T) {
store := NewStore(100, 10, 5*time.Minute)

```
store.Set("key1", []byte("value1"))

// Delete existing key
deleted := store.Delete("key1")
if !deleted {
	t.Error("Expected delete to return true for existing key")
}

// Verify it's gone
_, found := store.Get("key1")
if found {
	t.Error("Expected key to be deleted")
}

// Delete non-existent key
deleted = store.Delete("non-existent")
if deleted {
	t.Error("Expected delete to return false for non-existent key")
}
```

}

func TestClear(t *testing.T) {
store := NewStore(100, 10, 5*time.Minute)

```
// Add multiple items
store.Set("key1", []byte("value1"))
store.Set("key2", []byte("value2"))
store.Set("key3", []byte("value3"))

// Clear
store.Clear()

// All should be gone
keys := store.Keys()
if len(keys) != 0 {
	t.Errorf("Expected 0 keys after clear, got %d", len(keys))
}
```

}

func TestGetMulti(t *testing.T) {
store := NewStore(100, 10, 5*time.Minute)

```
// Set multiple keys
store.Set("key1", []byte("value1"))
store.Set("key2", []byte("value2"))
store.Set("key3", []byte("value3"))

// Get multiple including non-existent
keys := []string{"key1", "key2", "non-existent", "key3"}
results := store.GetMulti(keys)

if len(results) != 3 {
	t.Errorf("Expected 3 results, got %d", len(results))
}

if string(results["key1"]) != "value1" {
	t.Error("Unexpected value for key1")
}

if _, exists := results["non-existent"]; exists {
	t.Error("Non-existent key should not be in results")
}
```

}

func TestConcurrentAccess(t *testing.T) {
store := NewStore(1000, 10, 5*time.Minute)

```
var wg sync.WaitGroup
numGoroutines := 10
numOperations := 100

// Concurrent writes
for i := 0; i < numGoroutines; i++ {
	wg.Add(1)
	go func(id int) {
		defer wg.Done()
		for j := 0; j < numOperations; j++ {
			key := fmt.Sprintf("key-%d-%d", id, j)
			value := []byte(fmt.Sprintf("value-%d-%d", id, j))
			store.Set(key, value)
		}
	}(i)
}

// Concurrent reads
for i := 0; i < numGoroutines; i++ {
	wg.Add(1)
	go func(id int) {
		defer wg.Done()
		for j := 0; j < numOperations; j++ {
			key := fmt.Sprintf("key-%d-%d", id, j)
			store.Get(key)
		}
	}(i)
}

wg.Wait()

// Verify stats
stats := store.Stats()
if stats["items"].(int) == 0 {
	t.Error("Expected items in cache after concurrent operations")
}
```

}

func TestStats(t *testing.T) {
store := NewStore(100, 10, 5*time.Minute)

```
// Add some items
store.Set("key1", []byte("value1"))
store.Set("key2", []byte("value2"))

// Get one (hit)
store.Get("key1")

// Get non-existent (miss)
store.Get("non-existent")

stats := store.Stats()

if stats["items"].(int) != 2 {
	t.Errorf("Expected 2 items, got %v", stats["items"])
}

if stats["hits"].(int64) != 1 {
	t.Errorf("Expected 1 hit, got %v", stats["hits"])
}

if stats["misses"].(int64) != 1 {
	t.Errorf("Expected 1 miss, got %v", stats["misses"])
}

hitRate := stats["hit_rate"].(float64)
expectedHitRate := 0.5
if hitRate != expectedHitRate {
	t.Errorf("Expected hit rate %f, got %f", expectedHitRate, hitRate)
}
```

}

func BenchmarkSet(b *testing.B) {
store := NewStore(10000, 100, 5*time.Minute)
value := []byte(“benchmark-value”)

```
b.ResetTimer()
for i := 0; i < b.N; i++ {
	key := fmt.Sprintf("key-%d", i)
	store.Set(key, value)
}
```

}

func BenchmarkGet(b *testing.B) {
store := NewStore(10000, 100, 5*time.Minute)

```
// Pre-populate
for i := 0; i < 1000; i++ {
	key := fmt.Sprintf("key-%d", i)
	store.Set(key, []byte("value"))
}

b.ResetTimer()
for i := 0; i < b.N; i++ {
	key := fmt.Sprintf("key-%d", i%1000)
	store.Get(key)
}
```

}

func BenchmarkConcurrentGetSet(b *testing.B) {
store := NewStore(10000, 100, 5*time.Minute)

```
b.RunParallel(func(pb *testing.PB) {
	i := 0
	for pb.Next() {
		key := fmt.Sprintf("key-%d", i%1000)
		if i%2 == 0 {
			store.Set(key, []byte("value"))
		} else {
			store.Get(key)
		}
		i++
	}
})
```

}