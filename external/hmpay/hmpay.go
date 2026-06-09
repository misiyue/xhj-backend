package hmpay

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gzydong/go-chat/internal/pkg/logger"
)

var defaultClient *Client

type Client struct {
	OrderURL string
	Parter   string
	ReqType  string
	Key      string
	HTTP     *http.Client
}

func Init(orderURL, parter, reqType, key string) {
	defaultClient = &Client{
		OrderURL: strings.TrimSpace(orderURL),
		Parter:   strings.TrimSpace(parter),
		ReqType:  strings.TrimSpace(reqType),
		Key:      strings.TrimSpace(key),
		HTTP: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func GetClient() *Client {
	return defaultClient
}

// OrderSign Md5(parter={}&type={}&orderid={}&value={}&callbackurl={}key)
func OrderSign(parter, payType, orderID, value, callbackURL, key string) string {
	raw := fmt.Sprintf("parter=%s&type=%s&orderid=%s&value=%s&callbackurl=%s%s",
		parter, payType, orderID, value, callbackURL, key)
	sum := md5.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// NotifySign Md5(orderid={}&restate={}&ovalue={}key)
func NotifySign(orderID, restate, ovalue, key string) string {
	raw := fmt.Sprintf("orderid=%s&restate=%s&ovalue=%s%s", orderID, restate, ovalue, key)
	sum := md5.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func VerifyNotifySign(orderID, restate, ovalue, key, sign string) bool {
	if strings.TrimSpace(sign) == "" {
		return false
	}
	return strings.EqualFold(NotifySign(orderID, restate, ovalue, key), strings.TrimSpace(sign))
}

type CreateOrderRequest struct {
	OrderID      string
	Value        string
	PayType      string
	CallbackURL  string
	HrefBackURL  string
	Attach       string
}

type CreateOrderResponse struct {
	Code   string `json:"code"`
	PayURL string `json:"payUrl"`
}

func (c *Client) CreateOrder(req *CreateOrderRequest) (payURL string, err error) {
	if c == nil || c.OrderURL == "" {
		return "", fmt.Errorf("hmpay client not configured")
	}
	sign := OrderSign(c.Parter, req.PayType, req.OrderID, req.Value, req.CallbackURL, c.Key)
	q := url.Values{}
	q.Set("type", req.PayType)
	q.Set("parter", c.Parter)
	q.Set("value", req.Value)
	q.Set("orderid", req.OrderID)
	q.Set("callbackurl", req.CallbackURL)
	if href := strings.TrimSpace(req.HrefBackURL); href != "" {
		q.Set("hrefbackurl", href)
	}
	if attach := strings.TrimSpace(req.Attach); attach != "" {
		q.Set("attach", attach)
	}
	q.Set("sign", sign)
	if c.ReqType != "" {
		q.Set("reqType", c.ReqType)
	}
	reqURL := c.OrderURL + "?" + q.Encode()
	logger.Infof("[hmpay] create order url: %s", reqURL)

	httpReq, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var out CreateOrderResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", fmt.Errorf("hmpay decode: %w, body=%s", err, string(respBody))
	}
	if out.Code != "0" {
		return "", fmt.Errorf("汇美支付下单失败(code=%s)", out.Code)
	}
	payURL = strings.TrimSpace(out.PayURL)
	if payURL == "" {
		return "", fmt.Errorf("汇美支付未返回付款链接")
	}
	return payURL, nil
}
