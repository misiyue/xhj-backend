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
	TargetChannel     string              `json:"target_channel"`
	IOSBadgeType      string              `json:"ios_badgeType"`
	IOSBadgeCount     int                 `json:"ios_badgeCount"`
	HuaweiBadgeClass  string              `json:"huawei_badge_class"`
	HuaweiBadgeAddNum int                 `json:"huawei_badge_add_num"`
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
		TargetChannel:     "push",
		IOSBadgeType:      "Increase",
		IOSBadgeCount:     1,
		HuaweiBadgeClass:  "com.xhj.im.MainActivity",
		HuaweiBadgeAddNum: 1,
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

// VoIPCallData 音视频通话 VoIP 推送自定义数据
type VoIPCallData struct {
	Event          string `json:"event"`
	FromUserId     int    `json:"from_user_id"`
	ToUserId       int    `json:"to_user_id"`
	RoomId         int    `json:"room_id"`
	CallType       int    `json:"call_type"`
	FromUserName   string `json:"from_user_name"`
	FromUserAvatar string `json:"from_user_avatar"`
}

type voipSendRequest struct {
	AppID                string              `json:"app_id"`
	IncludeAliases       map[string][]string `json:"include_aliases"`
	TargetChannel        string              `json:"target_channel"`
	ApnsPushTypeOverride string              `json:"apns_push_type_override"`
	Priority             int                 `json:"priority"`
	HuaweiCategory       string              `json:"huawei_category"`
	ContentAvailable     bool                `json:"content_available"`
	Data                 map[string]string   `json:"data"`
}

// SendVoIPToUser 向指定 user_id 发送 VoIP 推送（用于音视频通话唤醒）
func (c *Client) SendVoIPToUser(userID int, data VoIPCallData) error {
	if userID <= 0 {
		return fmt.Errorf("user_id invalid")
	}
	return c.sendVoIP(strconv.Itoa(userID), data)
}

func (c *Client) sendVoIP(externalID string, data VoIPCallData) error {
	if c == nil || c.URL == "" {
		return fmt.Errorf("push client not configured")
	}
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return fmt.Errorf("user_id empty")
	}
	body, err := json.Marshal(voipSendRequest{
		AppID: c.AppID,
		IncludeAliases: map[string][]string{
			"external_id": {externalID},
		},
		TargetChannel:        "push",
		ApnsPushTypeOverride: "voip",
		Priority:             10,
		HuaweiCategory:       "VOIP",
		ContentAvailable:     true,
		Data: map[string]string{
			"event":            data.Event,
			"from_user_id":     strconv.Itoa(data.FromUserId),
			"to_user_id":       strconv.Itoa(data.ToUserId),
			"room_id":          strconv.Itoa(data.RoomId),
			"call_type":        strconv.Itoa(data.CallType),
			"from_user_name":   data.FromUserName,
			"from_user_avatar": data.FromUserAvatar,
		},
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
		return fmt.Errorf("onesignal voip push http %d: %s", resp.StatusCode, string(respBody))
	}

	var out sendResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return fmt.Errorf("onesignal voip push decode: %w, body=%s", err, string(respBody))
	}
	if out.Error != "" {
		return fmt.Errorf("onesignal voip push: %s", out.Error)
	}
	if out.Errors != nil && !isEmptyErrors(out.Errors) {
		return fmt.Errorf("onesignal voip push: %v", out.Errors)
	}
	logger.Infof("[push] sent voip notification id=%s user_id=%s", out.ID, externalID)
	return nil
}

// SendVoIPToUser 使用包级客户端向 user_id 发送 VoIP 推送
func SendVoIPToUser(userID int, data VoIPCallData) error {
	if defaultClient == nil {
		return fmt.Errorf("push client not configured")
	}
	return defaultClient.SendVoIPToUser(userID, data)
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
