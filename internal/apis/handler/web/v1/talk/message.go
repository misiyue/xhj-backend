package talk

import (
	"context"
	"net/http"
	"slices"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/pkg/filesystem"
	"github.com/gzydong/go-chat/internal/pkg/jsonutil"
	"github.com/gzydong/go-chat/internal/pkg/strutil"
	"github.com/gzydong/go-chat/internal/pkg/timeutil"
	"github.com/gzydong/go-chat/internal/repository/cache"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"github.com/gzydong/go-chat/internal/service"
	"github.com/samber/lo"
)

var _ web.IMessageHandler = (*Message)(nil)

type Message struct {
	TalkService            service.ITalkService
	AuthService            service.IAuthService
	RedEnvelopeService     service.IRedEnvelopeService
	Filesystem             filesystem.IFilesystem
	GroupMemberRepo        *repo.GroupMember
	TalkRecordFriendRepo   *repo.TalkUserMessage
	TalkRecordGroupRepo    *repo.TalkGroupMessage
	TalkGroupMsgReaderRepo *repo.TalkGroupMsgReader
	TalkRecordsService     service.ITalkRecordService
	GroupMemberService     service.IGroupMemberService
	MentionStorage         *cache.MentionStorage
}

// Records 获取会话消息记录
//
//	@Summary		获取消息记录
//	@Description	获取会话的近期消息历史
//	@Tags			消息
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.MessageRecordsRequest	true	"消息记录请求"
//	@Success		200		{object}	web.MessageRecordsResponse
//	@Router			/api/v1/message/records [post]
//	@Security		Bearer
func (m *Message) Records(ctx context.Context, in *web.MessageRecordsRequest) (*web.MessageRecordsResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	if in.TalkMode == entity.ChatGroupMode {
		err := m.AuthService.IsAuth(ctx, &service.AuthOption{
			TalkType:   int(in.TalkMode),
			UserId:     uid,
			ReceiverId: int(in.ReceiverId),
		})

		if err != nil {
			return &web.MessageRecordsResponse{
				Items: []*web.MessageRecord{
					{
						MsgId:     strutil.NewMsgId(),
						Sequence:  1,
						MsgType:   entity.ChatMsgSysText,
						FromId:    0,
						IsRevoked: model.No,
						SendTime:  timeutil.DateTime(),
						Extra: jsonutil.Encode(model.TalkRecordExtraText{
							Content: err.Error(),
						}),
						Quote: "{}",
					},
				},
				Cursor: 1,
			}, nil
		}
	}

	records, err := m.TalkRecordsService.FindAllTalkRecords(ctx, &service.FindAllTalkRecordsOpt{
		TalkType:   int(in.TalkMode),
		UserId:     uid,
		ReceiverId: int(in.ReceiverId),
		Cursor:     int64(in.Cursor),
		Limit:      int(in.Limit),
	})

	if err != nil {
		return nil, err
	}

	cursor := int64(0)
	if length := len(records); length > 0 {
		cursor = records[length-1].Id
	}

	var readerMap map[string][]int
	if in.TalkMode == entity.ChatGroupMode {
		readerMap = m.buildGroupReaderMap(ctx, uid, int(in.ReceiverId), records)
	}

	// 补充红包消息的状态信息
	items := m.enrichRedEnvelopeStatus(ctx, uid, records, readerMap)

	if in.TalkMode == entity.ChatPrivateMode {
		m.markPrivateMessagesRead(ctx, uid, int(in.ReceiverId), records)
		m.applyPrivateReadStatus(items, int(in.ReceiverId))
	}

	return &web.MessageRecordsResponse{
		Items:  items,
		Cursor: int32(cursor),
	}, nil
}

// HistoryRecords 获取会话历史消息记录
//
//	@Summary		获取历史消息记录
//	@Description	搜索和筛选会话的历史消息记录
//	@Tags			消息
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.MessageHistoryRecordsRequest	true	"历史消息请求"
//	@Success		200		{object}	web.MessageHistoryRecordsResponse
//	@Router			/api/v1/message/history-records [post]
//	@Security		Bearer
func (m *Message) HistoryRecords(ctx context.Context, in *web.MessageHistoryRecordsRequest) (*web.MessageHistoryRecordsResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	if in.TalkMode == entity.ChatGroupMode {
		err := m.AuthService.IsAuth(ctx, &service.AuthOption{
			TalkType:   int(in.TalkMode),
			UserId:     uid,
			ReceiverId: int(in.ReceiverId),
		})

		if err != nil {
			return &web.MessageHistoryRecordsResponse{}, nil
		}
	}

	msgTypes := []int{
		entity.ChatMsgTypeText,
		entity.ChatMsgTypeMixed,
		entity.ChatMsgTypeCode,
		entity.ChatMsgTypeImage,
		entity.ChatMsgTypeVideo,
		entity.ChatMsgTypeAudio,
		entity.ChatMsgTypeFile,
		entity.ChatMsgTypeLocation,
		entity.ChatMsgTypeForward,
		entity.ChatMsgTypeVote,
		entity.ChatMsgTypeRedEnvelope,
		entity.ChatMsgTypeTransfer,
	}

	if slices.Contains(msgTypes, int(in.MsgType)) {
		msgTypes = []int{int(in.MsgType)}
	}

	records, err := m.TalkRecordsService.FindAllTalkRecords(ctx, &service.FindAllTalkRecordsOpt{
		TalkType:   int(in.TalkMode),
		MsgType:    msgTypes,
		UserId:     uid,
		ReceiverId: int(in.ReceiverId),
		Keyword:    in.Keyword, // 新增：按内容关键字模糊搜索
		Cursor:     int64(in.Cursor),
		Limit:      int(in.Limit),
	})

	if err != nil {
		return nil, err
	}

	cursor := int64(0)
	if length := len(records); length > 0 {
		cursor = records[length-1].Id
	}

	var readerMap map[string][]int
	if in.TalkMode == entity.ChatGroupMode {
		readerMap = m.buildGroupReaderMap(ctx, uid, int(in.ReceiverId), records)
	}

	// 补充红包消息的状态信息
	items := m.enrichRedEnvelopeStatus(ctx, uid, records, readerMap)

	if in.TalkMode == entity.ChatPrivateMode {
		m.markPrivateMessagesRead(ctx, uid, int(in.ReceiverId), records)
		m.applyPrivateReadStatus(items, int(in.ReceiverId))
	}

	return &web.MessageHistoryRecordsResponse{
		Items:  items,
		Cursor: int32(cursor),
	}, nil
}

// ForwardRecords 转发消息记录
//
//	@Summary		转发消息记录
//	@Description	获取待转发的消息列表
//	@Tags			消息
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.MessageForwardRecordsRequest	true	"转发消息请求"
//	@Success		200		{object}	web.MessageRecordsClearResponse
//	@Router			/api/v1/message/forward-records [post]
//	@Security		Bearer
func (m *Message) ForwardRecords(ctx context.Context, in *web.MessageForwardRecordsRequest) (*web.MessageRecordsClearResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	records, err := m.TalkRecordsService.FindForwardRecords(ctx, uid, in.MsgIds, int(in.TalkMode))
	if err != nil {
		return nil, err
	}

	// 补充红包消息的状态信息
	items := m.enrichRedEnvelopeStatus(ctx, uid, records, nil)

	return &web.MessageRecordsClearResponse{
		Items: items,
	}, nil
}

// enrichRedEnvelopeStatus 补充红包消息的状态信息；readerMap 为群聊已读用户（msg_id -> user_ids）
func (m *Message) enrichRedEnvelopeStatus(ctx context.Context, userId int, records []*model.TalkMessageRecord, readerMap map[string][]int) []*web.MessageRecord {
	return lo.Map(records, func(item *model.TalkMessageRecord, _ int) *web.MessageRecord {
		extra := item.Extra
		if item.IsRevoked == model.Yes {
			extra = "{}"
		}

		// 如果是红包消息且未撤回，补充状态信息
		if item.MsgType == entity.ChatMsgTypeRedEnvelope && item.IsRevoked == model.No {
			var redEnvelopeData model.TalkRecordExtraRedEnvelope
			if err := jsonutil.Unmarshal(item.Extra, &redEnvelopeData); err == nil && redEnvelopeData.EnvelopeId != "" {
				// 获取红包状态
				if status, err := m.RedEnvelopeService.GetStatus(ctx, redEnvelopeData.EnvelopeId, userId); err == nil {
					// 合并状态信息到原有的红包数据
					enrichedData := map[string]interface{}{
						"envelope_id":    redEnvelopeData.EnvelopeId,
						"amount":         redEnvelopeData.Amount,
						"count":          redEnvelopeData.Count,
						"type":           redEnvelopeData.Type,
						"greeting":       redEnvelopeData.Greeting,
						"status":         status.Status,
						"status_text":    status.StatusText,
						"has_received":   status.HasReceived,
						"received_amt":   status.ReceivedAmt,
						"is_best":        status.IsBest,
						"best_user_id":   status.BestUserId,
						"best_user_name": status.BestUserName,
						"best_amount":    status.BestAmount,
					}
					extra = jsonutil.Encode(enrichedData)
				}
			}
		}

		var readerUserIDs []int32
		if readerMap != nil {
			if ids, ok := readerMap[item.MsgId]; ok {
				readerUserIDs = make([]int32, 0, len(ids))
				for _, id := range ids {
					readerUserIDs = append(readerUserIDs, int32(id))
				}
			}
		}
		if readerUserIDs == nil {
			readerUserIDs = []int32{}
		}

		return &web.MessageRecord{
			FromId:         int32(item.FromId),
			MsgId:          item.MsgId,
			Sequence:       int32(item.Id),
			MsgType:        int32(item.MsgType),
			Nickname:       item.Nickname,
			Avatar:         item.Avatar,
			IsRevoked:      int32(item.IsRevoked),
			IsRead:         int32(item.IsRead),
			SendTime:       item.SendTime.Format(time.DateTime),
			Extra:          extra,
			Quote:          item.Quote,
			ReaderUserIds:  readerUserIDs,
		}
	})
}

// Revoke 撤回消息接口（HTTP 入口）
//
// 路由注册：api.go → web2.RegisterMessageHandler → message.bff.go 内 r.POST("/api/v1/message/revoke", ...)
// 实际处理：本方法 → TalkService.Revoke（落库 is_revoked + Redis 推送 sub.im.message.revoke）→ Comet 写 WebSocket im.message.revoke
//
//	@Summary		撤回消息
//	@Description	撤回之前发送的消息
//	@Tags			消息
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.MessageRevokeRequest	true	"撤回请求"
//	@Success		200		{object}	web.MessageRevokeResponse
//	@Router			/api/v1/message/revoke [post]
//	@Security		Bearer
func (m *Message) Revoke(ctx context.Context, in *web.MessageRevokeRequest) (*web.MessageRevokeResponse, error) {
	// 当前登录用户 ID（JWT）；只有「发送者本人」可在 TalkService 里通过 from_id 校验后撤回
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	// 调用领域服务：校验消息归属、是否在 3 分钟内、是否已撤回，然后更新 DB 并 defer 里发 Redis
	if err := m.TalkService.Revoke(ctx, &service.TalkRevokeOption{
		UserId:   uid,              // 必须是该条消息的 from_id
		TalkMode: int(in.TalkMode), // 1 单聊 2 群聊，决定查 talk_user_message 还是 talk_group_message
		MsgId:    in.MsgId,         // 要撤回的那条消息的 msg_id
	}); err != nil {
		return nil, err // 如：消息不存在、已撤回、超时、非发送者等
	}

	// proto 里 MessageRevokeResponse 为空，故 HTTP 成功时 data 多为 {}；多端同步靠 WebSocket
	return &web.MessageRevokeResponse{}, nil
}

// Delete 删除消息记录
//
//	@Summary		删除消息
//	@Description	从历史记录中永久移除消息
//	@Tags			消息
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.MessageDeleteRequest	true	"删除请求"
//	@Success		200		{object}	web.MessageDeleteResponse
//	@Router			/api/v1/message/delete [post]
//	@Security		Bearer
func (m *Message) Delete(ctx context.Context, in *web.MessageDeleteRequest) (*web.MessageDeleteResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	if err := m.TalkService.DeleteRecord(ctx, &service.TalkDeleteRecordOption{
		UserId:     uid,
		TalkMode:   int(in.TalkMode),
		ReceiverId: int(in.ReceiverId),
		MsgIds:     in.MsgIds,
	}); err != nil {
		return nil, err
	}

	return &web.MessageDeleteResponse{}, nil
}

type DownloadChatFileRequest struct {
	TalkMode int    `form:"talk_mode" json:"talk_mode" binding:"required,oneof=1 2"`
	MsgId    string `form:"msg_id" json:"msg_id" binding:"required"`
}

// Download 聊天文件下载
//
//	@Summary		下载聊天文件
//	@Description	下载聊天会话中分享的文件
//	@Tags			消息
//	@Accept			json
//	@Produce		octet-stream
//	@Param			talk_mode	query		int		true	"对话模式 (1:私聊, 2:群聊)"
//	@Param			msg_id		query		string	true	"消息 ID"
//	@Success		200			{file}		binary
//	@Router			/api/v1/talk/file-download [get]
//	@Security		Bearer
func (m *Message) Download(ctx *gin.Context) error {
	params := &DownloadChatFileRequest{}
	if err := ctx.ShouldBind(params); err != nil {
		return errorx.New(400, err.Error())
	}

	uid := middleware.FormContextAuthId[entity.WebClaims](ctx.Request.Context())

	var fileInfo model.TalkRecordExtraFile
	if params.TalkMode == entity.ChatGroupMode {
		record, err := m.TalkRecordGroupRepo.FindByWhere(ctx, "msg_id = ?", params.MsgId)
		if err != nil {
			return ctx.Error(err)
		}

		if !m.GroupMemberRepo.IsMember(ctx, record.GroupId, uid, false) {
			return entity.ErrPermissionDenied
		}

		if err := jsonutil.Unmarshal(record.Extra, &fileInfo); err != nil {
			return err
		}
	} else {
		// 私聊消息：只需根据 msg_id 查询即可
		record, err := m.TalkRecordFriendRepo.FindByWhere(ctx, "msg_id = ?", params.MsgId)
		if err != nil {
			return errorx.New(400, "文件不存在")
		}

		// 验证用户是否有权限访问（是发送者或接收者）
		if record.FromId != uid && record.ReceiverId != uid {
			return entity.ErrPermissionDenied
		}

		if err := jsonutil.Unmarshal(record.Extra, &fileInfo); err != nil {
			return err
		}
	}

	switch m.Filesystem.Driver() {
	case filesystem.LocalDriver:
		filePath := m.Filesystem.(*filesystem.LocalFilesystem).Path(m.Filesystem.BucketPrivateName(), fileInfo.Path)
		ctx.FileAttachment(filePath, fileInfo.Name)
	case filesystem.MinioDriver:
		ctx.Redirect(http.StatusFound, m.Filesystem.PrivateUrl(m.Filesystem.BucketPrivateName(), fileInfo.Path, fileInfo.Name, 60*time.Second))
	default:
		return errorx.New(400, "未知文件驱动类型")
	}

	return nil
}

// MentionListRequest 获取@提及消息列表请求
type MentionListRequest struct {
	GroupId int `json:"group_id" binding:"required,gt=0"` // 群组ID
}

// MentionListResponse 获取@提及消息列表响应
type MentionListResponse struct {
	MsgIds []string `json:"msg_ids"` // 未读提及消息ID列表
	Count  int      `json:"count"`   // 未读提及消息数量
}

// GetMentions 获取未读@提及消息列表
//
//	@Summary		获取未读@提及消息
//	@Description	获取用户在某群组中未读的@提及消息列表
//	@Tags			消息
//	@Accept			json
//	@Produce		json
//	@Param			request	body		MentionListRequest	true	"请求参数"
//	@Success		200		{object}	MentionListResponse
//	@Router			/api/v1/message/mentions [post]
//	@Security		Bearer
func (m *Message) GetMentions(ctx context.Context, in *MentionListRequest) (*MentionListResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	// 验证用户是否是群成员
	if !m.GroupMemberRepo.IsMember(ctx, in.GroupId, uid, true) {
		return nil, entity.ErrPermissionDenied
	}

	msgIds, err := m.MentionStorage.GetMentions(ctx, uid, in.GroupId)
	if err != nil {
		return nil, err
	}

	return &MentionListResponse{
		MsgIds: msgIds,
		Count:  len(msgIds),
	}, nil
}

// ClearMentionRequest 清除@提及消息请求
type ClearMentionRequest struct {
	GroupId int      `json:"group_id" binding:"required,gt=0"` // 群组ID
	MsgIds  []string `json:"msg_ids"`                          // 要清除的消息ID列表，为空则清除所有
}

// ClearMentionResponse 清除@提及消息响应
type ClearMentionResponse struct {
}

// ClearMentions 清除@提及消息
//
//	@Summary		清除@提及消息
//	@Description	标记@提及消息为已读
//	@Tags			消息
//	@Accept			json
//	@Produce		json
//	@Param			request	body		ClearMentionRequest	true	"请求参数"
//	@Success		200		{object}	ClearMentionResponse
//	@Router			/api/v1/message/mentions/clear [post]
//	@Security		Bearer
func (m *Message) ClearMentions(ctx context.Context, in *ClearMentionRequest) (*ClearMentionResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	// 验证用户是否是群成员
	if !m.GroupMemberRepo.IsMember(ctx, in.GroupId, uid, true) {
		return nil, entity.ErrPermissionDenied
	}

	var err error
	if len(in.MsgIds) == 0 {
		// 清除所有未读提及
		err = m.MentionStorage.ClearMentions(ctx, uid, in.GroupId)
	} else {
		// 清除指定的提及消息
		err = m.MentionStorage.RemoveMentions(ctx, uid, in.GroupId, in.MsgIds)
	}

	if err != nil {
		return nil, err
	}

	return &ClearMentionResponse{}, nil
}

// AllMentionsResponse 获取所有群未读@提及统计响应
type AllMentionsResponse struct {
	Groups map[int]int64 `json:"groups"` // map[groupId]count
}

// GetAllMentions 获取所有群的未读@提及统计
//
//	@Summary		获取所有未读@提及
//	@Description	获取用户在所有群组中未读的@提及消息统计
//	@Tags			消息
//	@Accept			json
//	@Produce		json
//	@Success		200		{object}	AllMentionsResponse
//	@Router			/api/v1/message/mentions/all [post]
//	@Security		Bearer
func (m *Message) GetAllMentions(ctx context.Context) (*AllMentionsResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	groups, err := m.MentionStorage.GetAllGroupMentions(ctx, uid)
	if err != nil {
		return nil, err
	}

	return &AllMentionsResponse{
		Groups: groups,
	}, nil
}

func (m *Message) buildGroupReaderMap(ctx context.Context, readerId, groupId int, records []*model.TalkMessageRecord) map[string][]int {
	if readerId <= 0 || groupId <= 0 || len(records) == 0 {
		return nil
	}
	if m.TalkService != nil {
		_ = m.TalkService.MarkGroupMessagesRead(ctx, readerId, groupId, records)
	}
	if m.TalkGroupMsgReaderRepo == nil {
		return nil
	}
	msgIds := make([]string, 0, len(records))
	for _, rec := range records {
		if rec != nil && rec.MsgId != "" {
			msgIds = append(msgIds, rec.MsgId)
		}
	}
	if len(msgIds) == 0 {
		return nil
	}
	readerMap, _ := m.TalkGroupMsgReaderRepo.MapReaderUserIDsByMsgIDs(ctx, msgIds)
	return readerMap
}

func (m *Message) markPrivateMessagesRead(ctx context.Context, readerId, peerId int, records []*model.TalkMessageRecord) {
	if m.TalkService == nil || readerId <= 0 || peerId <= 0 || len(records) == 0 {
		return
	}

	msgIds := make([]string, 0, len(records))
	for _, record := range records {
		if record == nil || record.FromId != peerId || record.IsRead == model.TalkUserMessageIsReadYes {
			continue
		}
		msgIds = append(msgIds, record.MsgId)
	}
	if len(msgIds) == 0 {
		return
	}

	_ = m.TalkService.MarkPrivateMessagesRead(ctx, readerId, peerId, msgIds)
}

func (m *Message) applyPrivateReadStatus(items []*web.MessageRecord, peerId int) {
	for _, item := range items {
		if item != nil && item.GetFromId() == int32(peerId) {
			item.IsRead = int32(model.TalkUserMessageIsReadYes)
		}
	}
}
