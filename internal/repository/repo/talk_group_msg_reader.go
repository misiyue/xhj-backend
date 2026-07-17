package repo

import (
	"context"

	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TalkGroupMsgReader struct {
	db *gorm.DB
}

func NewTalkGroupMsgReader(db *gorm.DB) *TalkGroupMsgReader {
	return &TalkGroupMsgReader{db: db}
}

// FilterUnreadMsgIDs 从候选 msg_id 中筛出该用户尚未标记已读的项
func (r *TalkGroupMsgReader) FilterUnreadMsgIDs(ctx context.Context, userId int, msgIds []string) ([]string, error) {
	if userId <= 0 || len(msgIds) == 0 {
		return nil, nil
	}
	var read []string
	if err := r.db.WithContext(ctx).Model(&model.TalkGroupMsgReader{}).
		Where("user_id = ? AND msg_id IN ?", userId, msgIds).
		Pluck("msg_id", &read).Error; err != nil {
		return nil, err
	}
	if len(read) == 0 {
		return append([]string(nil), msgIds...), nil
	}
	readSet := make(map[string]struct{}, len(read))
	for _, id := range read {
		readSet[id] = struct{}{}
	}
	unread := make([]string, 0, len(msgIds))
	for _, id := range msgIds {
		if _, ok := readSet[id]; !ok {
			unread = append(unread, id)
		}
	}
	return unread, nil
}

// BatchInsert 批量插入已读记录（唯一键冲突忽略）
func (r *TalkGroupMsgReader) BatchInsert(ctx context.Context, userId int, msgIds []string) (int64, error) {
	if userId <= 0 || len(msgIds) == 0 {
		return 0, nil
	}
	rows := make([]model.TalkGroupMsgReader, 0, len(msgIds))
	for _, msgId := range msgIds {
		if msgId == "" {
			continue
		}
		rows = append(rows, model.TalkGroupMsgReader{
			MsgId:  msgId,
			UserId: userId,
		})
	}
	if len(rows) == 0 {
		return 0, nil
	}
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&rows)
	return result.RowsAffected, result.Error
}

type groupMsgReaderRow struct {
	MsgId  string
	UserId int
}

// MapReaderUserIDsByMsgIDs 批量查询各 msg_id 的已读 user_id 列表
func (r *TalkGroupMsgReader) MapReaderUserIDsByMsgIDs(ctx context.Context, msgIds []string) (map[string][]int, error) {
	out := make(map[string][]int)
	if len(msgIds) == 0 {
		return out, nil
	}
	var rows []groupMsgReaderRow
	if err := r.db.WithContext(ctx).Model(&model.TalkGroupMsgReader{}).
		Select("msg_id", "user_id").
		Where("msg_id IN ?", msgIds).
		Order("msg_id ASC, user_id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.MsgId] = append(out[row.MsgId], row.UserId)
	}
	return out, nil
}
