package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

const version = "1.0.2"

func main() {
	// Get configuration from environment
	apiToken := os.Getenv("ARCPOINT_API_TOKEN")
	apiURL := os.Getenv("ARCPOINT_API_URL")

	// Default to production if not specified
	if apiURL == "" {
		apiURL = "https://mcp.arcpoint.ai"
	}

	// Validate required configuration
	if apiToken == "" {
		fmt.Fprintln(os.Stderr, "Error: ARCPOINT_API_TOKEN environment variable is required")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Get your API token from https://arcpoint.ai/settings/tokens")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Configuration example:")
		fmt.Fprintln(os.Stderr, `{`)
		fmt.Fprintln(os.Stderr, `  "mcpServers": {`)
		fmt.Fprintln(os.Stderr, `    "arcpoint": {`)
		fmt.Fprintln(os.Stderr, `      "command": "arcpoint-mcp",`)
		fmt.Fprintln(os.Stderr, `      "args": [],`)
		fmt.Fprintln(os.Stderr, `      "env": {`)
		fmt.Fprintln(os.Stderr, `        "ARCPOINT_API_TOKEN": "apt_your_token_here"`)
		fmt.Fprintln(os.Stderr, `      }`)
		fmt.Fprintln(os.Stderr, `    }`)
		fmt.Fprintln(os.Stderr, `  }`)
		fmt.Fprintln(os.Stderr, `}`)
		os.Exit(1)
	}

	// Ensure URL doesn't have trailing slash
	apiURL = strings.TrimSuffix(apiURL, "/")

	// Log startup to stderr (stdout is for JSON-RPC)
	log.SetOutput(os.Stderr)
	log.Printf("Arcpoint MCP Client v%s", version)
	log.Printf("Connecting to: %s", apiURL)

	// Set up context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down...")
		cancel()
	}()

	// Start the SSE client
	client := NewSSEClient(apiURL, apiToken)
	if err := client.Run(ctx); err != nil {
		log.Fatalf("Client error: %v", err)
	}
}

// SSEClient handles the SSE connection and stdio proxying
type SSEClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
	sessionID  string
	mu         sync.RWMutex
}

// NewSSEClient creates a new SSE client
func NewSSEClient(baseURL, token string) *SSEClient {
	return &SSEClient{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 0, // No timeout for SSE connection
			Transport: &http.Transport{
				MaxIdleConns:        10,
				IdleConnTimeout:     90 * time.Second,
				DisableCompression:  true, // SSE doesn't work well with compression
				DisableKeepAlives:   false,
				MaxIdleConnsPerHost: 5,
			},
		},
	}
}

// Run starts the SSE connection and stdio proxy
func (c *SSEClient) Run(ctx context.Context) error {
	// Start reading from stdin and sending messages
	go c.readStdin(ctx)

	// Keep reconnecting SSE connection if it drops
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		log.Println("Connecting to SSE stream...")
		err := c.connectSSE(ctx)
		if err != nil {
			if ctx.Err() != nil {
				// Context cancelled, exit cleanly
				return nil
			}
			log.Printf("SSE connection error: %v, reconnecting in 2s...", err)
			time.Sleep(2 * time.Second)
			continue
		}

		// Connection closed cleanly, try to reconnect
		if ctx.Err() == nil {
			log.Println("SSE connection closed, reconnecting in 2s...")
			time.Sleep(2 * time.Second)
		}
	}
}

// connectSSE establishes and maintains the SSE connection
func (c *SSEClient) connectSSE(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/sse", nil)
	if err != nil {
		return fmt.Errorf("failed to create SSE request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("User-Agent", fmt.Sprintf("arcpoint-mcp-client/%s", version))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("SSE connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("SSE connection failed with status %d: %s", resp.StatusCode, string(body))
	}

	log.Println("SSE stream connected")

	// Parse SSE events
	scanner := bufio.NewScanner(resp.Body)
	var eventType string
	var eventData []string

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			// Empty line marks end of event
			if eventType == "endpoint" && len(eventData) > 0 {
				// Extract session ID from endpoint URL
				endpointData := strings.Join(eventData, "\n")
				c.extractSessionID(endpointData)
				log.Printf("Session established: %s", c.getSessionID())
			} else if eventType == "message" && len(eventData) > 0 {
				// Forward message to stdout
				messageData := strings.Join(eventData, "\n")
				fmt.Println(messageData)
			}
			eventType = ""
			eventData = nil
			continue
		}

		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			eventData = append(eventData, data)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading SSE stream: %w", err)
	}

	return nil
}

// extractSessionID parses the endpoint URL to extract the session ID
func (c *SSEClient) extractSessionID(endpoint string) {
	// Endpoint format: "/message?sessionId=xxx"
	parts := strings.Split(endpoint, "sessionId=")
	if len(parts) == 2 {
		sessionID := strings.TrimSpace(parts[1])
		c.setSessionID(sessionID)
	}
}

// setSessionID safely sets the session ID
func (c *SSEClient) setSessionID(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessionID = id
}

// getSessionID safely gets the session ID
func (c *SSEClient) getSessionID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sessionID
}

// readStdin reads JSON-RPC messages from stdin and sends them to the server
func (c *SSEClient) readStdin(ctx context.Context) {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024) // Support large messages

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Wait for session ID if not available yet
		sessionID := c.getSessionID()
		if sessionID == "" {
			// Try a few times with backoff
			for i := 0; i < 10 && sessionID == ""; i++ {
				time.Sleep(100 * time.Millisecond)
				sessionID = c.getSessionID()
			}
			if sessionID == "" {
				log.Println("Warning: Session not established yet, attempting to send anyway")
			}
		}

		// Send message via POST
		messageURL := c.baseURL + "/message"
		if sessionID != "" {
			messageURL += "?sessionId=" + sessionID
		}

		req, err := http.NewRequestWithContext(ctx, "POST", messageURL, bytes.NewReader(line))
		if err != nil {
			log.Printf("Failed to create request: %v", err)
			continue
		}

		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", fmt.Sprintf("arcpoint-mcp-client/%s", version))

		// Create a new client with timeout for message sending
		msgClient := &http.Client{Timeout: 30 * time.Second}
		resp, err := msgClient.Do(req)
		if err != nil {
			log.Printf("Request failed: %v", err)
			c.writeError(-32603, fmt.Sprintf("Connection error: %s", err.Error()))
			continue
		}

		// For SSE transport, we expect 202 Accepted (response comes via SSE)
		// or 200 OK with immediate response
		if resp.StatusCode == http.StatusAccepted {
			resp.Body.Close()
			// Response will come via SSE
			continue
		}

		// Read immediate response
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			log.Printf("Failed to read response: %v", err)
			c.writeError(-32603, "Failed to read response")
			continue
		}

		if resp.StatusCode != http.StatusOK {
			log.Printf("HTTP error %d: %s", resp.StatusCode, string(body))
			c.writeHTTPError(resp.StatusCode)
			continue
		}

		// Forward immediate response to stdout
		fmt.Println(string(body))
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading stdin: %v", err)
	}
}

// writeError writes a JSON-RPC error to stdout
func (c *SSEClient) writeError(code int, message string) {
	err := map[string]interface{}{
		"jsonrpc": "2.0",
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}
	data, _ := json.Marshal(err)
	fmt.Println(string(data))
}

// writeHTTPError maps HTTP errors to JSON-RPC errors
func (c *SSEClient) writeHTTPError(statusCode int) {
	var errorCode int
	var errorMessage string

	switch statusCode {
	case http.StatusUnauthorized:
		errorCode = -32001
		errorMessage = "Invalid API token"
	case http.StatusForbidden:
		errorCode = -32002
		errorMessage = "Access denied"
	case http.StatusTooManyRequests:
		errorCode = -32003
		errorMessage = "Rate limit exceeded"
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		errorCode = -32004
		errorMessage = "Service temporarily unavailable"
	default:
		errorCode = -32603
		errorMessage = fmt.Sprintf("Server error: %d", statusCode)
	}

	c.writeError(errorCode, errorMessage)
}
