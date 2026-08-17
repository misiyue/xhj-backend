package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const noticeTemplateCacheTTL = 10 * time.Minute

type NoticeTemplate struct {
	db  *gorm.DB
	rds *redis.Client
}

func NewNoticeTemplate(db *gorm.DB, rds *redis.Client) *NoticeTemplate {
	return &NoticeTemplate{db: db, rds: rds}
}

func noticeTemplateCacheKey(flag string) string {
	return fmt.Sprintf("notice_template:flag:%s", strings.TrimSpace(flag))
}

// FindByFlagCached 按 flag 查模板，Redis 缓存 10 分钟
func (r *NoticeTemplate) FindByFlagCached(ctx context.Context, flag string) (*model.NoticeTemplate, error) {
	flag = strings.TrimSpace(flag)
	if flag == "" {
		return nil, nil
	}
	if r.rds != nil {
		if cached, err := r.rds.Get(ctx, noticeTemplateCacheKey(flag)).Bytes(); err == nil {
			var row model.NoticeTemplate
			if json.Unmarshal(cached, &row) == nil {
				return &row, nil
			}
		} else if err != redis.Nil {
			return nil, err
		}
	}
	row, err := r.findByFlag(ctx, flag)
	if err != nil {
		return nil, err
	}
	if row != nil && r.rds != nil {
		if b, err := json.Marshal(row); err == nil {
			_ = r.rds.Set(ctx, noticeTemplateCacheKey(flag), b, noticeTemplateCacheTTL).Err()
		}
	}
	return row, nil
}

func (r *NoticeTemplate) findByFlag(ctx context.Context, flag string) (*model.NoticeTemplate, error) {
	var row model.NoticeTemplate
	err := r.db.WithContext(ctx).Where("flag = ?", flag).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}
