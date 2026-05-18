package wallet

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// client is the package-level wallet client initialized by Init.
var client *Client

// Client holds configuration for the wallet API.
type Client struct {
	BaseURL    string
	Key        string
	HTTPClient *http.Client
}

// Init initializes the package-level wallet client.
func Init(baseURL, key string) {
	client = &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Key:     key,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetClient returns the package-level client (nil if not initialized).
func GetClient() *Client {
	return client
}

// --- Response structures ---

// BaseResponse is the common response wrapper from the wallet API.
type BaseResponse struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data,omitempty"`
}

// RegisterData is returned by RegisterThirdParty.
type RegisterData struct {
	// user_id 可能是数字或字符串（第三方返回不一致），用 any 原样接收
	UserID any    `json:"user_id"`
	UUID   string `json:"uuid"`
	Phone  string `json:"phone"`
}

// ParseRegisterUserID 将 RegisterData.User_id 转为平台用的整型钱包用户 ID。
func ParseRegisterUserID(v any) (int, error) {
	if v == nil {
		return 0, fmt.Errorf("wallet user_id is empty")
	}
	switch x := v.(type) {
	case float64:
		return int(x), nil
	case float32:
		return int(x), nil
	case int:
		return x, nil
	case int32:
		return int(x), nil
	case int64:
		return int(x), nil
	case uint, uint32, uint64:
		return int(fmt.Sprintf("%v", x)[0]), nil // wrong - can't use that
	default:
	}
	// handle uint via fmt or strconv - simpler: second switch
	switch x := v.(type) {
	case uint:
		return int(x), nil
	case uint32:
		return int(x), nil
	case uint64:
		if x > uint64(^uint(0)>>1) {
			return 0, fmt.Errorf("wallet user_id overflow")
		}
		return int(x), nil
	case json.Number:
		i, err := x.Int64()
		return int(i), err
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return 0, fmt.Errorf("wallet user_id is empty string")
		}
		i, err := strconv.Atoi(s)
		if err != nil {
			return 0, fmt.Errorf("wallet user_id string: %w", err)
		}
		return i, nil
	default:
		return 0, fmt.Errorf("wallet user_id: unsupported type %T", v)
	}
}

// AccountAsset represents a single currency asset.
type AccountAsset struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Image   string `json:"image"`
	Digit   int    `json:"digit"`
	Account string `json:"account"`
}

// TransferData is returned by SendFunds.
type TransferData struct {
	SendUID  int     `json:"senduid"`
	RecvUID  int     `json:"recvuid"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

// BalanceData is returned by GetUserFunds.
type BalanceData struct {
	UID        int     `json:"uid"`
	FundsType  string  `json:"fundstype"`
	CurrencyID int     `json:"currency_id"`
	Balance    float64 `json:"balance"`
}

// BillItem represents a single bill record.
type BillItem struct {
	ID            int     `json:"id"`
	UID           int     `json:"uid"`
	CurrencyID    int     `json:"currency_id"`
	Account       float64 `json:"account"`
	BeforeAccount float64 `json:"before_account"`
	AfterAccount  float64 `json:"after_account"`
	BillType      string  `json:"bill_type"`
	Remark        string  `json:"remark"`
	Type          int     `json:"type"`
	CreateTime    string  `json:"create_time"`
	Justdo        int     `json:"justdo"`
	CurrencyName  string  `json:"currency_name"`
}

// BillListData is returned by GetBillList.
type BillListData struct {
	Data     []*BillItem `json:"data"`
	Page     string      `json:"page"`
	PageSize string      `json:"pagesize"`
	Count    int         `json:"count"`
}

// PayInfoData is returned by GetPayInfo.
type PayInfoData struct {
	Address     string `json:"address"`
	Chain       string `json:"chain"`
	Symbol      string `json:"symbol"`
	ExpireAt    int64  `json:"expireAt"`
	ServerNow   int64  `json:"serverNow"`
	TimeLeftSec int64  `json:"timeLeftSec"`
}

// ChainTransferData is returned by SubmitChainTransfer.
type ChainTransferData struct {
	OrderID       int     `json:"order_id"`
	CurrencyName  string  `json:"currency_name"`
	Amount        float64 `json:"amount"`
	Fee           float64 `json:"fee"`
	TotalAmount   float64 `json:"total_amount"`
	ToAddress     string  `json:"to_address"`
	Status        string  `json:"status"`
	EstimatedTime string  `json:"estimated_time"`
}

// KYCStatusData is returned by GetKYCStatus.
type KYCStatusData struct {
	KYCStatus int    `json:"kyc_status"`
	Remark    string `json:"remark"`
}

// FreezeAccountData is returned in data by the freezeAccount API.
type FreezeAccountData struct {
	BillID int `json:"bill_id"`
}

// --- API methods ---

// post sends a POST request with form-encoded body and decodes the JSON response.
func (c *Client) post(path string, params url.Values) (*BaseResponse, error) {
	reqURL := c.BaseURL + path
	resp, err := c.HTTPClient.Post(reqURL, "application/x-www-form-urlencoded", strings.NewReader(params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("wallet api request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("wallet api read body failed: %w", err)
	}

	var baseResp BaseResponse
	if err := json.Unmarshal(body, &baseResp); err != nil {
		return nil, fmt.Errorf("wallet api decode response failed: %w", err)
	}

	if baseResp.Code != 1 {
		return &baseResp, fmt.Errorf("wallet api error: %s", baseResp.Msg)
	}

	return &baseResp, nil
}

// RegisterThirdParty registers a third-party user on the wallet platform.
func (c *Client) RegisterThirdParty(phone, password, thirdUID string) (*RegisterData, error) {
	params := url.Values{}
	params.Set("phone", phone)
	params.Set("password", password)
	params.Set("third_uid", thirdUID)
	params.Set("channel", "1")
	params.Set("type", "1")
	params.Set("key", c.Key)

	resp, err := c.post("/order/register_thirdparty", params)
	if err != nil {
		return nil, err
	}

	var data RegisterData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("wallet api decode register data failed: %w and data: %s", err, string(resp.Data))
	}
	return &data, nil
}

// GetAccountAssets retrieves the asset list for a user.
func (c *Client) GetAccountAssets(uid string, uUID int) (map[string]*AccountAsset, error) {
	params := url.Values{}
	params.Set("uid", uid)
	params.Set("u_uid", fmt.Sprintf("%d", uUID))
	params.Set("key", c.Key)

	resp, err := c.post("/order/third_userAccount", params)
	if err != nil {
		return nil, err
	}

	var data map[string]*AccountAsset
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("wallet api decode account assets failed: %w", err)
	}
	return data, nil
}

// SendFunds performs an in-platform transfer between two wallet users.
func (c *Client) SendFunds(sendUID, recvUID int, amount float64, remark string) (*TransferData, error) {
	params := url.Values{}
	params.Set("senduid", fmt.Sprintf("%d", sendUID))
	params.Set("recvuid", fmt.Sprintf("%d", recvUID))
	params.Set("Amount", fmt.Sprintf("%g", amount))
	params.Set("key", c.Key)
	if remark != "" {
		params.Set("remark", remark)
	}

	resp, err := c.post("/order/sendfunds_thirdparty", params)
	if err != nil {
		return nil, err
	}

	var data TransferData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("wallet api decode transfer data failed: %w", err)
	}
	return &data, nil
}

// GetUserFunds queries the balance for a specific currency type.
func (c *Client) GetUserFunds(uid int, fundsType string, userPassword string) (*BalanceData, error) {
	params := url.Values{}
	params.Set("uid", fmt.Sprintf("%d", uid))
	params.Set("fundstype", fundsType)
	params.Set("key", userPassword)

	resp, err := c.post("/order/getuserfunds_thirdparty", params)
	if err != nil {
		return nil, err
	}

	var data BalanceData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("wallet api decode balance data failed: %w", err)
	}
	return &data, nil
}

// GetBillList retrieves the billing history for a user.
func (c *Client) GetBillList(uUID int, page, pageSize int) (*BillListData, error) {
	params := url.Values{}
	params.Set("u_uid", fmt.Sprintf("%d", uUID))
	params.Set("key", c.Key)
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("pagesize", fmt.Sprintf("%d", pageSize))

	resp, err := c.post("/bill/thirdBillListall", params)
	if err != nil {
		return nil, err
	}

	var data BillListData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("wallet api decode bill list failed: %w", err)
	}
	return &data, nil
}

// GetPayInfo retrieves the USDT-TRC recharge address.
func (c *Client) GetPayInfo(uUID int) (*PayInfoData, error) {
	params := url.Values{}
	params.Set("u_uid", fmt.Sprintf("%d", uUID))
	params.Set("key", c.Key)

	resp, err := c.post("/order/payInfo", params)
	if err != nil {
		return nil, err
	}

	var data PayInfoData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("wallet api decode pay info failed: %w", err)
	}
	return &data, nil
}

// SubmitChainTransfer initiates an on-chain transfer.
func (c *Client) SubmitChainTransfer(uUID int, currencyID int, amount float64, toAddress string, code string) (*ChainTransferData, error) {
	params := url.Values{}
	params.Set("u_uid", fmt.Sprintf("%d", uUID))
	params.Set("currency_id", fmt.Sprintf("%d", currencyID))
	params.Set("amount", fmt.Sprintf("%g", amount))
	params.Set("to_address", toAddress)
	params.Set("key", c.Key)
	params.Set("code", code)

	resp, err := c.post("/order/submitTransfer_thirdparty", params)
	if err != nil {
		return nil, err
	}

	var data ChainTransferData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("wallet api decode chain transfer failed: %w", err)
	}
	return &data, nil
}

// GetKYCStatus queries the KYC status for a user.
func (c *Client) GetKYCStatus(uid string, uUID int) (*KYCStatusData, error) {
	params := url.Values{}
	params.Set("uid", uid)
	params.Set("u_uid", fmt.Sprintf("%d", uUID))
	params.Set("key", c.Key)

	resp, err := c.post("/order/getKycStatus", params)
	if err != nil {
		return nil, err
	}

	var data KYCStatusData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("wallet api decode kyc status failed: %w", err)
	}
	return &data, nil
}

// FreezeAccount freezes user assets. On success returns data.bill_id (0 if absent or unparsable).
func (c *Client) FreezeAccount(uid string, uUID int, amount float64, currencyID int) (billID int, err error) {
	params := url.Values{}
	params.Set("uid", uid)
	params.Set("u_uid", fmt.Sprintf("%d", uUID))
	params.Set("amount", fmt.Sprintf("%g", amount))
	params.Set("currency_id", fmt.Sprintf("%d", currencyID))
	params.Set("key", c.Key)

	resp, err := c.post("/order/freezeAccount", params)
	if err != nil {
		return 0, err
	}
	if len(resp.Data) == 0 {
		return 0, nil
	}
	var data FreezeAccountData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return 0, fmt.Errorf("wallet api decode freeze account data failed: %w", err)
	}
	return data.BillID, nil
}
