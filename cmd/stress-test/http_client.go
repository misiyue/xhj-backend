package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPClient handles all HTTP API operations
type HTTPClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewHTTPClient creates a new HTTP client
func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Register creates a new user account
func (c *HTTPClient) Register(ctx context.Context, username, password string) error {
	payload := map[string]interface{}{
		"nickname": username,
		"mobile":   username,
		"password": password,
		"platform": "web",
	}

	_, err := c.doRequest(ctx, "POST", "/api/v1/auth/register", payload, "")
	return err
}

// Login authenticates a user and returns a token
func (c *HTTPClient) Login(ctx context.Context, username, password string) (string, int, error) {
	payload := map[string]interface{}{
		"mobile":   username,
		"password": password,
		"platform": "web",
	}

	resp, err := c.doRequest(ctx, "POST", "/api/v1/auth/login", payload, "")
	if err != nil {
		return "", 0, err
	}

	token, ok := resp["access_token"].(string)
	if !ok {
		return "", 0, fmt.Errorf("invalid response: missing access_token")
	}

	// Get user info to retrieve user ID
	userResp, err := c.doRequest(ctx, "POST", "/api/v1/user/detail", nil, token)
	if err != nil {
		return "", 0, err
	}

	userID := 0
	if id, ok := userResp["id"].(float64); ok {
		userID = int(id)
	}

	return token, userID, nil
}

// RegisterAndLogin creates a user and logs them in
func (c *HTTPClient) RegisterAndLogin(ctx context.Context, username, password string) (*User, error) {
	// Try to register
	_ = c.Register(ctx, username, password)

	// Login (might work even if register failed due to duplicate)
	token, userID, err := c.Login(ctx, username, password)
	if err != nil {
		return nil, err
	}

	return &User{
		ID:       userID,
		Username: username,
		Token:    token,
	}, nil
}

// SendFriendRequest sends a friend request to another user
func (c *HTTPClient) SendFriendRequest(ctx context.Context, token string, friendID int) error {
	payload := map[string]interface{}{
		"user_id":      friendID,
		"remark":       fmt.Sprintf("Friend_%d", friendID),
		"apply_reason": "Stress test friend request",
	}

	_, err := c.doRequest(ctx, "POST", "/api/v1/contact-apply/create", payload, token)
	return err
}

// AcceptFriendRequest accepts a friend request
func (c *HTTPClient) AcceptFriendRequest(ctx context.Context, token string, applyID int) error {
	payload := map[string]interface{}{
		"apply_id": applyID,
		"remark":   "Friend",
	}

	_, err := c.doRequest(ctx, "POST", "/api/v1/contact-apply/accept", payload, token)
	return err
}

// GetFriendList retrieves the user's friend list
func (c *HTTPClient) GetFriendList(ctx context.Context, token string) error {
	_, err := c.doRequest(ctx, "POST", "/api/v1/contact/list", nil, token)
	return err
}

// CreateGroup creates a new group
func (c *HTTPClient) CreateGroup(ctx context.Context, token string, groupName string, memberIDs []int) (*Group, error) {
	payload := map[string]interface{}{
		"name": groupName,
		"ids":  memberIDs,
	}

	resp, err := c.doRequest(ctx, "POST", "/api/v1/group/create", payload, token)
	if err != nil {
		return nil, err
	}

	groupID := 0
	if id, ok := resp["id"].(float64); ok {
		groupID = int(id)
	}

	return &Group{
		ID:   groupID,
		Name: groupName,
	}, nil
}

// SendTextMessage sends a text message
func (c *HTTPClient) SendTextMessage(ctx context.Context, token string, talkMode int, receiverID int, content string) error {
	payload := map[string]interface{}{
		"type":        "text",
		"talk_mode":   talkMode,
		"receiver_id": receiverID,
		"body": map[string]interface{}{
			"content": content,
		},
	}

	_, err := c.doRequest(ctx, "POST", "/api/v1/message/send", payload, token)
	return err
}

// GetSessionList retrieves the user's session list
func (c *HTTPClient) GetSessionList(ctx context.Context, token string) error {
	_, err := c.doRequest(ctx, "POST", "/api/v1/talk/session-list", nil, token)
	return err
}

// doRequest performs an HTTP request
func (c *HTTPClient) doRequest(ctx context.Context, method, path string, payload interface{}, token string) (map[string]interface{}, error) {
	var body io.Reader
	if payload != nil {
		jsonData, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload: %w", err)
		}
		body = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Check if the response has a "data" field
	if data, ok := result["data"].(map[string]interface{}); ok {
		return data, nil
	}

	return result, nil
}
