package v1

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/config"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/email"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/pkg/timeutil"
	"github.com/gzydong/go-chat/internal/repository/cache"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"github.com/gzydong/go-chat/internal/service"
	"google.golang.org/protobuf/types/known/structpb"
)

var _ web.ICommonHandler = (*Common)(nil)

type Common struct {
	Config              *config.Config
	UsersRepo           *repo.Users
	AppVersionRepo      *repo.AppVersion
	AppExploreRepo      *repo.AppExplore
	AppModuleRepo       *repo.AppModule
	AppDictRepo         *repo.AppDict
	AppNewsRepo         *repo.AppNews
	AppNewsCategoryRepo *repo.AppNewsCategory
	AppNewsViewRepo     *repo.AppNewsView
	AppNewsViewCache    *cache.AppNewsViewStorage
	SmsService          service.ISmsService
	EmailService        service.IEmailService
	UserService         service.IUserService
	EmailClient         *email.Client
	TemplateService     service.ITemplateService
}

// SendSms 发送短信验证码接口
//
//	@Summary		发送短信
//	@Description	发送用于登录、注册或更换账号的短信验证码
//	@Tags			公共
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.CommonSendSmsRequest	true	"发送短信请求"
//	@Success		200		{object}	web.CommonSendSmsResponse
//	@Router			/api/v1/common/send-sms [post]
func (c *Common) SendSms(ctx context.Context, in *web.CommonSendSmsRequest) (*web.CommonSendSmsResponse, error) {
	// 检查手机号注册是否被允许
	if in.Channel == entity.SmsRegisterChannel && !c.Config.App.AllowPhoneRegistration {
		return nil, entity.ErrPhoneRegistrationDisabled
	}

	switch in.Channel {
	// 需要判断账号是否存在
	case entity.SmsLoginChannel, entity.SmsForgetAccountChannel:
		if !c.UsersRepo.IsMobileExist(ctx, in.Mobile) {
			return nil, entity.ErrAccountOrPassword
		}

	// 需要判断账号是否存在
	case entity.SmsRegisterChannel, entity.SmsChangeAccountChannel:
		if c.UsersRepo.IsMobileExist(ctx, in.Mobile) {
			return nil, entity.ErrPhoneExist
		}
	case entity.SmsOauthBindChannel:
	default:
		return nil, entity.ErrSmsChannelInvalid
	}

	// 发送短信验证码
	code, err := c.SmsService.Send(ctx, in.Channel, in.Mobile)
	if err != nil {
		return nil, err
	}

	if in.Channel == entity.SmsRegisterChannel || in.Channel == entity.SmsChangeAccountChannel || in.Channel == entity.SmsOauthBindChannel {
		return &web.CommonSendSmsResponse{
			SmsCode: code,
		}, nil
	}

	return &web.CommonSendSmsResponse{}, nil
}

// SendEmail 发送邮件验证码接口
//
//	@Summary		发送邮件
//	@Description	发送邮件验证码
//	@Tags			公共
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.CommonSendEmailRequest	true	"发送邮件请求"
//	@Success		200		{object}	web.CommonSendEmailResponse
//	@Router			/api/v1/common/send-email [post]
func (c *Common) SendEmail(ctx context.Context, req *web.CommonSendEmailRequest) (*web.CommonSendEmailResponse, error) {
	// Determine the channel based on request
	channel := entity.EmailVerifyChannel
	if req.Channel != "" {
		channel = req.Channel
	}

	// Send verification code using EmailService
	code, err := c.EmailService.Send(ctx, channel, req.Email)
	if err != nil {
		return nil, err
	}

	// Prepare email template data
	templateData := map[string]string{
		"code":         code,
		"service_name": "邮箱验证",
		"company_name": "小火箭 IM",
		"domain":       "https://xhj.im",
	}

	// Render email template
	body, err := c.TemplateService.CodeTemplate(templateData)
	if err != nil {
		return nil, err
	}

	// Send email
	if c.EmailClient != nil {
		err = c.EmailClient.SendMail(&email.Option{
			To:      []string{req.Email},
			Subject: "验证码",
			Body:    body,
		})
		if err != nil {
			return nil, err
		}
	} else {
		// If email client is not configured, just log the code (for development)
		logger.Infof("Email verification code for %s: %s", req.Email, code)
	}

	return &web.CommonSendEmailResponse{}, nil
}

// Test 发送测试接口
//
//	@Summary		测试端点
//	@Description	内部测试端点
//	@Tags			公共
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.CommonSendTestRequest	true	"测试请求"
//	@Success		200		{object}	web.CommonSendTestResponse
//	@Router			/api/v1/common/send-test [post]
func (c *Common) Test(ctx context.Context, req *web.CommonSendTestRequest) (*web.CommonSendTestResponse, error) {
	// This is a test endpoint for internal testing purposes
	// Log the request for debugging
	logger.Infof("Test endpoint called with email: %s", req.Email)

	// Return empty response indicating success
	return &web.CommonSendTestResponse{}, nil
}

// jsonJSONArrayToListValue 将库中 JSON 数组文本反序列化为 proto ListValue（接口 JSON 输出为 JSON 数组）
func jsonJSONArrayToListValue(raw string) *structpb.ListValue {
	s := strings.TrimSpace(raw)
	if s == "" || s == "null" {
		return &structpb.ListValue{}
	}
	var arr []any
	if err := json.Unmarshal([]byte(s), &arr); err != nil {
		logger.Warnf("app_version release_notes/download_urls 期望 JSON 数组，解析失败: %v", err)
		return &structpb.ListValue{}
	}
	lv, err := structpb.NewList(arr)
	if err != nil {
		logger.Warnf("app_version JSON 数组转为 ListValue 失败: %v", err)
		return &structpb.ListValue{}
	}
	return lv
}

// AppVersionLatest 查询当前已发布（published_at <= 现在）的该平台最新版本记录
func (c *Common) AppVersionLatest(ctx context.Context, in *web.CommonAppVersionLatestRequest) (*web.CommonAppVersionLatestResponse, error) {
	row, err := c.AppVersionRepo.FindLatestPublished(ctx, strings.TrimSpace(in.GetPlatform()))
	if err != nil {
		return nil, err
	}
	if row == nil {
		return &web.CommonAppVersionLatestResponse{}, nil
	}
	return &web.CommonAppVersionLatestResponse{
		Id:                int32(row.Id),
		Platform:          row.Platform,
		Channel:           row.Channel,
		LatestVersionName: row.LatestVersionName,
		UpgradeType:       row.UpgradeType,
		Title:             row.Title,
		ReleaseNotes:      jsonJSONArrayToListValue(row.ReleaseNotes),
		DownloadUrls:      jsonJSONArrayToListValue(row.DownloadUrls),
		PublishedAt:       timeutil.FormatDatetime(row.PublishedAt),
	}, nil
}

// ExploreList 探索位列表（仅 is_open=1）
func (c *Common) ExploreList(ctx context.Context, _ *web.CommonExploreListRequest) (*web.CommonExploreListResponse, error) {
	if c.AppExploreRepo == nil {
		return nil, errors.New("AppExploreRepo 未注入，请执行 go generate 更新 wire_gen.go")
	}
	list, err := c.AppExploreRepo.ListOpen(ctx)
	if err != nil {
		return nil, err
	}
	out := &web.CommonExploreListResponse{Items: make([]*web.CommonExploreListResponse_Item, 0, len(list))}
	for _, row := range list {
		out.Items = append(out.Items, &web.CommonExploreListResponse_Item{
			Id:       int32(row.Id),
			Title:    row.Title,
			Image:    row.Image,
			Url:      row.Url,
			Position: row.Position,
			Sort:     int32(row.Sort),
		})
	}
	return out, nil
}

// AppModules 功能模块列表
func (c *Common) AppModules(ctx context.Context, _ *web.CommonAppModulesRequest) (*web.CommonAppModulesResponse, error) {
	if c.AppModuleRepo == nil {
		return nil, errors.New("AppModuleRepo 未注入，请执行 go generate 更新 wire_gen.go")
	}
	list, err := c.AppModuleRepo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	out := &web.CommonAppModulesResponse{Items: make([]*web.CommonAppModulesResponse_Item, 0, len(list))}
	for _, row := range list {
		out.Items = append(out.Items, &web.CommonAppModulesResponse_Item{
			Id:     int32(row.Id),
			Code:   row.Code,
			Title:  row.Title,
			IsOpen: int32(row.IsOpen),
		})
	}
	return out, nil
}

// NewsList 火箭资讯列表（仅已发布；支持 category_id 筛选与分页）
func (c *Common) NewsList(ctx context.Context, in *web.CommonNewsListRequest) (*web.CommonNewsListResponse, error) {
	if c.AppNewsRepo == nil {
		return nil, errors.New("AppNewsRepo 未注入，请执行 go generate 更新 wire_gen.go")
	}

	page := int(in.GetPage())
	if page <= 0 {
		page = 1
	}
	pageSize := int(in.GetPageSize())
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	total, list, err := c.AppNewsRepo.ListPublished(ctx, page, pageSize, int(in.GetCategoryId()))
	if err != nil {
		return nil, err
	}

	out := &web.CommonNewsListResponse{
		Items: make([]*web.CommonNewsListResponse_Item, 0, len(list)),
		Total: int32(total),
		Paginate: &web.Paginate{
			Page:  int32(page),
			Size:  int32(pageSize),
			Total: int32(total),
		},
	}
	for _, row := range list {
		item := &web.CommonNewsListResponse_Item{
			Id:         int32(row.Id),
			Title:      row.Title,
			CategoryId: int32(row.CategoryId),
			Cover:      row.Cover,
			Status:     int32(row.Status),
			CreatedAt:  timeutil.FormatDatetime(row.CreatedAt),
			Pv:         int32(row.Pv),
			Uv:         int32(row.Uv),
		}
		if row.TypeId != nil {
			item.TypeId = int32(*row.TypeId)
		}
		if row.UploadTime != nil {
			item.UploadTime = timeutil.FormatDatetime(*row.UploadTime)
		}
		if row.PublishTime != nil {
			item.PublishTime = timeutil.FormatDatetime(*row.PublishTime)
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}

// NewsDetail 火箭资讯详情（仅已发布；含 content、source_url）
func (c *Common) NewsDetail(ctx context.Context, in *web.CommonNewsDetailRequest) (*web.CommonNewsDetailResponse, error) {
	if c.AppNewsRepo == nil {
		return nil, errors.New("AppNewsRepo 未注入，请执行 go generate 更新 wire_gen.go")
	}

	row, err := c.AppNewsRepo.FindPublishedById(ctx, int(in.GetId()))
	if err != nil {
		return nil, err
	}
	if row == nil || row.Id == 0 {
		return nil, entity.ErrDataNotFound
	}

	out := &web.CommonNewsDetailResponse{
		Id:         int32(row.Id),
		Title:      row.Title,
		CategoryId: int32(row.CategoryId),
		Content:    row.Content,
		Cover:      row.Cover,
		SourceUrl:  row.SourceURL,
		Status:     int32(row.Status),
		CreatedAt:  timeutil.FormatDatetime(row.CreatedAt),
		Pv:         int32(row.Pv),
		Uv:         int32(row.Uv),
	}
	if row.TypeId != nil {
		out.TypeId = int32(*row.TypeId)
	}
	if row.UploadTime != nil {
		out.UploadTime = timeutil.FormatDatetime(*row.UploadTime)
	}
	if row.PublishTime != nil {
		out.PublishTime = timeutil.FormatDatetime(*row.PublishTime)
	}
	return out, nil
}

// NewsCategoryList 资讯分类列表（status=1，sort 倒序，不分页）
func (c *Common) NewsCategoryList(ctx context.Context, in *web.CommonNewsCategoryListRequest) (*web.CommonNewsCategoryListResponse, error) {
	if c.AppNewsCategoryRepo == nil {
		return nil, errors.New("AppNewsCategoryRepo 未注入，请执行 go generate 更新 wire_gen.go")
	}

	list, err := c.AppNewsCategoryRepo.ListVisibleByCollect(ctx, in.GetCollect())
	if err != nil {
		return nil, err
	}

	out := &web.CommonNewsCategoryListResponse{
		Items: make([]*web.CommonNewsCategoryListResponse_Item, 0, len(list)),
	}
	for _, row := range list {
		out.Items = append(out.Items, &web.CommonNewsCategoryListResponse_Item{
			Id:      int32(row.Id),
			Title:   row.Title,
			Collect: row.Collect,
			Status:  int32(row.Status),
			Sort:    int32(row.Sort),
		})
	}
	return out, nil
}

// NewsView 资讯阅读上报：未登录直接返回；已登录写 pv/uv，同用户同资讯 Redis 30s 去重
func (c *Common) NewsView(ctx context.Context, in *web.CommonNewsViewRequest) (*web.CommonNewsViewResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)
	if uid <= 0 {
		return &web.CommonNewsViewResponse{}, nil
	}
	if c.AppNewsViewRepo == nil || c.AppNewsViewCache == nil {
		return nil, errors.New("AppNewsView 依赖未注入，请执行 go generate 更新 wire_gen.go")
	}

	newsId := int(in.GetNewsId())
	ok, err := c.AppNewsViewCache.Acquire(ctx, newsId, uid)
	if err != nil {
		return nil, err
	}
	if !ok {
		return &web.CommonNewsViewResponse{}, nil
	}

	if err := c.AppNewsViewRepo.RecordView(ctx, newsId, uid); err != nil {
		return nil, err
	}
	return &web.CommonNewsViewResponse{}, nil
}

func dedupeAppDictKeys(keys []string) []string {
	seen := make(map[string]struct{}, len(keys))
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}

// parseAppDictValue 按字典 type 将库中 value 文本解析为 google.protobuf.Value（JSON 语义）
func parseAppDictValue(dictType, raw string) (*structpb.Value, error) {
	t := strings.TrimSpace(strings.ToLower(dictType))
	switch t {
	case "text":
		return structpb.NewStringValue(raw), nil
	case "num":
		s := strings.TrimSpace(raw)
		if s == "" {
			return structpb.NewNullValue(), nil
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, err
		}
		return structpb.NewNumberValue(f), nil
	case "list":
		s := strings.TrimSpace(raw)
		if s == "" || s == "null" {
			return structpb.NewListValue(&structpb.ListValue{}), nil
		}
		var arr []any
		if err := json.Unmarshal([]byte(s), &arr); err != nil {
			return nil, err
		}
		lv, err := structpb.NewList(arr)
		if err != nil {
			return nil, err
		}
		return structpb.NewListValue(lv), nil
	case "object":
		s := strings.TrimSpace(raw)
		if s == "" || s == "null" {
			return structpb.NewStructValue(&structpb.Struct{}), nil
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(s), &m); err != nil {
			return nil, err
		}
		st, err := structpb.NewStruct(m)
		if err != nil {
			return nil, err
		}
		return structpb.NewStructValue(st), nil
	default:
		return structpb.NewStringValue(raw), nil
	}
}

// AppDictGet 按 key 批量获取已启用字典项，parsed_value 已按 type 解析
func (c *Common) AppDictGet(ctx context.Context, in *web.CommonAppDictGetRequest) (*web.CommonAppDictGetResponse, error) {
	if c.AppDictRepo == nil {
		return nil, errors.New("AppDictRepo 未注入，请执行 go generate 更新 wire_gen.go")
	}
	keys := dedupeAppDictKeys(in.GetKeys())
	if len(keys) == 0 {
		return nil, errorx.New(400, "keys 不能为空")
	}
	rows, err := c.AppDictRepo.ListEnabledByKeys(ctx, keys)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]model.AppDict, len(rows))
	for _, row := range rows {
		byKey[row.DictKey] = row
	}
	out := &web.CommonAppDictGetResponse{Items: make([]*web.CommonAppDictItem, 0, len(byKey))}
	for _, k := range keys {
		row, ok := byKey[k]
		if !ok {
			continue
		}
		pv, err := parseAppDictValue(row.Type, row.Value)
		if err != nil {
			return nil, errorx.New(400, "字典 "+row.DictKey+" 的 value 与 type 不匹配，无法解析")
		}
		out.Items = append(out.Items, &web.CommonAppDictItem{
			Key:         row.DictKey,
			Title:       row.Title,
			Type:        row.Type,
			ParsedValue: pv,
		})
	}
	return out, nil
}
