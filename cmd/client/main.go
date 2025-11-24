package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	var serverAddr string
	flag.StringVar(&serverAddr, "server", "localhost:6379", "Server address")
	flag.Parse()
	
	// Connect to server
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		fmt.Printf("Failed to connect to %s: %v\n", serverAddr, err)
		os.Exit(1)
	}
	defer conn.Close()
	
	fmt.Printf("Connected to %s\n", serverAddr)
	
	// Read welcome message
	reader := bufio.NewReader(conn)
	welcome, _ := reader.ReadString('\n')
	fmt.Print(welcome)
	
	// Interactive mode
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("\nDCache Client. Commands:")
	fmt.Println("  GET key")
	fmt.Println("  SET key value [ttl_seconds]")
	fmt.Println("  DELETE key")
	fmt.Println("  EXISTS key")
	fmt.Println("  KEYS")
	fmt.Println("  STATS")
	fmt.Println("  CLEAR")
	fmt.Println("  PING")
	fmt.Println("  QUIT")
	fmt.Println()
	
	for {
		fmt.Print("dcache> ")
		if !scanner.Scan() {
			break
		}
		
		input := scanner.Text()
		if input == "" {
			continue
		}
		
		parts := strings.Fields(input)
		if len(parts) == 0 {
			continue
		}
		
		command := strings.ToUpper(parts[0])
		
		// Handle SET specially (multi-line)
		if command == "SET" && len(parts) >= 3 {
			key := parts[1]
			value := strings.Join(parts[2:len(parts)], " ")
			
			// Check for TTL
			ttl := ""
			if len(parts) >= 4 {
				// Last part might be TTL
				lastPart := parts[len(parts)-1]
				if _, err := fmt.Sscanf(lastPart, "%d", &ttl); err == nil {
					value = strings.Join(parts[2:len(parts)-1], " ")
				} else {
					ttl = ""
				}
			}
			
			// Send SET command
			if ttl != "" {
				fmt.Fprintf(conn, "SET %s %d %s\r\n", key, len(value), ttl)
			} else {
				fmt.Fprintf(conn, "SET %s %d\r\n", key, len(value))
			}
			fmt.Fprintf(conn, "%s\r\n", value)
			
		} else if command == "QUIT" || command == "EXIT" {
			fmt.Fprintf(conn, "%s\r\n", command)
			response, _ := reader.ReadString('\n')
			fmt.Print(response)
			break
		} else {
			// Send command as-is
			fmt.Fprintf(conn, "%s\r\n", input)
		}
		
		// Read response
		response := readResponse(reader)
		fmt.Print(response)
		
		// If QUIT command, exit
		if command == "QUIT" || command == "EXIT" {
			break
		}
	}
	
	fmt.Println("\nGoodbye!")
}

func readResponse(reader *bufio.Reader) string {
	line, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Sprintf("Error reading response: %v\n", err)
	}
	
	// Parse response based on first character (Redis-like protocol)
	if len(line) == 0 {
		return ""
	}
	
	switch line[0] {
	case '+':
		// Simple string
		return line[1:]
		
	case '-':
		// Error
		return fmt.Sprintf("Error: %s", line[1:])
		
	case '$':
		// Bulk string
		var length int
		fmt.Sscanf(line[1:], "%d", &length)
		if length == -1 {
			return "(nil)\n"
		}
		data := make([]byte, length)
		reader.Read(data)
		reader.ReadString('\n') // Read trailing \r\n
		return fmt.Sprintf("%s\n", string(data))
		
	case ':':
		// Integer
		return line[1:]
		
	case '*':
		// Array
		var count int
		fmt.Sscanf(line[1:], "%d", &count)
		result := fmt.Sprintf("Array with %d elements:\n", count)
		for i := 0; i < count; i++ {
			element := readResponse(reader)
			result += fmt.Sprintf("  [%d]: %s", i, element)
		}
		return result
		
	default:
		return line
	}
}
