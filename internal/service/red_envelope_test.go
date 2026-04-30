package service

import (
	"context"
	"testing"

	"github.com/gzydong/go-chat/internal/entity"
)

func TestInMemoryRedEnvelopeService_Create(t *testing.T) {
	svc := NewInMemoryRedEnvelopeService()
	ctx := context.Background()

	tests := []struct {
		name    string
		req     *SendRedEnvelopeRequest
		wantErr bool
	}{
		{
			name: "create normal red envelope",
			req: &SendRedEnvelopeRequest{
				SenderId: 1001,
				ChatType: 1,
				ChatId:   1002,
				Amount:   100.00,
				Count:    1,
				Type:     entity.RedEnvelopeTypeNormal,
				Greeting: "恭喜发财",
			},
		},
		{
			name: "create lucky red envelope",
			req: &SendRedEnvelopeRequest{
				SenderId: 1001,
				ChatType: 2,
				ChatId:   5001,
				Amount:   50.00,
				Count:    5,
				Type:     entity.RedEnvelopeTypeLucky,
				Greeting: "拼手气红包",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := svc.Create(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if info == nil {
				t.Fatal("Create() returned nil info")
			}
			if info.EnvelopeId == "" {
				t.Error("Create() returned empty envelope ID")
			}
			if info.Status != entity.RedEnvelopeStatusAvailable {
				t.Errorf("Create() status = %v, want %v", info.Status, entity.RedEnvelopeStatusAvailable)
			}
			if info.StatusText != "待领取" {
				t.Errorf("Create() statusText = %v, want 待领取", info.StatusText)
			}
			if info.Amount != tt.req.Amount {
				t.Errorf("Create() amount = %v, want %v", info.Amount, tt.req.Amount)
			}
			if info.Count != tt.req.Count {
				t.Errorf("Create() count = %v, want %v", info.Count, tt.req.Count)
			}
			if info.RemainAmount != tt.req.Amount {
				t.Errorf("Create() remainAmount = %v, want %v", info.RemainAmount, tt.req.Amount)
			}
			if info.RemainCount != tt.req.Count {
				t.Errorf("Create() remainCount = %v, want %v", info.RemainCount, tt.req.Count)
			}
		})
	}
}

func TestInMemoryRedEnvelopeService_ReceiveNormal(t *testing.T) {
	svc := NewInMemoryRedEnvelopeService()
	ctx := context.Background()

	// Create a normal red envelope with 3 copies
	info, err := svc.Create(ctx, &SendRedEnvelopeRequest{
		SenderId: 1001,
		ChatType: 2,
		ChatId:   5001,
		Amount:   30.00,
		Count:    3,
		Type:     entity.RedEnvelopeTypeNormal,
		Greeting: "普通红包",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// User 1 receives
	result1, err := svc.Receive(ctx, info.EnvelopeId, 2001)
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if result1.Status != entity.RedEnvelopeReceiveStatusSuccess {
		t.Errorf("Receive() status = %v, want %v", result1.Status, entity.RedEnvelopeReceiveStatusSuccess)
	}
	if result1.Amount != 10.00 {
		t.Errorf("Receive() amount = %v, want 10.00", result1.Amount)
	}

	// User 1 tries to receive again - should get "repeated"
	result1Again, err := svc.Receive(ctx, info.EnvelopeId, 2001)
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if result1Again.Status != entity.RedEnvelopeReceiveStatusRepeated {
		t.Errorf("Receive() again status = %v, want %v", result1Again.Status, entity.RedEnvelopeReceiveStatusRepeated)
	}

	// User 2 and 3 receive
	_, err = svc.Receive(ctx, info.EnvelopeId, 2002)
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	result3, err := svc.Receive(ctx, info.EnvelopeId, 2003)
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if result3.Status != entity.RedEnvelopeReceiveStatusSuccess {
		t.Errorf("Receive() last status = %v, want %v", result3.Status, entity.RedEnvelopeReceiveStatusSuccess)
	}

	// All received - check detail
	detail, err := svc.GetDetail(ctx, info.EnvelopeId)
	if err != nil {
		t.Fatalf("GetDetail() error = %v", err)
	}
	if detail.Status != entity.RedEnvelopeStatusFinished {
		t.Errorf("GetDetail() status = %v, want %v", detail.Status, entity.RedEnvelopeStatusFinished)
	}
	if detail.StatusText != "已领完" {
		t.Errorf("GetDetail() statusText = %v, want 已领完", detail.StatusText)
	}
	if detail.ReceivedCount != 3 {
		t.Errorf("GetDetail() receivedCount = %v, want 3", detail.ReceivedCount)
	}

	// User 4 tries to receive - should get "finished"
	result4, err := svc.Receive(ctx, info.EnvelopeId, 2004)
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if result4.Status != entity.RedEnvelopeReceiveStatusFinished {
		t.Errorf("Receive() after finished status = %v, want %v", result4.Status, entity.RedEnvelopeReceiveStatusFinished)
	}
}

func TestInMemoryRedEnvelopeService_ReceiveLucky(t *testing.T) {
	svc := NewInMemoryRedEnvelopeService()
	ctx := context.Background()

	// Create a lucky red envelope
	info, err := svc.Create(ctx, &SendRedEnvelopeRequest{
		SenderId: 1001,
		ChatType: 2,
		ChatId:   5001,
		Amount:   100.00,
		Count:    5,
		Type:     entity.RedEnvelopeTypeLucky,
		Greeting: "拼手气红包",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// 5 users receive
	var totalReceived float64
	for i := 0; i < 5; i++ {
		result, err := svc.Receive(ctx, info.EnvelopeId, 2001+i)
		if err != nil {
			t.Fatalf("Receive() error = %v", err)
		}
		if result.Status != entity.RedEnvelopeReceiveStatusSuccess {
			t.Errorf("Receive() status = %v, want %v", result.Status, entity.RedEnvelopeReceiveStatusSuccess)
		}
		if result.Amount <= 0 {
			t.Errorf("Receive() amount = %v, want > 0", result.Amount)
		}
		totalReceived += result.Amount
	}

	// Total received should approximately equal total amount (within rounding)
	if totalReceived < 99.95 || totalReceived > 100.05 {
		t.Errorf("Total received = %v, want ~100.00", totalReceived)
	}

	// Check detail for best luck info
	detail, err := svc.GetDetail(ctx, info.EnvelopeId)
	if err != nil {
		t.Fatalf("GetDetail() error = %v", err)
	}
	if detail.Status != entity.RedEnvelopeStatusFinished {
		t.Errorf("GetDetail() status = %v, want %v", detail.Status, entity.RedEnvelopeStatusFinished)
	}
	if detail.BestUserId == 0 {
		t.Error("GetDetail() bestUserId should not be 0 for lucky envelope")
	}
	if detail.BestAmount <= 0 {
		t.Error("GetDetail() bestAmount should be > 0 for lucky envelope")
	}
	// Verify best user is in receiver list
	foundBest := false
	for _, r := range detail.ReceivedList {
		if r.UserId == detail.BestUserId && r.IsBest {
			foundBest = true
			break
		}
	}
	if !foundBest {
		t.Error("Best user should be marked in receiver list")
	}
}

func TestInMemoryRedEnvelopeService_GetStatus(t *testing.T) {
	svc := NewInMemoryRedEnvelopeService()
	ctx := context.Background()

	info, err := svc.Create(ctx, &SendRedEnvelopeRequest{
		SenderId: 1001,
		ChatType: 1,
		ChatId:   1002,
		Amount:   50.00,
		Count:    1,
		Type:     entity.RedEnvelopeTypeNormal,
		Greeting: "测试红包",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Status before receiving
	status, err := svc.GetStatus(ctx, info.EnvelopeId, 1002)
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}
	if status.Status != entity.RedEnvelopeStatusAvailable {
		t.Errorf("GetStatus() = %v, want %v", status.Status, entity.RedEnvelopeStatusAvailable)
	}
	if status.StatusText != "待领取" {
		t.Errorf("GetStatus() statusText = %v, want 待领取", status.StatusText)
	}
	if status.HasReceived {
		t.Error("GetStatus() hasReceived should be false before receiving")
	}

	// Receive
	_, err = svc.Receive(ctx, info.EnvelopeId, 1002)
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}

	// Status after receiving
	status, err = svc.GetStatus(ctx, info.EnvelopeId, 1002)
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}
	if status.Status != entity.RedEnvelopeStatusFinished {
		t.Errorf("GetStatus() after receive = %v, want %v", status.Status, entity.RedEnvelopeStatusFinished)
	}
	if status.StatusText != "已领完" {
		t.Errorf("GetStatus() statusText = %v, want 已领完", status.StatusText)
	}
	if !status.HasReceived {
		t.Error("GetStatus() hasReceived should be true after receiving")
	}
	if status.ReceivedAmt != 50.00 {
		t.Errorf("GetStatus() receivedAmt = %v, want 50.00", status.ReceivedAmt)
	}
}

func TestInMemoryRedEnvelopeService_NotFound(t *testing.T) {
	svc := NewInMemoryRedEnvelopeService()
	ctx := context.Background()

	_, err := svc.Receive(ctx, "nonexistent", 1001)
	if err == nil {
		t.Error("Receive() should return error for nonexistent envelope")
	}

	_, err = svc.GetDetail(ctx, "nonexistent")
	if err == nil {
		t.Error("GetDetail() should return error for nonexistent envelope")
	}

	_, err = svc.GetStatus(ctx, "nonexistent", 1001)
	if err == nil {
		t.Error("GetStatus() should return error for nonexistent envelope")
	}
}
