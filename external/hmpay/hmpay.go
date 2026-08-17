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

// CreateOrderError 第三方下单接口调用失败（含请求/响应，供业务层写错误日志）
type CreateOrderError struct {
	URL      string
	Request  string
	Response string
	Message  string
}

func (e *CreateOrderError) Error() string {
	if e != nil && e.Message != "" {
		return e.Message
	}
	return "下单失败"
}

func newCreateOrderError(url, request, response, message string) *CreateOrderError {
	return &CreateOrderError{
		URL:      url,
		Request:  request,
		Response: response,
		Message:  message,
	}
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
		return "", newCreateOrderError(reqURL, reqURL, "", err.Error())
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", newCreateOrderError(reqURL, reqURL, "", err.Error())
	}
	responseBody := string(respBody)
	var out CreateOrderResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", newCreateOrderError(reqURL, reqURL, responseBody, fmt.Sprintf("hmpay decode: %v", err))
	}
	if out.Code != "0" {
		return "", newCreateOrderError(reqURL, reqURL, responseBody, fmt.Sprintf("汇美支付下单失败(code=%s)", out.Code))
	}
	payURL = strings.TrimSpace(out.PayURL)
	if payURL == "" {
		return "", newCreateOrderError(reqURL, reqURL, responseBody, "汇美支付未返回付款链接")
	}
	return payURL, nil
}
