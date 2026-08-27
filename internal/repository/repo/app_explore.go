package repo

import (
	"context"
	"strings"

	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type AppExplore struct {
	db *gorm.DB
}

func NewAppExplore(db *gorm.DB) *AppExplore {
	return &AppExplore{db: db}
}

// List 返回探索位列表；positions 为逗号分隔的 position 筛选，end 非空时筛选 ends 字段（如 h5,pc,app）
func (r *AppExplore) List(ctx context.Context, positions, end string) ([]*model.AppExplore, error) {
	q := r.db.WithContext(ctx)

	if parts := splitPositions(positions); len(parts) > 0 {
		q = q.Where("position IN ?", parts)
	}
	end = strings.TrimSpace(end)
	if end != "" {
		q = q.Where("FIND_IN_SET(?, ends)", end)
	}

	var list []*model.AppExplore
	err := q.Order("`sort` DESC, id DESC").Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

func splitPositions(positions string) []string {
	if positions == "" {
		return nil
	}
	raw := strings.Split(positions, ",")
	out := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}
