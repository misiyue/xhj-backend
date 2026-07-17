package message

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gzydong/go-chat/internal/logic"

	"github.com/google/uuid"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/filesystem"
	"github.com/gzydong/go-chat/internal/pkg/jsonutil"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/pkg/strutil"
	"github.com/gzydong/go-chat/internal/repository/cache"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"gorm.io/gorm"
)

var _ IService = (*Service)(nil)

// IPrivateMessage 私有消息
type IPrivateMessage interface {
	// CreatePrivateSysMessage 给指定用户创建私有的系统消息
	CreatePrivateSysMessage(ctx context.Context, option CreatePrivateSysMessageOption) error
	// CreatePrivateMessage 创建私有的消息
	CreatePrivateMessage(ctx context.Context, option CreatePrivateMessageOption) error
	// CreateToUserPrivateMessage 给指定用户信箱添加消息
	CreateToUserPrivateMessage(ctx context.Context, data *model.TalkUserMessage) error
	// CreatePrivateRetainDaysSetMessage 设置 retain_days 后通知双方
	CreatePrivateRetainDaysSetMessage(ctx context.Context, fromId, receiverId, retainDays int) error
}

// IGroupMessage 群消息
type IGroupMessage interface {
	// CreateGroupMessage 创建群消息
	CreateGroupMessage(ctx context.Context, option CreateGroupMessageOption) error
	// CreateGroupSysMessage 创建群系统消息
	CreateGroupSysMessage(ctx context.Context, option CreateGroupSysMessageOption) error
}

type IMessage interface {
	// CreateMessage 创建消息
	CreateMessage(ctx context.Context, option CreateMessageOption) error
	// CreateLoginMessage 创建登录消息
	CreateLoginMessage(ctx context.Context, option CreateLoginMessageOption) error
	// CreateTextMessage 文本消息
	CreateTextMessage(ctx context.Context, option CreateTextMessage) error
	// CreateImageMessage 图片文件消息
	CreateImageMessage(ctx context.Context, option CreateImageMessage) error
	// CreateVoiceMessage 语音文件消息
	CreateVoiceMessage(ctx context.Context, option CreateVoiceMessage) error
	// CreateVideoMessage 视频文件消息
	CreateVideoMessage(ctx context.Context, option CreateVideoMessage) error
	// CreateFileMessage 文件消息
	CreateFileMessage(ctx context.Context, option CreateFileMessage) error
	// CreateCodeMessage 代码消息
	CreateCodeMessage(ctx context.Context, option CreateCodeMessage) error
	// CreateVoteMessage 投票消息
	CreateVoteMessage(ctx context.Context, option CreateVoteMessage) error
	// CreateEmoticonMessage 表情消息
	CreateEmoticonMessage(ctx context.Context, option CreateEmoticonMessage) error
	// CreateForwardMessage 转发消息
	CreateForwardMessage(ctx context.Context, option CreateForwardMessage) error
	// CreateLocationMessage 位置消息
	CreateLocationMessage(ctx context.Context, option CreateLocationMessage) error
	// CreateBusinessCardMessage 推送用户名片消息
	CreateBusinessCardMessage(ctx context.Context, option CreateBusinessCardMessage) error
	// CreateMixedMessage 图文消息
	CreateMixedMessage(ctx context.Context, option CreateMixedMessage) error
	// CreateRTCCallMessage 音视频通话消息
	CreateRTCCallMessage(ctx context.Context, option CreateRTCCallMessage) error
	// SendRTCCallInvite 发起音视频通话邀请（仅 OneSignal VoIP 推送，不落库、不推 WebSocket）
	SendRTCCallInvite(ctx context.Context, option SendRTCCallInvite) error
	// CreateRedEnvelopeMessage 红包消息
	CreateRedEnvelopeMessage(ctx context.Context, option CreateRedEnvelopeMessage) error
	// CreateTransferMessage 转账消息
	CreateTransferMessage(ctx context.Context, option CreateTransferMessage) error
}

type IService interface {
	IPrivateMessage
	IGroupMessage
	IMessage
}

type Service struct {
	*repo.Source
	GroupMemberRepo     *repo.GroupMember
	SplitUploadRepo     *repo.FileUpload
	TalkRecordsVoteRepo *repo.GroupVote
	TalkSessionRepo     *repo.TalkSession
	UsersRepo           *repo.Users
	Filesystem          filesystem.IFilesystem
	UnreadStorage       *cache.UnreadStorage
	MessageStorage      *cache.MessageStorage
	ServerStorage       *cache.ServerStorage
	MentionStorage      *cache.MentionStorage
	RobotRepo           *repo.Robot
	PushMessage         *logic.PushMessage
	UserClient          *cache.UserClient
	NoticeTemplateRepo  *repo.NoticeTemplate
}

// CreateMessage 是所有消息入库的统一入口：
//   - 根据 TalkMode 决定走「私聊」还是「群聊」分支
//   - 将通用的 CreateMessageOption 拆分为 CreatePrivateMessageOption / CreateGroupMessageOption
//   - 内部再由 CreatePrivateMessage / CreateGroupMessage 负责：
//   - 写入对应表（单聊：talk_user_message，群聊：talk_group_message）
//   - 通过 Redis 推送到 ImTopicChat，让长连接服务分发到在线端
//   - 维护未读计数、会话最后一条消息缓存等
func (s *Service) CreateMessage(ctx context.Context, option CreateMessageOption) error {
	if option.TalkMode == 1 {
		return s.CreatePrivateMessage(ctx, CreatePrivateMessageOption{
			MsgId:      option.MsgId,
			MsgType:    option.MsgType,
			FromId:     option.FromId,
			ReceiverId: option.ReceiverId,
			QuoteId:    option.QuoteId,
			Extra:      option.Extra,
		})
	}

	return s.CreateGroupMessage(ctx, CreateGroupMessageOption{
		MsgId:      option.MsgId,
		MsgType:    option.MsgType,
		FromId:     option.FromId,
		ReceiverId: option.ReceiverId,
		QuoteId:    option.QuoteId,
		Extra:      option.Extra,
	})
}

// CreateTextMessage 文本消息创建入口：
//  1. 根据入参组装 CreateMessageOption（指定 MsgType = 文本）
//  2. 将文本内容和 @ 列表封装到 TalkRecordExtraText 中序列化到 Extra 字段
//  3. 统一交给 CreateMessage，后续逻辑与其它消息类型一致
func (s *Service) CreateTextMessage(ctx context.Context, option CreateTextMessage) error {
	return s.CreateMessage(ctx, CreateMessageOption{
		MsgId:      option.MsgId,
		TalkMode:   option.TalkMode,
		FromId:     option.FromId,
		ReceiverId: option.ReceiverId,
		MsgType:    entity.ChatMsgTypeText,
		QuoteId:    option.QuoteId,
		Extra: jsonutil.Encode(model.TalkRecordExtraText{
			Content:  option.Content,
			Mentions: option.Mentions,
		}),
	})
}

func (s *Service) CreateImageMessage(ctx context.Context, option CreateImageMessage) error {
	return s.CreateMessage(ctx, CreateMessageOption{
		MsgId:      option.MsgId,
		TalkMode:   option.TalkMode,
		FromId:     option.FromId,
		ReceiverId: option.ReceiverId,
		MsgType:    entity.ChatMsgTypeImage,
		QuoteId:    option.QuoteId,
		Extra: jsonutil.Encode(model.TalkRecordExtraImage{
			Size:   option.Size,
			Url:    option.Url,
			Width:  option.Width,
			Height: option.Height,
		}),
	})
}

func (s *Service) CreateVoiceMessage(ctx context.Context, option CreateVoiceMessage) error {
	return s.CreateMessage(ctx, CreateMessageOption{
		MsgId:      option.MsgId,
		TalkMode:   option.TalkMode,
		FromId:     option.FromId,
		ReceiverId: option.ReceiverId,
		MsgType:    entity.ChatMsgTypeAudio,
		Extra: jsonutil.Encode(model.TalkRecordExtraAudio{
			Name:     "",
			Size:     option.Size,
			Url:      option.Url,
			Duration: option.Duration,
		}),
	})
}

func (s *Service) CreateVideoMessage(ctx context.Context, option CreateVideoMessage) error {
	return s.CreateMessage(ctx, CreateMessageOption{
		MsgId:      option.MsgId,
		TalkMode:   option.TalkMode,
		FromId:     option.FromId,
		ReceiverId: option.ReceiverId,
		MsgType:    entity.ChatMsgTypeVideo,
		Extra: jsonutil.Encode(model.TalkRecordExtraVideo{
			Name:     "",
			Cover:    option.Cover,
			Size:     option.Size,
			Url:      option.Url,
			Duration: option.Duration,
		}),
	})
}

func (s *Service) CreateFileMessage(ctx context.Context, option CreateFileMessage) error {
	if option.Url != "" {
		return s.CreateMessage(ctx, CreateMessageOption{
			MsgId:      option.MsgId,
			TalkMode:   option.TalkMode,
			FromId:     option.FromId,
			ReceiverId: option.ReceiverId,
			MsgType:    entity.ChatMsgTypeFile,
			Extra: jsonutil.Encode(&model.TalkRecordExtraFile{
				Name: option.Name,
				Size: option.Size,
				Url:  option.Url,
			}),
		})
	}

	now := time.Now()

	file, err := s.SplitUploadRepo.GetFile(ctx, option.FromId, option.UploadId)
	if err != nil {
		return err
	}

	publicUrl := ""
	filePath := fmt.Sprintf("talk-files/%s/%s.%s", now.Format("200601"), uuid.New().String(), file.FileExt)

	// 公开文件
	if entity.GetMediaType(file.FileExt) <= 3 {
		filePath = strutil.GenMediaObjectName(option.Name, 0, 0)
		// 如果是多媒体文件，则将私有文件转移到公开文件
		if err := s.Filesystem.CopyObject(
			s.Filesystem.BucketPrivateName(), file.Path,
			s.Filesystem.BucketPublicName(), filePath,
		); err != nil {
			return err
		}

		publicUrl = s.Filesystem.PublicUrl(s.Filesystem.BucketPublicName(), filePath)
	} else {
		if err := s.Filesystem.Copy(s.Filesystem.BucketPrivateName(), file.Path, filePath); err != nil {
			return err
		}
	}

	message := CreateMessageOption{
		MsgId:      option.MsgId,
		TalkMode:   option.TalkMode,
		FromId:     option.FromId,
		ReceiverId: option.ReceiverId,
	}

	switch entity.GetMediaType(file.FileExt) {
	case entity.MediaFileAudio:
		message.MsgType = entity.ChatMsgTypeAudio
		message.Extra = jsonutil.Encode(&model.TalkRecordExtraAudio{
			Size:     int(file.FileSize),
			Url:      publicUrl,
			Duration: 0,
		})
	case entity.MediaFileVideo:
		message.MsgType = entity.ChatMsgTypeVideo
		message.Extra = jsonutil.Encode(&model.TalkRecordExtraVideo{
			Cover:    "",
			Size:     int(file.FileSize),
			Url:      publicUrl,
			Duration: 0,
		})
	case entity.MediaFileOther:
		message.MsgType = entity.ChatMsgTypeFile
		message.Extra = jsonutil.Encode(&model.TalkRecordExtraFile{
			Name: file.OriginalName,
			Size: int(file.FileSize),
			Path: filePath,
		})
	}

	return s.CreateMessage(ctx, message)
}

func (s *Service) CreateCodeMessage(ctx context.Context, option CreateCodeMessage) error {
	return s.CreateMessage(ctx, CreateMessageOption{
		MsgId:      option.MsgId,
		TalkMode:   option.TalkMode,
		FromId:     option.FromId,
		ReceiverId: option.ReceiverId,
		MsgType:    entity.ChatMsgTypeCode,
		Extra: jsonutil.Encode(model.TalkRecordExtraCode{
			Lang: option.Lang,
			Code: option.Code,
		}),
	})
}

func (s *Service) CreateVoteMessage(ctx context.Context, option CreateVoteMessage) error {
	return s.CreateMessage(ctx, CreateMessageOption{
		TalkMode:   option.TalkMode,
		FromId:     option.FromId,
		ReceiverId: option.ReceiverId,
		MsgType:    entity.ChatMsgTypeVote,
		Extra: jsonutil.Encode(model.TalkRecordExtraVote{
			VoteId: option.VoteId,
		}),
	})
}

func (s *Service) CreateEmoticonMessage(ctx context.Context, option CreateEmoticonMessage) error {
	var emoticon model.EmoticonItem
	if err := s.Source.Db().First(&emoticon, "id = ? and user_id = ?", option.EmoticonId, option.FromId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("表情信息不存在")
		}

		return err
	}

	return s.CreateMessage(ctx, CreateMessageOption{
		TalkMode:   option.TalkMode,
		FromId:     option.FromId,
		ReceiverId: option.ReceiverId,
		MsgType:    entity.ChatMsgTypeImage,
		Extra: jsonutil.Encode(model.TalkRecordExtraImage{
			Url: emoticon.Url,
		}),
	})
}

// CreateForwardMessage todo 待完善
func (s *Service) CreateForwardMessage(ctx context.Context, option CreateForwardMessage) error {
	items := make([]ForwardMessageOpt, 0)

	// 发送方式 1:逐条发送 2:合并发送
	if option.Action == 1 {
		for _, userId := range option.Uids {
			item := ForwardMessageOpt{
				MsgIds:       option.MsgIds,
				TalkMode:     option.TalkMode,
				ReceiverId:   option.ReceiverId,
				UserId:       option.FromId,
				ToUserId:     userId,
				ToUserIdType: 1,
			}

			items = append(items, item)

			err := s.toSplitForward(ctx, item)
			if err != nil {
				logger.WithFields(
					logger.LevelError,
					fmt.Sprintf("[uid] split forward message failed err:%s", err.Error()),
					item,
				)
			}
		}

		for _, groupId := range option.Gids {
			item := ForwardMessageOpt{
				MsgIds:       option.MsgIds,
				TalkMode:     option.TalkMode,
				ReceiverId:   option.ReceiverId,
				UserId:       option.FromId,
				ToUserId:     groupId,
				ToUserIdType: 2,
			}

			items = append(items, item)

			err := s.toSplitForward(ctx, item)
			if err != nil {
				logger.WithFields(
					logger.LevelError,
					fmt.Sprintf("[group] split forward message failed err:%s", err.Error()),
					item,
				)
			}
		}
	} else {
		for _, userId := range option.Uids {
			item := ForwardMessageOpt{
				MsgIds:     option.MsgIds,
				TalkMode:   option.TalkMode,
				ReceiverId: option.ReceiverId,

				UserId:       option.UserId,
				ToUserId:     userId,
				ToUserIdType: 1,
			}

			items = append(items, item)

			err := s.toCombineForward(ctx, item)
			if err != nil {
				logger.WithFields(
					logger.LevelError,
					fmt.Sprintf("[uid] combin forward message failed err:%s", err.Error()),
					item,
				)
			}
		}

		for _, groupId := range option.Gids {
			item := ForwardMessageOpt{
				MsgIds:       option.MsgIds,
				TalkMode:     option.TalkMode,
				ReceiverId:   option.ReceiverId,
				UserId:       option.UserId,
				ToUserId:     groupId,
				ToUserIdType: 2,
			}

			items = append(items, item)

			err := s.toCombineForward(ctx, item)
			if err != nil {
				logger.WithFields(
					logger.LevelError,
					fmt.Sprintf("[group] combin forward message failed err:%s", err.Error()),
					item,
				)
			}
		}
	}

	return nil
}

func (s *Service) CreateLocationMessage(ctx context.Context, option CreateLocationMessage) error {
	return s.CreateMessage(ctx, CreateMessageOption{
		MsgId:      option.MsgId,
		TalkMode:   option.TalkMode,
		FromId:     option.FromId,
		ReceiverId: option.ReceiverId,
		MsgType:    entity.ChatMsgTypeLocation,
		Extra: jsonutil.Encode(model.TalkRecordExtraLocation{
			Longitude:   option.Longitude,
			Latitude:    option.Latitude,
			Description: option.Description,
		}),
	})
}

func (s *Service) CreateBusinessCardMessage(ctx context.Context, option CreateBusinessCardMessage) error {
	userInfo, err := s.UsersRepo.FindById(ctx, option.UserId)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if userInfo == nil {
		return errors.New("用户不存在")
	}

	return s.CreateMessage(ctx, CreateMessageOption{
		MsgId:      option.MsgId,
		TalkMode:   option.TalkMode,
		FromId:     option.FromId,
		ReceiverId: option.ReceiverId,
		MsgType:    entity.ChatMsgTypeCard,
		Extra: jsonutil.Encode(model.TalkRecordExtraUserShare{
			UserId:   option.UserId,
			Nickname: userInfo.Nickname,
			Avatar:   userInfo.Avatar,
			Describe: userInfo.Motto,
		}),
	})
}

func (s *Service) CreateMixedMessage(ctx context.Context, option CreateMixedMessage) error {
	items := make([]*model.TalkRecordExtraMixedItem, 0)
	for _, item := range option.MessageList {
		items = append(items, &model.TalkRecordExtraMixedItem{
			Type:    item.Type,
			Content: item.Content,
		})
	}

	return s.CreateMessage(ctx, CreateMessageOption{
		MsgId:      option.MsgId,
		TalkMode:   option.TalkMode,
		FromId:     option.FromId,
		ReceiverId: option.ReceiverId,
		MsgType:    entity.ChatMsgTypeMixed,
		Extra: jsonutil.Encode(model.TalkRecordExtraMixed{
			Items: items,
		}),
	})
}

func (s *Service) CreateRTCCallMessage(ctx context.Context, option CreateRTCCallMessage) error {
	return s.CreateMessage(ctx, CreateMessageOption{
		MsgId:      option.MsgId,
		TalkMode:   option.TalkMode,
		FromId:     option.FromId,
		ReceiverId: option.ReceiverId,
		MsgType:    entity.ChatMsgTypeRTCCall,
		Extra: jsonutil.Encode(model.TalkRecordExtraRTC{
			Type:     option.Type,
			Status:   option.Status,
			Duration: option.Duration,
		}),
	})
}

func (s *Service) SendRTCCallInvite(ctx context.Context, option SendRTCCallInvite) error {
	if option.TalkMode != entity.ChatPrivateMode {
		return errors.New("rtc_invite 仅支持私聊")
	}
	if option.FromId <= 0 || option.ReceiverId <= 0 {
		return errors.New("无效的发送者或接收者")
	}
	if option.Type != 1 && option.Type != 2 {
		return errors.New("通话类型无效")
	}
	s.tryOneSignalVoIPCall(ctx, option.FromId, option.ReceiverId, option.Type)
	return nil
}

func (s *Service) CreateRedEnvelopeMessage(ctx context.Context, option CreateRedEnvelopeMessage) error {
	return s.CreateMessage(ctx, CreateMessageOption{
		MsgId:      option.MsgId,
		TalkMode:   option.TalkMode,
		FromId:     option.FromId,
		ReceiverId: option.ReceiverId,
		MsgType:    entity.ChatMsgTypeRedEnvelope,
		Extra: jsonutil.Encode(model.TalkRecordExtraRedEnvelope{
			EnvelopeId: option.EnvelopeId,
			Amount:     option.Amount,
			Count:      option.Count,
			Type:       option.Type,
			Greeting:   option.Greeting,
		}),
	})
}

func (s *Service) CreateTransferMessage(ctx context.Context, option CreateTransferMessage) error {
	return s.CreateMessage(ctx, CreateMessageOption{
		MsgId:      option.MsgId,
		TalkMode:   option.TalkMode,
		FromId:     option.FromId,
		ReceiverId: option.ReceiverId,
		MsgType:    entity.ChatMsgTypeTransfer,
		Extra: jsonutil.Encode(model.TalkRecordExtraTransfer{
			TransferId: option.TransferId,
			Amount:     option.Amount,
			Remark:     option.Remark,
		}),
	})
}

func (s *Service) CreateLoginMessage(ctx context.Context, option CreateLoginMessageOption) error {
	robot, err := s.RobotRepo.GetLoginRobot(ctx)
	if err != nil {
		return err
	}

	return s.CreateToUserPrivateMessage(ctx, &model.TalkUserMessage{
		MsgType:    entity.ChatMsgTypeLogin,
		UserId:     option.UserId,
		ReceiverId: robot.UserId,
		FromId:     robot.UserId,
		Extra: jsonutil.Encode(&model.TalkRecordExtraLogin{
			IP:       option.Ip,
			Platform: option.Platform,
			Agent:    option.Agent,
			Address:  option.Address,
			Reason:   option.Reason,
			Datetime: option.LoginAt,
		}),
	})
}

func (s *Service) getTextMessage(msgType int, extra string) string {
	return PreviewText(msgType, extra)
}

