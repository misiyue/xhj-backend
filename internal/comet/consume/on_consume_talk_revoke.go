package consume

// 本文件：消费 Redis 订阅里的「消息撤回」事件（SubEventImMessageRevoke），
// 把撤回结果通过 WebSocket 推给在线用户，使各端把对应气泡更新为「已撤回」。

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/logger"
)

// onConsumeTalkRevoke 处理「聊天消息撤回」订阅事件。
//
// 上游：HTTP /api/v1/message/revoke 成功后，TalkService.Revoke 会向 Redis 发布
// SubscribeMessage{ Event: sub.im.message.revoke, Payload: SubEventTalkRevokePayload }。
// Comet 订阅到后调用本函数，body 即为 Payload 的 JSON。
//
// 本函数职责：
//   - 私聊：按 org_msg_id 找出双方信箱里对应的多条 talk_records，分别给双方用户已连接会话写 WebSocket
//   - 群聊：查出该条群消息记录，给群内每个成员已连接会话写同一条撤回帧
//
// 推给前端的 WebSocket 帧格式：{"event":"im.message.revoke","payload":{talk_mode,from_id,receiver_id,msg_id,remark}}
func (h *Handler) onConsumeTalkRevoke(ctx context.Context, body []byte) {
	// 1. 反序列化 Redis 里带来的撤回参数（哪条消息、单聊还是群聊、展示用 remark）
	var in entity.SubEventTalkRevokePayload
	if err := json.Unmarshal(body, &in); err != nil {
		logger.Errorf("[ChatSubscribe] onConsumeTalkRevoke Unmarshal err: %s", err.Error())
		return
	}
	logger.Infof("onConsumeTalkRevoke received: talk_mode=%d msg_id=%s", in.TalkMode, in.MsgId)

	// ---------- 单聊：兼容新旧两种存储 ----------
	// 旧模型：双方各有一条记录（按 user_id 分发）
	// 新模型：只存一条记录（user_id=0），需要按 from_id/receiver_id 给双方推送
	switch in.TalkMode {
	case entity.ChatPrivateMode:
		// 2. 用「业务层统一 msg_id」找到其中一条信箱记录（例如发送方视角）
		record, err := h.TalkRecordsService.FindPrivateRecordByMsgId(ctx, in.MsgId)
		if err != nil {
			logger.Errorf("onConsumeTalkRevoke FindPrivateRecordByMsgId err: %s", err.Error())
			return
		}

		// 查不到记录则无法知道 org_msg_id，直接结束（可能数据不一致或已清理）
		if record == nil {
			logger.Warnf("onConsumeTalkRevoke private record not found for msg_id=%s", in.MsgId)
			return
		}

		// 3. 按 OrgMsgId 取相关记录（旧模型通常 2 条，新模型可能 1 条且 user_id=0）
		records, err := h.TalkRecordsService.FindAllPrivateRecordByOriMsgId(ctx, record.OrgMsgId)
		if err != nil {
			logger.Errorf("onConsumeTalkRevoke FindAllPrivateRecordByOriMsgId err: %s", err.Error())
			return
		}

		// 4. 先按新模型兜底：给发送方和接收方都推送
		targets := map[int]string{
			record.FromId:     record.MsgId,
			record.ReceiverId: record.MsgId,
		}
		// 旧模型兼容：如果记录里带 user_id，则按该用户的 msg_id 精确推
		for _, r := range records {
			if r.UserId > 0 {
				targets[r.UserId] = r.MsgId
			}
		}

		for uid, msgId := range targets {
			data := Message(entity.PushEventImMessageRevoke, entity.ImMessageRevokePayload{
				TalkMode:   entity.ChatPrivateMode,
				FromId:     record.FromId,
				ReceiverId: record.ReceiverId,
				MsgId:      msgId,
				Remark:     in.Remark,
			})

			sessions := h.serv.SessionManager().GetSessions(int64(uid))
			if len(sessions) == 0 {
				logger.Infof("onConsumeTalkRevoke private: user_id=%d no ws session (offline or connected to other node)", uid)
			}
			for _, session := range sessions {
				if err := session.Write(data); err != nil {
					slog.Error("session write message error", "error", err)
				}
			}
		}
	case entity.ChatGroupMode:
		// ---------- 群聊：一条群消息对应一条群记录，所有成员需要同一条撤回通知（msg_id 一致）----------
		// 6. 按 msg_id 查群消息在 talk_records 中的展示记录（含 from_id、群 receiver_id 等）
		record, err := h.TalkRecordsService.FindTalkGroupRecord(ctx, in.MsgId)
		if err != nil {
			logger.Errorf("onConsumeTalkRevoke FindTalkGroupRecord err: %s", err.Error())
			return
		}

		// 7. 群聊只组一帧 payload（全员同一条 msg_id）
		data := Message(entity.PushEventImMessageRevoke, entity.ImMessageRevokePayload{
			TalkMode:   record.TalkMode,
			FromId:     record.FromId,
			ReceiverId: record.ReceiverId,
			MsgId:      record.MsgId,
			Remark:     in.Remark,
		})

		// 8. 遍历群成员，给每个在线成员的每个 WebSocket 连接写同一帧
		memberIds := h.GroupMemberRepo.GetMemberIds(ctx, record.ReceiverId)
		var written int
		for _, uid := range memberIds {
			for _, session := range h.serv.SessionManager().GetSessions(int64(uid)) {
				if err := session.Write(data); err != nil {
					slog.Error("session write message error", "error", err)
				} else {
					written++
				}
			}
		}
		logger.Infof("onConsumeTalkRevoke group: receiver_id=%d members=%d ws_written=%d", record.ReceiverId, len(memberIds), written)
	}
}
