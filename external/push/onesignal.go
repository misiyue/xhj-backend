package push

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gzydong/go-chat/internal/pkg/logger"
)

var defaultClient *Client

type Client struct {
	AppID string
	Key   string
	URL   string
	HTTP  *http.Client
}

func Init(appID, key, url string) {
	defaultClient = &Client{
		AppID: strings.TrimSpace(appID),
		Key:   strings.TrimSpace(key),
		URL:   strings.TrimSpace(url),
		HTTP: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func GetClient() *Client {
	return defaultClient
}

type sendRequest struct {
	AppID          string              `json:"app_id"`
	Contents       map[string]string   `json:"contents"`
	Headings       map[string]string   `json:"headings,omitempty"`
	Subtitle       map[string]string   `json:"subtitle,omitempty"`
	IncludeAliases map[string][]string `json:"include_aliases"`
	TargetChannel  string              `json:"target_channel"`
	IOSBadgeType   string              `json:"ios_badgeType"`
	IOSBadgeCount  int                 `json:"ios_badgeCount"`
}

type sendResponse struct {
	ID     string `json:"id"`
	Errors any    `json:"errors"`
	Error  string `json:"error"`
}

// Message 推送文案（对应 headings.en / subtitle.en / contents.en）
type Message struct {
	Title    string
	Subtitle string
	Contents string
}

// SendToUser 向指定 user_id 推送；OneSignal include_aliases.external_id 使用 user_id 字符串
func (c *Client) SendToUser(userID int, msg Message) error {
	if userID <= 0 {
		return fmt.Errorf("user_id invalid")
	}
	return c.send(strconv.Itoa(userID), msg)
}

// SendTextToUser 向指定 user_id 推送；title/subtitle/contents 对应 headings.en / subtitle.en / contents.en
func (c *Client) SendTextToUser(userID int, title, subtitle, contents string) error {
	return c.SendToUser(userID, Message{Title: title, Subtitle: subtitle, Contents: contents})
}

func (c *Client) send(externalID string, msg Message) error {
	if c == nil || c.URL == "" {
		return fmt.Errorf("push client not configured")
	}
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return fmt.Errorf("user_id empty")
	}
	body, err := json.Marshal(sendRequest{
		AppID: c.AppID,
		Contents: map[string]string{
			"en": msg.Contents,
		},
		Headings: map[string]string{
			"en": msg.Title,
		},
		Subtitle: map[string]string{
			"en": msg.Subtitle,
		},
		IncludeAliases: map[string][]string{
			"external_id": {externalID},
		},
		TargetChannel: "push",
		IOSBadgeType:  "Increase",
		IOSBadgeCount: 1,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, c.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.Key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("onesignal push http %d: %s", resp.StatusCode, string(respBody))
	}

	var out sendResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return fmt.Errorf("onesignal push decode: %w, body=%s", err, string(respBody))
	}
	if out.Error != "" {
		return fmt.Errorf("onesignal push: %s", out.Error)
	}
	if out.Errors != nil && !isEmptyErrors(out.Errors) {
		return fmt.Errorf("onesignal push: %v", out.Errors)
	}
	logger.Infof("[push] sent notification id=%s user_id=%s", out.ID, externalID)
	return nil
}

// SendToUser 使用包级客户端向 user_id 推送
func SendToUser(userID int, msg Message) error {
	if defaultClient == nil {
		return fmt.Errorf("push client not configured")
	}
	return defaultClient.SendToUser(userID, msg)
}

// SendTextToUser 使用包级客户端向 user_id 推送文案
func SendTextToUser(userID int, title, subtitle, contents string) error {
	if defaultClient == nil {
		return fmt.Errorf("push client not configured")
	}
	return defaultClient.SendTextToUser(userID, title, subtitle, contents)
}

func isEmptyErrors(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(x) == ""
	case []any:
		return len(x) == 0
	case []string:
		return len(x) == 0
	case map[string]any:
		return len(x) == 0
	default:
		return false
	}
}
