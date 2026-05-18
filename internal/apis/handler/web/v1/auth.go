package v1

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/mojocn/base64Captcha"

	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/pkg/encrypt/aesutil"
	"github.com/gzydong/go-chat/internal/pkg/encrypt/rsautil"
	"github.com/gzydong/go-chat/internal/pkg/jwtutil"
	"github.com/gzydong/go-chat/internal/pkg/utils"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/redis/go-redis/v9"

	"github.com/gzydong/go-chat/api/pb/queue/v1"
	"github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/config"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/jsonutil"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/repository/cache"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"github.com/gzydong/go-chat/internal/service"
)

var _ web.IAuthHandler = (*Auth)(nil)

type Auth struct {
	Config              *config.Config
	Redis               *redis.Client
	RegisterLimiter     *cache.RegisterLimiter
	JwtTokenStorage     *cache.JwtTokenStorage
	RedisLock           *cache.RedisLock
	RobotRepo           *repo.Robot
	OAuthUsersRepo      *repo.OAuthUsers
	UsersRepo           *repo.Users
	SmsService          service.ISmsService
	EmailService        service.IEmailService
	UserService         service.IUserService
	ArticleClassService service.IArticleClassService
	InviteCodeService   service.IInviteCodeService
	Rsa                 rsautil.IRsa
	OauthService        service.IOAuthService
	AesUtil             aesutil.IAesUtil
	ICaptcha            *base64Captcha.Captcha
}

// Login 登录
//
//	@Summary		登录
//	@Description	使用账号（请求字段 mobile，对应 users.username）与密码进行身份验证
//	@Tags			认证
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.AuthLoginRequest	true	"登录请求"
//	@Success		200		{object}	web.AuthLoginResponse
//	@Router			/api/v1/auth/login [post]
func (a *Auth) Login(ctx context.Context, in *web.AuthLoginRequest) (*web.AuthLoginResponse, error) {
	skipCaptcha := a.Config.App != nil && a.Config.App.DisableLoginCaptcha
	if !skipCaptcha && in.CaptchaVoucher != "" {
		if in.GetCaptchaVoucher() == "" || in.GetCaptcha() == "" {
			return nil, errorx.New(400, "请输入图形验证码")
		}
		if a.ICaptcha == nil || !a.ICaptcha.Verify(in.GetCaptchaVoucher(), in.GetCaptcha(), true) {
			return nil, errorx.New(400, "图形验证码错误或已过期")
		}
	}

	password, err := a.Rsa.Decrypt(in.Password)
	if err != nil {
		return nil, err
	}

	account := strings.TrimSpace(in.GetMobile())
	if account == "" {
		return nil, errorx.New(400, "请填写登录账号")
	}

	user, err := a.UserService.Login(ctx, account, string(password))
	if err != nil {
		return nil, err
	}

	ip := ""
	userAgent := ""

	data := jsonutil.Marshal(queue.UserLoginRequest{
		UserId:   int32(user.Id),
		IpAddr:   ip,
		Platform: in.Platform,
		Agent:    userAgent,
		LoginAt:  time.Now().Format(time.DateTime),
	})

	if err := a.Redis.Publish(ctx, entity.LoginTopic, data).Err(); err != nil {
		logger.ErrorWithFields(
			"投递登录消息异常", err,
			queue.UserLoginRequest{
				UserId:   int32(user.Id),
				IpAddr:   ip,
				Platform: in.Platform,
				Agent:    userAgent,
				LoginAt:  time.Now().Format(time.DateTime),
			},
		)
	}

	authorize, err := a.authorize(user.Id)
	if err != nil {
		return nil, err
	}

	return &web.AuthLoginResponse{
		Type:        authorize.Type,
		AccessToken: authorize.AccessToken,
		ExpiresIn:   authorize.ExpiresIn,
		IsTrans:     int32(user.IsTrans),
	}, nil
}

// Captcha 获取登录用图形验证码（先调本接口再登录）；响应中的 disable_login_captcha 与配置 app.disable_login_captcha 一致
func (a *Auth) Captcha(ctx context.Context, _ *web.AuthCaptchaRequest) (*web.AuthCaptchaResponse, error) {
	disabled := a.Config.App != nil && a.Config.App.DisableLoginCaptcha
	if disabled {
		return &web.AuthCaptchaResponse{
			DisableLoginCaptcha: true,
			Voucher:             "",
			Captcha:             "",
		}, nil
	}
	if a.ICaptcha == nil {
		return nil, errorx.New(500, "验证码服务未就绪")
	}
	voucher, imageB64, _, err := a.ICaptcha.Generate()
	if err != nil {
		return nil, err
	}
	return &web.AuthCaptchaResponse{
		DisableLoginCaptcha: false,
		Voucher:             voucher,
		Captcha:             imageB64,
	}, nil
}

// Register 注册
//
//	@Summary		注册
//	@Description	创建新用户账户
//	@Tags			认证
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.AuthRegisterRequest	true	"注册请求"
//	@Success		200		{object}	web.AuthRegisterResponse
//	@Router			/api/v1/auth/register [post]
func (a *Auth) Register(ctx context.Context, in *web.AuthRegisterRequest) (*web.AuthRegisterResponse, error) {
	// 至少需要手机号或邮箱其中一个
	if in.Mobile == "" && in.Email == "" {
		return nil, errorx.New(400, "手机号或邮箱至少需要提供一个")
	}

	// 检查是否允许手机号注册
	if in.Mobile != "" && !a.Config.App.AllowPhoneRegistration {
		return nil, entity.ErrPhoneRegistrationDisabled
	}

	if in.Mobile != "" && !utils.IsMobile(in.Mobile) {
		return nil, errorx.New(400, "手机号格式不对")
	}

	// 邀请码：必填时不能为空；若填写且有效则解析邀请人 user_id 写入 users.invite_user_id
	var inviteUserId int
	if a.Config.App.RequireInviteCode && in.InviteCode == "" {
		return nil, errorx.New(400, "邀请码不能为空")
	}
	if in.InviteCode != "" {
		ok, inviterId, err := a.InviteCodeService.ResolveInviter(ctx, in.InviteCode)
		if err != nil {
			return nil, err
		}
		if !ok {
			if a.Config.App.RequireInviteCode {
				return nil, errorx.New(400, "邀请码无效或已过期")
			}
			// 非必填场景：无效邀请码忽略，不记录邀请人
		} else {
			inviteUserId = inviterId
		}
	}

	// 如果提供了手机号，验证短信验证码
	if in.Mobile != "" {
		if in.SmsCode == "" {
			return nil, errorx.New(400, "短信验证码不能为空")
		}
		// 验证短信验证码是否正确
		if !a.SmsService.Verify(ctx, entity.SmsRegisterChannel, in.Mobile, in.SmsCode) {
			return nil, entity.ErrSmsCodeError
		}
	}

	// 如果提供了邮箱，验证邮箱验证码
	if in.Email != "" {
		if in.EmailCode == "" {
			return nil, errorx.New(400, "邮箱验证码不能为空")
		}
		logger.InfofContext(ctx, "[Register] Verifying email code - email: %s, channel: %s, code: %s", in.Email, entity.EmailRegisterChannel, in.EmailCode)
		// 验证邮箱验证码是否正确
		if !a.EmailService.Verify(ctx, entity.EmailRegisterChannel, in.Email, in.EmailCode) {
			logger.ErrorfContext(ctx, "[Register] Email verification failed - email: %s, channel: %s, code: %s", in.Email, entity.EmailRegisterChannel, in.EmailCode)
			return nil, errorx.New(400, "邮箱验证码错误或已过期")
		}
		logger.InfofContext(ctx, "[Register] Email verification succeeded - email: %s", in.Email)
	}

	password, err := a.Rsa.Decrypt(in.Password)
	if err != nil {
		return nil, err
	}

	appCfg := a.Config.App
	ipLimit, devLimit := 0, 0
	if appCfg != nil {
		ipLimit = appCfg.RegisterIPLimit
		devLimit = appCfg.RegisterDeviceLimit
	}
	deviceCode := strings.TrimSpace(in.GetDeviceCode())
	if devLimit > 0 && deviceCode == "" {
		return nil, errorx.New(400, "请提供设备码")
	}

	clientIP := middleware.ClientIPFromContext(ctx)

	var releaseIP, releaseDev func(context.Context)

	if a.RegisterLimiter != nil && ipLimit > 0 {
		var relErr error
		releaseIP, relErr = a.RegisterLimiter.TryAcquireIP(ctx, clientIP, ipLimit)
		if relErr != nil {
			if errors.Is(relErr, cache.ErrRegisterIPExceeded) {
				return nil, errorx.New(429, "该 IP 24 小时内注册次数已达上限")
			}
			return nil, relErr
		}
	}

	if a.RegisterLimiter != nil && devLimit > 0 {
		var relErr error
		releaseDev, relErr = a.RegisterLimiter.TryAcquireDevice(ctx, deviceCode, devLimit)
		if relErr != nil {
			if releaseIP != nil {
				releaseIP(ctx)
			}
			if errors.Is(relErr, cache.ErrRegisterDeviceExceeded) {
				return nil, errorx.New(429, "该设备注册次数已达上限")
			}
			return nil, relErr
		}
	}

	user, err := a.UserService.Register(ctx, &service.UserRegisterOpt{
		Nickname:     in.Nickname,
		Mobile:       in.Mobile,
		Email:        in.Email,
		Password:     string(password),
		Platform:     in.Platform,
		InviteUserId: inviteUserId,
		DeviceCode:   deviceCode,
		// Username 没有前端字段时，内部会自动用 mobile/email/nickname 生成
	})

	if err != nil {
		if releaseIP != nil {
			releaseIP(ctx)
		}
		if releaseDev != nil {
			releaseDev(ctx)
		}
		return nil, err
	}

	// 使用邀请码（如果提供了）
	if in.InviteCode != "" {
		if err := a.InviteCodeService.UseInviteCode(ctx, in.InviteCode, user.Id); err != nil {
			logger.ErrorWithFields("使用邀请码失败", err, map[string]interface{}{
				"invite_code": in.InviteCode,
				"user_id":     user.Id,
			})
		}
	}

	// 删除短信验证码（如果使用了）
	if in.Mobile != "" && in.SmsCode != "" {
		a.SmsService.Delete(ctx, entity.SmsRegisterChannel, in.Mobile)
	}

	// 删除邮箱验证码（如果使用了）
	if in.Email != "" && in.EmailCode != "" {
		a.EmailService.Delete(ctx, entity.EmailRegisterChannel, in.Email)
	}

	authorize, err := a.authorize(user.Id)
	if err != nil {
		return nil, err
	}

	return &web.AuthRegisterResponse{
		Type:        authorize.Type,
		AccessToken: authorize.AccessToken,
		ExpiresIn:   authorize.ExpiresIn,
	}, nil
}

// Forget 找回密码
//
//	@Summary		找回密码
//	@Description	使用邮箱验证码重置用户密码
//	@Tags			认证
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.AuthForgetRequest	true	"找回密码请求"
//	@Success		200		{object}	web.AuthForgetResponse
//	@Router			/api/v1/auth/forget [post]
func (a *Auth) Forget(ctx context.Context, in *web.AuthForgetRequest) (*web.AuthForgetResponse, error) {
	if !utils.IsEmail(in.Email) {
		return nil, errorx.New(400, "邮箱格式不正确")
	}

	// 验证邮箱验证码是否正确
	if !a.EmailService.Verify(ctx, entity.EmailForgetAccountChannel, in.Email, in.EmailCode) {
		return nil, errorx.New(400, "邮箱验证码错误")
	}

	password, err := a.Rsa.Decrypt(in.Password)
	if err != nil {
		return nil, err
	}

	if _, err := a.UserService.Forget(ctx, &service.UserForgetOpt{
		Email:     in.Email,
		Password:  string(password),
		EmailCode: in.EmailCode,
	}); err != nil {
		return nil, err
	}

	a.EmailService.Delete(ctx, entity.EmailForgetAccountChannel, in.Email)

	return &web.AuthForgetResponse{}, nil
}

// Oauth 获取 oauth2.0 跳转地址
//
//	@Summary		OAuth 授权链接
//	@Description	获取 OAuth2.0 授权跳转地址
//	@Tags			认证
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.AuthOauthRequest	true	"OAuth 请求"
//	@Success		200		{object}	web.AuthOauthResponse
//	@Router			/api/v1/auth/oauth [post]
func (a *Auth) Oauth(ctx context.Context, in *web.AuthOauthRequest) (*web.AuthOauthResponse, error) {
	uri, err := a.OauthService.GetAuthURL(ctx, model.OAuthType(in.OauthType))
	if err != nil {
		return nil, err
	}

	return &web.AuthOauthResponse{Uri: uri}, nil
}

// OauthBind 绑定第三方登录接口
//
//	@Summary		OAuth 绑定
//	@Description	将第三方账户绑定到用户账户
//	@Tags			认证
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.AuthOAuthBindRequest	true	"OAuth 绑定请求"
//	@Success		200		{object}	web.AuthOAuthBindResponse
//	@Router			/api/v1/auth/oauth/bind [post]
func (a *Auth) OauthBind(ctx context.Context, in *web.AuthOAuthBindRequest) (*web.AuthOAuthBindResponse, error) {
	decrypt, err := a.AesUtil.Decrypt(in.BindToken)
	if err != nil {
		return nil, err
	}

	var data = BindTokenInfo{}
	if err := jsonutil.Unmarshal(decrypt, &data); err != nil {
		return nil, err
	}

	info, err := a.OAuthUsersRepo.FindById(ctx, data.Id)
	if err != nil {
		return nil, err
	}

	if info.UserId != 0 {
		authorize, err := a.authorize(int(info.UserId))
		if err != nil {
			return nil, err
		}

		return &web.AuthOAuthBindResponse{
			Authorize: authorize,
		}, nil
	}

	if !a.SmsService.Verify(ctx, entity.SmsOauthBindChannel, in.Mobile, in.SmsCode) {
		return nil, entity.ErrSmsCodeError
	}

	userId, err := a.UserService.OauthBind(ctx, in.Mobile, info)
	if err != nil {
		return nil, err
	}

	a.SmsService.Delete(ctx, entity.SmsOauthBindChannel, in.Mobile)

	authorize, err := a.authorize(userId)
	if err != nil {
		return nil, err
	}

	return &web.AuthOAuthBindResponse{
		Authorize: authorize,
	}, nil
}

// OauthLogin 第三方登录接口
//
//	@Summary		OAuth 登录
//	@Description	使用第三方账户登录
//	@Tags			认证
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.AuthOauthLoginRequest	true	"OAuth 登录请求"
//	@Success		200		{object}	web.AuthOauthLoginResponse
//	@Router			/api/v1/auth/oauth/login [post]
func (a *Auth) OauthLogin(ctx context.Context, in *web.AuthOauthLoginRequest) (*web.AuthOauthLoginResponse, error) {

	oAuthInfo, err := a.OauthService.HandleCallback(ctx, model.OAuthType(in.OauthType), in.Code, in.State)
	if err != nil {
		return nil, err
	}

	// 有会员信息直接返回登录信息
	if oAuthInfo.UserId > 0 {
		authorize, err := a.authorize(int(oAuthInfo.UserId))
		if err != nil {
			return nil, err
		}

		return &web.AuthOauthLoginResponse{
			IsAuthorize: "Y",
			Authorize:   authorize,
		}, nil
	}

	ciphertext, err := a.AesUtil.Encrypt(jsonutil.Encode(BindTokenInfo{
		Id:        oAuthInfo.Id,
		Type:      string(oAuthInfo.OAuthType),
		Timestamp: time.Now().Unix(),
	}))

	if err != nil {
		return nil, err
	}

	return &web.AuthOauthLoginResponse{
		IsAuthorize: "N",
		BindToken:   ciphertext,
	}, nil
}

// 生成 JWT Token
func (a *Auth) authorize(uid int) (*web.Authorize, error) {
	token, err := jwtutil.NewTokenWithClaims(
		[]byte(a.Config.Jwt.Secret), entity.WebClaims{
			UserId: int32(uid),
		},
		func(c *jwt.RegisteredClaims) {
			c.Issuer = entity.JwtIssuerWeb
		},
		jwtutil.WithTokenExpiresAt(time.Duration(a.Config.Jwt.ExpiresTime)*time.Second),
	)

	if err != nil {
		return nil, err
	}

	return &web.Authorize{
		AccessToken: token,
		ExpiresIn:   int32(a.Config.Jwt.ExpiresTime),
		Type:        "Bearer",
	}, nil
}

type BindTokenInfo struct {
	Id        int32  `json:"id"`
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
}

// EmailLogin 邮箱验证码登录
//
//	@Summary		邮箱登录
//	@Description	使用邮箱和验证码进行身份验证
//	@Tags			认证
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.AuthEmailLoginRequest	true	"邮箱登录请求"
//	@Success		200		{object}	web.AuthEmailLoginResponse
//	@Router			/api/v1/auth/email-login [post]
func (a *Auth) EmailLogin(ctx context.Context, in *web.AuthEmailLoginRequest) (*web.AuthEmailLoginResponse, error) {
	if in.Email == "" {
		return nil, errorx.New(400, "邮箱不能为空")
	}

	// 验证邮箱验证码是否正确
	if !a.EmailService.Verify(ctx, entity.EmailLoginChannel, in.Email, in.EmailCode) {
		return nil, errorx.New(400, "邮箱验证码错误或已过期")
	}

	// 根据邮箱查找用户
	user, err := a.UsersRepo.FindByEmail(ctx, in.Email)
	if err != nil {
		return nil, errorx.New(400, "该邮箱尚未注册")
	}

	// 记录登录事件
	ip := ""
	userAgent := ""

	data := jsonutil.Marshal(queue.UserLoginRequest{
		UserId:   int32(user.Id),
		IpAddr:   ip,
		Platform: in.Platform,
		Agent:    userAgent,
		LoginAt:  time.Now().Format(time.DateTime),
	})

	if err := a.Redis.Publish(ctx, entity.LoginTopic, data).Err(); err != nil {
		logger.ErrorWithFields(
			"投递登录消息异常", err,
			queue.UserLoginRequest{
				UserId:   int32(user.Id),
				IpAddr:   ip,
				Platform: in.Platform,
				Agent:    userAgent,
				LoginAt:  time.Now().Format(time.DateTime),
			},
		)
	}

	// 删除验证码
	a.EmailService.Delete(ctx, entity.EmailLoginChannel, in.Email)

	// 生成授权token
	authorize, err := a.authorize(user.Id)
	if err != nil {
		return nil, err
	}

	return &web.AuthEmailLoginResponse{
		Type:        authorize.Type,
		AccessToken: authorize.AccessToken,
		ExpiresIn:   authorize.ExpiresIn,
	}, nil
}

// RefreshToken 刷新 Token
//
//	@Summary		刷新 Token
//	@Description	刷新用户访问令牌
//	@Tags			认证
//	@Accept			json
//	@Produce		json
//	@Success		200		{object}	web.AuthRefreshTokenResponse
//	@Router			/api/v1/auth/refresh-token [post]
//	@Security		Bearer
func (a *Auth) RefreshToken(ctx context.Context, in *web.AuthRefreshTokenRequest) (*web.AuthRefreshTokenResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)
	if uid == 0 {
		return nil, errorx.New(401, "未授权")
	}

	// 刷新为 72 小时
	expiresIn := int32(72 * 3600)
	token, err := jwtutil.NewTokenWithClaims(
		[]byte(a.Config.Jwt.Secret), entity.WebClaims{
			UserId: int32(uid),
		},
		func(c *jwt.RegisteredClaims) {
			c.Issuer = entity.JwtIssuerWeb
		},
		jwtutil.WithTokenExpiresAt(time.Duration(expiresIn)*time.Second),
	)

	if err != nil {
		return nil, err
	}

	return &web.AuthRefreshTokenResponse{
		AccessToken: token,
		ExpiresIn:   expiresIn,
		Type:        "Bearer",
	}, nil
}

// Logout 退出登录：将当前 token 加入黑名单使其失效
//
//	@Summary		退出登录
//	@Description	用户退出登录，使当前 token 失效
//	@Tags			认证
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	web.AuthLogoutResponse
//	@Router			/api/v1/auth/logout [post]
//	@Security		Bearer
func (a *Auth) Logout(ctx context.Context, _ *web.AuthLogoutRequest) (*web.AuthLogoutResponse, error) {
	token := middleware.GetAuthTokenFromContext(ctx)
	if token == "" {
		return &web.AuthLogoutResponse{Success: true, Message: "ok"}, nil
	}
	if a.JwtTokenStorage != nil {
		claims, err := jwtutil.ParseWithClaims[entity.WebClaims]([]byte(a.Config.Jwt.Secret), token)
		if err == nil && claims.ExpiresAt != nil {
			exp := time.Until(claims.ExpiresAt.Time)
			if exp > 0 {
				if e := a.JwtTokenStorage.SetBlackList(ctx, token, exp); e != nil {
					return &web.AuthLogoutResponse{Success: false, Message: e.Error()}, nil
				}
			} else {
				if e := a.JwtTokenStorage.SetBlackList(ctx, token, 24*time.Hour); e != nil {
					return &web.AuthLogoutResponse{Success: false, Message: e.Error()}, nil
				}
			}
		} else {
			if e := a.JwtTokenStorage.SetBlackList(ctx, token, 24*time.Hour); e != nil {
				return &web.AuthLogoutResponse{Success: false, Message: e.Error()}, nil
			}
		}
	}
	return &web.AuthLogoutResponse{Success: true, Message: "ok"}, nil
}
