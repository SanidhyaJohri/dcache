package main

import (
    "fmt"
    "log"
    "github.com/sjohri/dcache/pkg/client"
)

func main() {
    // Initialize smart client with seed nodes
    c := client.NewSmartClient([]string{
        "localhost:8001",
        "localhost:8002", 
        "localhost:8003",
    })
    
    // Test operations
    for i := 0; i < 10; i++ {
        key := fmt.Sprintf("test-key-%d", i)
        value := fmt.Sprintf("test-value-%d", i)
        
        // Set
        if err := c.Set(key, []byte(value)); err != nil {
            log.Printf("Failed to set %s: %v", key, err)
        }
        
        // Get
        if val, err := c.Get(key); err == nil {
            fmt.Printf("%s = %s\n", key, string(val))
        }
    }
    
    // Check distribution
    dist := c.GetNodeDistribution(1000)
    fmt.Printf("\nKey Distribution:\n")
    for node, count := range dist {
        fmt.Printf("%s: %d keys (%.1f%%)\n", 
            node, count, float64(count)*100/1000)
    }
}
