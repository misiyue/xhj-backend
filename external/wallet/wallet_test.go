package wallet

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInit(t *testing.T) {
	Init("https://example.com/api", "test_key")
	c := GetClient()
	if c == nil {
		t.Fatal("GetClient() returned nil after Init")
	}
	if c.BaseURL != "https://example.com/api" {
		t.Errorf("BaseURL = %q, want %q", c.BaseURL, "https://example.com/api")
	}
	if c.Key != "test_key" {
		t.Errorf("Key = %q, want %q", c.Key, "test_key")
	}
}

func TestInitTrimsTrailingSlash(t *testing.T) {
	Init("https://example.com/api/", "key")
	c := GetClient()
	if c.BaseURL != "https://example.com/api" {
		t.Errorf("BaseURL = %q, want trailing slash trimmed", c.BaseURL)
	}
}

func TestRegisterThirdParty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/order/register_thirdparty" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm failed: %v", err)
		}
		if r.FormValue("phone") != "13800138000" {
			t.Errorf("phone = %q, want %q", r.FormValue("phone"), "13800138000")
		}
		if r.FormValue("key") != "mh13813M" {
			t.Errorf("key = %q, want %q", r.FormValue("key"), "mh13813M")
		}
		if r.FormValue("third_uid") != "9527" {
			t.Errorf("third_uid = %q, want %q", r.FormValue("third_uid"), "9527")
		}

		resp := BaseResponse{
			Code: 1,
			Msg:  "注册成功",
		}
		data := RegisterData{UserID: 12345, UUID: "test-uuid", Phone: "13800138000"}
		resp.Data, _ = json.Marshal(data)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := &Client{
		BaseURL:    server.URL,
		Key:        "mh13813M",
		HTTPClient: server.Client(),
	}

	result, err := c.RegisterThirdParty("13800138000", "abc123456", "9527")
	if err != nil {
		t.Fatalf("RegisterThirdParty() error = %v", err)
	}
	uid, err := ParseRegisterUserID(result.UserID)
	if err != nil || uid != 12345 {
		t.Errorf("UserID = %v (%v), want 12345", result.UserID, err)
	}
	if result.UUID != "test-uuid" {
		t.Errorf("UUID = %q, want %q", result.UUID, "test-uuid")
	}
}

func TestSendFunds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/order/sendfunds_thirdparty" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm failed: %v", err)
		}
		if r.FormValue("senduid") != "12345" {
			t.Errorf("senduid = %q, want %q", r.FormValue("senduid"), "12345")
		}
		if r.FormValue("recvuid") != "23456" {
			t.Errorf("recvuid = %q, want %q", r.FormValue("recvuid"), "23456")
		}

		resp := BaseResponse{
			Code: 1,
			Msg:  "转账成功",
		}
		data := TransferData{SendUID: 12345, RecvUID: 23456, Amount: 25.5, Currency: "USDT"}
		resp.Data, _ = json.Marshal(data)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := &Client{
		BaseURL:    server.URL,
		Key:        "mh13813M",
		HTTPClient: server.Client(),
	}

	result, err := c.SendFunds(12345, 23456, 25.5, "reward")
	if err != nil {
		t.Fatalf("SendFunds() error = %v", err)
	}
	if result.Amount != 25.5 {
		t.Errorf("Amount = %v, want 25.5", result.Amount)
	}
	if result.Currency != "USDT" {
		t.Errorf("Currency = %q, want %q", result.Currency, "USDT")
	}
}

func TestGetUserFunds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/order/getuserfunds_thirdparty" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := BaseResponse{
			Code: 1,
			Msg:  "success",
		}
		data := BalanceData{UID: 12345, FundsType: "tronusdt", CurrencyID: 1, Balance: 100.5}
		resp.Data, _ = json.Marshal(data)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := &Client{
		BaseURL:    server.URL,
		Key:        "mh13813M",
		HTTPClient: server.Client(),
	}

	result, err := c.GetUserFunds(12345, "tronusdt", "abc123456")
	if err != nil {
		t.Fatalf("GetUserFunds() error = %v", err)
	}
	if result.Balance != 100.5 {
		t.Errorf("Balance = %v, want 100.5", result.Balance)
	}
}

func TestGetBillList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bill/thirdBillListall" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := BaseResponse{
			Code: 1,
			Msg:  "success",
		}
		data := BillListData{
			Data: []*BillItem{
				{ID: 32370, UID: 12345, CurrencyID: 1, Account: 11, BillType: "站内转账-转出", CurrencyName: "USDT"},
			},
			Page:     "0",
			PageSize: "20",
			Count:    1,
		}
		resp.Data, _ = json.Marshal(data)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := &Client{
		BaseURL:    server.URL,
		Key:        "mh13813M",
		HTTPClient: server.Client(),
	}

	result, err := c.GetBillList(12345, 0, 20)
	if err != nil {
		t.Fatalf("GetBillList() error = %v", err)
	}
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.Data) != 1 {
		t.Fatalf("Data length = %d, want 1", len(result.Data))
	}
	if result.Data[0].CurrencyName != "USDT" {
		t.Errorf("CurrencyName = %q, want %q", result.Data[0].CurrencyName, "USDT")
	}
}

func TestGetPayInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := BaseResponse{
			Code: 1,
			Msg:  "ok",
		}
		data := PayInfoData{
			Address:     "TGXtDYHZdtXFXUAz1GRTtvhSZiRQM8UmUV",
			Chain:       "tron",
			Symbol:      "USDT-TRC",
			TimeLeftSec: 10800,
		}
		resp.Data, _ = json.Marshal(data)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := &Client{
		BaseURL:    server.URL,
		Key:        "mh13813M",
		HTTPClient: server.Client(),
	}

	result, err := c.GetPayInfo(12345)
	if err != nil {
		t.Fatalf("GetPayInfo() error = %v", err)
	}
	if result.Address != "TGXtDYHZdtXFXUAz1GRTtvhSZiRQM8UmUV" {
		t.Errorf("Address = %q, want expected address", result.Address)
	}
	if result.Chain != "tron" {
		t.Errorf("Chain = %q, want %q", result.Chain, "tron")
	}
}

func TestAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := BaseResponse{
			Code: 0,
			Msg:  "密钥验证失败",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := &Client{
		BaseURL:    server.URL,
		Key:        "wrong_key",
		HTTPClient: server.Client(),
	}

	_, err := c.RegisterThirdParty("test", "test", "test")
	if err == nil {
		t.Fatal("expected error for code=0 response")
	}
}

func TestFreezeAccount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/order/freezeAccount" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := BaseResponse{
			Code: 1,
			Msg:  "success",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := &Client{
		BaseURL:    server.URL,
		Key:        "mh13813M",
		HTTPClient: server.Client(),
	}

	err := c.FreezeAccount("9527", 12345, 50, 1)
	if err != nil {
		t.Fatalf("FreezeAccount() error = %v", err)
	}
}

func TestGetAccountAssets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := BaseResponse{
			Code: 1,
			Msg:  "success",
		}
		data := map[string]*AccountAsset{
			"1": {ID: 1, Name: "USDT", Digit: 6, Account: "100.500000"},
		}
		resp.Data, _ = json.Marshal(data)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := &Client{
		BaseURL:    server.URL,
		Key:        "mh13813M",
		HTTPClient: server.Client(),
	}

	result, err := c.GetAccountAssets("9527", 12345)
	if err != nil {
		t.Fatalf("GetAccountAssets() error = %v", err)
	}
	asset, ok := result["1"]
	if !ok {
		t.Fatal("expected key '1' in assets")
	}
	if asset.Name != "USDT" {
		t.Errorf("Name = %q, want %q", asset.Name, "USDT")
	}
}
