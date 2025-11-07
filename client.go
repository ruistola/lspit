package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// LSPClient manages communication with gopls
type LSPClient struct {
	cmd           *exec.Cmd
	stdin         io.WriteCloser
	stdout        io.ReadCloser
	stderr        io.ReadCloser
	workspaceRoot string
	nextID        int
	mu            sync.Mutex
	responses     map[int]chan json.RawMessage
	done          chan struct{}
}

// NewLSPClient creates and starts a new LSP client
func NewLSPClient(workspaceRoot string) (*LSPClient, error) {
	cmd := exec.Command("gopls")
	
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdin pipe: %w", err)
	}
	
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start gopls: %w", err)
	}

	client := &LSPClient{
		cmd:           cmd,
		stdin:         stdin,
		stdout:        stdout,
		stderr:        stderr,
		workspaceRoot: workspaceRoot,
		nextID:        1,
		responses:     make(map[int]chan json.RawMessage),
		done:          make(chan struct{}),
	}

	// Start reading responses in background
	go client.readLoop()
	
	return client, nil
}

// Close shuts down the LSP client
func (c *LSPClient) Close() error {
	// Send shutdown request
	shutdownReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      c.getNextID(),
		"method":  "shutdown",
		"params":  nil,
	}
	c.sendRequest(shutdownReq)
	
	// Send exit notification
	exitNotif := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "exit",
	}
	c.sendNotification(exitNotif)
	
	close(c.done)
	c.stdin.Close()
	return c.cmd.Wait()
}

// Initialize performs LSP initialization handshake
func (c *LSPClient) Initialize() error {
	// Send initialize request
	initReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      c.getNextID(),
		"method":  "initialize",
		"params": map[string]interface{}{
			"processId": nil,
			"rootUri":   "file://" + c.workspaceRoot,
			"capabilities": map[string]interface{}{
				"textDocument": map[string]interface{}{
					"hover": map[string]interface{}{
						"contentFormat": []string{"plaintext", "markdown"},
					},
					"definition": map[string]interface{}{
						"linkSupport": true,
					},
					"references": map[string]interface{}{},
				},
			},
		},
	}
	
	id := initReq["id"].(int)
	respChan := c.registerResponse(id)
	
	if err := c.sendRequest(initReq); err != nil {
		return fmt.Errorf("failed to send initialize request: %w", err)
	}

	// Wait for initialize response
	<-respChan
	
	// Send initialized notification
	initializedNotif := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "initialized",
		"params":  map[string]interface{}{},
	}
	
	return c.sendNotification(initializedNotif)
}

// Hover gets type information at the specified position
func (c *LSPClient) Hover(filePath string, line, col int) error {
	// Open the file and send didOpen notification
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	
	didOpenNotif := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "textDocument/didOpen",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri":        "file://" + filePath,
				"languageId": "go",
				"version":    1,
				"text":       string(content),
			},
		},
	}
	
	if err := c.sendNotification(didOpenNotif); err != nil {
		return err
	}
	
	// Send hover request
	hoverReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      c.getNextID(),
		"method":  "textDocument/hover",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file://" + filePath,
			},
			"position": map[string]interface{}{
				"line":      line - 1, // LSP uses 0-indexed
				"character": col - 1,
			},
		},
	}
	
	id := hoverReq["id"].(int)
	respChan := c.registerResponse(id)
	
	if err := c.sendRequest(hoverReq); err != nil {
		return fmt.Errorf("failed to send hover request: %w", err)
	}

	// Wait for response
	resp := <-respChan
	
	// Parse and display hover result
	var result struct {
		Contents interface{} `json:"contents"`
		Range    interface{} `json:"range"`
	}
	
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("failed to parse hover response: %w", err)
	}
	
	// Display the hover information
	return c.displayHoverInfo(result.Contents)
}

// Definition finds the definition of the symbol at the specified position
func (c *LSPClient) Definition(filePath string, line, col int) error {
	// Open the file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	
	didOpenNotif := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "textDocument/didOpen",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri":        "file://" + filePath,
				"languageId": "go",
				"version":    1,
				"text":       string(content),
			},
		},
	}
	c.sendNotification(didOpenNotif)
	
	// Send definition request
	defReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      c.getNextID(),
		"method":  "textDocument/definition",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file://" + filePath,
			},
			"position": map[string]interface{}{
				"line":      line - 1,
				"character": col - 1,
			},
		},
	}
	
	id := defReq["id"].(int)
	respChan := c.registerResponse(id)

	if err := c.sendRequest(defReq); err != nil {
		return fmt.Errorf("failed to send definition request: %w", err)
	}
	
	resp := <-respChan
	
	// Parse and display definition result
	var locations []struct {
		URI   string `json:"uri"`
		Range struct {
			Start struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			} `json:"start"`
		} `json:"range"`
	}
	
	if err := json.Unmarshal(resp, &locations); err != nil {
		return fmt.Errorf("failed to parse definition response: %w", err)
	}
	
	return c.displayLocations(locations)
}

// References finds all references to the symbol at the specified position
func (c *LSPClient) References(filePath string, line, col int) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	didOpenNotif := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "textDocument/didOpen",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri":        "file://" + filePath,
				"languageId": "go",
				"version":    1,
				"text":       string(content),
			},
		},
	}
	c.sendNotification(didOpenNotif)
	
	// Send references request
	refsReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      c.getNextID(),
		"method":  "textDocument/references",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file://" + filePath,
			},
			"position": map[string]interface{}{
				"line":      line - 1,
				"character": col - 1,
			},
			"context": map[string]interface{}{
				"includeDeclaration": true,
			},
		},
	}
	
	id := refsReq["id"].(int)
	respChan := c.registerResponse(id)
	
	if err := c.sendRequest(refsReq); err != nil {
		return fmt.Errorf("failed to send references request: %w", err)
	}
	
	resp := <-respChan
	
	// Parse and display references result
	var locations []struct {
		URI   string `json:"uri"`
		Range struct {
			Start struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			} `json:"start"`
		} `json:"range"`
	}
	
	if err := json.Unmarshal(resp, &locations); err != nil {
		return fmt.Errorf("failed to parse references response: %w", err)
	}
	
	return c.displayLocations(locations)
}

// Helper methods for protocol communication

func (c *LSPClient) getNextID() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	id := c.nextID
	c.nextID++
	return id
}

func (c *LSPClient) registerResponse(id int) chan json.RawMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	ch := make(chan json.RawMessage, 1)
	c.responses[id] = ch
	return ch
}

func (c *LSPClient) sendRequest(req map[string]interface{}) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	return c.sendMessage(data)
}

func (c *LSPClient) sendNotification(notif map[string]interface{}) error {
	data, err := json.Marshal(notif)
	if err != nil {
		return err
	}
	return c.sendMessage(data)
}

func (c *LSPClient) sendMessage(data []byte) error {
	msg := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(data), data)
	_, err := c.stdin.Write([]byte(msg))
	return err
}

func (c *LSPClient) readLoop() {
	reader := bufio.NewReader(c.stdout)
	
	for {
		select {
		case <-c.done:
			return
		default:
		}
		
		// Read Content-Length header
		var contentLength int
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			
			line = strings.TrimSpace(line)
			if line == "" {
				break
			}
			
			if strings.HasPrefix(line, "Content-Length:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					contentLength, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
				}
			}
		}
		
		// Read message content
		content := make([]byte, contentLength)
		if _, err := io.ReadFull(reader, content); err != nil {
			return
		}
		
		// Parse message
		var msg struct {
			ID     *int            `json:"id"`
			Result json.RawMessage `json:"result"`
		}
		
		if err := json.Unmarshal(content, &msg); err != nil {
			continue
		}
		
		// Dispatch response to waiting goroutine
		if msg.ID != nil {
			c.mu.Lock()
			if ch, ok := c.responses[*msg.ID]; ok {
				ch <- msg.Result
				delete(c.responses, *msg.ID)
			}
			c.mu.Unlock()
		}
	}
}

// Display methods for formatting output

func (c *LSPClient) displayHoverInfo(contents interface{}) error {
	if contents == nil {
		fmt.Println("No hover information available")
		return nil
	}
	
	// Handle different content formats
	switch v := contents.(type) {
	case string:
		fmt.Println(v)
	case map[string]interface{}:
		if value, ok := v["value"].(string); ok {
			// Clean up markdown code blocks
			value = strings.TrimPrefix(value, "```go\n")
			value = strings.TrimPrefix(value, "```\n")
			value = strings.TrimSuffix(value, "\n```")
			fmt.Println(value)
		}
	case []interface{}:
		for _, item := range v {
			if str, ok := item.(string); ok {
				fmt.Println(str)
			} else if m, ok := item.(map[string]interface{}); ok {
				if value, ok := m["value"].(string); ok {
					fmt.Println(value)
				}
			}
		}
	}
	
	return nil
}

func (c *LSPClient) displayLocations(locations []struct {
	URI   string `json:"uri"`
	Range struct {
		Start struct {
			Line      int `json:"line"`
			Character int `json:"character"`
		} `json:"start"`
	} `json:"range"`
}) error {
	if len(locations) == 0 {
		fmt.Println("No locations found")
		return nil
	}
	
	for _, loc := range locations {
		// Convert file:// URI to path
		path := strings.TrimPrefix(loc.URI, "file://")
		// Convert 0-indexed to 1-indexed for display
		line := loc.Range.Start.Line + 1
		col := loc.Range.Start.Character + 1
		fmt.Printf("%s:%d:%d\n", path, line, col)
	}
	
	return nil
}
