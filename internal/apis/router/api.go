package router

import (
	"context"
	"errors"
	"strings"

	"buf.build/go/protovalidate"
	"github.com/gin-gonic/gin"
	web2 "github.com/gzydong/go-chat/api/pb/web/v1"
	_ "github.com/gzydong/go-chat/docs" // 注册 swag 文档，/swagger/doc.json 依赖此包 init
	"github.com/gzydong/go-chat/internal/apis/handler/web"
	v1 "github.com/gzydong/go-chat/internal/apis/handler/web/v1"
	"github.com/gzydong/go-chat/internal/apis/handler/web/v1/talk"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/logic"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/pkg/jwtutil"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"github.com/gzydong/go-chat/internal/service"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"
)

// RegisterWebRoute 注册 Web 路由
func RegisterWebRoute(secret string, router *gin.Engine, handler *web.Handler, storage middleware.IStorage) {
	// 授权验证中间件
	authorize := middleware.NewJwtMiddleware[entity.WebClaims](
		[]byte(secret), storage,
		func(ctx context.Context, claims *jwtutil.JwtClaims[entity.WebClaims]) error {
			if claims.RegisteredClaims.Issuer != entity.JwtIssuerWeb {
				return errors.New("授权异常，请登录后操作")
			}

			user, err := handler.UserRepo.FindById(ctx, claims.Metadata.UserId)
			if err != nil {
				return errors.New("授权异常，请登录后操作")
			}

			if user.IsCancelled() {
				return entity.ErrAccountCancelled
			}

			if user.IsDisabled() {
				return entity.ErrAccountDisabled
			}

			return nil
		},
		func(option *middleware.JwtMiddlewareOption) {
			option.ExclusionPaths = []string{
				"/api/v1/marzban/user",
				"/api/v1/auth/login",
				"/api/v1/auth/register",
				"/api/v1/auth/forget",
				"/api/v1/auth/email-login",
				"/api/v1/auth/refresh-token",
				"/api/v1/auth/oauth",
				"/api/v1/auth/oauth/bind",
				"/api/v1/auth/oauth/login",
				"/api/v1/auth/captcha",
				"/api/v1/common/send-email",
				"/api/v1/common/send-sms",
				"/api/v1/common/send-test",
				"/api/v1/common/app-dict",
				"/api/v1/common/app-modules",
				"/api/v1/common/explore-list",
				"/api/v1/common/news-list",
				"/api/v1/common/news-detail",
				"/api/v1/notice/article",
				"/api/v1/merchant/order/hdpay-notify",
				"/api/v1/merchant/order/hmpay-notify",
			}
		},
	)

	api := router.Group("/").Use(authorize)

	resp := &Interceptor{}

	// Swagger documentation route
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	web2.RegisterAuthHandler(api, resp, handler.V1.Auth)
	web2.RegisterCommonHandler(api, resp, handler.V1.Common)
	web2.RegisterUserHandler(api, resp, handler.V1.User)
	web2.RegisterEmoticonHandler(api, resp, handler.V1.Emoticon)
	web2.RegisterOrganizeHandler(api, resp, handler.V1.Organize)
	web2.RegisterArticleClassHandler(api, resp, handler.V1.ArticleClass)
	web2.RegisterArticleHandler(api, resp, handler.V1.Article)
	web2.RegisterArticleAnnexHandler(api, resp, handler.V1.ArticleAnnex)
	web2.RegisterContactHandler(api, resp, handler.V1.Contact)
	web2.RegisterContactApplyHandler(api, resp, handler.V1.ContactApply)
	web2.RegisterContactGroupHandler(api, resp, handler.V1.ContactGroup)
	web2.RegisterTalkHandler(api, resp, handler.V1.Talk)
	web2.RegisterGroupHandler(api, resp, handler.V1.Group)
	web2.RegisterGroupApplyHandler(api, resp, handler.V1.GroupApply)
	web2.RegisterGroupVoteHandler(api, resp, handler.V1.GroupVote)
	web2.RegisterGroupNoticeHandler(api, resp, handler.V1.GroupNotice)
	web2.RegisterMessageHandler(api, resp, handler.V1.TalkMessage)
	patchTalkMessageDeps(handler.V1)
	patchCommonDeps(handler.V1, handler.UserRepo)

	// Invite 需要 *repo.Users：子 Handler 若未在 wire_gen 里注入 UsersRepo 会为空指针。
	// 顶层 web.Handler.UserRepo 与之一致，此处补齐引用，避免 /api/v1/invite/friends 空指针 panic。
	if handler.V1 != nil && handler.V1.Invite != nil && handler.UserRepo != nil {
		handler.V1.Invite.UsersRepo = handler.UserRepo
	}

	// Notice 需要 *repo.NoticeLetter：wire_gen 未更新时补齐，避免 /api/v1/notice/* 空指针 500。
	if handler.V1 != nil && handler.V1.User != nil && handler.V1.User.NoticeLetterRepo != nil {
		if handler.V1.Notice == nil {
			handler.V1.Notice = &v1.Notice{NoticeLetterRepo: handler.V1.User.NoticeLetterRepo}
		} else if handler.V1.Notice.NoticeLetterRepo == nil {
			handler.V1.Notice.NoticeLetterRepo = handler.V1.User.NoticeLetterRepo
		}
	}

	web2.RegisterInviteHandler(api, resp, handler.V1.Invite)
	web2.RegisterNoticeHandler(api, resp, handler.V1.Notice)

	registerCustomApiRouter(resp, router, api, handler)
}

// patchTalkMessageDeps 补齐 wire_gen 未更新时 talk.Message / TalkService 的已读相关依赖。
func patchTalkMessageDeps(v1 *web.V1) {
	if v1 == nil {
		return
	}

	var msg *talk.Message
	if v1.TalkMessage != nil {
		msg = v1.TalkMessage
	}
	if msg != nil {
		msg.PushMessage = bootstrapPushMessage(v1, msg)
		if msg.TalkGroupMsgReaderRepo == nil {
			msg.TalkGroupMsgReaderRepo = bootstrapTalkGroupMsgReaderRepo(msg)
		}
	}

	pushMessage := bootstrapPushMessage(v1, msg)
	groupReaderRepo := bootstrapTalkGroupMsgReaderRepo(msg)
	if pushMessage == nil {
		logger.Warnf("PushMessage 未注入，私聊/群聊已读 WebSocket 推送将不可用")
	}
	if groupReaderRepo == nil {
		logger.Warnf("TalkGroupMsgReaderRepo 未注入，群聊已读将不可用")
	}

	patchTalkServiceReadDeps(v1.TalkMessage, pushMessage, groupReaderRepo)
	patchTalkServiceReadDeps(v1.Talk, pushMessage, groupReaderRepo)
}

func patchTalkServiceReadDeps(handler any, pushMessage *logic.PushMessage, groupReaderRepo *repo.TalkGroupMsgReader) {
	var ts *service.TalkService
	switch h := handler.(type) {
	case *talk.Message:
		if h == nil {
			return
		}
		if h.PushMessage == nil {
			h.PushMessage = pushMessage
		}
		if h.TalkGroupMsgReaderRepo == nil {
			h.TalkGroupMsgReaderRepo = groupReaderRepo
		}
		ts, _ = h.TalkService.(*service.TalkService)
	case *talk.Session:
		if h == nil {
			return
		}
		if h.PushMessage == nil {
			h.PushMessage = pushMessage
		}
		ts, _ = h.TalkService.(*service.TalkService)
	default:
		return
	}
	if ts == nil {
		return
	}
	if ts.PushMessage == nil {
		ts.PushMessage = pushMessage
	}
	if ts.TalkGroupMsgReaderRepo == nil {
		ts.TalkGroupMsgReaderRepo = groupReaderRepo
	}
}

func bootstrapPushMessage(v1 *web.V1, msg *talk.Message) *logic.PushMessage {
	if msg != nil && msg.PushMessage != nil {
		return msg.PushMessage
	}
	if v1 == nil {
		return nil
	}
	if v1.User != nil && v1.User.PushMessage != nil {
		return v1.User.PushMessage
	}
	if v1.Talk != nil && v1.Talk.PushMessage != nil {
		return v1.Talk.PushMessage
	}
	if v1.GroupApply != nil && v1.GroupApply.PushMessage != nil {
		return v1.GroupApply.PushMessage
	}
	return nil
}

func bootstrapTalkGroupMsgReaderRepo(msg *talk.Message) *repo.TalkGroupMsgReader {
	if msg == nil {
		return nil
	}
	var db *gorm.DB
	switch {
	case msg.TalkRecordGroupRepo != nil && msg.TalkRecordGroupRepo.Db != nil:
		db = msg.TalkRecordGroupRepo.Db
	case msg.TalkRecordFriendRepo != nil && msg.TalkRecordFriendRepo.Db != nil:
		db = msg.TalkRecordFriendRepo.Db
	case msg.GroupMemberRepo != nil && msg.GroupMemberRepo.Db != nil:
		db = msg.GroupMemberRepo.Db
	case msg.TalkRecordsService != nil:
		if trs, ok := msg.TalkRecordsService.(*service.TalkRecordService); ok && trs != nil && trs.Source != nil {
			db = trs.Source.Db()
		}
	}
	if db == nil {
		return nil
	}
	return repo.NewTalkGroupMsgReader(db)
}

// patchCommonDeps 补齐 wire_gen 未更新时 Common 的 repo 依赖。
func patchCommonDeps(v1 *web.V1, userRepo *repo.Users) {
	if v1 == nil || v1.Common == nil {
		return
	}
	db := bootstrapGormDB(userRepo, v1.Common)
	if db == nil {
		logger.Warnf("AppModuleRepo 等 Common 依赖无法 bootstrap：未找到 *gorm.DB")
		return
	}
	if v1.Common.AppModuleRepo == nil {
		v1.Common.AppModuleRepo = repo.NewAppModule(db)
	}
}

func bootstrapGormDB(userRepo *repo.Users, c *v1.Common) *gorm.DB {
	if userRepo != nil && userRepo.Db != nil {
		return userRepo.Db
	}
	if c != nil && c.UsersRepo != nil && c.UsersRepo.Db != nil {
		return c.UsersRepo.Db
	}
	return nil
}

func registerCustomApiRouter(resp *Interceptor, router *gin.Engine, api gin.IRoutes, handler *web.Handler) {
	api.POST("/api/v1/marzban/user", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		return handler.V1.Marzban.CreateUser(c)
	}))

	api.GET("/api/v1/marzban/user/:id", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		return handler.V1.Marzban.GetUser(c)
	}))
	// 第三方支付回调：无 JWT，响应纯文本 success / fail
	router.POST("/api/v1/merchant/order/hdpay-notify", func(c *gin.Context) {
		handler.V1.User.MerchantOrderHdpayNotify(c)
	})
	router.GET("/api/v1/merchant/order/hdpay-notify", func(c *gin.Context) {
		handler.V1.User.MerchantOrderHdpayNotify(c)
	})
	router.POST("/api/v1/merchant/order/hmpay-notify", func(c *gin.Context) {
		handler.V1.User.MerchantOrderHmpayNotify(c)
	})
	router.GET("/api/v1/merchant/order/hmpay-notify", func(c *gin.Context) {
		handler.V1.User.MerchantOrderHmpayNotify(c)
	})

	router.GET("/api/v1/common/app-version", resp.Do(func(c *gin.Context) (any, error) {
		in := &web2.CommonAppVersionLatestRequest{
			Platform: strings.ToLower(strings.TrimSpace(c.Query("platform"))),
		}
		if err := protovalidate.Validate(in); err != nil {
			return nil, errorx.New(400, err.Error())
		}
		return handler.V1.Common.AppVersionLatest(c.Request.Context(), in)
	}))

	api.POST("/api/v1/emoticon/customize/upload", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		return handler.V1.Emoticon.Upload(c, &web2.EmoticonUploadRequest{})
	}))

	api.POST("/api/v1/article-annex/upload", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		return handler.V1.ArticleAnnex.Upload(c, &web2.ArticleAnnexUploadRequest{})
	}))

	api.GET("/api/v1/article-annex/download", func(c *gin.Context) {
		_, err := handler.V1.ArticleAnnex.Download(c, nil)
		if err != nil {
			resp.Error(c, err)
		}
	})

	api.POST("/api/v1/upload/media-file", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		return handler.V1.Upload.Image(c)
	}))

	api.POST("/api/v1/upload/multipart", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		return handler.V1.Upload.MultipartUpload(c)
	}))

	api.POST("/api/v1/upload/init-multipart", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		return handler.V1.Upload.InitiateMultipart(c)
	}))

	api.POST("/api/v1/upload/sts", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		return handler.V1.STS.GetCosSTSCredentials(c)
	}))

	api.POST("/api/v1/upload/presigned-url", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		return handler.V1.STS.GetUploadPresignedUrl(c)
	}))

	api.GET("/api/v1/talk/file-download", func(c *gin.Context) {
		if err := handler.V1.TalkMessage.Download(c); err != nil {
			resp.Error(c, err)
		}
	})

	api.POST("/api/v1/message/send", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		return handler.V1.Message.Send(c)
	}))

	api.GET("/api/v1/trtc/user-sig", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		return handler.V1.Trtc.GetSignature(c)
	}))

	// KYC routes
	api.POST("/api/v1/kyc/status", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		return handler.V1.KYC.GetKYCStatus(c.Request.Context(), &v1.KYCStatusRequest{})
	}))

	api.POST("/api/v1/kyc/submit", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		var req v1.KYCSubmitRequestData
		if err := c.ShouldBindJSON(&req); err != nil {
			return nil, err
		}
		return handler.V1.KYC.SubmitKYC(c.Request.Context(), &req)
	}))

	api.POST("/api/v1/kyc/detail", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		return handler.V1.KYC.GetKYCDetail(c.Request.Context(), &v1.KYCDetailRequest{})
	}))

	api.POST("/api/v1/kyc/upload-idcard", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		var req v1.KYCUploadIDCardRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			return nil, err
		}
		return handler.V1.KYC.UploadIDCard(c.Request.Context(), &req)
	}))

	api.POST("/api/v1/kyc/upload-face", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		var req v1.KYCUploadFaceRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			return nil, err
		}
		return handler.V1.KYC.UploadFaceImage(c.Request.Context(), &req)
	}))

	// Wallet routes
	api.POST("/api/v1/wallet/balance", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		return handler.V1.Wallet.GetBalance(c.Request.Context(), &v1.WalletBalanceRequest{})
	}))

	api.POST("/api/v1/wallet/verify-password", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		var req v1.WalletVerifyPasswordRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			return nil, err
		}
		return handler.V1.Wallet.VerifyPaymentPassword(c.Request.Context(), &req)
	}))

	api.POST("/api/v1/wallet/recharge", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		var req v1.WalletRechargeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			return nil, err
		}
		return handler.V1.Wallet.Recharge(c.Request.Context(), &req)
	}))

	api.POST("/api/v1/wallet/transfer", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		var req v1.WalletTransferRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			return nil, err
		}
		return handler.V1.Wallet.Transfer(c.Request.Context(), &req)
	}))

	api.POST("/api/v1/wallet/history", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		var req v1.WalletHistoryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			return nil, err
		}
		return handler.V1.Wallet.GetTransactionHistory(c.Request.Context(), &req)
	}))

	api.POST("/api/v1/wallet/red-envelope/send", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		var req v1.WalletSendRedEnvelopeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			return nil, err
		}
		return handler.V1.Wallet.SendRedEnvelope(c.Request.Context(), &req)
	}))

	api.POST("/api/v1/wallet/red-envelope/receive", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		var req v1.WalletReceiveRedEnvelopeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			return nil, err
		}
		return handler.V1.Wallet.ReceiveRedEnvelope(c.Request.Context(), &req)
	}))

	api.POST("/api/v1/wallet/red-envelope/detail", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		var req v1.WalletRedEnvelopeDetailRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			return nil, err
		}
		return handler.V1.Wallet.GetRedEnvelopeDetail(c.Request.Context(), &req)
	}))

	// GroupRobot routes
	api.POST("/api/v1/group/robot/create", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		var req v1.GroupRobotCreateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			return nil, err
		}
		return handler.V1.GroupRobot.CreateRobot(c.Request.Context(), &req)
	}))

	api.POST("/api/v1/group/robot/list", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		var req v1.GroupRobotListRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			return nil, err
		}
		return handler.V1.GroupRobot.GetRobotList(c.Request.Context(), &req)
	}))

	api.POST("/api/v1/group/robot/delete", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		var req v1.GroupRobotDeleteRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			return nil, err
		}
		return handler.V1.GroupRobot.DeleteRobot(c.Request.Context(), &req)
	}))

	api.POST("/api/v1/group/robot/update", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		var req v1.GroupRobotUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			return nil, err
		}
		return handler.V1.GroupRobot.UpdateRobot(c.Request.Context(), &req)
	}))

	api.POST("/api/v1/group/robot/messages", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		var req v1.GroupRobotMessagesRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			return nil, err
		}
		return handler.V1.GroupRobot.GetRobotMessages(c.Request.Context(), &req)
	}))

	// Webhook route (note: this doesn't require auth, so might need special handling)
	api.POST("/api/v1/webhook/robot/:webhook_url", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		webhookUrl := c.Param("webhook_url")
		var req v1.WebhookSendRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			return nil, err
		}
		// Get headers
		req.Timestamp = c.GetHeader("timestamp")
		req.Signature = c.GetHeader("signature")
		return handler.V1.GroupRobot.SendWebhookMessage(c.Request.Context(), webhookUrl, &req)
	}))

	// Mention routes
	api.POST("/api/v1/message/mentions", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		var req talk.MentionListRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			return nil, err
		}
		return handler.V1.TalkMessage.GetMentions(c.Request.Context(), &req)
	}))

	api.POST("/api/v1/message/mentions/clear", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		var req talk.ClearMentionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			return nil, err
		}
		return handler.V1.TalkMessage.ClearMentions(c.Request.Context(), &req)
	}))

	api.POST("/api/v1/message/mentions/all", HandlerFunc(resp, func(c *gin.Context) (any, error) {
		return handler.V1.TalkMessage.GetAllMentions(c.Request.Context())
	}))
}
