package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const version = "1.0.0"

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

	// Create HTTP client with reasonable timeouts
	client := &http.Client{
		Timeout: 5 * time.Minute, // Long timeout for streaming responses
		Transport: &http.Transport{
			MaxIdleConns:        10,
			IdleConnTimeout:     90 * time.Second,
			DisableCompression:  false,
			DisableKeepAlives:   false,
			MaxIdleConnsPerHost: 5,
		},
	}

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

	// Start the stdio-to-HTTP proxy
	if err := proxyStdioToHTTP(ctx, client, apiURL, apiToken); err != nil {
		log.Fatalf("Proxy error: %v", err)
	}
}

// proxyStdioToHTTP reads JSON-RPC messages from stdin and forwards them to the MCP HTTP server
func proxyStdioToHTTP(ctx context.Context, client *http.Client, baseURL, token string) error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024) // Support large messages (up to 10MB)

	messageEndpoint := baseURL + "/message"

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Forward the JSON-RPC message to the MCP server
		req, err := http.NewRequestWithContext(ctx, "POST", messageEndpoint, strings.NewReader(string(line)))
		if err != nil {
			log.Printf("Failed to create request: %v", err)
			continue
		}

		// Add authentication and headers
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", fmt.Sprintf("arcpoint-mcp-client/%s", version))

		// Send request
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Request failed: %v", err)
			// Write error response to stdout
			fmt.Fprintf(os.Stdout, `{"jsonrpc":"2.0","error":{"code":-32603,"message":"Connection error: %s"}}`+"\n", err.Error())
			continue
		}

		// Read response body
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			log.Printf("Failed to read response: %v", err)
			fmt.Fprintf(os.Stdout, `{"jsonrpc":"2.0","error":{"code":-32603,"message":"Failed to read response"}}`+"\n")
			continue
		}

		// Check for HTTP errors
		if resp.StatusCode != http.StatusOK {
			log.Printf("HTTP error %d: %s", resp.StatusCode, string(body))

			// Map HTTP errors to JSON-RPC errors
			var errorCode int
			var errorMessage string

			switch resp.StatusCode {
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
				errorMessage = fmt.Sprintf("Server error: %d", resp.StatusCode)
			}

			fmt.Fprintf(os.Stdout, `{"jsonrpc":"2.0","error":{"code":%d,"message":"%s"}}`+"\n", errorCode, errorMessage)
			continue
		}

		// Forward successful response to stdout
		os.Stdout.Write(body)
		os.Stdout.Write([]byte("\n"))
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading stdin: %w", err)
	}

	return nil
}
