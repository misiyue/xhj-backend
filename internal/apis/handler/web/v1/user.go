package v1

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/config"
	"github.com/gzydong/go-chat/external/wallet"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/logic"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/pkg/encrypt"
	"github.com/gzydong/go-chat/internal/pkg/encrypt/rsautil"
	"github.com/gzydong/go-chat/internal/pkg/timeutil"
	"github.com/gzydong/go-chat/internal/repository/cache"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"github.com/gzydong/go-chat/internal/service"
	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{5,19}$`)

var _ web.IUserHandler = (*User)(nil)

type User struct {
	Config              *config.Config
	Redis               *redis.Client
	UsersRepo           *repo.Users
	WalletUserRepo      *repo.WalletUser
	MerchantRepo        *repo.Merchant
	MerchantTaskRepo    *repo.MerchantTask
	MerchantPaytypeRepo   *repo.MerchantPaytype
	MerchantPaymentRepo   *repo.MerchantPayment
	MerchantOrderRepo     *repo.MerchantOrder
	MerchantOrderErrLogRepo *repo.MerchantOrderErrLog
	MerchantHdOrderRepo *repo.MerchantHdOrder
	MerchantHmOrderRepo *repo.MerchantHmOrder
	MerchantSessionRepo *repo.MerchantSession
	MerchantMessageRepo *repo.MerchantMessage
	PushMessage         *logic.PushMessage
	MessageStorage      *cache.MessageStorage
	UnreadStorage       *cache.UnreadStorage
	UserClient          *cache.UserClient
	NoticeTemplateRepo  *repo.NoticeTemplate
	SysNotice           *logic.SysNotice
	TalkSessionService  service.ITalkSessionService
	ContactApplyService service.IContactApplyService
	GroupApplyStorage   *cache.GroupApplyStorage
	NoticeLetterRepo    *repo.NoticeLetter
	OrganizeRepo        *repo.Organize
	UserService         service.IUserService
	SmsService          service.ISmsService
	EmailService        service.IEmailService
	Rsa                 rsautil.IRsa
}

// Detail 获取登录用户详情接口
//
//	@Summary		用户详情
//	@Description	获取当前登录用户的详细信息
//	@Tags			用户
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.UserDetailRequest	true	"用户详情请求"
//	@Success		200		{object}	web.UserDetailResponse
//	@Router			/api/v1/user/detail [post]
//	@Security		Bearer
func (u *User) Detail(ctx context.Context, _ *web.UserDetailRequest) (*web.UserDetailResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)

	user, err := u.UsersRepo.FindByIdWithCache(ctx, int(session.UserId))
	if err != nil {
		return nil, err
	}

	return &web.UserDetailResponse{
		Id:       int32(user.Id),
		UserId:   int32(user.UserId),
		Username: user.Username,
		Mobile:   lo.FromPtr(user.Mobile),
		Nickname: user.Nickname,
		Avatar:   user.Avatar,
		Gender:   int32(user.Gender),
		Motto:    user.Motto,
		Email:    user.Email,
		Birthday: user.Birthday,
		Uuid:     int32(user.Uuid),
		UserCode:  user.UserCode,
		IsTrans:   int32(user.IsTrans),
		CreatedAt: timeutil.FormatDatetime(user.CreatedAt),
		IsSubscribe: int32(user.IsSubscribe),
		UnUpdateAt: func() string {
			if user.UnUpdateAt == nil {
				return ""
			}
			return timeutil.FormatDatetime(*user.UnUpdateAt)
		}(),
		UnUpdateEnable: usernameUpdateEnable(user.UnUpdateAt),
	}, nil
}

// Setting 获取用户配置信息接口
//
//	@Summary		用户设置
//	@Description	获取用户配置和个人资料设置
//	@Tags			用户
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.UserSettingRequest	true	"用户设置请求"
//	@Success		200		{object}	web.UserSettingResponse
//	@Router			/api/v1/user/setting [post]
//	@Security		Bearer
func (u *User) Setting(ctx context.Context, req *web.UserSettingRequest) (*web.UserSettingResponse, error) {
	session, err := middleware.FormContext[entity.WebClaims](ctx)
	if err != nil {
		return nil, err
	}

	user, err := u.UsersRepo.FindByIdWithCache(ctx, int(session.UserId))
	if err != nil {
		return nil, err
	}

	isOk, err := u.OrganizeRepo.IsQiyeMember(ctx, int(session.UserId))
	if err != nil {
		return nil, err
	}

	return &web.UserSettingResponse{
		UserInfo: &web.UserSettingResponse_UserInfo{
			Uid:      int32(user.Id),
			UserId:   int32(user.UserId),
			Username: user.Username,
			Nickname: user.Nickname,
			Avatar:   user.Avatar,
			Motto:    user.Motto,
			Gender:   int32(user.Gender),
			IsQiye:   isOk,
			Mobile:   lo.FromPtr(user.Mobile),
			Email:    user.Email,
			Uuid:     int32(user.Uuid),
			UserCode: user.UserCode,
			IsTrans:  int32(user.IsTrans),
		},
		Setting: &web.UserSettingResponse_ConfigInfo{},
	}, nil
}

// DetailUpdate 更新用户信息接口
//
//	@Summary		更新用户详情
//	@Description	更新用户个人资料信息，如昵称、头像、性别等
//	@Tags			用户
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.UserDetailUpdateRequest	true	"更新详情请求"
//	@Success		200		{object}	web.UserDetailUpdateResponse
//	@Router			/api/v1/user/detail-update [post]
//	@Security		Bearer
func (u *User) DetailUpdate(ctx context.Context, req *web.UserDetailUpdateRequest) (*web.UserDetailUpdateResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)

	if req.Birthday != "" {
		if !timeutil.IsDate(req.Birthday) {
			return nil, errorx.New(400, "birthday 错误")
		}
	}

	uid := session.UserId

	_, err := u.UsersRepo.UpdateById(ctx, uid, map[string]any{
		"nickname": strings.TrimSpace(strings.ReplaceAll(req.Nickname, " ", "")),
		"avatar":   req.Avatar,
		"gender":   req.Gender,
		"motto":    req.Motto,
		"birthday": req.Birthday,
	})

	if err != nil {
		return nil, err
	}

	_ = u.UsersRepo.ClearTableCache(ctx, int(uid))
	return &web.UserDetailUpdateResponse{}, nil
}

// PasswordUpdate 更新用户密码接口
//
//	@Summary		更新密码
//	@Description	修改用户登录密码
//	@Tags			用户
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.UserPasswordUpdateRequest	true	"更新密码请求"
//	@Success		200		{object}	web.UserPasswordUpdateResponse
//	@Router			/api/v1/user/password-update [post]
//	@Security		Bearer
func (u *User) PasswordUpdate(ctx context.Context, in *web.UserPasswordUpdateRequest) (*web.UserPasswordUpdateResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)

	uid := session.UserId
	if uid == 2054 || uid == 2055 {
		return nil, entity.ErrPermissionDenied
	}

	oldPassword, err := u.Rsa.Decrypt(in.OldPassword)
	if err != nil {
		return nil, err
	}

	newPassword, err := u.Rsa.Decrypt(in.NewPassword)
	if err != nil {
		return nil, err
	}

	if err := u.UserService.UpdatePassword(ctx, int(uid), string(oldPassword), string(newPassword)); err != nil {
		return nil, err
	}

	_ = u.UsersRepo.ClearTableCache(ctx, int(uid))
	return nil, nil
}

// MobileUpdate 更新用户手机号接口
//
//	@Summary		更新手机号
//	@Description	修改用户绑定的手机号码
//	@Tags			用户
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.UserMobileUpdateRequest	true	"更新手机号请求"
//	@Success		200		{object}	web.UserMobileUpdateResponse
//	@Router			/api/v1/user/mobile-update [post]
//	@Security		Bearer
func (u *User) MobileUpdate(ctx context.Context, in *web.UserMobileUpdateRequest) (*web.UserMobileUpdateResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := session.UserId

	user, _ := u.UsersRepo.FindById(ctx, uid)
	if lo.FromPtr(user.Mobile) == in.Mobile {
		return nil, errorx.New(400, "手机号与原手机号一致无需修改")
	}

	password, err := u.Rsa.Decrypt(in.Password)
	if err != nil {
		return nil, err
	}

	if !encrypt.VerifyPassword(user.Password, string(password), user.Salt) {
		return nil, entity.ErrAccountOrPasswordError
	}

	if uid == 2054 || uid == 2055 {
		return nil, entity.ErrPermissionDenied
	}

	if !u.SmsService.Verify(ctx, entity.SmsChangeAccountChannel, in.Mobile, in.SmsCode) {
		return nil, entity.ErrSmsCodeError
	}

	_, err = u.UsersRepo.UpdateById(ctx, user.Id, map[string]any{
		"mobile": in.Mobile,
	})

	if err != nil {
		return nil, err
	}

	_ = u.UsersRepo.ClearTableCache(ctx, user.Id)
	return nil, nil
}

// EmailUpdate 更新用户邮箱接口
//
//	@Summary		更新邮箱
//	@Description	修改用户绑定的邮箱地址
//	@Tags			用户
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.UserEmailUpdateRequest	true	"更新邮箱请求"
//	@Success		200		{object}	web.UserEmailUpdateResponse
//	@Router			/api/v1/user/email-update [post]
//	@Security		Bearer
func (u *User) EmailUpdate(ctx context.Context, req *web.UserEmailUpdateRequest) (*web.UserEmailUpdateResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := session.UserId

	newEmail := strings.TrimSpace(req.Email)
	user, _ := u.UsersRepo.FindById(ctx, uid)
	if user.Email == newEmail {
		return nil, errorx.New(400, "邮箱与原邮箱一致无需修改")
	}

	password, err := u.Rsa.Decrypt(req.Password)
	if err != nil {
		return nil, err
	}

	if !encrypt.VerifyPassword(user.Password, string(password), user.Salt) {
		return nil, entity.ErrAccountOrPasswordError
	}

	if uid == 2054 || uid == 2055 {
		return nil, entity.ErrPermissionDenied
	}

	if !u.EmailService.Verify(ctx, entity.EmailVerifyChannel, newEmail, req.Code) {
		return nil, errorx.New(400, "邮箱验证码错误")
	}

	// 全局查重：已被其他未注销账号绑定则禁止
	if other, _ := u.UsersRepo.FindByEmail(ctx, newEmail); other != nil && other.Id > 0 && other.Id != user.Id && !other.IsCancelled() {
		return nil, errorx.New(400, "该邮箱已被其他账号使用")
	}

	_, err = u.UsersRepo.UpdateById(ctx, user.Id, map[string]any{
		"email": newEmail,
	})

	if err != nil {
		return nil, err
	}

	u.EmailService.Delete(ctx, entity.EmailVerifyChannel, newEmail)

	_ = u.UsersRepo.ClearTableCache(ctx, user.Id)
	return &web.UserEmailUpdateResponse{}, nil
}

// UsernameUpdate 更新登录用户名
//
//	@Summary		更新用户名
//	@Description	修改登录用户名（username）；3 个月仅可修改一次
//	@Tags			用户
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.UserUsernameUpdateRequest	true	"更新用户名请求"
//	@Success		200		{object}	web.UserUsernameUpdateResponse
//	@Router			/api/v1/user/username-update [post]
//	@Security		Bearer
func (u *User) UsernameUpdate(ctx context.Context, req *web.UserUsernameUpdateRequest) (*web.UserUsernameUpdateResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := int(session.UserId)

	username := strings.TrimSpace(req.GetUsername())
	if !isValidUsername(username) {
		return nil, errorx.New(400, "用户名须为 6-20 位，以字母开头，仅支持字母、数字、下划线、减号")
	}

	password := strings.TrimSpace(req.GetPassword())
	emailCode := strings.TrimSpace(req.GetEmailCode())
	if password == "" && emailCode == "" {
		return nil, errorx.New(400, "请提供登录密码或邮箱验证码进行验证")
	}

	user, err := u.UsersRepo.FindById(ctx, uid)
	if err != nil || user == nil || user.Id == 0 {
		return nil, entity.ErrUserNotExist
	}
	if user.Username == username {
		return nil, errorx.New(400, "用户名与原用户名一致无需修改")
	}

	// 密码或邮箱验证码至少一种验证通过
	verified := false
	var verifyErr error
	if password != "" {
		plain, decErr := u.Rsa.Decrypt(password)
		if decErr != nil {
			verifyErr = decErr
		} else if encrypt.VerifyPassword(user.Password, string(plain), user.Salt) {
			verified = true
		} else {
			verifyErr = entity.ErrAccountOrPasswordError
		}
	}
	if !verified && emailCode != "" {
		email := strings.TrimSpace(user.Email)
		if email == "" {
			return nil, errorx.New(400, "当前账号未绑定邮箱，无法使用邮箱验证码")
		}
		if u.EmailService.Verify(ctx, entity.EmailVerifyChannel, email, emailCode) {
			verified = true
			u.EmailService.Delete(ctx, entity.EmailVerifyChannel, email)
		} else {
			verifyErr = errorx.New(400, "邮箱验证码错误或已过期")
		}
	}
	if !verified {
		if verifyErr != nil {
			return nil, verifyErr
		}
		return nil, errorx.New(400, "请提供登录密码或邮箱验证码进行验证")
	}

	if user.UnUpdateAt != nil {
		nextAt := user.UnUpdateAt.AddDate(0, 3, 0)
		if time.Now().Before(nextAt) {
			return nil, errorx.New(400, fmt.Sprintf("用户名每 3 个月仅可修改一次，下次可修改时间：%s", nextAt.Format(time.DateTime)))
		}
	}

	if other, _ := u.UsersRepo.FindByUsername(ctx, username); other != nil && other.Id > 0 && other.Id != user.Id {
		return nil, errorx.New(400, "该用户名已被占用")
	}

	now := time.Now()
	if _, err := u.UsersRepo.UpdateById(ctx, user.Id, map[string]any{
		"username":     username,
		"un_update_at": now,
		"updated_at":   now,
	}); err != nil {
		return nil, err
	}
	_ = u.UsersRepo.ClearTableCache(ctx, user.Id)
	return &web.UserUsernameUpdateResponse{}, nil
}

// isValidUsername 6-20 位；字母开头；仅字母、数字、下划线、减号
func isValidUsername(username string) bool {
	if len(username) < 6 || len(username) > 20 {
		return false
	}
	return usernamePattern.MatchString(username)
}

// usernameUpdateEnable 是否可修改 username：0-否，1-是（从未改过或距上次已满 3 个月）
func usernameUpdateEnable(unUpdateAt *time.Time) int32 {
	if unUpdateAt == nil {
		return 1
	}
	if time.Now().Before(unUpdateAt.AddDate(0, 3, 0)) {
		return 0
	}
	return 1
}

// SubscribeUpdate 更新通知订阅状态
func (u *User) SubscribeUpdate(ctx context.Context, in *web.UserSubscribeUpdateRequest) (*web.UserSubscribeUpdateResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	isSubscribe := int(in.GetIsSubscribe())
	if isSubscribe != model.UsersSubscribeNo && isSubscribe != model.UsersSubscribeYes {
		return nil, errorx.New(400, "is_subscribe 必须为 0 或 1")
	}
	if _, err := u.UsersRepo.UpdateById(ctx, session.UserId, map[string]any{
		"is_subscribe": isSubscribe,
	}); err != nil {
		return nil, err
	}
	_ = u.UsersRepo.ClearTableCache(ctx, int(session.UserId))
	return &web.UserSubscribeUpdateResponse{}, nil
}

func (u *User) resolveWalletUID(ctx context.Context, uid int) (int, error) {
	user, err := u.UsersRepo.FindById(ctx, uid)
	if err != nil || user == nil || user.Id == 0 {
		return 0, errorx.New(400, "用户不存在")
	}
	if u.WalletUserRepo != nil {
		if wu, werr := u.WalletUserRepo.FindByUserId(ctx, uid); werr == nil && wu != nil && wu.WalletUID != 0 {
			return wu.WalletUID, nil
		}
	}
	if user.Uuid != 0 {
		return user.Uuid, nil
	}
	return 0, errorx.New(400, "钱包账户未就绪，无法冻结保证金")
}

func walletFreezeErr(err error) error {
	msg := strings.TrimSpace(strings.TrimPrefix(err.Error(), "wallet api error: "))
	if msg == "" {
		msg = "保证金冻结失败"
	}
	return errorx.New(400, msg)
}

const merchantSuretyFreezeTimeout = 5 * time.Second

// freezeMerchantSurety 调用钱包冻结保证金，返回 bill_id 写入 merchant.surety_bill_id。
func (u *User) freezeMerchantSurety(ctx context.Context, uid int, surety float64) (int, error) {
	if surety < 500 {
		return 0, errorx.New(400, "保证金不得低于 500 元")
	}
	wc := wallet.GetClient()
	if wc == nil {
		return 0, errorx.New(500, "钱包服务未初始化，请检查配置 wallet.base_url 与 wallet.key")
	}
	walletUID, err := u.resolveWalletUID(ctx, uid)
	if err != nil {
		return 0, err
	}

	freezeCtx, cancel := context.WithTimeout(ctx, merchantSuretyFreezeTimeout)
	defer cancel()

	type freezeResult struct {
		billID int
		err    error
	}
	ch := make(chan freezeResult, 1)
	go func() {
		billID, err := wc.FreezeAccount(fmt.Sprintf("%d", uid), walletUID, surety, 1)
		ch <- freezeResult{billID: billID, err: err}
	}()

	select {
	case <-freezeCtx.Done():
		return 0, errorx.New(400, "保证金冻结失败")
	case res := <-ch:
		if res.err != nil {
			return 0, walletFreezeErr(res.err)
		}
		if res.billID <= 0 {
			return 0, errorx.New(400, "保证金冻结失败")
		}
		return res.billID, nil
	}
}

func newMerchantApply(uid int, in *web.MerchantApplyRequest, surety float64, billID int) *model.Merchant {
	return &model.Merchant{
		UserId:       uid,
		Nickname:     strings.TrimSpace(in.GetNickname()),
		Realname:     strings.TrimSpace(in.GetRealname()),
		Nation:       strings.TrimSpace(in.GetNation()),
		IdType:       int(in.GetIdType()),
		Idcard:       strings.TrimSpace(in.GetIdcard()),
		Image:        strings.TrimSpace(in.GetImage()),
		Backimage:    strings.TrimSpace(in.GetBackImage()),
		Surety:       surety,
		SuretyBillId: billID,
		Status:       model.MerchantStatusPending,
		Reason:       "",
	}
}

func merchantApplyUpdates(rec *model.Merchant) map[string]any {
	return map[string]any{
		"nickname":       rec.Nickname,
		"realname":       rec.Realname,
		"nation":         rec.Nation,
		"id_type":        rec.IdType,
		"idcard":         rec.Idcard,
		"image":          rec.Image,
		"backimage":      rec.Backimage,
		"surety":         rec.Surety,
		"surety_bill_id": rec.SuretyBillId,
		"status":         rec.Status,
		"reason":         rec.Reason,
	}
}

// MerchantApply 商户入驻申请（POST /api/v1/merchant/apply）
// 首次申请插入新记录；驳回后再次申请在同一记录上更新资料并将 status 置为待审核。
func (u *User) MerchantApply(ctx context.Context, in *web.MerchantApplyRequest) (*web.MerchantApplyResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := int(session.UserId)

	password := strings.TrimSpace(in.GetPassword())
	if password == "" {
		return nil, errorx.New(400, "密码不能为空")
	}
	user, err := u.UsersRepo.FindById(ctx, uid)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errorx.New(404, "用户不存在")
	}
	if strings.TrimSpace(user.Trans) == "" {
		return nil, errorx.New(400, "请先设置交易密码")
	}
	if !encrypt.VerifyPassword(user.Trans, password, user.Salt) {
		return nil, errorx.New(400, "交易密码错误")
	}

	surety := in.GetSurety()

	hasApproved, err := u.MerchantRepo.HasApprovedByUserId(ctx, uid)
	if err != nil {
		return nil, err
	}
	if hasApproved {
		return nil, errorx.New(400, "您已有审核通过的商户资质，无法再次申请")
	}

	latest, err := u.MerchantRepo.FindLatestByUserId(ctx, uid)
	if err != nil {
		return nil, err
	}
	if latest != nil {
		switch latest.Status {
		case model.MerchantStatusPending:
			return nil, errorx.New(400, "您有待审核的商户申请，请勿重复提交")
		case model.MerchantStatusApproved:
			return nil, errorx.New(400, "您已是认证商户，无法再次申请")
		case model.MerchantStatusRejected:
			// 驳回后可重新申请
		case model.MerchantStatusApplyCancel:
			return nil, errorx.New(400, "商户注销申请审核中，请勿重复提交")
		case model.MerchantStatusCancelled:
			// 已注销，可重新入驻
		default:
			return nil, errorx.New(400, "当前申请状态不允许重复提交")
		}
	}

	billID, err := u.freezeMerchantSurety(ctx, uid, surety)
	if err != nil {
		return nil, err
	}
	rec := newMerchantApply(uid, in, surety, billID)

	if latest != nil && latest.Status == model.MerchantStatusRejected {
		if err := u.MerchantRepo.UpdateById(ctx, latest.Id, merchantApplyUpdates(rec)); err != nil {
			return nil, err
		}
		return &web.MerchantApplyResponse{Id: int32(latest.Id)}, nil
	}

	if err := u.MerchantRepo.Create(ctx, rec); err != nil {
		return nil, err
	}
	return &web.MerchantApplyResponse{Id: int32(rec.Id)}, nil
}

// MerchantStatus 查询本人最近一次商户申请状态
func (u *User) MerchantStatus(ctx context.Context, _ *web.MerchantStatusRequest) (*web.MerchantStatusResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	latest, err := u.MerchantRepo.FindLatestByUserId(ctx, int(session.UserId))
	if err != nil {
		return nil, err
	}
	if latest == nil {
		return &web.MerchantStatusResponse{HasApplication: false}, nil
	}
	return u.merchantToStatusResponse(ctx, latest)
}

func (u *User) merchantPayTypesToProto(ctx context.Context, payTypesJSON string) (map[string]*web.MerchantPayTypeLimit, error) {
	refs := model.ParseMerchantPayTypeRefs(payTypesJSON)
	if len(refs) == 0 {
		return nil, nil
	}
	if u.MerchantPaymentRepo == nil {
		return nil, errors.New("MerchantPaymentRepo 未注入，请执行 go generate 更新 wire_gen.go")
	}
	ids := make([]int, 0, len(refs))
	for _, id := range refs {
		ids = append(ids, id)
	}
	paymentMap, err := u.MerchantPaymentRepo.MapByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	payTypes := make(map[string]*web.MerchantPayTypeLimit, len(refs))
	for key, id := range refs {
		p, ok := paymentMap[id]
		if !ok || p == nil {
			continue
		}
		var min, max float64
		if p.Min != nil {
			min = float64(*p.Min)
		}
		if p.Max != nil {
			max = float64(*p.Max)
		}
		payTypes[key] = &web.MerchantPayTypeLimit{
			Min:     min,
			Max:     max,
			PayType: p.Code,
		}
	}
	return payTypes, nil
}

func (u *User) merchantToStatusResponse(ctx context.Context, m *model.Merchant) (*web.MerchantStatusResponse, error) {
	payTypes, err := u.merchantPayTypesToProto(ctx, m.PayTypes)
	if err != nil {
		return nil, err
	}
	cancelApplyAt := ""
	if m.CancelApplyAt != nil {
		cancelApplyAt = timeutil.FormatDatetime(*m.CancelApplyAt)
	}
	cancelAt := ""
	if m.CancelAt != nil {
		cancelAt = timeutil.FormatDatetime(*m.CancelAt)
	}
	return &web.MerchantStatusResponse{
		HasApplication: true,
		Id:             int32(m.Id),
		Nickname:       m.Nickname,
		Realname:       m.Realname,
		Nation:         m.Nation,
		IdType:         int32(m.IdType),
		Idcard:         m.Idcard,
		Image:          m.Image,
		BackImage:      m.Backimage,
		Surety:         m.Surety,
		Status:         int32(m.Status),
		Reason:         m.Reason,
		IsLimit:        int32(m.IsLimit),
		LimitTime:      int32(m.LimitTime),
		IsFrozen:       int32(m.IsFrozen),
		FrozenTime:     int32(m.FrozenTime),
		IsClose:        int32(m.IsClose),
		CreatedAt:      timeutil.FormatDatetime(m.CreatedAt),
		UpdatedAt:      timeutil.FormatDatetime(m.UpdatedAt),
		PayTypes:       payTypes,
		CarReason:      m.CarReason,
		CancelApplyAt:  cancelApplyAt,
		CancelAt:       cancelAt,
	}, nil
}

// MerchantProfile 获取本人商户资料（与 MerchantStatus 数据一致，供表单查看/编辑回填）
func (u *User) MerchantProfile(ctx context.Context, _ *web.MerchantProfileRequest) (*web.MerchantStatusResponse, error) {
	return u.MerchantStatus(ctx, &web.MerchantStatusRequest{})
}

// MerchantCancelApply 商户注销申请（POST /api/v1/merchant/cancel-apply）
func (u *User) MerchantCancelApply(ctx context.Context, _ *web.MerchantCancelApplyRequest) (*web.MerchantCancelApplyResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := int(session.UserId)

	m, err := u.MerchantRepo.FindLatestByUserId(ctx, uid)
	if err != nil {
		return nil, err
	}
	if m == nil || m.Status != model.MerchantStatusApproved {
		return nil, errorx.New(400, "仅审核通过且正常营业的商户可申请注销")
	}

	now := time.Now().Unix()
	if m.IsClose != 0 {
		return nil, errorx.New(400, "商户已关停，无法申请注销")
	}
	if m.IsFrozen != 0 && int64(m.FrozenTime) > now {
		return nil, errorx.New(400, "商户处于封禁状态，无法申请注销")
	}
	if m.IsLimit != 0 && int64(m.LimitTime) > now {
		return nil, errorx.New(400, "商户处于限制状态，无法申请注销")
	}

	hasListed, err := u.MerchantTaskRepo.HasListedUnsoldByUserId(ctx, uid)
	if err != nil {
		return nil, err
	}
	if hasListed {
		return nil, errorx.New(400, "存在未卖完的上架挂单，请先下架或等待售完后再申请注销")
	}

	if u.MerchantOrderRepo != nil {
		hasActiveOrder, err := u.MerchantOrderRepo.HasActiveOrdersBySalerId(ctx, uid)
		if err != nil {
			return nil, err
		}
		if hasActiveOrder {
			return nil, errorx.New(400, "你的商户有订单未结束")
		}
	}

	if err := u.MerchantRepo.UpdateById(ctx, m.Id, map[string]any{
		"status":          model.MerchantStatusApplyCancel,
		"cancel_apply_at": time.Now(),
	}); err != nil {
		return nil, err
	}
	return &web.MerchantCancelApplyResponse{}, nil
}
