package message

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/jsonutil"
	"github.com/gzydong/go-chat/internal/repository/model"
)

// mixedSegmentAllowed 允许作为混合消息子段的 IM 消息类型（不含转发、登录、混合、群公告及系统消息）
var mixedSegmentAllowed = map[int]struct{}{
	entity.ChatMsgTypeText:        {},
	entity.ChatMsgTypeCode:        {},
	entity.ChatMsgTypeImage:       {},
	entity.ChatMsgTypeAudio:       {},
	entity.ChatMsgTypeVideo:       {},
	entity.ChatMsgTypeFile:        {},
	entity.ChatMsgTypeLocation:    {},
	entity.ChatMsgTypeCard:        {},
	entity.ChatMsgTypeVote:        {},
	entity.ChatMsgTypeRTCCall:     {},
	entity.ChatMsgTypeRedEnvelope: {},
	entity.ChatMsgTypeTransfer:    {},
	entity.ChatMsgTypeCmd:         {},
}

func normalizeMixedSegment(msgType int, content, link string) (string, string, error) {
	content = strings.TrimSpace(content)
	link = strings.TrimSpace(link)
	if _, ok := mixedSegmentAllowed[msgType]; !ok {
		return "", "", errorx.New(400, "混合消息不支持的子项 type")
	}
	if content == "" {
		return "", "", errorx.New(400, "混合消息子项 content 不能为空")
	}

	switch msgType {
	case entity.ChatMsgTypeText:
		var extra model.TalkRecordExtraText
		if err := json.Unmarshal([]byte(content), &extra); err != nil || strings.TrimSpace(extra.Content) == "" {
			extra = model.TalkRecordExtraText{Content: content}
		}
		return jsonutil.Encode(extra), link, nil
	case entity.ChatMsgTypeImage:
		var extra model.TalkRecordExtraImage
		if err := json.Unmarshal([]byte(content), &extra); err != nil {
			if strings.HasPrefix(content, "http://") || strings.HasPrefix(content, "https://") {
				extra = model.TalkRecordExtraImage{Url: content}
			} else {
				return "", "", errorx.New(400, "图片子项 content 须为图片 URL 或 TalkRecordExtraImage JSON")
			}
		}
		if strings.TrimSpace(extra.Url) == "" {
			return "", "", errorx.New(400, "图片子项缺少 url")
		}
		return jsonutil.Encode(extra), link, nil
	case entity.ChatMsgTypeCode:
		return encodeMixedJSON[model.TalkRecordExtraCode](content, "代码")
	case entity.ChatMsgTypeAudio:
		return encodeMixedJSON[model.TalkRecordExtraAudio](content, "语音")
	case entity.ChatMsgTypeVideo:
		return encodeMixedJSON[model.TalkRecordExtraVideo](content, "视频")
	case entity.ChatMsgTypeFile:
		return encodeMixedJSON[model.TalkRecordExtraFile](content, "文件")
	case entity.ChatMsgTypeLocation:
		return encodeMixedJSON[model.TalkRecordExtraLocation](content, "位置")
	case entity.ChatMsgTypeCard:
		return encodeMixedJSON[model.TalkRecordExtraUserShare](content, "名片")
	case entity.ChatMsgTypeVote:
		return encodeMixedJSON[model.TalkRecordExtraVote](content, "投票")
	case entity.ChatMsgTypeRTCCall:
		return encodeMixedJSON[model.TalkRecordExtraRTC](content, "通话")
	case entity.ChatMsgTypeRedEnvelope:
		return encodeMixedJSON[model.TalkRecordExtraRedEnvelope](content, "红包")
	case entity.ChatMsgTypeTransfer:
		return encodeMixedJSON[model.TalkRecordExtraTransfer](content, "转账")
	case entity.ChatMsgTypeCmd:
		return encodeMixedJSON[model.TalkRecordExtraCmd](content, "指令")
	default:
		return "", "", errorx.New(400, "混合消息不支持的子项 type")
	}
}

func encodeMixedJSON[T any](content, label string) (string, string, error) {
	var extra T
	if err := json.Unmarshal([]byte(content), &extra); err != nil {
		return "", "", errorx.New(400, label+"子项 content 须为合法 JSON")
	}
	return jsonutil.Encode(extra), "", nil
}

func normalizeMixedMessageList(list []CreateMixedMessageItem) ([]CreateMixedMessageItem, error) {
	if len(list) == 0 {
		return nil, errorx.New(400, "混合消息 items 不能为空")
	}
	out := make([]CreateMixedMessageItem, 0, len(list))
	for i, item := range list {
		norm, link, err := normalizeMixedSegment(item.Type, item.Content, item.Link)
		if err != nil {
			return nil, errorx.New(400, err.Error()+" (items["+strconv.Itoa(i)+"])")
		}
		out = append(out, CreateMixedMessageItem{
			Type:    item.Type,
			Content: norm,
			Link:    link,
		})
	}
	return out, nil
}
