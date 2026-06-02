package hdpay

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gzydong/go-chat/internal/pkg/logger"
)

var defaultClient *Client

// Client 宏达支付 HTTP 客户端
type Client struct {
	OrderURL  string
	AppID     string
	AppSecret string
	HTTP      *http.Client
}

// Init 初始化包级客户端
func Init(orderURL, appID, appSecret string) {
	defaultClient = &Client{
		OrderURL:  strings.TrimSpace(orderURL),
		AppID:     strings.TrimSpace(appID),
		AppSecret: strings.TrimSpace(appSecret),
		HTTP: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetClient 返回已初始化客户端
func GetClient() *Client {
	return defaultClient
}

// Sign 除 sign 外参数按 key 字典序拼接 k=v&...&key=密钥，转大写后 md5 小写
func Sign(params map[string]string, appSecret string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "sign" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(params[k])
	}
	b.WriteString("&key=")
	b.WriteString(appSecret)
	sum := md5.Sum([]byte(strings.ToUpper(b.String())))
	return hex.EncodeToString(sum[:])
}

// VerifySign 校验签名（支持扩展字段）
func VerifySign(params map[string]string, appSecret, sign string) bool {
	if strings.TrimSpace(sign) == "" {
		return false
	}
	return strings.EqualFold(Sign(params, appSecret), strings.TrimSpace(sign))
}

// ParamsFromMap 将 JSON 解析为 map 后转为签名字符串 map（跳过 sign）
func ParamsFromMap(raw map[string]any) map[string]string {
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		if k == "sign" {
			continue
		}
		out[k] = valueToSignString(v)
	}
	return out
}

func valueToSignString(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case json.Number:
		return x.String()
	default:
		return fmt.Sprint(v)
	}
}

// CreateOrderRequest 统一下单请求
type CreateOrderRequest struct {
	SubmitAmount string `json:"submit_amount"`
	OrderNo      string `json:"order_no"`
	NotifyURL    string `json:"notify_url"`
	ReturnURL    string `json:"return_url,omitempty"`
	AppID        string `json:"app_id"`
	Time         int64  `json:"time"`
	PayType      string `json:"pay_type"`
	Subject      string `json:"subject,omitempty"`
	Body         string `json:"body,omitempty"`
	UserIP       string `json:"user_ip,omitempty"`
	Extra        string `json:"extra,omitempty"`
	Sign         string `json:"sign"`
}

// CreateOrderData 下单成功 data
type CreateOrderData struct {
	OrderNo string `json:"order_no"`
	LocalNo string `json:"local_no"`
	PayURL  string `json:"pay_url"`
}

// CreateOrderResponse API 响应
type CreateOrderResponse struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Data    *CreateOrderData `json:"data"`
}

// CreateOrder 调用统一下单
func (c *Client) CreateOrder(req *CreateOrderRequest) (*CreateOrderData, error) {
	if c == nil || c.OrderURL == "" {
		return nil, fmt.Errorf("hdpay client not configured")
	}
	params := map[string]string{
		"submit_amount": req.SubmitAmount,
		"order_no":      req.OrderNo,
		"notify_url":    req.NotifyURL,
		"app_id":        req.AppID,
		"time":          strconv.FormatInt(req.Time, 10),
		"pay_type":      req.PayType,
	}

	if req.ReturnURL != "" {
		params["return_url"] = req.ReturnURL
	}
	if req.Subject != "" {
		params["subject"] = req.Subject
	}
	if req.Body != "" {
		params["body"] = req.Body
	}
	if req.UserIP != "" {
		params["user_ip"] = req.UserIP
	}
	if req.Extra != "" {
		params["extra"] = req.Extra
	}
	logger.Infof("[hdpay] params: %+v", params)
	req.Sign = Sign(params, c.AppSecret)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequest(http.MethodPost, c.OrderURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var out CreateOrderResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("hdpay decode: %w, body=%s", err, string(respBody))
	}
	if out.Code != 0 {
		msg := out.Message
		if msg == "" {
			msg = "下单失败"
		}
		return nil, fmt.Errorf("%s", msg)
	}
	if out.Data == nil || out.Data.PayURL == "" {
		return nil, fmt.Errorf("下单成功但未返回支付链接")
	}
	return out.Data, nil
}
