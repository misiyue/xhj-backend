package v1

import (
	"context"
	"errors"
	"math"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/service"
)

type Marzban struct {
	MarzbanService service.IMarzbanService
}

const bytesPerGB = int64(1 << 30)

type MarzbanCreateUserRequest struct {
	ID          int     `json:"id" binding:"required,gt=0"`
	DataLimitGB float64 `json:"data_limit_gb" binding:"required,gt=0"`
	ExpireDays  int     `json:"expire_days" binding:"required,gt=0"`
}

type MarzbanUserResponse struct {
	ID                    int     `json:"id"`
	Username              string  `json:"username"`
	Status                string  `json:"status"`
	DataLimitBytes        int64   `json:"data_limit_bytes"`
	DataLimitGB           float64 `json:"data_limit_gb"`
	UsedTrafficBytes      int64   `json:"used_traffic_bytes"`
	RemainingTrafficBytes int64   `json:"remaining_traffic_bytes"`
	RemainingTrafficGB    float64 `json:"remaining_traffic_gb"`
	UnlimitedTraffic      bool    `json:"unlimited_traffic"`
	Expire                int64   `json:"expire"`
	ExpireAt              string  `json:"expire_at"`
	UnlimitedExpire       bool    `json:"unlimited_expire"`
	SubscriptionURL       string  `json:"subscription_url"`
	Created               bool    `json:"created"`
	Message               string  `json:"message"`
}

// CreateUser 按当前系统用户ID幂等创建 Marzban 用户。
//
//	@Summary		创建 Marzban 用户
//	@Description	按本系统用户ID创建 Marzban 用户；用户已存在时直接返回现有账户
//	@Tags			Marzban
//	@Accept			json
//	@Produce		json
//	@Param			request	body		MarzbanCreateUserRequest	true	"创建用户请求"
//	@Success		200		{object}	MarzbanUserResponse
//	@Router			/api/v1/marzban/user [post]
//	@Security		Bearer
func (m *Marzban) CreateUser(ctx *gin.Context) (*MarzbanUserResponse, error) {
	var req MarzbanCreateUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		return nil, errorx.New(400, "用户ID、流量额度（GB）和有效天数均为必填项，且必须大于0")
	}
	if err := validateMarzbanOwner(ctx.Request.Context(), req.ID); err != nil {
		return nil, err
	}
	dataLimitBytes, err := dataLimitGBToBytes(req.DataLimitGB)
	if err != nil {
		return nil, err
	}
	user, err := m.MarzbanService.CreateByID(ctx.Request.Context(), req.ID, dataLimitBytes, req.ExpireDays)
	if err != nil {
		return nil, err
	}
	resp := marzbanUserResponse(user)
	if !user.Created {
		resp.Message = "Marzban 用户已存在，已返回现有账号"
	}
	return resp, nil
}

// GetUser 按当前系统用户ID获取 Marzban 剩余流量和到期日期。
//
//	@Summary		获取 Marzban 用户信息
//	@Description	获取当前用户的剩余流量、到期日期和订阅地址
//	@Tags			Marzban
//	@Produce		json
//	@Param			id	path		int	true	"本系统用户ID"
//	@Success		200	{object}	MarzbanUserResponse
//	@Router			/api/v1/marzban/user/{id} [get]
//	@Security		Bearer
func (m *Marzban) GetUser(ctx *gin.Context) (*MarzbanUserResponse, error) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		return nil, errorx.New(400, "用户ID无效")
	}
	if err := validateMarzbanOwner(ctx.Request.Context(), id); err != nil {
		return nil, err
	}
	user, err := m.MarzbanService.GetByID(ctx.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrMarzbanUserNotFound) {
			return nil, errorx.New(404, err.Error())
		}
		return nil, err
	}
	return marzbanUserResponse(user), nil
}

func validateMarzbanOwner(ctx context.Context, id int) error {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)
	if uid <= 0 {
		return errorx.New(401, "请先登录")
	}
	if uid != id {
		return errorx.New(403, "无权操作其他用户的 Marzban 账号")
	}
	return nil
}

func marzbanUserResponse(user *service.MarzbanUserInfo) *MarzbanUserResponse {
	message := "查询成功"
	if user.Created {
		message = "Marzban 用户创建成功"
	}
	return &MarzbanUserResponse{
		ID:                    user.ID,
		Username:              user.Username,
		Status:                user.Status,
		DataLimitBytes:        user.DataLimit,
		DataLimitGB:           float64(user.DataLimit) / float64(bytesPerGB),
		UsedTrafficBytes:      user.UsedTraffic,
		RemainingTrafficBytes: user.RemainingTraffic,
		RemainingTrafficGB:    float64(user.RemainingTraffic) / float64(1<<30),
		UnlimitedTraffic:      user.UnlimitedTraffic,
		Expire:                user.Expire,
		ExpireAt:              user.ExpireAt,
		UnlimitedExpire:       user.UnlimitedExpire,
		SubscriptionURL:       user.SubscriptionURL,
		Created:               user.Created,
		Message:               message,
	}
}

func dataLimitGBToBytes(dataLimitGB float64) (int64, error) {
	maxGB := float64(math.MaxInt64) / float64(bytesPerGB)
	if dataLimitGB <= 0 || dataLimitGB > maxGB {
		return 0, errorx.New(400, "流量额度必须大于0且不能超过系统上限")
	}
	dataLimitBytes := int64(math.Round(dataLimitGB * float64(bytesPerGB)))
	if dataLimitBytes <= 0 {
		return 0, errorx.New(400, "流量额度过小")
	}
	return dataLimitBytes, nil
}
