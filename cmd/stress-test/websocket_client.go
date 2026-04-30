package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketClient handles WebSocket connections
type WebSocketClient struct {
	URL    string
	conn   *websocket.Conn
	mu     sync.Mutex
	closed bool
}

// NewWebSocketClient creates a new WebSocket client
func NewWebSocketClient(url string) *WebSocketClient {
	return &WebSocketClient{
		URL: url,
	}
}

// Connect establishes a WebSocket connection
func (c *WebSocketClient) Connect(ctx context.Context, token string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return fmt.Errorf("already connected")
	}

	// Add token to URL
	url := fmt.Sprintf("%s?token=%s", c.URL, token)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, url, nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.conn = conn
	c.closed = false

	// Start ping/pong handler
	go c.handlePingPong()

	return nil
}

// Disconnect closes the WebSocket connection
func (c *WebSocketClient) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil
	}

	c.closed = true
	err := c.conn.Close()
	c.conn = nil
	return err
}

// SendMessage sends a message through the WebSocket
func (c *WebSocketClient) SendMessage(message interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// ReceiveMessage receives a message from the WebSocket
func (c *WebSocketClient) ReceiveMessage() (map[string]interface{}, error) {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	_, message, err := conn.ReadMessage()
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(message, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return result, nil
}

// handlePingPong handles ping/pong messages to keep the connection alive
func (c *WebSocketClient) handlePingPong() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		if c.closed || c.conn == nil {
			c.mu.Unlock()
			return
		}

		// Send ping message
		ping := map[string]string{
			"event": "ping",
		}
		data, err := json.Marshal(ping)
		if err != nil {
			log.Printf("Failed to marshal ping message: %v", err)
			c.mu.Unlock()
			return
		}
		if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("Failed to send ping: %v", err)
			c.mu.Unlock()
			return
		}
		c.mu.Unlock()
	}
}

// WebSocketMessageSender manages message sending through WebSocket
type WebSocketMessageSender struct {
	client *WebSocketClient
}

// NewWebSocketMessageSender creates a new WebSocket message sender
func NewWebSocketMessageSender(wsURL string, token string) (*WebSocketMessageSender, error) {
	client := NewWebSocketClient(wsURL)
	if err := client.Connect(context.Background(), token); err != nil {
		return nil, err
	}

	// Wait for connect confirmation
	time.Sleep(1 * time.Second)

	return &WebSocketMessageSender{
		client: client,
	}, nil
}

// SendTextMessage sends a text message through WebSocket
func (s *WebSocketMessageSender) SendTextMessage(talkMode int, receiverID int, content string) error {
	message := map[string]interface{}{
		"event": "im.message.text",
		"payload": map[string]interface{}{
			"talk_mode":   talkMode,
			"receiver_id": receiverID,
			"content":     content,
		},
	}

	return s.client.SendMessage(message)
}

// Close closes the WebSocket connection
func (s *WebSocketMessageSender) Close() error {
	return s.client.Disconnect()
}
